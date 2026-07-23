import { useEffect, useState, type FormEvent } from 'react'
import { LoadingScreen, LoginScreen } from './features/auth/AuthScreens'
import { Workspace } from './features/workspace/Workspace'
import { api, errorMessage } from './lib/api'
import type { User } from './types'
import './app.css'

export function App() {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    api('/api/auth/session')
      .then((body) => setUser(body.user))
      .catch(() => setUser(null))
      .finally(() => setLoading(false))
  }, [])

  async function login(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')
    const data = new FormData(event.currentTarget)

    try {
      const body = await api('/api/auth/login', {
        method: 'POST',
        body: JSON.stringify({
          email: data.get('email'),
          password: data.get('password'),
        }),
      })
      setUser(body.user)
    } catch (reason) {
      setError(errorMessage(reason, 'Login failed'))
    }
  }

  async function logout() {
    try {
      await api('/api/auth/logout', { method: 'POST', body: '{}' })
    } finally {
      setUser(null)
    }
  }

  if (loading) return <LoadingScreen />
  if (!user) return <LoginScreen onSubmit={login} error={error} />
  return <Workspace user={user} logout={logout} />
}
