import { useEffect, useState } from 'react'
import { AppShell } from '../../components/AppShell'
import { StatusMessage } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { Course, CourseSummary, CourseVideo, Organization, PopularVideo, ProgressMap, StudyNote, User, Video, View } from '../../types'
import { AdminScreen } from '../admin/AdminScreen'
import { AnalyticsScreen } from '../analytics/AnalyticsScreen'
import { HomeScreen, LibraryScreen } from '../library/LibraryScreens'
import { StudyScreen } from '../study/StudyScreen'
import { StudyHub } from '../study/StudyHub'
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
  const [popularVideos, setPopularVideos] = useState<PopularVideo[]>([])
  const [courses, setCourses] = useState<CourseSummary[]>([])
  const [activeCourse, setActiveCourse] = useState<Course | null>(null)
  const [autoplay, setAutoplay] = useState(false)
  const [watchLater, setWatchLater] = useState<Video[]>([])
  const [studyNotes, setStudyNotes] = useState<StudyNote[]>([])
  const [initialSeek, setInitialSeek] = useState<number | undefined>()
  const [organizations, setOrganizations] = useState<Organization[]>([])
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

  async function refreshCourses() {
    setCourses((await api('/api/courses')).courses ?? [])
  }

  async function refreshStudy() {
    const body = await api('/api/study')
    setWatchLater(body.watch_later ?? [])
    setStudyNotes(body.notes ?? [])
  }

  async function refreshPopular() {
    setPopularVideos((await api('/api/popular')).videos ?? [])
  }

  useEffect(() => {
    let cancelled = false
    void Promise.all([api('/api/videos'), api('/api/courses'), api('/api/study'), api('/api/popular')])
      .then(([body, courseBody, studyBody, popularBody]) => {
        if (!cancelled) {
          setVideos(body.videos)
          setCourses(courseBody.courses ?? [])
          setWatchLater(studyBody.watch_later ?? [])
          setStudyNotes(studyBody.notes ?? [])
          setPopularVideos(popularBody.videos ?? [])
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

  useEffect(() => {
    if (!user.is_platform_owner) return
    void api('/api/platform/organizations').then((body) => setOrganizations(body.organizations)).catch((reason) => setError(errorMessage(reason, 'Unable to load gyms')))
  }, [user.is_platform_owner])

  function navigate(next: View) {
    setSelectedVideo(null)
    setActiveCourse(null)
    setInitialSeek(undefined)
    setView(next)
    setError('')
    if (next === 'home') void refreshPopular().catch(() => undefined)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  function openVideo(video: Video, timestamp?: number) {
    setActiveCourse(null)
    setAutoplay(false)
    setInitialSeek(timestamp)
    setSelectedVideo(video)
    setError('')
    window.scrollTo({ top: 0 })
  }

  async function toggleWatchLater(video: Video) {
    const saved = watchLater.some((item) => item.id === video.id)
    try {
      await api(`/api/videos/${video.id}/watch-later`, { method: saved ? 'DELETE' : 'PUT', body: '{}' })
      await refreshStudy()
      setNotice(saved ? 'Removed from Watch later.' : 'Saved to Watch later.')
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to update Watch later'))
    }
  }

  async function openCourse(summary: CourseSummary) {
    try {
      const body = await api(`/api/courses/${summary.id}`)
      const course: Course = body.course
      if (!course.videos.length) {
        setError('This course has no videos you can access.')
        return
      }
      setActiveCourse(course)
      setAutoplay(false)
      setSelectedVideo(course.videos[0])
      window.scrollTo({ top: 0 })
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to open course'))
    }
  }

  function selectCourseVideo(video: CourseVideo, playAutomatically = false) {
    setAutoplay(playAutomatically)
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
          key={selectedVideo.id}
          video={selectedVideo}
          course={activeCourse}
          autoPlay={autoplay}
          initialSeek={initialSeek}
          savedForLater={watchLater.some((item) => item.id === selectedVideo.id)}
          onToggleWatchLater={() => void toggleWatchLater(selectedVideo)}
          onSelectCourseVideo={selectCourseVideo}
          onBack={() => {
            setSelectedVideo(null)
            setActiveCourse(null)
          }}
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
          courses={courses}
          progress={progress}
          loading={loadingVideos}
          initialFilter={librarySeed}
          openVideo={openVideo}
          watchLaterIDs={new Set(watchLater.map((video) => video.id))}
          onToggleWatchLater={(video) => void toggleWatchLater(video)}
          openCourse={(course) => void openCourse(course)}
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

    if (view === 'study') {
      return <StudyHub
        watchLater={watchLater}
        notes={studyNotes}
        onOpenVideo={openVideo}
        onRemoveWatchLater={(video) => void toggleWatchLater(video)}
      />
    }

    if (view === 'upload' && canUpload) {
      return (
        <UploadScreen
          user={user}
          videos={videos}
          courses={courses}
          onUploaded={async () => {
            await Promise.all([refreshVideos(), refreshCourses(), refreshStudy()])
            setNotice('Upload complete. The video is ready to study.')
          }}
          onError={setError}
          onUpdate={refreshVideos}
        />
      )
    }

    if (view === 'analytics' && canUpload) {
      return <AnalyticsScreen organizations={organizations} platformOwner={Boolean(user.is_platform_owner)} setError={setError} />
    }

    if (view === 'admin' && user.role === 'admin') {
      return (
        <AdminScreen
          users={users}
          videos={videos}
          courses={courses}
          organizations={organizations}
          platformOwner={Boolean(user.is_platform_owner)}
          onRefreshUsers={refreshUsers}
          setError={setError}
          setNotice={setNotice}
        />
      )
    }

    return (
      <HomeScreen
        videos={videos}
        popularVideos={popularVideos}
        popularTitle={user.is_platform_owner ? 'Popular across gyms' : 'Popular in your gym'}
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
