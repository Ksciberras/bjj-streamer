import '@testing-library/jest-dom/vitest'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { App } from './App'

describe('App', () => {
  it('identifies the application without exposing unfinished features', () => {
    render(<App />)
    expect(screen.getByRole('heading', { name: 'BJJ Study' })).toBeInTheDocument()
    expect(screen.getByText(/foundation is ready/i)).toBeInTheDocument()
  })
})

