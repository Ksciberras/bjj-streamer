import { useState, type FormEvent } from 'react'
import { Dialog, EmptyState, SectionHeading } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import { labelize } from '../../lib/format'
import { uploadToStorage } from '../../lib/objectUpload'
import type { Video } from '../../types'

type ManageVideosProps = {
  videos: Video[]
  onUpdate: () => Promise<void>
  onError: (value: string) => void
}

export function ManageVideos({ videos, onUpdate, onError }: ManageVideosProps) {
  const [editingVideo, setEditingVideo] = useState<Video>()
  async function deleteVideo(video: Video) {
    if (!window.confirm(`Delete “${video.title}” from the library? Its stored file and study history will be retained.`)) return
    try {
      await api(`/api/videos/${video.id}`, { method: 'DELETE', body: '{}' })
      await onUpdate()
      setEditingVideo(undefined)
    } catch (reason) {
      onError(errorMessage(reason, 'Unable to delete video'))
    }
  }

  async function uploadThumbnail(video: Video, file: File) {
    if (file.size === 0) return
    try {
      const body = await api(`/api/videos/${video.id}/thumbnail-upload-request`, {
        method: 'POST',
        body: JSON.stringify({
          filename: file.name,
          mime_type: file.type,
          byte_size: file.size,
        }),
      })
      await uploadToStorage(body.upload_url, file, () => undefined)
      await api(`/api/videos/${video.id}/thumbnail-complete`, {
        method: 'POST',
        body: '{}',
      })
      await onUpdate()
    } catch (reason) {
      onError(errorMessage(reason, 'Unable to upload thumbnail'))
    }
  }

  async function updateVideo(event: FormEvent<HTMLFormElement>, video: Video) {
    event.preventDefault()
    const data = new FormData(event.currentTarget)

    try {
      await api(`/api/videos/${video.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          title: data.get('title'),
          instructor_name: data.get('instructor_name'),
          instructional_name: data.get('instructional_name') || null,
          chapter_name: data.get('chapter_name') || null,
          description: data.get('description'),
          tags: String(data.get('tags') ?? '').split(',').map((tag) => tag.trim()).filter(Boolean),
          visibility: data.get('visibility'),
          content_basis: data.get('content_basis'),
          archived: data.get('archived') === 'on',
        }),
      })
      await onUpdate()
      setEditingVideo(undefined)
    } catch (reason) {
      onError(errorMessage(reason, 'Unable to update video'))
    }
  }

  return (<>
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
                  <div className="table-video">
                    {video.thumbnail_url
                      ? <img src={video.thumbnail_url} alt="" loading="lazy" />
                      : <span aria-hidden="true" />}
                    <div>
                      <strong>{video.title}</strong>
                      <small>{video.instructor_name}</small>
                    </div>
                  </div>
                </td>
                <td>{video.visibility}</td>
                <td>{labelize(video.content_basis)}</td>
                <td>{labelize(video.status)}</td>
                <td>
                  <button type="button" className="table-action" onClick={() => setEditingVideo(video)}>Edit</button>
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
    {editingVideo && <Dialog title="Edit video" description={editingVideo.title} onClose={() => setEditingVideo(undefined)}>
      <form className="dialog-form" onSubmit={(event) => void updateVideo(event, editingVideo)}>
        <div className="dialog-field-grid">
          <label>Title<input name="title" defaultValue={editingVideo.title} required /></label>
          <label>Instructor<input name="instructor_name" defaultValue={editingVideo.instructor_name} required /></label>
          <label>Instructional / series<input name="instructional_name" defaultValue={editingVideo.instructional_name} /></label>
          <label>Chapter<input name="chapter_name" defaultValue={editingVideo.chapter_name} /></label>
          <label className="dialog-full">Description<textarea name="description" defaultValue={editingVideo.description} /></label>
          <label className="dialog-full">Tags, comma separated<input name="tags" defaultValue={editingVideo.tags.join(', ')} /></label>
          <label>Visibility<select name="visibility" defaultValue={editingVideo.visibility}><option value="shared">Shared</option><option value="private">Private</option></select></label>
          <label>Content basis<select name="content_basis" defaultValue={editingVideo.content_basis}><option value="self_created">Self-created</option><option value="licensed_for_group">Licensed</option><option value="personal_purchase">Personal purchase</option></select></label>
          <label className="dialog-full">Replace thumbnail<input type="file" accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp" onChange={(event) => { const file = event.target.files?.[0]; if (file) void uploadThumbnail(editingVideo, file) }} /></label>
        </div>
        <div className="dialog-actions dialog-actions-split"><button type="button" className="danger-button" onClick={() => void deleteVideo(editingVideo)}>Delete video</button><span><button type="button" className="secondary-button" onClick={() => setEditingVideo(undefined)}>Cancel</button><button type="submit">Save changes</button></span></div>
      </form>
    </Dialog>}
  </>)
}
