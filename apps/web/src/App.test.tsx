import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'

afterEach(() => vi.restoreAllMocks())

describe('App', () => {
  it('shows login when there is no session', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, json: async () => ({ error: 'authentication required' }) }))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.queryByText(/create account/i)).not.toBeInTheDocument()
  })

  it('shows the invitation flow only for a fragment capability', async () => {
    window.location.hash = '#invite=secret-token'
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, json: async () => ({}) }))
    render(<App />)
    expect(await screen.findByRole('heading', { name: 'Accept invitation' })).toBeInTheDocument()
    window.location.hash = ''
  })
})
