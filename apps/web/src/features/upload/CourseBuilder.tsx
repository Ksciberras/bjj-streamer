import { useState, type FormEvent } from 'react'
import { api, errorMessage } from '../../lib/api'
import type { Video } from '../../types'

type CourseBuilderProps = {
  videos: Video[]
  onComplete: () => Promise<void>
  onError: (message: string) => void
}

export function CourseBuilder({ videos, onComplete, onError }: CourseBuilderProps) {
  const [selected, setSelected] = useState<Video[]>([])
  const [saving, setSaving] = useState(false)
  const available = videos.filter((video) => !selected.some((item) => item.id === video.id))

  function add(id: string) {
    const video = videos.find((item) => item.id === id)
    if (video) setSelected((current) => [...current, video])
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
      await api('/api/courses', {
        method: 'POST',
        body: JSON.stringify({
          title: data.get('title'),
          instructor_name: data.get('instructor_name'),
          videos: selected.map((video) => ({
            video_id: video.id,
            chapter_name: video.chapter_name || video.title,
          })),
        }),
      })
      form.reset()
      setSelected([])
      await onComplete()
    } catch (reason) {
      onError(errorMessage(reason, 'Unable to create course'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form className="course-builder" onSubmit={submit}>
      <div className="field-grid">
        <label>Course title<input name="title" required maxLength={200} disabled={saving} /></label>
        <label>Instructor<input name="instructor_name" required maxLength={200} disabled={saving} /></label>
      </div>
      <label>
        Add an uploaded video
        <select value="" disabled={saving || available.length === 0} onChange={(event) => add(event.target.value)}>
          <option value="">{available.length ? 'Choose a video…' : 'All manageable videos added'}</option>
          {available.map((video) => <option key={video.id} value={video.id}>{video.title} — {video.instructor_name}</option>)}
        </select>
      </label>
      {selected.length > 0 && (
        <ol className="course-order">
          {selected.map((video, index) => (
            <li key={video.id}>
              <span className="queue-order">{String(index + 1).padStart(2, '0')}</span>
              <span><strong>{video.title}</strong><small>{video.instructor_name}</small></span>
              <span className="course-order-actions">
                <button type="button" className="secondary-button" disabled={saving || index === 0} aria-label={`Move ${video.title} earlier`} onClick={() => move(index, -1)}>↑</button>
                <button type="button" className="secondary-button" disabled={saving || index === selected.length - 1} aria-label={`Move ${video.title} later`} onClick={() => move(index, 1)}>↓</button>
                <button type="button" className="secondary-button" disabled={saving} onClick={() => setSelected((current) => current.filter((item) => item.id !== video.id))}>Remove</button>
              </span>
            </li>
          ))}
        </ol>
      )}
      <button type="submit" disabled={saving || !selected.length}>{saving ? 'Creating course…' : 'Create course'}</button>
    </form>
  )
}
