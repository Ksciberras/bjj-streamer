import { api } from './api'
import { uploadToStorage } from './objectUpload'

export type VideoMetadata = {
  title: string
  instructor_name: string
  instructional_name: string | null
  chapter_name: string | null
  description: string
  tags: string[]
  visibility: 'shared' | 'private'
  content_basis: 'self_created' | 'licensed_for_group' | 'personal_purchase'
}

type UploadVideoInput = {
  file: File
  thumbnail: File | null
  metadata: VideoMetadata
  onProgress: (percentage: number) => void
}

export async function uploadVideo({ file, thumbnail, metadata, onProgress }: UploadVideoInput) {
  const body = await api('/api/videos/upload-requests', {
    method: 'POST',
    body: JSON.stringify({
      ...metadata,
      filename: file.name,
      mime_type: file.type,
      byte_size: file.size,
    }),
  })

  await uploadToStorage(body.upload_url, file, onProgress)
  const completed = await api(`/api/videos/${body.video.id}/complete`, { method: 'POST', body: '{}' })

  if (!thumbnail) return { thumbnailSaved: false, video: completed.video }
  try {
    const thumbnailUpload = await api(
      `/api/videos/${body.video.id}/thumbnail-upload-request`,
      {
        method: 'POST',
        body: JSON.stringify({
          filename: thumbnail.name,
          mime_type: thumbnail.type,
          byte_size: thumbnail.size,
        }),
      },
    )
    await uploadToStorage(thumbnailUpload.upload_url, thumbnail, () => undefined)
    await api(`/api/videos/${body.video.id}/thumbnail-complete`, {
      method: 'POST',
      body: '{}',
    })
    return { thumbnailSaved: true, video: completed.video }
  } catch {
    return { thumbnailSaved: false, video: completed.video }
  }
}
