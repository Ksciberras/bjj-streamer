import type { ReactNode } from 'react'
import type { User, View } from '../types'
import { TruncatedText, Wordmark } from './ui'

type NavigationItem = {
  id: View
  label: string
  show: boolean
  icon: IconName
}
type IconName = 'home' | 'library' | 'study' | 'content' | 'analytics' | 'admin'

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
    { id: 'home', label: 'Home', icon: 'home', show: true },
    { id: 'library', label: 'Library', icon: 'library', show: true },
    { id: 'study', label: 'Study', icon: 'study', show: true },
    { id: 'upload', label: 'Content', icon: 'content', show: canUpload },
    { id: 'analytics', label: 'Analytics', icon: 'analytics', show: canUpload },
    { id: 'admin', label: 'Admin', icon: 'admin', show: user.role === 'admin' },
  ]
  const primaryNavigation = navigation.filter((item) => item.show)

  return <div className="app-shell">
    <a className="skip-link" href="#main-content">Skip to content</a>
    <aside className="sidebar">
      <Wordmark detail />
      <Navigation items={primaryNavigation} active={active} onNavigate={onNavigate} />
      <DesktopAccount user={user} onLogout={onLogout} />
    </aside>

    <header className="mobile-header">
      <Wordmark />
      <MobileAccount user={user} canUpload={canUpload} onNavigate={onNavigate} onLogout={onLogout} />
    </header>

    <main className="content" id="main-content" tabIndex={-1}>{children}</main>

    <nav className="mobile-nav" aria-label="Mobile navigation">
      {primaryNavigation.filter((item) => item.id === 'home' || item.id === 'library' || item.id === 'study').map((item) =>
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
    <NavIcon name={item.icon} />
    <span>{item.label}</span>
  </button>
}

function DesktopAccount({ user, onLogout }: { user: User; onLogout: () => Promise<void> }) {
  return <div className="account">
    <AccountAvatar user={user} />
    <div className="account-copy">
      <strong><TruncatedText text={user.email} /></strong>
      <TruncatedText text={user.is_platform_owner ? 'Platform owner · All gyms' : `${user.role} · ${user.organization_name ?? 'Gym workspace'}`} />
    </div>
    <button className="account-action" onClick={() => void onLogout()}>Sign out</button>
  </div>
}

function MobileAccount({ user, canUpload, onNavigate, onLogout }: { user: User; canUpload: boolean; onNavigate: (view: View) => void; onLogout: () => Promise<void> }) {
  return <details className="account-menu">
    <summary aria-label="Open account menu"><AccountAvatar user={user} /></summary>
    <div>
      {canUpload && <button onClick={() => onNavigate('upload')}>Content</button>}
      {canUpload && <button onClick={() => onNavigate('analytics')}>Analytics</button>}
      {user.role === 'admin' && <button onClick={() => onNavigate('admin')}>Admin</button>}
      <button onClick={() => void onLogout()}>Sign out</button>
    </div>
  </details>
}

function AccountAvatar({ user }: { user: User }) {
  return <span className="account-avatar" aria-hidden="true">{user.email.charAt(0).toUpperCase()}</span>
}

function NavIcon({ name }: { name: IconName }) {
  const paths: Record<IconName, string> = {
    home: 'M3 10.5 10 4l7 6.5V18H6v-7.5',
    library: 'M4 4h12v14H4zM7 7h6M7 10h6',
    study: 'M5 4h8a2 2 0 0 1 2 2v10H7a2 2 0 0 0-2 2V4Zm0 12h10',
    content: 'M10 3v11M6 7l4-4 4 4M4 14v3h12v-3',
    analytics: 'M4 17V9M10 17V4M16 17v-6',
    admin: 'M10 3 4 5v5c0 4 2.5 6.5 6 8 3.5-1.5 6-4 6-8V5l-6-2Zm-2 7 1.5 1.5L13 8',
  }
  return <svg className="nav-icon" viewBox="0 0 20 20" aria-hidden="true"><path d={paths[name]} /></svg>
}
