export type Role = 'admin' | 'instructor' | 'student'
export type View = 'home' | 'library' | 'upload' | 'admin'

export type User = {
  id: string
  email: string
  role: Role
  disabled?: boolean
  created_at?: string
}

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

export type Note = {
  id: string
  timestamp_seconds: number
  body: string
}

export type ProgressMap = Record<string, number>
