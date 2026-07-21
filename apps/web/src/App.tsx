import { FormEvent, useEffect, useState } from 'react'
import './app.css'

type User = { id: string; email: string; role: 'admin' | 'instructor' | 'student' }

async function api(path: string, options: RequestInit = {}) {
  const csrf = document.cookie.split('; ').find((item) => item.startsWith('bjj_csrf='))?.split('=')[1]
  const response = await fetch(path, {
    ...options,
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json', ...(csrf ? { 'X-CSRF-Token': decodeURIComponent(csrf) } : {}), ...options.headers },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: 'Request failed' }))
    throw new Error(body.error ?? 'Request failed')
  }
  return response.status === 204 ? null : response.json()
}

export function App() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const invitationToken = window.location.hash.startsWith('#invite=') ? window.location.hash.slice(8) : ''

  useEffect(() => {
    api('/api/auth/session').then((body) => setUser(body.user)).catch(() => setUser(null)).finally(() => setLoading(false))
  }, [])

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError('')
    const data = new FormData(event.currentTarget)
    try { const body = await api('/api/auth/login', { method: 'POST', body: JSON.stringify({ email: data.get('email'), password: data.get('password') }) }); setUser(body.user) }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Login failed') }
  }

  async function acceptInvitation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError('')
    const data = new FormData(event.currentTarget)
    if (data.get('password') !== data.get('confirmation')) { setError('Passwords do not match'); return }
    try { const body = await api('/api/auth/invitations/accept', { method: 'POST', body: JSON.stringify({ token: invitationToken, password: data.get('password') }) }); window.history.replaceState(null, '', window.location.pathname); setUser(body.user) }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Invitation failed') }
  }

  async function logout() { await api('/api/auth/logout', { method: 'POST', body: '{}' }); setUser(null) }

  if (loading) return <main><p>Loading…</p></main>
  if (invitationToken && !user) return <AuthCard title="Accept invitation" error={error}><form onSubmit={acceptInvitation}><label>Password<input name="password" type="password" minLength={12} required autoComplete="new-password" /></label><label>Confirm password<input name="confirmation" type="password" minLength={12} required autoComplete="new-password" /></label><button type="submit">Create account</button></form></AuthCard>
  if (!user) return <AuthCard title="Sign in" error={error}><form onSubmit={login}><label>Email<input name="email" type="email" required autoComplete="email" /></label><label>Password<input name="password" type="password" required autoComplete="current-password" /></label><button type="submit">Sign in</button></form></AuthCard>

  return <main><section className="card"><p className="eyebrow">Private training library</p><h1>Welcome</h1><p>Signed in as {user.email} ({user.role}).</p>{user.role === 'admin' && <InvitationForm setError={setError} />}<button className="secondary" onClick={() => void logout()}>Sign out</button>{error && <p role="alert" className="error">{error}</p>}</section></main>
}

function InvitationForm({ setError }: { setError: (message: string) => void }) {
  const [link, setLink] = useState('')
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(''); setLink('')
    const data = new FormData(event.currentTarget)
    try { const body = await api('/api/auth/invitations', { method: 'POST', body: JSON.stringify({ email: data.get('email'), role: data.get('role') }) }); setLink(`${window.location.origin}/#invite=${body.token}`) }
    catch (reason) { setError(reason instanceof Error ? reason.message : 'Invitation failed') }
  }
  return <div className="invite"><h2>Create invitation</h2><form onSubmit={create}><label>Email<input name="email" type="email" required /></label><label>Role<select name="role" defaultValue="student"><option value="student">Student</option><option value="instructor">Instructor</option><option value="admin">Admin</option></select></label><button type="submit">Create invitation</button></form>{link && <p className="invitation-link">Copy this link once: <code>{link}</code></p>}</div>
}

function AuthCard({ title, error, children }: { title: string; error: string; children: React.ReactNode }) {
  return <main><section className="card"><p className="eyebrow">Private training library</p><h1>{title}</h1>{children}{error && <p role="alert" className="error">{error}</p>}</section></main>
}
