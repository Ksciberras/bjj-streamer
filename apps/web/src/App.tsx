import { FormEvent, ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import './app.css'

type Role = 'admin' | 'instructor' | 'student'
type View = 'home' | 'library' | 'upload' | 'admin'
type User = { id: string; email: string; role: Role; disabled?: boolean }
type Video = { id: string; uploaded_by_user_id: string; title: string; instructor_name: string; instructional_name?: string; chapter_name?: string; description: string; tags: string[]; visibility: 'shared' | 'private'; content_basis: 'self_created' | 'licensed_for_group' | 'personal_purchase'; original_filename: string; byte_size: number; status: 'pending_upload' | 'ready' | 'archived' }
type Note = { id: string; timestamp_seconds: number; body: string }
type ProgressMap = Record<string, number>

async function api(path: string, options: RequestInit = {}) {
  const csrf = document.cookie.split('; ').find((item) => item.startsWith('bjj_csrf='))?.split('=')[1]
  const response = await fetch(path, {
    ...options,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': decodeURIComponent(csrf) } : {}), ...options.headers },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: 'Request failed' }))
    throw new Error(body.error ?? 'Request failed')
  }
  return response.status === 204 ? null : response.json()
}

export function App() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    api('/api/auth/session').then((body) => setUser(body.user)).catch(() => setUser(null)).finally(() => setLoading(false))
  }, [])

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError('')
    const data = new FormData(event.currentTarget)
    try {
      const body = await api('/api/auth/login', { method: 'POST', body: JSON.stringify({ email: data.get('email'), password: data.get('password') }) })
      setUser(body.user)
    } catch (reason) { setError(message(reason, 'Login failed')) }
  }

  async function logout() {
    try { await api('/api/auth/logout', { method: 'POST', body: '{}' }) } finally { setUser(null) }
  }

  if (loading) return <LoadingScreen />
  if (!user) return <LoginScreen onSubmit={login} error={error} />
  return <Workspace user={user} logout={logout} />
}

function Workspace({ user, logout }: { user: User; logout: () => Promise<void> }) {
  const [view, setView] = useState<View>('home')
  const [users, setUsers] = useState<User[]>([])
  const [videos, setVideos] = useState<Video[]>([])
  const [progress, setProgress] = useState<ProgressMap>({})
  const [selectedVideo, setSelectedVideo] = useState<Video | null>(null)
  const [loadingVideos, setLoadingVideos] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const canUpload = user.role !== 'student'

  const refreshUsers = async () => {
    if (user.role === 'admin') setUsers((await api('/api/admin/users')).users)
  }
  const refreshVideos = async (query = '') => {
    const next: Video[] = (await api(`/api/videos${query ? `?q=${encodeURIComponent(query)}` : ''}`)).videos
    setVideos(next)
    void loadProgress(next)
  }
  async function loadProgress(items: Video[]) {
    const results = await Promise.allSettled(items.map(async (video) => {
      const body = await api(`/api/videos/${video.id}/progress`)
      return [video.id, Number(body.progress.position_seconds) || 0] as const
    }))
    const next: ProgressMap = {}
    results.forEach((result) => { if (result.status === 'fulfilled') next[result.value[0]] = result.value[1] })
    setProgress(next)
  }

  useEffect(() => {
    let cancelled = false
    void api('/api/videos').then((body) => {
      if (!cancelled) { setVideos(body.videos); void loadProgress(body.videos) }
    }).catch((reason) => { if (!cancelled) setError(message(reason, 'Unable to load videos')) }).finally(() => { if (!cancelled) setLoadingVideos(false) })
    return () => { cancelled = true }
  }, [])
  useEffect(() => {
    if (user.role !== 'admin') return
    let cancelled = false
    void api('/api/admin/users').then((body) => { if (!cancelled) setUsers(body.users) }).catch((reason) => { if (!cancelled) setError(message(reason, 'Unable to load users')) })
    return () => { cancelled = true }
  }, [user.role])

  function navigate(next: View) { setSelectedVideo(null); setView(next); setError(''); window.scrollTo({ top: 0, behavior: 'smooth' }) }
  function openVideo(video: Video) { setSelectedVideo(video); setError(''); window.scrollTo({ top: 0 }) }
  function browse(filter: { instructor?: string; tag?: string }) { setView('library'); setSelectedVideo(null); setLibrarySeed(filter) }
  const [librarySeed, setLibrarySeed] = useState<{ instructor?: string; tag?: string }>({})

  return <AppShell user={user} active={selectedVideo ? 'study' : view} canUpload={canUpload} onNavigate={navigate} onLogout={logout}>
    {error && <StatusMessage tone="error" onDismiss={() => setError('')}>{error}</StatusMessage>}
    {notice && <StatusMessage tone="success" onDismiss={() => setNotice('')}>{notice}</StatusMessage>}
    {selectedVideo ? <StudyScreen video={selectedVideo} onBack={() => setSelectedVideo(null)} setError={setError} onProgress={(seconds) => setProgress((current) => ({ ...current, [selectedVideo.id]: seconds }))} /> : view === 'home' ?
      <HomeScreen videos={videos} progress={progress} loading={loadingVideos} openVideo={openVideo} browse={browse} /> : view === 'library' ?
      <LibraryScreen key={`${librarySeed.instructor ?? ''}:${librarySeed.tag ?? ''}`} videos={videos} progress={progress} loading={loadingVideos} initialFilter={librarySeed} openVideo={openVideo} onSearch={async (query) => { setLoadingVideos(true); try { await refreshVideos(query) } catch (reason) { setError(message(reason, 'Unable to search')) } finally { setLoadingVideos(false) } }} /> : view === 'upload' && canUpload ?
      <UploadScreen user={user} videos={videos} onUploaded={async () => { await refreshVideos(); setNotice('Upload complete. The video is ready to study.') }} onError={setError} onUpdate={async () => refreshVideos()} /> : view === 'admin' && user.role === 'admin' ?
      <AdminScreen users={users} videos={videos} onRefreshUsers={refreshUsers} onRefreshVideos={refreshVideos} setError={setError} setNotice={setNotice} /> :
      <HomeScreen videos={videos} progress={progress} loading={loadingVideos} openVideo={openVideo} browse={browse} />}
  </AppShell>
}

function AppShell({ user, active, canUpload, onNavigate, onLogout, children }: { user: User; active: View | 'study'; canUpload: boolean; onNavigate: (view: View) => void; onLogout: () => Promise<void>; children: ReactNode }) {
  const navigation: { id: View; label: string; icon: string; show: boolean }[] = [
    { id: 'home', label: 'Home', icon: '⌂', show: true },
    { id: 'library', label: 'Library', icon: '▦', show: true },
    { id: 'upload', label: 'Upload', icon: '↑', show: canUpload },
    { id: 'admin', label: 'Admin', icon: '⚙', show: user.role === 'admin' },
  ]
  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><span className="brand-mark">B</span><span><strong>Study Mat</strong><small>Private BJJ library</small></span></div>
      <nav aria-label="Primary navigation">{navigation.filter((item) => item.show).map((item) => <button key={item.id} className={active === item.id ? 'nav-link active' : 'nav-link'} onClick={() => onNavigate(item.id)} aria-current={active === item.id ? 'page' : undefined}><span aria-hidden="true">{item.icon}</span>{item.label}</button>)}</nav>
      <div className="account"><div className="account-avatar" aria-hidden="true">{user.email.charAt(0).toUpperCase()}</div><div className="account-copy"><strong>{user.email}</strong><span>{user.role}</span></div><button className="icon-button" onClick={() => void onLogout()} aria-label="Sign out" title="Sign out">↪</button></div>
    </aside>
    <header className="mobile-header"><div className="brand"><span className="brand-mark">B</span><strong>Study Mat</strong></div><div className="mobile-account"><span>{user.role}</span>{user.role === 'admin' && <button onClick={() => onNavigate('admin')}>Admin</button>}<button onClick={() => void onLogout()}>Sign out</button></div></header>
    <main className="content" id="main-content">{children}</main>
    <nav className="mobile-nav" aria-label="Mobile navigation">{navigation.filter((item) => item.show && item.id !== 'admin').map((item) => <button key={item.id} className={active === item.id ? 'active' : ''} onClick={() => onNavigate(item.id)} aria-current={active === item.id ? 'page' : undefined}><span aria-hidden="true">{item.icon}</span>{item.label}</button>)}</nav>
  </div>
}

function HomeScreen({ videos, progress, loading, openVideo, browse }: { videos: Video[]; progress: ProgressMap; loading: boolean; openVideo: (video: Video) => void; browse: (filter: { instructor?: string; tag?: string }) => void }) {
  const ready = videos.filter((video) => video.status === 'ready')
  const continueVideo = ready.filter((video) => (progress[video.id] ?? 0) > 0).sort((a, b) => (progress[b.id] ?? 0) - (progress[a.id] ?? 0))[0]
  const instructors = [...new Set(ready.map((video) => video.instructor_name))].slice(0, 8)
  const tags = [...new Set(ready.flatMap((video) => video.tags))].slice(0, 12)
  return <div className="screen home-screen"><PageHeader eyebrow="Your study space" title="Ready when you are." description="Resume a session or choose the next technique to study." />
    <section className="section" aria-labelledby="continue-title"><SectionHeading id="continue-title" title="Continue watching" />
      {loading ? <LoadingSkeleton /> : continueVideo ? <ContinueCard video={continueVideo} savedAt={progress[continueVideo.id]} onResume={() => openVideo(continueVideo)} /> : <EmptyState title="No saved progress yet" body="Start a video from your library and it will appear here." action={<button onClick={() => browse({})}>Browse library</button>} />}
    </section>
    <section className="section" aria-labelledby="recent-title"><SectionHeading id="recent-title" title="Recently added" action={<button className="text-button" onClick={() => browse({})}>View library →</button>} />
      {loading ? <LoadingSkeleton /> : ready.length ? <div className="video-grid">{ready.slice(0, 4).map((video) => <VideoCard key={video.id} video={video} savedAt={progress[video.id]} onOpen={() => openVideo(video)} />)}</div> : <EmptyState title="The library is empty" body="Ready videos will appear here." />}
    </section>
    {instructors.length > 0 && <section className="section" aria-labelledby="instructors-title"><SectionHeading id="instructors-title" title="Browse by instructor" /><div className="browse-list">{instructors.map((instructor) => <button key={instructor} onClick={() => browse({ instructor })}><span className="browse-monogram">{initials(instructor)}</span><span>{instructor}</span><span aria-hidden="true">→</span></button>)}</div></section>}
    {tags.length > 0 && <section className="section" aria-labelledby="tags-title"><SectionHeading id="tags-title" title="Browse by tag" /><div className="tag-list">{tags.map((tag) => <button key={tag} onClick={() => browse({ tag })}>{tag}</button>)}</div></section>}
  </div>
}

function LibraryScreen({ videos, progress, loading, initialFilter, openVideo, onSearch }: { videos: Video[]; progress: ProgressMap; loading: boolean; initialFilter: { instructor?: string; tag?: string }; openVideo: (video: Video) => void; onSearch: (query: string) => Promise<void> }) {
  const [query, setQuery] = useState('')
  const [instructor, setInstructor] = useState(initialFilter.instructor ?? '')
  const [tag, setTag] = useState(initialFilter.tag ?? '')
  const [visibility, setVisibility] = useState('')
  const [studyState, setStudyState] = useState('')
  const instructors = [...new Set(videos.map((video) => video.instructor_name))].sort()
  const tags = [...new Set(videos.flatMap((video) => video.tags))].sort()
  const filtered = videos.filter((video) => video.status === 'ready' && (!instructor || video.instructor_name === instructor) && (!tag || video.tags.includes(tag)) && (!visibility || video.visibility === visibility) && (!studyState || (studyState === 'started' ? (progress[video.id] ?? 0) > 0 : (progress[video.id] ?? 0) === 0)))
  async function search(event: FormEvent) { event.preventDefault(); await onSearch(query) }
  function clearFilters() { setQuery(''); setInstructor(''); setTag(''); setVisibility(''); setStudyState(''); void onSearch('') }
  const hasFilters = query || instructor || tag || visibility || studyState
  return <div className="screen"><PageHeader eyebrow="Catalog" title="Library" description={`${filtered.length} accessible ${filtered.length === 1 ? 'video' : 'videos'}`} />
    <form className="library-tools" onSubmit={search} role="search"><label className="search-field"><span className="sr-only">Search videos</span><span aria-hidden="true">⌕</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search title, instructor, series, or tags" /><button type="submit">Search</button></label>
      <div className="filter-bar"><Filter label="Instructor" value={instructor} onChange={setInstructor} options={instructors} /><Filter label="Tag" value={tag} onChange={setTag} options={tags} /><Filter label="Visibility" value={visibility} onChange={setVisibility} options={['shared', 'private']} /><Filter label="Progress" value={studyState} onChange={setStudyState} options={['started', 'not started']} />{hasFilters && <button type="button" className="text-button" onClick={clearFilters}>Clear filters</button>}</div>
    </form>
    {loading ? <LoadingSkeleton /> : filtered.length ? <div className="video-grid library-grid">{filtered.map((video) => <VideoCard key={video.id} video={video} savedAt={progress[video.id]} onOpen={() => openVideo(video)} />)}</div> : <EmptyState title="No videos found" body="Try clearing a filter or using a broader search." action={hasFilters ? <button onClick={clearFilters}>Clear filters</button> : undefined} />}
  </div>
}

function VideoCard({ video, savedAt = 0, onOpen }: { video: Video; savedAt?: number; onOpen: () => void }) {
  return <article className="video-card"><button className="video-cover" onClick={onOpen} aria-label={`Study ${video.title}`}><span className="cover-grid" aria-hidden="true" /><span className="play-mark" aria-hidden="true">▶</span>{savedAt > 0 && <span className="resume-chip">Resume {formatTime(savedAt)}</span>}</button><div className="video-card-body"><div className="video-title-row"><h2>{video.title}</h2><Visibility value={video.visibility} /></div><p>{video.instructor_name}</p>{(video.instructional_name || video.chapter_name) && <small>{[video.instructional_name, video.chapter_name].filter(Boolean).join(' · ')}</small>}<button className="card-action" onClick={onOpen}>{savedAt > 0 ? 'Resume study' : 'Start studying'} <span aria-hidden="true">→</span></button></div></article>
}

function ContinueCard({ video, savedAt, onResume }: { video: Video; savedAt: number; onResume: () => void }) {
  return <article className="continue-card"><div className="continue-cover"><span className="cover-grid" aria-hidden="true" /><button onClick={onResume} aria-label={`Resume ${video.title}`}><span aria-hidden="true">▶</span></button></div><div className="continue-copy"><Visibility value={video.visibility} /><h2>{video.title}</h2><p>{video.instructor_name}{video.chapter_name ? ` · ${video.chapter_name}` : ''}</p><div className="saved-position"><span>Saved position</span><strong>{formatTime(savedAt)}</strong></div><button onClick={onResume}>Resume at {formatTime(savedAt)} <span aria-hidden="true">→</span></button></div></article>
}

function StudyScreen({ video, onBack, setError, onProgress }: { video: Video; onBack: () => void; setError: (message: string) => void; onProgress: (seconds: number) => void }) {
  const player = useRef<HTMLVideoElement>(null)
  const noteInput = useRef<HTMLTextAreaElement>(null)
  const lastSaved = useRef(0)
  const [url, setURL] = useState('')
  const [resumeAt, setResumeAt] = useState(0)
  const [notes, setNotes] = useState<Note[]>([])
  const [draftTimestamp, setDraftTimestamp] = useState(0)
  const [loading, setLoading] = useState(true)
  useEffect(() => {
    let cancelled = false
    void Promise.all([api(`/api/videos/${video.id}/playback`), api(`/api/videos/${video.id}/progress`), api(`/api/videos/${video.id}/notes`)]).then(([playback, progress, noteBody]) => {
      if (!cancelled) { const saved = Number(progress.progress.position_seconds) || 0; setURL(playback.playback_url); setResumeAt(saved); lastSaved.current = saved; setDraftTimestamp(saved); setNotes(noteBody.notes); setLoading(false) }
    }).catch((reason) => { if (!cancelled) { setError(message(reason, 'Unable to load video')); setLoading(false) } })
    return () => { cancelled = true }
  }, [video.id, setError])
  useEffect(() => {
    const save = () => { const position = player.current?.currentTime; if (position !== undefined && Number.isFinite(position)) void api(`/api/videos/${video.id}/progress`, { method: 'PUT', body: JSON.stringify({ position_seconds: position }), keepalive: true }) }
    window.addEventListener('pagehide', save)
    return () => { window.removeEventListener('pagehide', save); save() }
  }, [video.id])
  useEffect(() => {
    const keys = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement
      if (target.matches('input, textarea, select, button, [contenteditable="true"]')) return
      if (!player.current) return
      if (event.code === 'Space') { event.preventDefault(); if (player.current.paused) void player.current.play(); else player.current.pause() }
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') { event.preventDefault(); player.current.currentTime = Math.max(0, player.current.currentTime + (event.key === 'ArrowLeft' ? -10 : 10)) }
      if (event.key.toLowerCase() === 'n') { event.preventDefault(); captureNoteTime() }
    }
    window.addEventListener('keydown', keys)
    return () => window.removeEventListener('keydown', keys)
  })
  async function saveProgress() { const position = player.current?.currentTime ?? 0; lastSaved.current = position; onProgress(position); await api(`/api/videos/${video.id}/progress`, { method: 'PUT', body: JSON.stringify({ position_seconds: position }) }) }
  function timeUpdate() { const position = player.current?.currentTime ?? 0; if (Math.abs(position - lastSaved.current) >= 10) void saveProgress().catch(() => undefined) }
  async function back() { await saveProgress().catch(() => undefined); onBack() }
  async function refreshNotes() { setNotes((await api(`/api/videos/${video.id}/notes`)).notes) }
  function captureNoteTime() { setDraftTimestamp(player.current?.currentTime ?? 0); requestAnimationFrame(() => noteInput.current?.focus()) }
  async function addNote(event: FormEvent<HTMLFormElement>) { event.preventDefault(); const form = event.currentTarget; const data = new FormData(form); try { await api(`/api/videos/${video.id}/notes`, { method: 'POST', body: JSON.stringify({ timestamp_seconds: draftTimestamp, body: data.get('body') }) }); form.reset(); await refreshNotes() } catch (reason) { setError(message(reason, 'Unable to add note')) } }
  async function updateNote(event: FormEvent<HTMLFormElement>, note: Note) { event.preventDefault(); const data = new FormData(event.currentTarget); try { await api(`/api/videos/${video.id}/notes/${note.id}`, { method: 'PATCH', body: JSON.stringify({ timestamp_seconds: Number(data.get('timestamp_seconds')), body: data.get('body') }) }); await refreshNotes() } catch (reason) { setError(message(reason, 'Unable to update note')) } }
  async function deleteNote(note: Note) { try { await api(`/api/videos/${video.id}/notes/${note.id}`, { method: 'DELETE' }); await refreshNotes() } catch (reason) { setError(message(reason, 'Unable to delete note')) } }
  function seek(seconds: number) { if (player.current) { player.current.currentTime = seconds; void player.current.play() } }
  const sortedNotes = useMemo(() => [...notes].sort((a, b) => a.timestamp_seconds - b.timestamp_seconds), [notes])
  return <div className="screen study-screen"><button className="back-button" onClick={() => void back()}>← Back to library</button><div className="study-layout"><section className="player-column" aria-labelledby="video-title"><div className="player-frame">{loading ? <div className="player-loading">Loading video…</div> : url ? <video ref={player} src={url} controls playsInline preload="metadata" onLoadedMetadata={() => { if (player.current && resumeAt > 0) player.current.currentTime = resumeAt }} onTimeUpdate={timeUpdate} onPause={() => void saveProgress()} /> : <ErrorState title="Playback unavailable" body="The secure playback URL could not be loaded." />}</div><div className="player-details"><div><div className="detail-line"><Visibility value={video.visibility} /><span>{video.instructional_name || 'Instructional video'}</span></div><h1 id="video-title">{video.title}</h1><p>{video.instructor_name}{video.chapter_name ? ` · ${video.chapter_name}` : ''}</p></div>{video.tags.length > 0 && <div className="tag-list compact">{video.tags.map((tag) => <span key={tag}>{tag}</span>)}</div>}</div></section><NotesPanel notes={sortedNotes} draftTimestamp={draftTimestamp} noteInput={noteInput} onCapture={captureNoteTime} onAdd={addNote} onSeek={seek} onUpdate={updateNote} onDelete={deleteNote} /></div></div>
}

function NotesPanel({ notes, draftTimestamp, noteInput, onCapture, onAdd, onSeek, onUpdate, onDelete }: { notes: Note[]; draftTimestamp: number; noteInput: React.RefObject<HTMLTextAreaElement | null>; onCapture: () => void; onAdd: (event: FormEvent<HTMLFormElement>) => void; onSeek: (seconds: number) => void; onUpdate: (event: FormEvent<HTMLFormElement>, note: Note) => void; onDelete: (note: Note) => void }) {
  return <aside className="notes-panel" aria-labelledby="notes-title"><div className="notes-heading"><div><p className="eyebrow">Personal study</p><h2 id="notes-title">Timestamped notes</h2></div><span>{notes.length}</span></div><button className="capture-button" onClick={onCapture}>＋ Add note at current time <kbd>N</kbd></button><form className="note-composer" onSubmit={onAdd}><label><span>Note at <code>{formatTime(draftTimestamp)}</code></span><textarea ref={noteInput} name="body" required maxLength={5000} placeholder="What should you remember here?" /></label><button type="submit">Save note</button></form><div className="notes-list">{notes.length ? notes.map((note) => <details className="note-item" key={note.id}><summary><button type="button" className="timestamp" onClick={(event) => { event.preventDefault(); onSeek(note.timestamp_seconds) }}>{formatTime(note.timestamp_seconds)}</button><span>{note.body}</span><span className="edit-hint">Edit</span></summary><form onSubmit={(event) => onUpdate(event, note)}><label><span className="sr-only">Timestamp</span><input name="timestamp_seconds" type="number" min="0" step="0.1" defaultValue={note.timestamp_seconds} /></label><label><span className="sr-only">Note</span><textarea name="body" defaultValue={note.body} required maxLength={5000} /></label><div className="note-actions"><button type="submit">Save changes</button><button type="button" className="danger-button" onClick={() => onDelete(note)}>Delete</button></div></form></details>) : <EmptyState title="No notes yet" body="Capture a detail while you watch. Notes stay private to you." />}</div></aside>
}

function UploadScreen({ user, videos, onUploaded, onError, onUpdate }: { user: User; videos: Video[]; onUploaded: () => Promise<void>; onError: (value: string) => void; onUpdate: () => Promise<void> }) {
  const [file, setFile] = useState<File | null>(null)
  const [progress, setProgress] = useState<number | null>(null)
  const [state, setState] = useState<'idle' | 'uploading' | 'success' | 'error'>('idle')
  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); onError('')
    const form = event.currentTarget; const data = new FormData(form); const chosen = data.get('file')
    if (!(chosen instanceof File) || chosen.size === 0) { onError('Choose an MP4 file before uploading.'); return }
    setState('uploading'); setProgress(0)
    try {
      const body = await api('/api/videos/upload-requests', { method: 'POST', body: JSON.stringify({ title: data.get('title'), instructor_name: data.get('instructor_name'), instructional_name: data.get('instructional_name') || null, chapter_name: data.get('chapter_name') || null, description: data.get('description'), tags: String(data.get('tags') ?? '').split(',').map((tag) => tag.trim()).filter(Boolean), visibility: data.get('visibility'), content_basis: data.get('content_basis'), filename: chosen.name, mime_type: chosen.type, byte_size: chosen.size }) })
      await new Promise<void>((resolve, reject) => { const request = new XMLHttpRequest(); request.open('PUT', body.upload_url); request.setRequestHeader('Content-Type', 'video/mp4'); request.upload.onprogress = (event) => { if (event.lengthComputable) setProgress(Math.round(event.loaded / event.total * 100)) }; request.onload = () => request.status >= 200 && request.status < 300 ? resolve() : reject(new Error('The storage upload failed. Try again.')); request.onerror = () => reject(new Error('The storage upload failed. Check your connection and try again.')); request.send(chosen) })
      await api(`/api/videos/${body.video.id}/complete`, { method: 'POST', body: '{}' }); form.reset(); setFile(null); setProgress(100); setState('success'); await onUploaded()
    } catch (reason) { setState('error'); onError(message(reason, 'Unable to upload video')) }
  }
  const manageable = videos.filter((video) => user.role === 'admin' || video.uploaded_by_user_id === user.id)
  return <div className="screen"><PageHeader eyebrow="Direct to private storage" title="Upload a video" description="Add one browser-compatible MP4 to the study library." /><div className="upload-layout"><section className="surface upload-panel"><form className="upload-form" onSubmit={upload}><div className="form-step"><span>1</span><label>Choose MP4<input name="file" type="file" accept="video/mp4,.mp4" required onChange={(event) => setFile(event.target.files?.[0] ?? null)} /></label>{file && <div className="file-summary"><strong>{file.name}</strong><span>{formatBytes(file.size)} · MP4</span></div>}</div><div className="form-step"><span>2</span><div className="field-grid"><label>Title<input name="title" required maxLength={200} /></label><label>Instructor<input name="instructor_name" required maxLength={200} /></label><label>Instructional / series<input name="instructional_name" maxLength={200} /></label><label>Chapter<input name="chapter_name" maxLength={200} /></label><label className="full">Description<textarea name="description" maxLength={10000} /></label><label className="full">Tags, comma separated<input name="tags" /></label></div></div><div className="form-step"><span>3</span><div className="field-grid"><label>Visibility<select name="visibility"><option value="shared">Shared with all users</option><option value="private">Private to me and admins</option></select></label><label>Content basis<select name="content_basis"><option value="self_created">Self-created</option><option value="licensed_for_group">Licensed for group</option><option value="personal_purchase">Personal purchase (private only)</option></select></label></div></div><div className="upload-submit"><button type="submit" disabled={state === 'uploading'}>{state === 'uploading' ? `Uploading ${progress ?? 0}%` : state === 'error' ? 'Retry upload' : 'Upload video'}</button><p>Uploads are not resumable. Keep this page open until completion. Maximum file size: 5 GiB.</p>{progress !== null && <div className="progress-track" role="progressbar" aria-label="Upload progress" aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress}><span style={{ width: `${progress}%` }} /></div>}{state === 'success' && <p className="success-text" role="status">Upload complete and verified.</p>}</div></form></section><ManageVideos videos={manageable} onUpdate={onUpdate} onError={onError} /></div></div>
}

function ManageVideos({ videos, onUpdate, onError }: { videos: Video[]; onUpdate: () => Promise<void>; onError: (value: string) => void }) {
  async function updateVideo(event: FormEvent<HTMLFormElement>, video: Video) { event.preventDefault(); const data = new FormData(event.currentTarget); try { await api(`/api/videos/${video.id}`, { method: 'PATCH', body: JSON.stringify({ title: data.get('title'), instructor_name: data.get('instructor_name'), instructional_name: video.instructional_name ?? null, chapter_name: video.chapter_name ?? null, description: video.description, tags: video.tags, visibility: data.get('visibility'), content_basis: data.get('content_basis'), archived: data.get('archived') === 'on' }) }); await onUpdate() } catch (reason) { onError(message(reason, 'Unable to update video')) } }
  return <section className="surface manage-panel"><SectionHeading title="Manage videos" /><div className="responsive-table"><table><thead><tr><th>Video</th><th>Visibility</th><th>Basis</th><th>Status</th><th>Actions</th></tr></thead><tbody>{videos.map((video) => <tr key={video.id}><td><strong>{video.title}</strong><small>{video.instructor_name}</small></td><td>{video.visibility}</td><td>{labelize(video.content_basis)}</td><td>{labelize(video.status)}</td><td><details className="row-editor"><summary>Edit</summary><form onSubmit={(event) => void updateVideo(event, video)}><label>Title<input name="title" defaultValue={video.title} required /></label><label>Instructor<input name="instructor_name" defaultValue={video.instructor_name} required /></label><label>Visibility<select name="visibility" defaultValue={video.visibility}><option value="shared">Shared</option><option value="private">Private</option></select></label><label>Content basis<select name="content_basis" defaultValue={video.content_basis}><option value="self_created">Self-created</option><option value="licensed_for_group">Licensed</option><option value="personal_purchase">Personal purchase</option></select></label><label className="check"><input name="archived" type="checkbox" /> Archive video</label><button type="submit">Save</button></form></details></td></tr>)}</tbody></table>{videos.length === 0 && <EmptyState title="No videos to manage" body="Videos you upload will appear here." />}</div></section>
}

function AdminScreen({ users, videos, onRefreshUsers, onRefreshVideos, setError, setNotice }: { users: User[]; videos: Video[]; onRefreshUsers: () => Promise<void>; onRefreshVideos: () => Promise<void>; setError: (value: string) => void; setNotice: (value: string) => void }) {
  async function createUser(event: FormEvent<HTMLFormElement>) { event.preventDefault(); const form = event.currentTarget; const data = new FormData(form); try { await api('/api/admin/users', { method: 'POST', body: JSON.stringify({ email: data.get('email'), role: data.get('role'), password: data.get('password') }) }); form.reset(); await onRefreshUsers(); setNotice('Account created.') } catch (reason) { setError(message(reason, 'Unable to create user')) } }
  async function updateUser(event: FormEvent<HTMLFormElement>, target: User) { event.preventDefault(); const data = new FormData(event.currentTarget); try { await api(`/api/admin/users/${target.id}`, { method: 'PATCH', body: JSON.stringify({ role: data.get('role'), disabled: data.get('disabled') === 'on' }) }); const password = data.get('password'); if (typeof password === 'string' && password) await api(`/api/admin/users/${target.id}/password`, { method: 'POST', body: JSON.stringify({ password }) }); await onRefreshUsers(); setNotice(`Updated ${target.email}.`) } catch (reason) { setError(message(reason, 'Unable to update user')) } }
  return <div className="screen"><PageHeader eyebrow="Administration" title="People and content" description="Manage known users and the complete video catalog." /><section className="surface admin-create"><div><h2>Create account</h2><p>Accounts are created directly; no invitation email is sent.</p></div><form onSubmit={createUser}><label>Email<input name="email" type="email" required /></label><label>Role<select name="role" defaultValue="student"><option value="student">Student</option><option value="instructor">Instructor</option><option value="admin">Admin</option></select></label><label>Temporary password<input name="password" type="password" minLength={12} required autoComplete="new-password" /></label><button type="submit">Create account</button></form></section><section className="section"><SectionHeading title="Known users" /><div className="responsive-table surface"><table><thead><tr><th>Email</th><th>Role</th><th>State</th><th>Password reset</th><th>Action</th></tr></thead><tbody>{users.map((item) => <tr key={item.id}><td><strong>{item.email}</strong></td><td colSpan={4}><form className="table-form" onSubmit={(event) => void updateUser(event, item)}><select name="role" defaultValue={item.role} aria-label={`Role for ${item.email}`}><option value="admin">Admin</option><option value="instructor">Instructor</option><option value="student">Student</option></select><label className="check"><input name="disabled" type="checkbox" defaultChecked={item.disabled} /> Disabled</label><input name="password" type="password" minLength={12} placeholder="New password (optional)" aria-label={`New password for ${item.email}`} autoComplete="new-password" /><button type="submit">Save</button></form></td></tr>)}</tbody></table></div></section><ManageVideos videos={videos} onUpdate={onRefreshVideos} onError={setError} /></div>
}

function LoginScreen({ onSubmit, error }: { onSubmit: (event: FormEvent<HTMLFormElement>) => void; error: string }) { return <main className="login-page"><section className="login-brand"><div className="brand"><span className="brand-mark">B</span><span><strong>Study Mat</strong><small>Private BJJ library</small></span></div><div><p className="eyebrow">Focused practice</p><h1>Study the detail.<br />Keep the progress.</h1><p>A private workspace for instructional video, saved progress, and timestamped notes.</p></div></section><section className="login-panel"><div className="login-card"><p className="eyebrow">Welcome back</p><h2>Sign in</h2><p>Use the account created by your administrator.</p><form onSubmit={onSubmit}><label>Email address<input name="email" type="email" required autoComplete="email" /></label><label>Password<input name="password" type="password" required autoComplete="current-password" /></label><button type="submit">Sign in</button></form>{error && <StatusMessage tone="error">{error}</StatusMessage>}</div></section></main> }
function LoadingScreen() { return <main className="loading-screen"><div className="brand-mark">B</div><p>Opening your study space…</p></main> }
function PageHeader({ eyebrow, title, description }: { eyebrow: string; title: string; description: string }) { return <header className="page-header"><p className="eyebrow">{eyebrow}</p><h1>{title}</h1><p>{description}</p></header> }
function SectionHeading({ id, title, action }: { id?: string; title: string; action?: ReactNode }) { return <div className="section-heading"><h2 id={id}>{title}</h2>{action}</div> }
function Visibility({ value }: { value: Video['visibility'] }) { return <span className={`visibility ${value}`}><span aria-hidden="true">{value === 'private' ? '⌑' : '◉'}</span>{value}</span> }
function Filter({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (value: string) => void }) { return <label><span className="sr-only">{label}</span><select value={value} onChange={(event) => onChange(event.target.value)}><option value="">All {label.toLowerCase()}</option>{options.map((option) => <option key={option} value={option}>{labelize(option)}</option>)}</select></label> }
function EmptyState({ title, body, action }: { title: string; body: string; action?: ReactNode }) { return <div className="empty-state"><span aria-hidden="true">◇</span><h3>{title}</h3><p>{body}</p>{action}</div> }
function ErrorState({ title, body }: { title: string; body: string }) { return <div className="empty-state error-state"><span aria-hidden="true">!</span><h3>{title}</h3><p>{body}</p></div> }
function LoadingSkeleton() { return <div className="skeletons" aria-label="Loading videos"><span /><span /><span /></div> }
function StatusMessage({ tone, onDismiss, children }: { tone: 'error' | 'success'; onDismiss?: () => void; children: ReactNode }) { return <div className={`status-message ${tone}`} role={tone === 'error' ? 'alert' : 'status'}><span>{children}</span>{onDismiss && <button onClick={onDismiss} aria-label="Dismiss message">×</button>}</div> }
function formatTime(seconds: number) { const whole = Math.max(0, Math.floor(seconds)); const hours = Math.floor(whole / 3600); const minutes = Math.floor((whole % 3600) / 60); const rest = String(whole % 60).padStart(2, '0'); return hours ? `${hours}:${String(minutes).padStart(2, '0')}:${rest}` : `${minutes}:${rest}` }
function formatBytes(bytes: number) { if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`; return `${(bytes / 1024 / 1024).toFixed(1)} MiB` }
function initials(value: string) { return value.split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase() }
function labelize(value: string) { return value.replaceAll('_', ' ').replace(/^./, (letter) => letter.toUpperCase()) }
function message(reason: unknown, fallback: string) { return reason instanceof Error ? reason.message : fallback }
