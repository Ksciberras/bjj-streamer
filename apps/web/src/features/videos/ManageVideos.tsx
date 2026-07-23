import type { FormEvent } from 'react'
import { EmptyState, SectionHeading } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import { labelize } from '../../lib/format'
import type { Video } from '../../types'

type ManageVideosProps = {
  videos: Video[]
  onUpdate: () => Promise<void>
  onError: (value: string) => void
}

export function ManageVideos({ videos, onUpdate, onError }: ManageVideosProps) {
  async function updateVideo(event: FormEvent<HTMLFormElement>, video: Video) {
    event.preventDefault()
    const data = new FormData(event.currentTarget)

    try {
      await api(`/api/videos/${video.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          title: data.get('title'),
          instructor_name: data.get('instructor_name'),
          instructional_name: video.instructional_name ?? null,
          chapter_name: video.chapter_name ?? null,
          description: video.description,
          tags: video.tags,
          visibility: data.get('visibility'),
          content_basis: data.get('content_basis'),
          archived: data.get('archived') === 'on',
        }),
      })
      await onUpdate()
    } catch (reason) {
      onError(errorMessage(reason, 'Unable to update video'))
    }
  }

  return (
    <section className="surface manage-panel">
      <SectionHeading title="Manage videos" />
      <div className="responsive-table">
        <table>
          <thead>
            <tr>
              <th>Video</th>
              <th>Visibility</th>
              <th>Basis</th>
              <th>Status</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {videos.map((video) => (
              <tr key={video.id}>
                <td>
                  <strong>{video.title}</strong>
                  <small>{video.instructor_name}</small>
                </td>
                <td>{video.visibility}</td>
                <td>{labelize(video.content_basis)}</td>
                <td>{labelize(video.status)}</td>
                <td>
                  <details className="row-editor">
                    <summary>Edit</summary>
                    <form onSubmit={(event) => void updateVideo(event, video)}>
                      <label>
                        Title
                        <input name="title" defaultValue={video.title} required />
                      </label>
                      <label>
                        Instructor
                        <input name="instructor_name" defaultValue={video.instructor_name} required />
                      </label>
                      <label>
                        Visibility
                        <select name="visibility" defaultValue={video.visibility}>
                          <option value="shared">Shared</option>
                          <option value="private">Private</option>
                        </select>
                      </label>
                      <label>
                        Content basis
                        <select name="content_basis" defaultValue={video.content_basis}>
                          <option value="self_created">Self-created</option>
                          <option value="licensed_for_group">Licensed</option>
                          <option value="personal_purchase">Personal purchase</option>
                        </select>
                      </label>
                      <label className="check">
                        <input name="archived" type="checkbox" /> Archive video
                      </label>
                      <button type="submit">Save</button>
                    </form>
                  </details>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {videos.length === 0 && (
          <EmptyState title="No videos to manage" body="Videos you upload will appear here." />
        )}
      </div>
    </section>
  )
}
