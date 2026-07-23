export type Role = 'admin' | 'instructor' | 'student'
export type View = 'home' | 'library' | 'study' | 'upload' | 'analytics' | 'admin'

export type User = {
  id: string
  email: string
  role: Role
  disabled?: boolean
  created_at?: string
  organization_id?: string
  organization_name?: string
  is_platform_owner?: boolean
}

export type Organization = { id: string; name: string; slug: string }

export type Video = {
  id: string
  uploaded_by_user_id: string
  title: string
  instructor_name: string
  instructional_name?: string
  chapter_name?: string
  description: string
  tags: string[]
  visibility: 'shared' | 'private'
  content_basis: 'self_created' | 'licensed_for_group' | 'personal_purchase'
  original_filename: string
  thumbnail_url?: string
  byte_size: number
  status: 'pending_upload' | 'ready' | 'archived'
  created_at?: string
}

export type PopularVideo = Video & { study_count: number }

export type CourseSummary = {
  id: string
  created_by_user_id: string
  title: string
  instructor_name: string
  video_count: number
  organization_id?: string
  can_manage?: boolean
  thumbnail_url?: string
}

export type CourseVideo = Video & {
  sequence_number: number
  course_chapter_name?: string
}

export type Course = Omit<CourseSummary, 'video_count'> & {
  videos: CourseVideo[]
}

export type Note = {
  id: string
  timestamp_seconds: number
  body: string
}

export type StudyNote = Note & {
  video_id: string
  video_title: string
  instructor_name: string
  video: Video
}

export type ProgressMap = Record<string, number>

export type Analytics = {
  days: number
  overview: {
    active_learners: number
    videos_started: number
    resumes: number
    notes_created: number
  }
  activity: Array<{
    date: string
    active_learners: number
    study_actions: number
    notes_created: number
  }>
  content: Array<{
    video_id: string
    title: string
    instructor_name: string
    unique_viewers: number
    starts: number
    resumes: number
    completions: number
    notes: number
  }>
  members: Array<{
    user_id: string
    email: string
    organization_name: string
    last_active_at?: string
    videos_started: number
    notes: number
  }>
}
