import type { ReactNode } from 'react'
import type { User, View } from '../types'
import { Wordmark } from './ui'

type NavigationItem = {
  id: View
  label: string
  show: boolean
}

type AppShellProps = {
  user: User
  active: View | 'study'
  canUpload: boolean
  onNavigate: (view: View) => void
  onLogout: () => Promise<void>
  children: ReactNode
}

export function AppShell({ user, active, canUpload, onNavigate, onLogout, children }: AppShellProps) {
  const navigation: NavigationItem[] = [
    { id: 'home', label: 'Home', show: true },
    { id: 'library', label: 'Library', show: true },
    { id: 'upload', label: 'Upload', show: canUpload },
    { id: 'admin', label: 'Admin', show: user.role === 'admin' },
  ]
  const primaryNavigation = navigation.filter((item) => item.show)

  return <div className="app-shell">
    <aside className="sidebar">
      <Wordmark detail />
      <Navigation items={primaryNavigation} active={active} onNavigate={onNavigate} />
      <DesktopAccount user={user} onLogout={onLogout} />
    </aside>

    <header className="mobile-header">
      <Wordmark />
      <MobileAccount user={user} canUpload={canUpload} onNavigate={onNavigate} onLogout={onLogout} />
    </header>

    <main className="content" id="main-content">{children}</main>

    <nav className="mobile-nav" aria-label="Mobile navigation">
      {primaryNavigation.filter((item) => item.id === 'home' || item.id === 'library').map((item) =>
        <NavigationButton key={item.id} item={item} active={active} onNavigate={onNavigate} />,
      )}
    </nav>
  </div>
}

function Navigation({ items, active, onNavigate }: { items: NavigationItem[]; active: View | 'study'; onNavigate: (view: View) => void }) {
  return <nav aria-label="Primary navigation">
    {items.map((item) =>
      <NavigationButton key={item.id} item={item} active={active} onNavigate={onNavigate} />,
    )}
  </nav>
}

function NavigationButton({ item, active, onNavigate }: { item: NavigationItem; active: View | 'study'; onNavigate: (view: View) => void }) {
  const isActive = active === item.id
  return <button
    className={isActive ? 'nav-link active' : 'nav-link'}
    onClick={() => onNavigate(item.id)}
    aria-current={isActive ? 'page' : undefined}
  >
    {item.label}
  </button>
}

function DesktopAccount({ user, onLogout }: { user: User; onLogout: () => Promise<void> }) {
  return <div className="account">
    <AccountAvatar user={user} />
    <div className="account-copy">
      <strong>{user.email}</strong>
      <span>{user.role}</span>
    </div>
    <button className="account-action" onClick={() => void onLogout()}>Sign out</button>
  </div>
}

function MobileAccount({ user, canUpload, onNavigate, onLogout }: { user: User; canUpload: boolean; onNavigate: (view: View) => void; onLogout: () => Promise<void> }) {
  return <details className="account-menu">
    <summary aria-label="Open account menu"><AccountAvatar user={user} /></summary>
    <div>
      {canUpload && <button onClick={() => onNavigate('upload')}>Upload</button>}
      {user.role === 'admin' && <button onClick={() => onNavigate('admin')}>Admin</button>}
      <button onClick={() => void onLogout()}>Sign out</button>
    </div>
  </details>
}

function AccountAvatar({ user }: { user: User }) {
  return <span className="account-avatar" aria-hidden="true">{user.email.charAt(0).toUpperCase()}</span>
}
