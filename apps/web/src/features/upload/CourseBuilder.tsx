import { useState, type FormEvent } from 'react'
import { TruncatedText } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import { initials } from '../../lib/format'
import type { Course, Video } from '../../types'

type CourseBuilderProps = {
  videos: Video[]
  course?: Course
  onComplete: () => Promise<void>
  onError: (message: string) => void
  onCancel?: () => void
}

export function CourseBuilder({ videos, course, onComplete, onError, onCancel }: CourseBuilderProps) {
  const [selected, setSelected] = useState<Video[]>(course?.videos ?? [])
  const [saving, setSaving] = useState(false)
  const [query, setQuery] = useState('')
  const [instructor, setInstructor] = useState('')
  const selectedIDs = new Set(selected.map((video) => video.id))
  const instructors = [...new Set(videos.map((video) => video.instructor_name))].sort()
  const filtered = videos.filter((video) => {
    const haystack = [video.title, video.instructor_name, video.instructional_name, video.chapter_name, ...video.tags]
      .filter(Boolean)
      .join(' ')
      .toLocaleLowerCase()
    return (!query || haystack.includes(query.toLocaleLowerCase()))
      && (!instructor || video.instructor_name === instructor)
  })

  function toggle(video: Video) {
    setSelected((current) => selectedIDs.has(video.id)
      ? current.filter((item) => item.id !== video.id)
      : [...current, video])
  }

  function move(index: number, change: number) {
    setSelected((current) => {
      const target = index + change
      if (target < 0 || target >= current.length) return current
      const next = [...current]
      ;[next[index], next[target]] = [next[target], next[index]]
      return next
    })
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!selected.length) {
      onError('Add at least one video to the course.')
      return
    }
    const form = event.currentTarget
    const data = new FormData(form)
    setSaving(true)
    onError('')
    try {
      await api(course ? `/api/courses/${course.id}` : '/api/courses', {
        method: course ? 'PATCH' : 'POST',
        body: JSON.stringify({
          title: data.get('title'),
          instructor_name: data.get('instructor_name'),
          videos: selected.map((video) => ({
            video_id: video.id,
            chapter_name: ('course_chapter_name' in video && typeof video.course_chapter_name === 'string'
              ? video.course_chapter_name
              : video.chapter_name) || video.title,
          })),
        }),
      })
      form.reset()
      setSelected([])
      await onComplete()
    } catch (reason) {
      onError(errorMessage(reason, course ? 'Unable to update course' : 'Unable to create course'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className="course-builder" onSubmit={submit}>
      <div className="course-builder-heading">
        <div>
          <h2>{course ? 'Edit course' : 'Build a course'}</h2>
          <p>{course ? 'Update the details, chapters, and playback order.' : 'Choose videos, then arrange them in playback order.'}</p>
        </div>
        <span>{selected.length} selected</span>
      </div>
      <div className="field-grid course-details">
        <label>Course title<input name="title" required maxLength={200} disabled={saving} defaultValue={course?.title} placeholder="e.g. New Wave Half Guard" /></label>
        <label>Instructor<input name="instructor_name" required maxLength={200} disabled={saving} defaultValue={course?.instructor_name} placeholder="Instructor name" /></label>
      </div>
      <div className="course-builder-layout">
        <section className="course-picker" aria-labelledby="choose-videos-title">
          <div className="course-picker-heading">
            <div><h3 id="choose-videos-title">Choose videos</h3><span>{filtered.length} shown</span></div>
            <div className="course-picker-filters">
              <label><span className="sr-only">Search videos</span><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search videos…" /></label>
              <label><span className="sr-only">Filter by instructor</span><select value={instructor} onChange={(event) => setInstructor(event.target.value)}><option value="">All instructors</option>{instructors.map((name) => <option key={name}>{name}</option>)}</select></label>
            </div>
          </div>
          {filtered.length ? (
            <div className="course-picker-grid">
              {filtered.map((video) => {
                const isSelected = selectedIDs.has(video.id)
                return (
                  <button
                    key={video.id}
                    type="button"
                    className={`course-pick-card${isSelected ? ' selected' : ''}`}
                    title={`${video.title} — ${video.instructor_name}`}
                    aria-pressed={isSelected}
                    disabled={saving}
                    onClick={() => toggle(video)}
                  >
                    <span className="course-pick-cover">
                      {video.thumbnail_url
                        ? <img src={video.thumbnail_url} alt="" loading="lazy" />
                        : <span className="course-pick-placeholder" aria-hidden="true">{initials(video.instructor_name)}</span>}
                      <span className="course-pick-state">{isSelected ? 'Selected' : 'Add'}</span>
                    </span>
                    <span className="course-pick-copy">
                      <strong><TruncatedText text={video.title} focusable={false} /></strong>
                      <small><TruncatedText text={video.instructor_name} focusable={false} /></small>
                      {(video.instructional_name || video.chapter_name) && <TruncatedText text={[video.instructional_name, video.chapter_name].filter(Boolean).join(' · ')} focusable={false} />}
                    </span>
                  </button>
                )
              })}
            </div>
          ) : <p className="course-picker-empty">No videos match those filters.</p>}
        </section>
        <section className="course-sequence" aria-labelledby="course-sequence-title">
          <div><h3 id="course-sequence-title">Course order</h3><span>{selected.length} chapters</span></div>
          {selected.length > 0 ? (
            <ol className="course-order">
              {selected.map((video, index) => (
                <li key={video.id}>
                  <span className="queue-order">{String(index + 1).padStart(2, '0')}</span>
                  <span><strong><TruncatedText text={video.title} /></strong><small><TruncatedText text={video.instructor_name} /></small></span>
                  <span className="course-order-actions">
                    <button type="button" className="secondary-button" disabled={saving || index === 0} aria-label={`Move ${video.title} earlier`} onClick={() => move(index, -1)}>↑</button>
                    <button type="button" className="secondary-button" disabled={saving || index === selected.length - 1} aria-label={`Move ${video.title} later`} onClick={() => move(index, 1)}>↓</button>
                    <button type="button" className="secondary-button" disabled={saving} aria-label={`Remove ${video.title}`} onClick={() => toggle(video)}>×</button>
                  </span>
                </li>
              ))}
            </ol>
          ) : <div className="course-sequence-empty"><strong>No videos selected</strong><span>Pick video cards to build the course.</span></div>}
          <button className="course-create-button" type="submit" disabled={saving || !selected.length}>
            {saving ? 'Saving course…' : course ? 'Save course changes' : `Create course with ${selected.length} ${selected.length === 1 ? 'video' : 'videos'}`}
          </button>
          {course && <button className="secondary-button course-cancel-button" type="button" disabled={saving} onClick={onCancel}>Cancel editing</button>}
        </section>
      </div>
    </form>
  )
}
