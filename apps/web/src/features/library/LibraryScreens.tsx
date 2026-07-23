import { type FormEvent, useState } from 'react'
import { EmptyState, Filter, LoadingSkeleton, PageHeader, SectionHeading, Visibility } from '../../components/ui'
import { formatTime, initials } from '../../lib/format'
import type { CourseSummary, PopularVideo, ProgressMap, Video } from '../../types'

type OpenVideo = (video: Video) => void
type Browse = (filter: { instructor?: string; tag?: string }) => void
type LibrarySort = 'recent' | 'title' | 'instructor'

export function HomeScreen({ videos, popularVideos, popularTitle, progress, loading, openVideo, browse }: { videos: Video[]; popularVideos: PopularVideo[]; popularTitle: string; progress: ProgressMap; loading: boolean; openVideo: OpenVideo; browse: Browse }) {
  const ready = videos.filter((video) => video.status === 'ready')
  const continueVideo = ready
    .filter((video) => (progress[video.id] ?? 0) > 0)
    .sort((a, b) => (progress[b.id] ?? 0) - (progress[a.id] ?? 0))[0]
  const instructors = [...new Set(ready.map((video) => video.instructor_name))].slice(0, 8)
  const tags = [...new Set(ready.flatMap((video) => video.tags))].slice(0, 12)

  return <div className="screen home-screen">
    <PageHeader title="Home" description="Pick up where you left off." />
    <section className="section" aria-labelledby="continue-title">
      <SectionHeading id="continue-title" title="Continue watching" />
      {loading
        ? <LoadingSkeleton />
        : continueVideo
          ? <ContinueCard video={continueVideo} savedAt={progress[continueVideo.id]} onResume={() => openVideo(continueVideo)} />
          : <EmptyState title="No saved progress yet" body="Start a video from your library and it will appear here." action={<button onClick={() => browse({})}>Browse library</button>} />}
    </section>
    {!loading && popularVideos.length > 0 && <section className="section" aria-labelledby="popular-title">
      <SectionHeading id="popular-title" title={popularTitle} action={<span className="home-section-context">Most studied in the last 30 days</span>} />
      <div className="video-grid">
        {popularVideos.map((video) => (
          <VideoCard
            key={video.id}
            video={video}
            savedAt={progress[video.id]}
            context={`${video.study_count} ${video.study_count === 1 ? 'learner' : 'learners'}`}
            onOpen={() => openVideo(video)}
          />
        ))}
      </div>
    </section>}
    <section className="section" aria-labelledby="recent-title">
      <SectionHeading id="recent-title" title="Recently added" action={<button className="text-button" onClick={() => browse({})}>View library →</button>} />
      {loading
        ? <LoadingSkeleton />
        : ready.length
          ? <div className="video-grid">{ready.slice(0, 4).map((video) => <VideoCard key={video.id} video={video} savedAt={progress[video.id]} onOpen={() => openVideo(video)} />)}</div>
          : <EmptyState title="The library is empty" body="Ready videos will appear here." />}
    </section>
    {instructors.length > 0 && <section className="section" aria-labelledby="instructors-title">
      <SectionHeading id="instructors-title" title="Browse by instructor" />
      <div className="browse-list">{instructors.map((instructor) => <button key={instructor} onClick={() => browse({ instructor })}><span className="browse-monogram">{initials(instructor)}</span><span>{instructor}</span><span aria-hidden="true">→</span></button>)}</div>
    </section>}
    {tags.length > 0 && <section className="section" aria-labelledby="tags-title">
      <SectionHeading id="tags-title" title="Browse by tag" />
      <div className="tag-list">{tags.map((tag) => <button key={tag} onClick={() => browse({ tag })}>{tag}</button>)}</div>
    </section>}
  </div>
}

export function LibraryScreen({ videos, courses = [], progress, loading, initialFilter, openVideo, openCourse, watchLaterIDs = new Set(), onToggleWatchLater = () => undefined, onSearch }: { videos: Video[]; courses?: CourseSummary[]; progress: ProgressMap; loading: boolean; initialFilter: { instructor?: string; tag?: string }; openVideo: OpenVideo; openCourse: (course: CourseSummary) => void; watchLaterIDs?: Set<string>; onToggleWatchLater?: (video: Video) => void; onSearch: (query: string) => Promise<void> }) {
  const [query, setQuery] = useState('')
  const [instructor, setInstructor] = useState(initialFilter.instructor ?? '')
  const [instructional, setInstructional] = useState('')
  const [tag, setTag] = useState(initialFilter.tag ?? '')
  const [visibility, setVisibility] = useState('')
  const [studyState, setStudyState] = useState('')
  const [sort, setSort] = useState<LibrarySort>('recent')
  const instructors = [...new Set(videos.map((video) => video.instructor_name))].sort()
  const instructionals = [
    ...new Set(videos.flatMap((video) => video.instructional_name ? [video.instructional_name] : [])),
  ].sort()
  const tags = [...new Set(videos.flatMap((video) => video.tags))].sort()
  const filtered = videos
    .filter((video) =>
      video.status === 'ready'
      && (!instructor || video.instructor_name === instructor)
      && (!instructional || video.instructional_name === instructional)
      && (!tag || video.tags.includes(tag))
      && (!visibility || video.visibility === visibility)
      && (!studyState || (studyState === 'started' ? (progress[video.id] ?? 0) > 0 : (progress[video.id] ?? 0) === 0)),
    )
    .map((video, index) => ({ video, index }))
    .sort((left, right) => {
      if (sort === 'title') return left.video.title.localeCompare(right.video.title)
      if (sort === 'instructor') {
        return left.video.instructor_name.localeCompare(right.video.instructor_name)
          || left.video.title.localeCompare(right.video.title)
      }
      const dateDifference = Date.parse(right.video.created_at ?? '') - Date.parse(left.video.created_at ?? '')
      return Number.isFinite(dateDifference) && dateDifference !== 0
        ? dateDifference
        : left.index - right.index
    })
    .map(({ video }) => video)

  async function search(event: FormEvent) {
    event.preventDefault()
    await onSearch(query)
  }

  function clearFilters() {
    setQuery('')
    setInstructor('')
    setInstructional('')
    setTag('')
    setVisibility('')
    setStudyState('')
    void onSearch('')
  }

  const hasFilters = Boolean(query || instructor || instructional || tag || visibility || studyState)
  return <div className="screen">
    <PageHeader title="Library" description={`${filtered.length} accessible ${filtered.length === 1 ? 'video' : 'videos'}`} />
    {courses.length > 0 && (
      <section className="course-shelf" aria-labelledby="courses-title">
        <SectionHeading id="courses-title" title="Courses" />
        <div className="course-grid">
          {courses.map((course) => (
            <button type="button" className="course-card" key={course.id} onClick={() => openCourse(course)}>
              <span className="course-card-count">{course.video_count} {course.video_count === 1 ? 'chapter' : 'chapters'}</span>
              <strong>{course.title}</strong>
              <span>{course.instructor_name}</span>
              <span className="course-card-action">Start course →</span>
            </button>
          ))}
        </div>
      </section>
    )}
    <form className="library-tools" onSubmit={search} role="search">
      <label className="search-field">
        <span className="sr-only">Search videos</span>
        <span aria-hidden="true">⌕</span>
        <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search title, instructor, series, or tags" />
        <button type="submit">Search</button>
      </label>
      <div className="filter-bar">
        <label className="sort-control">
          <span>Sort</span>
          <select value={sort} onChange={(event) => setSort(event.target.value as LibrarySort)}>
            <option value="recent">Recently added</option>
            <option value="title">Title A–Z</option>
            <option value="instructor">Instructor A–Z</option>
          </select>
        </label>
        <Filter label="Instructor" value={instructor} onChange={setInstructor} options={instructors} />
        <Filter label="Instructional" value={instructional} onChange={setInstructional} options={instructionals} />
        <Filter label="Tag" value={tag} onChange={setTag} options={tags} />
        <Filter label="Visibility" value={visibility} onChange={setVisibility} options={['shared', 'private']} />
        <Filter label="Progress" value={studyState} onChange={setStudyState} options={['started', 'not started']} />
        {hasFilters && <button type="button" className="text-button" onClick={clearFilters}>Clear filters</button>}
      </div>
    </form>
    {loading
      ? <LoadingSkeleton />
      : filtered.length
        ? <div className="video-grid library-grid">{filtered.map((video) => <VideoCard key={video.id} video={video} savedAt={progress[video.id]} savedForLater={watchLaterIDs.has(video.id)} onToggleWatchLater={() => onToggleWatchLater(video)} onOpen={() => openVideo(video)} />)}</div>
        : <EmptyState title="No videos found" body="Try clearing a filter or using a broader search." action={hasFilters ? <button onClick={clearFilters}>Clear filters</button> : undefined} />}
  </div>
}

function VideoCard({ video, savedAt = 0, savedForLater = false, context, onToggleWatchLater, onOpen }: { video: Video; savedAt?: number; savedForLater?: boolean; context?: string; onToggleWatchLater?: () => void; onOpen: () => void }) {
  return <article className="video-card">
    <button className="video-cover" onClick={onOpen} aria-label={`Study ${video.title}`}>
      <VideoPlaceholder video={video} label="Study video" />
      {savedAt > 0 && <span className="resume-chip">{formatTime(savedAt)} saved</span>}
    </button>
    <div className="video-card-body">
      <div className="video-title-row"><h2>{video.title}</h2><Visibility value={video.visibility} /></div>
      <p>{video.instructor_name}</p>
      {context && <span className="video-card-context">{context}</span>}
      {(video.instructional_name || video.chapter_name) && <small>{[video.instructional_name, video.chapter_name].filter(Boolean).join(' · ')}</small>}
      <button className="card-action" onClick={onOpen}>{savedAt > 0 ? 'Resume' : 'Study video'} <span aria-hidden="true">→</span></button>
      {onToggleWatchLater && <button className="text-button save-later" onClick={onToggleWatchLater}>{savedForLater ? 'Saved for later' : 'Watch later'}</button>}
    </div>
  </article>
}

function ContinueCard({ video, savedAt, onResume }: { video: Video; savedAt: number; onResume: () => void }) {
  return <article className="continue-card">
    <button className="continue-cover" onClick={onResume} aria-label={`Resume ${video.title}`}><VideoPlaceholder video={video} label="Continue watching" /></button>
    <div className="continue-copy">
      <Visibility value={video.visibility} />
      <h2>{video.title}</h2>
      <p>{video.instructor_name}{video.instructional_name ? ` · ${video.instructional_name}` : ''}{video.chapter_name ? ` · ${video.chapter_name}` : ''}</p>
      <div className="saved-position"><span>Saved position</span><strong>{formatTime(savedAt)}</strong></div>
      <button onClick={onResume}>Resume at {formatTime(savedAt)} <span aria-hidden="true">→</span></button>
    </div>
  </article>
}

function VideoPlaceholder({ video, label }: { video: Video; label: string }) {
  if (video.thumbnail_url) {
    return <img className="video-thumbnail" src={video.thumbnail_url} alt="" loading="lazy" />
  }
  return <span className="video-placeholder" aria-hidden="true">
    <strong>{initials(video.instructor_name)}</strong>
    <small>{label}</small>
  </span>
}
