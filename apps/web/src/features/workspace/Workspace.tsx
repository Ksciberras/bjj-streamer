import { useEffect, useState } from 'react'
import { AppShell } from '../../components/AppShell'
import { StatusMessage } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { ProgressMap, User, Video, View } from '../../types'
import { AdminScreen } from '../admin/AdminScreen'
import { HomeScreen, LibraryScreen } from '../library/LibraryScreens'
import { StudyScreen } from '../study/StudyScreen'
import { UploadScreen } from '../upload/UploadScreen'

type WorkspaceProps = {
  user: User
  logout: () => Promise<void>
}

export function Workspace({ user, logout }: WorkspaceProps) {
  const [view, setView] = useState<View>('home')
  const [users, setUsers] = useState<User[]>([])
  const [videos, setVideos] = useState<Video[]>([])
  const [progress, setProgress] = useState<ProgressMap>({})
  const [selectedVideo, setSelectedVideo] = useState<Video | null>(null)
  const [loadingVideos, setLoadingVideos] = useState(true)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [librarySeed, setLibrarySeed] = useState<{ instructor?: string; tag?: string }>({})
  const canUpload = user.role !== 'student'

  async function loadProgress(items: Video[]) {
    const results = await Promise.allSettled(
      items.map(async (video) => {
        const body = await api(`/api/videos/${video.id}/progress`)
        return [video.id, Number(body.progress.position_seconds) || 0] as const
      }),
    )
    const next: ProgressMap = {}
    results.forEach((result) => {
      if (result.status === 'fulfilled') next[result.value[0]] = result.value[1]
    })
    setProgress(next)
  }

  async function refreshUsers() {
    if (user.role === 'admin') {
      setUsers((await api('/api/admin/users')).users)
    }
  }

  async function refreshVideos(query = '') {
    const suffix = query ? `?q=${encodeURIComponent(query)}` : ''
    const next: Video[] = (await api(`/api/videos${suffix}`)).videos
    setVideos(next)
    void loadProgress(next)
  }

  useEffect(() => {
    let cancelled = false
    void api('/api/videos')
      .then((body) => {
        if (!cancelled) {
          setVideos(body.videos)
          void loadProgress(body.videos)
        }
      })
      .catch((reason) => {
        if (!cancelled) setError(errorMessage(reason, 'Unable to load videos'))
      })
      .finally(() => {
        if (!cancelled) setLoadingVideos(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (user.role !== 'admin') return
    let cancelled = false
    void api('/api/admin/users')
      .then((body) => {
        if (!cancelled) setUsers(body.users)
      })
      .catch((reason) => {
        if (!cancelled) setError(errorMessage(reason, 'Unable to load users'))
      })
    return () => {
      cancelled = true
    }
  }, [user.role])

  function navigate(next: View) {
    setSelectedVideo(null)
    setView(next)
    setError('')
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  function openVideo(video: Video) {
    setSelectedVideo(video)
    setError('')
    window.scrollTo({ top: 0 })
  }

  function browse(filter: { instructor?: string; tag?: string }) {
    setView('library')
    setSelectedVideo(null)
    setLibrarySeed(filter)
  }

  function renderContent() {
    if (selectedVideo) {
      return (
        <StudyScreen
          video={selectedVideo}
          onBack={() => setSelectedVideo(null)}
          setError={setError}
          onProgress={(seconds) =>
            setProgress((current) => ({ ...current, [selectedVideo.id]: seconds }))
          }
        />
      )
    }

    if (view === 'library') {
      return (
        <LibraryScreen
          key={`${librarySeed.instructor ?? ''}:${librarySeed.tag ?? ''}`}
          videos={videos}
          progress={progress}
          loading={loadingVideos}
          initialFilter={librarySeed}
          openVideo={openVideo}
          onSearch={async (query) => {
            setLoadingVideos(true)
            try {
              await refreshVideos(query)
            } catch (reason) {
              setError(errorMessage(reason, 'Unable to search'))
            } finally {
              setLoadingVideos(false)
            }
          }}
        />
      )
    }

    if (view === 'upload' && canUpload) {
      return (
        <UploadScreen
          user={user}
          videos={videos}
          onUploaded={async () => {
            await refreshVideos()
            setNotice('Upload complete. The video is ready to study.')
          }}
          onError={setError}
          onUpdate={refreshVideos}
        />
      )
    }

    if (view === 'admin' && user.role === 'admin') {
      return (
        <AdminScreen
          users={users}
          videos={videos}
          onRefreshUsers={refreshUsers}
          onRefreshVideos={refreshVideos}
          setError={setError}
          setNotice={setNotice}
        />
      )
    }

    return (
      <HomeScreen
        videos={videos}
        progress={progress}
        loading={loadingVideos}
        openVideo={openVideo}
        browse={browse}
      />
    )
  }

  return (
    <AppShell
      user={user}
      active={selectedVideo ? 'study' : view}
      canUpload={canUpload}
      onNavigate={navigate}
      onLogout={logout}
    >
      {error && <StatusMessage tone="error" onDismiss={() => setError('')}>{error}</StatusMessage>}
      {notice && <StatusMessage tone="success" onDismiss={() => setNotice('')}>{notice}</StatusMessage>}
      {renderContent()}
    </AppShell>
  )
}
