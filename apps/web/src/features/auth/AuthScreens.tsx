import type { FormEvent } from 'react'
import { StatusMessage, Wordmark } from '../../components/ui'

export function LoginScreen({ onSubmit, error }: { onSubmit: (event: FormEvent<HTMLFormElement>) => void; error: string }) {
  return <main className="login-page">
    <section className="login-card">
      <Wordmark detail />
      <div className="login-heading">
        <h1>Sign in</h1>
        <p>Private access for invited members.</p>
      </div>
      <form onSubmit={onSubmit}>
        <label>Email address<input name="email" type="email" required autoComplete="email" /></label>
        <label>Password<input name="password" type="password" required autoComplete="current-password" /></label>
        <button type="submit">Sign in</button>
      </form>
      {error && <StatusMessage tone="error">{error}</StatusMessage>}
      <p className="login-tagline">Watch. Note. Drill.</p>
    </section>
  </main>
}

export function LoadingScreen() {
  return <main className="loading-screen">
    <Wordmark />
    <p>Loading RollStudy…</p>
  </main>
}
