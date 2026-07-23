import { useState, type FormEvent } from 'react'
import { PageHeader } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import { formatBytes } from '../../lib/format'
import { uploadToStorage } from '../../lib/objectUpload'
import { generateVideoThumbnail } from '../../lib/videoThumbnail'
import type { User, Video } from '../../types'
import { ManageVideos } from '../videos/ManageVideos'

type UploadScreenProps = {
  user: User
  videos: Video[]
  onUploaded: () => Promise<void>
  onError: (value: string) => void
  onUpdate: () => Promise<void>
}

export function UploadScreen({
  user,
  videos,
  onUploaded,
  onError,
  onUpdate,
}: UploadScreenProps) {
  const [file, setFile] = useState<File | null>(null)
  const [thumbnail, setThumbnail] = useState<File | null>(null)
  const [progress, setProgress] = useState<number | null>(null)
  const [state, setState] = useState<'idle' | 'preparing' | 'uploading' | 'success' | 'error'>('idle')

  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    onError('')
    const form = event.currentTarget
    const data = new FormData(form)
    const chosen = data.get('file')

    if (!(chosen instanceof File) || chosen.size === 0) {
      onError('Choose an MP4 file before uploading.')
      return
    }

    setState('preparing')
    setProgress(0)

    try {
      const selectedThumbnail = thumbnail
        ?? await generateVideoThumbnail(chosen).catch(() => null)
      setState('uploading')
      const body = await api('/api/videos/upload-requests', {
        method: 'POST',
        body: JSON.stringify({
          title: data.get('title'),
          instructor_name: data.get('instructor_name'),
          instructional_name: data.get('instructional_name') || null,
          chapter_name: data.get('chapter_name') || null,
          description: data.get('description'),
          tags: String(data.get('tags') ?? '')
            .split(',')
            .map((tag) => tag.trim())
            .filter(Boolean),
          visibility: data.get('visibility'),
          content_basis: data.get('content_basis'),
          filename: chosen.name,
          mime_type: chosen.type,
          byte_size: chosen.size,
        }),
      })

      await uploadToStorage(body.upload_url, chosen, setProgress)
      await api(`/api/videos/${body.video.id}/complete`, { method: 'POST', body: '{}' })
      let thumbnailWarning = ''
      if (selectedThumbnail) {
        try {
          const thumbnailUpload = await api(
            `/api/videos/${body.video.id}/thumbnail-upload-request`,
            {
              method: 'POST',
              body: JSON.stringify({
                filename: selectedThumbnail.name,
                mime_type: selectedThumbnail.type,
                byte_size: selectedThumbnail.size,
              }),
            },
          )
          await uploadToStorage(thumbnailUpload.upload_url, selectedThumbnail, () => undefined)
          await api(`/api/videos/${body.video.id}/thumbnail-complete`, {
            method: 'POST',
            body: '{}',
          })
        } catch {
          thumbnailWarning = 'The video uploaded successfully, but its thumbnail could not be saved.'
        }
      }
      form.reset()
      setFile(null)
      setThumbnail(null)
      setProgress(100)
      setState('success')
      await onUploaded()
      if (thumbnailWarning) onError(thumbnailWarning)
    } catch (reason) {
      setState('error')
      onError(errorMessage(reason, 'Unable to upload video'))
    }
  }

  const manageable = videos.filter(
    (video) => user.role === 'admin' || video.uploaded_by_user_id === user.id,
  )

  return (
    <div className="screen">
      <PageHeader title="Upload MP4" description="Add one browser-compatible MP4 to the library." />
      <div className="upload-layout">
        <section className="surface upload-panel">
          <form className="upload-form" onSubmit={upload}>
            <div className="form-step">
              <span>1</span>
              <label>
                Select MP4
                <input
                  name="file"
                  type="file"
                  accept="video/mp4,.mp4"
                  required
                  onChange={(event) => setFile(event.target.files?.[0] ?? null)}
                />
              </label>
              {file && (
                <div className="file-summary">
                  <strong>{file.name}</strong>
                  <span>{formatBytes(file.size)} · MP4</span>
                </div>
              )}
            </div>
            <div className="form-step">
              <span>2</span>
              <div className="field-grid">
                <label>Title<input name="title" required maxLength={200} /></label>
                <label>Instructor<input name="instructor_name" required maxLength={200} /></label>
                <label>Instructional / series<input name="instructional_name" maxLength={200} /></label>
                <label>Chapter<input name="chapter_name" maxLength={200} /></label>
                <label className="full">Description<textarea name="description" maxLength={10000} /></label>
                <label className="full">Tags, comma separated<input name="tags" /></label>
                <label className="full">
                  Thumbnail (optional)
                  <input
                    name="thumbnail"
                    type="file"
                    accept="image/jpeg,image/png,image/webp,.jpg,.jpeg,.png,.webp"
                    onChange={(event) => setThumbnail(event.target.files?.[0] ?? null)}
                  />
                  <small>JPEG, PNG, or WebP up to 5 MiB. If omitted, RollStudy creates one from the video.</small>
                </label>
                {thumbnail && (
                  <div className="thumbnail-summary">
                    <span aria-hidden="true">IMG</span>
                    <div>
                      <strong>{thumbnail.name}</strong>
                      <span>{formatBytes(thumbnail.size)} · Custom thumbnail</span>
                    </div>
                  </div>
                )}
              </div>
            </div>
            <div className="form-step">
              <span>3</span>
              <div className="field-grid">
                <label>
                  Visibility
                  <select name="visibility">
                    <option value="shared">Shared with members</option>
                    <option value="private">Private video</option>
                  </select>
                </label>
                <label>
                  Content basis
                  <select name="content_basis">
                    <option value="self_created">Self-created</option>
                    <option value="licensed_for_group">Licensed for group</option>
                    <option value="personal_purchase">Personal purchase (private only)</option>
                  </select>
                </label>
              </div>
            </div>
            <div className="upload-submit">
              <button type="submit" disabled={state === 'preparing' || state === 'uploading'}>
                {state === 'preparing'
                  ? 'Preparing thumbnail…'
                  : state === 'uploading'
                  ? `Uploading ${progress ?? 0}%`
                  : state === 'error'
                    ? 'Try upload again'
                    : 'Upload MP4'}
              </button>
              <p>Use a browser-compatible MP4 up to 5 GiB. Uploads are not resumable, so keep this page open.</p>
              {progress !== null && (
                <div className="progress-track" role="progressbar" aria-label="Upload progress" aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress}>
                  <span style={{ width: `${progress}%` }} />
                </div>
              )}
              {state === 'success' && <p className="success-text" role="status">Upload complete. The video is ready.</p>}
            </div>
          </form>
        </section>
        <ManageVideos videos={manageable} onUpdate={onUpdate} onError={onError} />
      </div>
    </div>
  )
}
