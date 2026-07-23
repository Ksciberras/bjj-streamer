import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'

afterEach(() => { cleanup(); vi.restoreAllMocks() })
beforeEach(() => { vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined) })

describe('App', () => {
  it('shows login when there is no session', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, json: async () => ({ error: 'authentication required' }) }))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.getByLabelText('RollStudy')).toBeInTheDocument()
    expect(screen.getByText('Private access for invited members.')).toBeInTheDocument()
    expect(screen.queryByText(/create account/i)).not.toBeInTheDocument()
  })

  it('loads account management for an administrator', async () => {
	vi.stubGlobal('fetch', vi.fn().mockImplementation(async (input: string) => {
	  if (input === '/api/auth/session') return { ok: true, status: 200, json: async () => ({ user: { id: 'a', email: 'admin@example.com', role: 'admin' } }) }
	  if (input === '/api/admin/users') return { ok: true, status: 200, json: async () => ({ users: [{ id: 'a', email: 'admin@example.com', role: 'admin' }] }) }
	  return { ok: true, status: 200, json: async () => ({ videos: [] }) }
	}))
	render(<App />)
	expect(await screen.findByText('admin@example.com')).toBeInTheDocument()
	expect(screen.getAllByRole('button', { name: 'Upload' })).toHaveLength(2)
	fireEvent.click(screen.getAllByRole('button', { name: 'Upload' })[0])
	expect(screen.getByRole('button', { name: 'Course batch' })).toBeInTheDocument()
	fireEvent.click(screen.getByRole('button', { name: 'Course batch' }))
	expect(screen.getByLabelText(/^Course MP4 files/)).toHaveAttribute('multiple')
	fireEvent.click(screen.getAllByRole('button', { name: 'Admin' })[0])
	expect(screen.getByRole('heading', { name: 'Create account' })).toBeInTheDocument()
  })

  it('shows upload but not administration to an instructor', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async (input: string) => {
      if (input === '/api/auth/session') return { ok: true, status: 200, json: async () => ({ user: { id: 'i', email: 'instructor@example.com', role: 'instructor' } }) }
      if (input.startsWith('/api/analytics')) return { ok: true, status: 200, json: async () => ({ analytics: { days: 30, overview: { active_learners: 2, videos_started: 3, resumes: 1, notes_created: 4 }, content: [], members: [] } }) }
      return { ok: true, status: 200, json: async () => ({ videos: [] }) }
    }))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Home' })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Upload' })).toHaveLength(2)
    expect(screen.queryByRole('button', { name: 'Admin' })).not.toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: 'Upload' })[0])
    expect(screen.queryByRole('button', { name: 'Course batch' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Build from library' })).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: 'Analytics' })[0])
    expect(await screen.findByRole('heading', { name: 'Analytics' })).toBeInTheDocument()
    expect(screen.getByText('Active learners')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Most studied' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Study flow' })).toBeInTheDocument()
  })

  it('keeps upload controls away from students', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async (input: string) => {
      if (input === '/api/auth/session') return { ok: true, status: 200, json: async () => ({ user: { id: 's', email: 'student@example.com', role: 'student' } }) }
      return { ok: true, status: 200, json: async () => ({ videos: [] }) }
    }))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Home' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Upload' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Analytics' })).not.toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: 'Library' })[0])
    expect(screen.getByRole('combobox', { name: 'Sort' })).toHaveValue('recent')
  })

  it('shows popular videos from the member gym on Home', async () => {
    const video = { id: 'popular', uploaded_by_user_id: 'i', title: 'Guard retention', instructor_name: 'Coach', description: '', tags: [], visibility: 'shared', content_basis: 'self_created', original_filename: 'guard.mp4', byte_size: 10, status: 'ready' }
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async (input: string) => {
      if (input === '/api/auth/session') return { ok: true, status: 200, json: async () => ({ user: { id: 's', email: 'student@example.com', role: 'student' } }) }
      if (input === '/api/videos') return { ok: true, status: 200, json: async () => ({ videos: [video] }) }
      if (input === '/api/popular') return { ok: true, status: 200, json: async () => ({ videos: [{ ...video, study_count: 3 }] }) }
      if (input.endsWith('/progress')) return { ok: true, status: 200, json: async () => ({ progress: { position_seconds: 0 } }) }
      return { ok: true, status: 200, json: async () => ({}) }
    }))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Popular in your gym' })).toBeInTheDocument()
    expect(screen.getByText('3 learners')).toBeInTheDocument()
  })

  it('opens the native player and seeks to a private note', async () => {
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async (input: string) => {
      if (input === '/api/auth/session') return { ok: true, status: 200, json: async () => ({ user: { id: 's', email: 'student@example.com', role: 'student' } }) }
      if (input === '/api/videos') return { ok: true, status: 200, json: async () => ({ videos: [{ id: 'v', uploaded_by_user_id: 'i', title: 'Armbar', instructor_name: 'Coach', description: '', tags: [], visibility: 'shared', content_basis: 'self_created', original_filename: 'armbar.mp4', byte_size: 10, status: 'ready' }] }) }
      if (input.endsWith('/playback')) return { ok: true, status: 200, json: async () => ({ playback_url: 'http://storage/video.mp4' }) }
      if (input.endsWith('/progress')) return { ok: true, status: 200, json: async () => ({ progress: { position_seconds: 12 } }) }
      return { ok: true, status: 200, json: async () => ({ notes: [{ id: 'n', timestamp_seconds: 42, body: 'Key detail' }] }) }
    }))
    render(<App />)
    fireEvent.click(await screen.findByRole('button', { name: 'Study Armbar' }))
    expect(await screen.findByRole('heading', { name: 'Notes' })).toBeInTheDocument()
    const video = document.querySelector('video') as HTMLVideoElement
    fireEvent.click(screen.getByRole('button', { name: '0:42' }))
    expect(video.currentTime).toBe(42)
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(screen.getAllByRole('textbox')).toHaveLength(2)
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.getAllByRole('textbox')).toHaveLength(1)
  })

  it('shows gym and availability controls only to the platform owner', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async (input: string) => {
      if (input === '/api/auth/session') return { ok: true, status: 200, json: async () => ({ user: { id: 'owner', email: 'kyranu2@gmail.com', role: 'admin', is_platform_owner: true } }) }
      if (input === '/api/platform/organizations') return { ok: true, status: 200, json: async () => ({ organizations: [{ id: 'gym', name: 'BJJ Cork', slug: 'bjj-cork' }] }) }
      if (input === '/api/platform/availability') return { ok: true, status: 200, json: async () => ({ videos: [], courses: [] }) }
      if (input.startsWith('/api/analytics')) return { ok: true, status: 200, json: async () => ({ analytics: { days: 30, overview: { active_learners: 0, videos_started: 0, resumes: 0, notes_created: 0 }, content: [], members: [] } }) }
      if (input === '/api/admin/users') return { ok: true, status: 200, json: async () => ({ users: [] }) }
      if (input === '/api/courses') return { ok: true, status: 200, json: async () => ({ courses: [] }) }
      if (input === '/api/study') return { ok: true, status: 200, json: async () => ({ watch_later: [], notes: [] }) }
      return { ok: true, status: 200, json: async () => ({ videos: [] }) }
    }))
    render(<App />)
    fireEvent.click((await screen.findAllByRole('button', { name: 'Admin' }))[0])
    expect(await screen.findByRole('heading', { name: 'Gyms' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Content availability' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Gym' })).toBeRequired()
    expect(screen.getByRole('button', { name: 'Create gym' })).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: 'Analytics' })[0])
    expect(await screen.findByRole('combobox', { name: 'Gym' })).toHaveValue('')
    expect(screen.getByRole('option', { name: 'All gyms' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'BJJ Cork' })).toBeInTheDocument()
  })
})
