import { EmptyState, PageHeader, SectionHeading, TruncatedText } from '../../components/ui'
import { formatTime } from '../../lib/format'
import type { StudyNote, Video } from '../../types'

type StudyHubProps = {
  watchLater: Video[]
  notes: StudyNote[]
  onOpenVideo: (video: Video, timestamp?: number) => void
  onRemoveWatchLater: (video: Video) => void
}

export function StudyHub({ watchLater, notes, onOpenVideo, onRemoveWatchLater }: StudyHubProps) {
  return (
    <div className="screen study-hub">
      <PageHeader title="Study" description="Your saved videos and private notes." />
      <section className="section" aria-labelledby="watch-later-title">
        <SectionHeading id="watch-later-title" title="Watch later" />
        {watchLater.length ? (
          <div className="saved-video-list">
            {watchLater.map((video) => (
              <article key={video.id}>
                {video.thumbnail_url
                  ? <img src={video.thumbnail_url} alt="" />
                  : <span className="saved-video-placeholder" aria-hidden="true" />}
                <div><strong><TruncatedText text={video.title} /></strong><TruncatedText text={video.instructor_name} /></div>
                <button onClick={() => onOpenVideo(video)}>Study video</button>
                <button className="secondary-button" onClick={() => onRemoveWatchLater(video)}>Remove</button>
              </article>
            ))}
          </div>
        ) : <EmptyState title="Nothing saved for later" body="Save a video from the library or player for quick access here." />}
      </section>
      <section className="section" aria-labelledby="all-notes-title">
        <SectionHeading id="all-notes-title" title="My notes" />
        {notes.length ? (
          <div className="study-note-list">
            {notes.map((note) => {
              const video = note.video
              return (
                <button key={note.id} title={`${note.video_title} — ${note.instructor_name}`} onClick={() => onOpenVideo(video, note.timestamp_seconds)}>
                  <code>{formatTime(note.timestamp_seconds)}</code>
                  <span><strong><TruncatedText text={note.video_title} focusable={false} /></strong><small><TruncatedText text={note.instructor_name} focusable={false} /></small><p>{note.body}</p></span>
                  <span aria-hidden="true">→</span>
                </button>
              )
            })}
          </div>
        ) : <EmptyState title="No notes yet" body="Notes you add while watching appear here as timestamped shortcuts." />}
      </section>
    </div>
  )
}
