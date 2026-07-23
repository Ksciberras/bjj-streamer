import { useState } from 'react'
import { EmptyState, SectionHeading, TruncatedText } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { Course, CourseSummary } from '../../types'

type ManageCoursesProps = {
  courses: CourseSummary[]
  onEdit: (course: Course) => void
  onUpdate: () => Promise<void>
  onError: (message: string) => void
}

export function ManageCourses({ courses, onEdit, onUpdate, onError }: ManageCoursesProps) {
  const [busyID, setBusyID] = useState('')

  async function edit(course: CourseSummary) {
    setBusyID(course.id)
    try {
      const body = await api(`/api/courses/${course.id}`)
      onEdit(body.course)
    } catch (reason) {
      onError(errorMessage(reason, 'Unable to load course'))
    } finally {
      setBusyID('')
    }
  }

  async function remove(course: CourseSummary) {
    if (!window.confirm(`Delete “${course.title}”? The videos will remain in your library.`)) return
    setBusyID(course.id)
    try {
      await api(`/api/courses/${course.id}`, { method: 'DELETE', body: '{}' })
      await onUpdate()
    } catch (reason) {
      onError(errorMessage(reason, 'Unable to delete course'))
    } finally {
      setBusyID('')
    }
  }

  return (
    <section className="surface manage-panel">
      <SectionHeading title="Manage courses" />
      {courses.length ? (
        <div className="manage-course-list">
          {courses.map((course) => (
            <article key={course.id} className="manage-course-row">
              <div>
                <strong><TruncatedText text={course.title} /></strong>
                <TruncatedText text={`${course.instructor_name} · ${course.video_count} ${course.video_count === 1 ? 'chapter' : 'chapters'}`} />
              </div>
              <div className="manage-row-actions">
                <button type="button" className="secondary-button" disabled={busyID === course.id} onClick={() => void edit(course)}>Edit</button>
                <button type="button" className="danger-button" disabled={busyID === course.id} onClick={() => void remove(course)}>Delete</button>
              </div>
            </article>
          ))}
        </div>
      ) : <EmptyState title="No courses to manage" body="Courses you create will appear here." />}
    </section>
  )
}
