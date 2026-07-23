const thumbnailWidth = 1280
const thumbnailHeight = 720
const thumbnailType = 'image/jpeg'

export function generateVideoThumbnail(videoFile: File): Promise<File> {
  return new Promise((resolve, reject) => {
    const video = document.createElement('video')
    const objectURL = URL.createObjectURL(videoFile)
    const timeout = window.setTimeout(() => finish(new Error('Thumbnail generation timed out')), 10000)
    let settled = false

    function cleanup() {
      window.clearTimeout(timeout)
      video.onerror = null
      video.onloadedmetadata = null
      video.onseeked = null
      video.removeAttribute('src')
      video.load()
      URL.revokeObjectURL(objectURL)
    }

    function finish(error?: Error, thumbnail?: File) {
      if (settled) return
      settled = true
      cleanup()
      if (error) reject(error)
      else if (thumbnail) resolve(thumbnail)
    }

    video.preload = 'metadata'
    video.muted = true
    video.playsInline = true
    video.onerror = () => finish(new Error('The browser could not decode this MP4'))
    video.onloadedmetadata = () => {
      if (!video.videoWidth || !video.videoHeight) {
        finish(new Error('The video has no readable dimensions'))
        return
      }
      const duration = Number.isFinite(video.duration) ? video.duration : 0
      video.currentTime = duration > 0 ? Math.min(5, Math.max(0.1, duration * 0.1)) : 0.1
    }
    video.onseeked = () => {
      const canvas = document.createElement('canvas')
      canvas.width = thumbnailWidth
      canvas.height = thumbnailHeight
      const context = canvas.getContext('2d')
      if (!context) {
        finish(new Error('Thumbnail canvas is unavailable'))
        return
      }

      const sourceRatio = video.videoWidth / video.videoHeight
      const targetRatio = thumbnailWidth / thumbnailHeight
      let sourceX = 0
      let sourceY = 0
      let sourceWidth = video.videoWidth
      let sourceHeight = video.videoHeight
      if (sourceRatio > targetRatio) {
        sourceWidth = video.videoHeight * targetRatio
        sourceX = (video.videoWidth - sourceWidth) / 2
      } else {
        sourceHeight = video.videoWidth / targetRatio
        sourceY = (video.videoHeight - sourceHeight) / 2
      }

      context.drawImage(
        video,
        sourceX,
        sourceY,
        sourceWidth,
        sourceHeight,
        0,
        0,
        thumbnailWidth,
        thumbnailHeight,
      )
      canvas.toBlob((blob) => {
        if (!blob) {
          finish(new Error('The browser could not encode the thumbnail'))
          return
        }
        const baseName = videoFile.name.replace(/\.[^.]+$/, '') || 'video'
        finish(undefined, new File([blob], `${baseName}-thumbnail.jpg`, { type: thumbnailType }))
      }, thumbnailType, 0.84)
    }
    video.src = objectURL
  })
}
