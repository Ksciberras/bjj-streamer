import { type FormEvent, useEffect, useMemo, useRef, useState } from 'react'
import { EmptyState, ErrorState, Visibility } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import { formatTime } from '../../lib/format'
import type { Course, CourseVideo, Note, Video } from '../../types'

type StudyScreenProps = {
  video: Video
  onBack: () => void
  setError: (message: string) => void
  onProgress: (seconds: number) => void
  course: Course | null
  autoPlay: boolean
  onSelectCourseVideo: (video: CourseVideo, autoPlay?: boolean) => void
  initialSeek?: number
  savedForLater: boolean
  onToggleWatchLater: () => void
}

export function StudyScreen({ video, course, autoPlay, initialSeek, savedForLater, onToggleWatchLater, onSelectCourseVideo, onBack, setError, onProgress }: StudyScreenProps) {
  const player = useRef<HTMLVideoElement>(null)
  const noteInput = useRef<HTMLTextAreaElement>(null)
  const lastSaved = useRef(0)
  const [url, setURL] = useState('')
  const [resumeAt, setResumeAt] = useState(0)
  const [notes, setNotes] = useState<Note[]>([])
  const [draftTimestamp, setDraftTimestamp] = useState(0)
  const [loading, setLoading] = useState(true)
  const [autoplayBlocked, setAutoplayBlocked] = useState(false)
  const courseIndex = course?.videos.findIndex((item) => item.id === video.id) ?? -1
  const previousVideo = courseIndex > 0 ? course?.videos[courseIndex - 1] : undefined
  const nextVideo = course && courseIndex >= 0 ? course.videos[courseIndex + 1] : undefined

  useEffect(() => {
    let cancelled = false
    void Promise.all([
      api(`/api/videos/${video.id}/playback`),
      api(`/api/videos/${video.id}/progress`),
      api(`/api/videos/${video.id}/notes`),
    ]).then(([playback, progress, noteBody]) => {
      if (cancelled) return
      const saved = initialSeek ?? (Number(progress.progress.position_seconds) || 0)
      setURL(playback.playback_url)
      setResumeAt(saved)
      lastSaved.current = saved
      setDraftTimestamp(saved)
      setNotes(noteBody.notes)
      setLoading(false)
      setAutoplayBlocked(false)
    }).catch((reason) => {
      if (!cancelled) {
        setError(errorMessage(reason, 'Unable to load video'))
        setLoading(false)
      }
    })
    return () => { cancelled = true }
  }, [video.id, initialSeek, setError])

  useEffect(() => {
    if (!autoPlay || !url || !player.current) return
    void player.current.play().catch(() => setAutoplayBlocked(true))
  }, [autoPlay, url])

  useEffect(() => {
    const save = () => {
      const position = player.current?.currentTime
      if (position !== undefined && Number.isFinite(position)) {
        void api(`/api/videos/${video.id}/progress`, {
          method: 'PUT',
          body: JSON.stringify({ position_seconds: position }),
          keepalive: true,
        })
      }
    }
    window.addEventListener('pagehide', save)
    return () => {
      window.removeEventListener('pagehide', save)
      save()
    }
  }, [video.id])

  useEffect(() => {
    const handleKeyboard = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement
      if (event.key === 'Escape') {
        const editor = document.querySelector<HTMLElement>('.note-item[data-editing="true"]')
        if (editor) {
          event.preventDefault()
          editor.querySelector<HTMLButtonElement>('[data-cancel-edit]')?.click()
        }
        return
      }
      if (target.matches('input, textarea, select, button, [contenteditable="true"]') || !player.current) return
      if (event.code === 'Space') {
        event.preventDefault()
        if (player.current.paused) void player.current.play()
        else player.current.pause()
      }
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') {
        event.preventDefault()
        player.current.currentTime = Math.max(0, player.current.currentTime + (event.key === 'ArrowLeft' ? -10 : 10))
      }
      if (event.key.toLowerCase() === 'n') {
        event.preventDefault()
        captureNoteTime()
      }
    }
    window.addEventListener('keydown', handleKeyboard)
    return () => window.removeEventListener('keydown', handleKeyboard)
  })

  async function saveProgress() {
    const position = player.current?.currentTime ?? 0
    lastSaved.current = position
    onProgress(position)
    await api(`/api/videos/${video.id}/progress`, {
      method: 'PUT',
      body: JSON.stringify({ position_seconds: position }),
    })
  }

  function timeUpdate() {
    const position = player.current?.currentTime ?? 0
    if (Math.abs(position - lastSaved.current) >= 10) void saveProgress().catch(() => undefined)
  }

  async function back() {
    await saveProgress().catch(() => undefined)
    onBack()
  }

  async function advance() {
    await saveProgress().catch(() => undefined)
    if (nextVideo) onSelectCourseVideo(nextVideo, true)
  }

  async function refreshNotes() {
    setNotes((await api(`/api/videos/${video.id}/notes`)).notes)
  }

  function captureNoteTime() {
    setDraftTimestamp(player.current?.currentTime ?? 0)
    requestAnimationFrame(() => noteInput.current?.focus())
  }

  async function addNote(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = event.currentTarget
    const data = new FormData(form)
    try {
      await api(`/api/videos/${video.id}/notes`, {
        method: 'POST',
        body: JSON.stringify({ timestamp_seconds: draftTimestamp, body: data.get('body') }),
      })
      form.reset()
      await refreshNotes()
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to add note'))
    }
  }

  async function updateNote(event: FormEvent<HTMLFormElement>, note: Note) {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    try {
      await api(`/api/videos/${video.id}/notes/${note.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ timestamp_seconds: Number(data.get('timestamp_seconds')), body: data.get('body') }),
      })
      await refreshNotes()
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to update note'))
    }
  }

  async function deleteNote(note: Note) {
    try {
      await api(`/api/videos/${video.id}/notes/${note.id}`, { method: 'DELETE' })
      await refreshNotes()
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to delete note'))
    }
  }

  function seek(seconds: number) {
    if (!player.current) return
    player.current.currentTime = seconds
    void player.current.play()
  }

  const sortedNotes = useMemo(
    () => [...notes].sort((a, b) => a.timestamp_seconds - b.timestamp_seconds),
    [notes],
  )

  return (
    <div className="screen study-screen">
      <button className="back-button" onClick={() => void back()}>← Back to library</button>
      <button className="secondary-button watch-later-player" type="button" onClick={onToggleWatchLater}>
        {savedForLater ? 'Remove from Watch later' : 'Save to Watch later'}
      </button>
      <div className="study-layout">
        <Player
          video={video}
          player={player}
          url={url}
          loading={loading}
          resumeAt={resumeAt}
          onTimeUpdate={timeUpdate}
          onPause={saveProgress}
          onEnded={advance}
          autoplayBlocked={autoplayBlocked}
          onResumeAutoplay={() => {
            setAutoplayBlocked(false)
            void player.current?.play()
          }}
        />
        <NotesPanel
          notes={sortedNotes}
          draftTimestamp={draftTimestamp}
          noteInput={noteInput}
          onCapture={captureNoteTime}
          onAdd={addNote}
          onSeek={seek}
          onUpdate={updateNote}
          onDelete={deleteNote}
        />
      </div>
      {course && courseIndex >= 0 && (
        <nav className="course-navigation" aria-label="Course chapters">
          <div>
            <span>Course · Chapter {courseIndex + 1} of {course.videos.length}</span>
            <strong>{course.title}</strong>
          </div>
          <div>
            <button type="button" className="secondary-button" disabled={!previousVideo} onClick={() => previousVideo && onSelectCourseVideo(previousVideo)}>← Previous</button>
            <button type="button" disabled={!nextVideo} onClick={() => nextVideo && onSelectCourseVideo(nextVideo)}>Next chapter →</button>
          </div>
        </nav>
      )}
    </div>
  )
}

type PlayerProps = {
  video: Video
  player: React.RefObject<HTMLVideoElement | null>
  url: string
  loading: boolean
  resumeAt: number
  onTimeUpdate: () => void
  onPause: () => Promise<void>
  onEnded: () => Promise<void>
  autoplayBlocked: boolean
  onResumeAutoplay: () => void
}

function Player({ video, player, url, loading, resumeAt, onTimeUpdate, onPause, onEnded, autoplayBlocked, onResumeAutoplay }: PlayerProps) {
  return (
    <section className="player-column" aria-labelledby="video-title">
      <div className="player-frame">
        {loading
          ? <div className="player-loading">Loading video…</div>
        : url
          ? <video
              ref={player}
              src={url}
              controls
              playsInline
              preload="metadata"
              onLoadedMetadata={(event) => {
                if (resumeAt > 0) event.currentTarget.currentTime = resumeAt
              }}
              onTimeUpdate={onTimeUpdate}
              onPause={() => void onPause()}
              onEnded={() => void onEnded()}
            />
            : <ErrorState title="Couldn’t load this video" body="Check your connection and try again." />}
        {autoplayBlocked && <button className="autoplay-prompt" type="button" onClick={onResumeAutoplay}>Play next chapter</button>}
      </div>
      <div className="player-details">
        <div>
          <div className="detail-line">
            <Visibility value={video.visibility} />
            <span>{video.instructional_name || 'Instructional video'}</span>
          </div>
          <h1 id="video-title">{video.title}</h1>
          <p>{video.instructor_name}{video.chapter_name ? ` · ${video.chapter_name}` : ''}</p>
          {resumeAt > 0 && (
            <p className="resume-detail">Resumed from <code>{formatTime(resumeAt)}</code></p>
          )}
        </div>
        {video.tags.length > 0 && (
          <div className="tag-list compact">
            {video.tags.map((tag) => <span key={tag}>{tag}</span>)}
          </div>
        )}
      </div>
    </section>
  )
}

type NotesPanelProps = {
  notes: Note[]
  draftTimestamp: number
  noteInput: React.RefObject<HTMLTextAreaElement | null>
  onCapture: () => void
  onAdd: (event: FormEvent<HTMLFormElement>) => void
  onSeek: (seconds: number) => void
  onUpdate: (event: FormEvent<HTMLFormElement>, note: Note) => void
  onDelete: (note: Note) => void
}

function NotesPanel(props: NotesPanelProps) {
  const { notes, draftTimestamp, noteInput, onCapture, onAdd, onSeek, onUpdate, onDelete } = props
  return (
    <aside className="notes-panel" aria-labelledby="notes-title">
      <div className="notes-heading">
        <div><h2 id="notes-title">Notes</h2><p>Private to you</p></div>
        <span aria-label={`${notes.length} notes`}>{notes.length}</span>
      </div>
      <button className="capture-button" onClick={onCapture}>
        Add note at {formatTime(draftTimestamp)} <kbd>N</kbd>
      </button>
      <form className="note-composer" onSubmit={onAdd}>
        <label>
          <span>Note at <code>{formatTime(draftTimestamp)}</code></span>
          <textarea ref={noteInput} name="body" required maxLength={5000} placeholder="Add a detail to revisit…" />
        </label>
        <button type="submit">Save note</button>
      </form>
      <div className="notes-list">
        {notes.length
          ? notes.map((note) => (
              <NoteItem key={note.id} note={note} onSeek={onSeek} onUpdate={onUpdate} onDelete={onDelete} />
            ))
          : <EmptyState title="No notes yet" body="Add a note at the current timestamp." />}
      </div>
    </aside>
  )
}

type NoteItemProps = {
  note: Note
  onSeek: (seconds: number) => void
  onUpdate: (event: FormEvent<HTMLFormElement>, note: Note) => void
  onDelete: (note: Note) => void
}

function NoteItem({ note, onSeek, onUpdate, onDelete }: NoteItemProps) {
  const [editing, setEditing] = useState(false)
  function submit(event: FormEvent<HTMLFormElement>) {
    onUpdate(event, note)
    setEditing(false)
  }
  return (
    <article className="note-item" data-editing={editing}>
      {editing ? (
        <form onSubmit={submit}>
          <label>
            <span className="sr-only">Timestamp</span>
            <input name="timestamp_seconds" type="number" min="0" step="0.1" defaultValue={note.timestamp_seconds} />
          </label>
          <label>
            <span className="sr-only">Note</span>
            <textarea name="body" defaultValue={note.body} required maxLength={5000} autoFocus />
          </label>
          <div className="note-actions">
            <button type="submit">Save</button>
            <button type="button" className="secondary-button" data-cancel-edit onClick={() => setEditing(false)}>Cancel</button>
            <button type="button" className="danger-button" onClick={() => onDelete(note)}>Delete</button>
          </div>
        </form>
      ) : (
        <div className="note-reading">
          <button type="button" className="timestamp" onClick={() => onSeek(note.timestamp_seconds)}>
            {formatTime(note.timestamp_seconds)}
          </button>
          <p>{note.body}</p>
          <button type="button" className="edit-note" onClick={() => setEditing(true)}>Edit</button>
        </div>
      )}
    </article>
  )
}
