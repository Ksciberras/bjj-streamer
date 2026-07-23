import { useState, type FormEvent } from 'react'
import { api, errorMessage } from '../../lib/api'
import { formatBytes } from '../../lib/format'
import { generateVideoThumbnail } from '../../lib/videoThumbnail'
import { uploadVideo, type VideoMetadata } from '../../lib/videoUpload'

type QueueItem = {
  id: string
  file: File
  title: string
  progress: number
  status: 'queued' | 'preparing' | 'uploading' | 'complete' | 'error'
  error: string
  videoID?: string
}

type BatchUploadFormProps = {
  onComplete: () => Promise<void>
  onError: (message: string) => void
}

function titleFromFilename(filename: string) {
  return filename.replace(/\.mp4$/i, '').replaceAll('_', ' ').trim()
}

export function BatchUploadForm({ onComplete, onError }: BatchUploadFormProps) {
  const [items, setItems] = useState<QueueItem[]>([])
  const [running, setRunning] = useState(false)

  function update(id: string, changes: Partial<QueueItem>) {
    setItems((current) => current.map((item) => item.id === id ? { ...item, ...changes } : item))
  }

  function selectFiles(files: FileList | null) {
    if (!files) return
    setItems(Array.from(files).map((file, index) => ({
      id: `${file.name}:${file.size}:${file.lastModified}:${index}`,
      file,
      title: titleFromFilename(file.name),
      progress: 0,
      status: 'queued',
      error: '',
    })))
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!items.length) {
      onError('Choose at least one MP4 file.')
      return
    }
    const data = new FormData(event.currentTarget)
    const shared = {
      instructor_name: String(data.get('instructor_name') ?? ''),
      instructional_name: String(data.get('instructional_name') ?? '') || null,
      chapter_name: null,
      description: String(data.get('description') ?? ''),
      tags: String(data.get('tags') ?? '').split(',').map((tag) => tag.trim()).filter(Boolean),
      visibility: data.get('visibility') as VideoMetadata['visibility'],
      content_basis: data.get('content_basis') as VideoMetadata['content_basis'],
    }
    const pendingItems = items.filter((item) => item.status !== 'complete')
    setRunning(true)
    onError('')

    let cursor = 0
    let failures = 0
    async function worker() {
      while (cursor < pendingItems.length) {
        const item = pendingItems[cursor++]
        update(item.id, { status: 'preparing', progress: 0, error: '' })
        try {
          const thumbnail = await generateVideoThumbnail(item.file).catch(() => null)
          update(item.id, { status: 'uploading' })
          const result = await uploadVideo({
            file: item.file,
            thumbnail,
            metadata: { ...shared, title: item.title },
            onProgress: (progress) => update(item.id, { progress }),
          })
          item.videoID = result.video.id
          update(item.id, { status: 'complete', progress: 100, videoID: result.video.id })
        } catch (reason) {
          failures += 1
          update(item.id, {
            status: 'error',
            error: errorMessage(reason, 'Upload failed'),
          })
        }
      }
    }

    await Promise.all([worker(), worker()])
    if (!failures) {
      try {
        await api('/api/courses', {
          method: 'POST',
          body: JSON.stringify({
            title: shared.instructional_name,
            instructor_name: shared.instructor_name,
            videos: items.map((item) => ({
              video_id: item.videoID,
              chapter_name: item.title,
            })),
          }),
        })
      } catch (reason) {
        onError(errorMessage(reason, 'Videos uploaded, but the course could not be created'))
      }
    }
    setRunning(false)
    await onComplete()
  }

  const completed = items.filter((item) => item.status === 'complete').length
  const failed = items.filter((item) => item.status === 'error').length

  return (
    <form className="batch-upload-form" onSubmit={submit}>
      <div className="batch-fields">
        <label className="full">
          Course MP4 files
          <input
            type="file"
            accept="video/mp4,.mp4"
            multiple
            required
            disabled={running}
            onChange={(event) => selectFiles(event.target.files)}
          />
          <small>Select files in course order. Filename prefixes such as 01, 02, and 03 are retained.</small>
        </label>
        <label>Instructor<input name="instructor_name" required maxLength={200} disabled={running} /></label>
        <label>Instructional / course<input name="instructional_name" required maxLength={200} disabled={running} /></label>
        <label className="full">Description<textarea name="description" maxLength={10000} disabled={running} /></label>
        <label className="full">Tags, comma separated<input name="tags" disabled={running} /></label>
        <label>
          Visibility
          <select name="visibility" disabled={running}>
            <option value="shared">Shared with members</option>
            <option value="private">Private video</option>
          </select>
        </label>
        <label>
          Content basis
          <select name="content_basis" disabled={running}>
            <option value="self_created">Self-created</option>
            <option value="licensed_for_group">Licensed for group</option>
            <option value="personal_purchase">Personal purchase (private only)</option>
          </select>
        </label>
      </div>

      {items.length > 0 && (
        <div className="upload-queue" aria-label="Upload queue">
          <div className="queue-heading">
            <strong>{items.length} videos selected</strong>
            {running && <span>{completed} complete</span>}
          </div>
          {items.map((item, index) => (
            <div className="queue-item" key={item.id}>
              <span className="queue-order">{String(index + 1).padStart(2, '0')}</span>
              <div className="queue-copy">
                <input
                  aria-label={`Title for ${item.file.name}`}
                  value={item.title}
                  maxLength={200}
                  required
                  disabled={running}
                  onChange={(event) => update(item.id, { title: event.target.value })}
                />
                <small>{item.file.name} · {formatBytes(item.file.size)}</small>
                {item.status === 'uploading' && (
                  <div className="progress-track" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={item.progress}>
                    <span style={{ width: `${item.progress}%` }} />
                  </div>
                )}
                {item.error && <small className="queue-error">{item.error}</small>}
              </div>
              <span className={`queue-status ${item.status}`}>
                {item.status === 'preparing' ? 'Thumbnail' : item.status}
              </span>
            </div>
          ))}
        </div>
      )}

      <div className="batch-submit">
        <button type="submit" disabled={running || !items.length || completed === items.length}>
          {running
            ? `Uploading ${completed} of ${items.length}`
            : failed
              ? `Retry ${failed} failed`
              : `Upload ${items.length || ''} videos`}
        </button>
        <p>Two files upload at a time. Keep this page open until every item finishes.</p>
      </div>
    </form>
  )
}
