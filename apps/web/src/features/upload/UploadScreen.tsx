import { useState, type FormEvent } from 'react'
import { PageHeader, WorkspaceTabs } from '../../components/ui'
import { errorMessage } from '../../lib/api'
import { formatBytes } from '../../lib/format'
import { generateVideoThumbnail } from '../../lib/videoThumbnail'
import { uploadVideo, type VideoMetadata } from '../../lib/videoUpload'
import type { Course, CourseSummary, User, Video } from '../../types'
import { ManageVideos } from '../videos/ManageVideos'
import { BatchUploadForm } from './BatchUploadForm'
import { CourseBuilder } from './CourseBuilder'
import { ManageCourses } from './ManageCourses'

type UploadScreenProps = {
  user: User
  videos: Video[]
  courses: CourseSummary[]
  onUploaded: () => Promise<void>
  onError: (value: string) => void
  onUpdate: () => Promise<void>
}

export function UploadScreen({
  user,
  videos,
  courses,
  onUploaded,
  onError,
  onUpdate,
}: UploadScreenProps) {
  const [file, setFile] = useState<File | null>(null)
  const [thumbnail, setThumbnail] = useState<File | null>(null)
  const [progress, setProgress] = useState<number | null>(null)
  const [state, setState] = useState<'idle' | 'preparing' | 'uploading' | 'success' | 'error'>('idle')
  const [mode, setMode] = useState<'single' | 'batch'>('single')
  const [workspace, setWorkspace] = useState<'upload' | 'courses' | 'videos'>('upload')
  const [editingCourse, setEditingCourse] = useState<Course | undefined>()

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
      const result = await uploadVideo({
        file: chosen,
        thumbnail: selectedThumbnail,
        onProgress: setProgress,
        metadata: {
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
        } as VideoMetadata,
      })
      form.reset()
      setFile(null)
      setThumbnail(null)
      setProgress(100)
      setState('success')
      await onUploaded()
      if (selectedThumbnail && !result.thumbnailSaved) {
        onError('The video uploaded successfully, but its thumbnail could not be saved.')
      }
    } catch (reason) {
      setState('error')
      onError(errorMessage(reason, 'Unable to upload video'))
    }
  }

  const manageable = videos.filter(
    (video) => user.role === 'admin' || video.uploaded_by_user_id === user.id,
  )
  const manageableCourses = courses.filter((course) => course.can_manage)

  return (
    <div className="screen">
      <PageHeader
        title={workspace === 'upload' ? 'Upload' : workspace === 'courses' ? 'Courses' : 'Videos'}
        description={workspace === 'upload'
          ? 'Add browser-compatible MP4s directly to your library.'
          : workspace === 'courses'
            ? 'Build, order, and maintain instructional courses.'
            : 'Edit metadata, thumbnails, visibility, or remove videos.'}
      />
      <WorkspaceTabs
        label="Content workspace"
        value={workspace}
        items={[
          { id: 'upload', label: 'Upload' },
          { id: 'courses', label: 'Courses', count: manageableCourses.length },
          { id: 'videos', label: 'Videos', count: manageable.length },
        ]}
        onChange={(next) => {
          setWorkspace(next)
          if (next !== 'courses') setEditingCourse(undefined)
        }}
      />
      <div className="upload-layout">
        {workspace === 'upload' && <section className="surface upload-panel">
          <div className="upload-mode" role="group" aria-label="Upload mode">
            <button type="button" className={mode === 'single' ? 'active' : ''} onClick={() => setMode('single')}>Single video</button>
            {user.role === 'admin' && (
              <button type="button" className={mode === 'batch' ? 'active' : ''} onClick={() => setMode('batch')}>Course batch</button>
            )}
          </div>
          {mode === 'batch' && user.role === 'admin'
            ? <BatchUploadForm onComplete={onUploaded} onError={onError} />
            : <form className="upload-form" onSubmit={upload}>
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
            </form>}
        </section>}
        {workspace === 'courses' && <div className="course-workspace">
          <section className="surface upload-panel">
            <CourseBuilder
              key={editingCourse?.id ?? 'new-course'}
              videos={manageable}
              course={editingCourse}
              onComplete={async () => {
                setEditingCourse(undefined)
                await onUploaded()
              }}
              onCancel={() => setEditingCourse(undefined)}
              onError={onError}
            />
          </section>
          <ManageCourses
          courses={manageableCourses}
          onEdit={(course) => {
            setEditingCourse(course)
            setWorkspace('courses')
            window.scrollTo({ top: 0, behavior: 'smooth' })
          }}
          onUpdate={onUploaded}
          onError={onError}
          />
        </div>}
        {workspace === 'videos' && <ManageVideos videos={manageable} onUpdate={onUpdate} onError={onError} />}
      </div>
    </div>
  )
}
