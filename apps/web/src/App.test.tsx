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
	fireEvent.click(screen.getAllByRole('button', { name: 'Admin' })[0])
	expect(screen.getByRole('heading', { name: 'Create account' })).toBeInTheDocument()
  })

  it('keeps upload controls away from students', async () => {
    vi.stubGlobal('fetch', vi.fn().mockImplementation(async (input: string) => {
      if (input === '/api/auth/session') return { ok: true, status: 200, json: async () => ({ user: { id: 's', email: 'student@example.com', role: 'student' } }) }
      return { ok: true, status: 200, json: async () => ({ videos: [] }) }
    }))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Ready when you are.' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Upload' })).not.toBeInTheDocument()
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
    expect(await screen.findByRole('heading', { name: 'Timestamped notes' })).toBeInTheDocument()
    const video = document.querySelector('video') as HTMLVideoElement
    fireEvent.click(screen.getByRole('button', { name: '0:42' }))
    expect(video.currentTime).toBe(42)
  })
})
