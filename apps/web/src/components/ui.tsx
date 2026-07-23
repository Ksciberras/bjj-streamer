import type { ReactNode } from 'react'
import type { Video } from '../types'
import { labelize } from '../lib/format'

export function Wordmark({ detail = false }: { detail?: boolean }) {
  return <div className="wordmark" aria-label="RollStudy">
    <strong><span>Roll</span><i aria-hidden="true" /><span>Study</span></strong>
    {detail && <small>Watch. Note. Drill.</small>}
  </div>
}

export function PageHeader({ title, description }: { title: string; description: string }) {
  return <header className="page-header">
    <h1>{title}</h1>
    <p>{description}</p>
  </header>
}

export function SectionHeading({ id, title, action }: { id?: string; title: string; action?: ReactNode }) {
  return <div className="section-heading">
    <h2 id={id}>{title}</h2>
    {action}
  </div>
}

export function Visibility({ value }: { value: Video['visibility'] }) {
  return <span className={`visibility ${value}`}>
    {value === 'private' ? 'Private video' : 'Shared with members'}
  </span>
}

export function Filter({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (value: string) => void }) {
  return <label>
    <span className="sr-only">{label}</span>
    <select value={value} onChange={(event) => onChange(event.target.value)}>
      <option value="">All {label.toLowerCase()}</option>
      {options.map((option) => <option key={option} value={option}>{labelize(option)}</option>)}
    </select>
  </label>
}

export function EmptyState({ title, body, action }: { title: string; body: string; action?: ReactNode }) {
  return <div className="empty-state">
    <h3>{title}</h3>
    <p>{body}</p>
    {action}
  </div>
}

export function ErrorState({ title, body }: { title: string; body: string }) {
  return <div className="empty-state error-state">
    <span aria-hidden="true">!</span>
    <h3>{title}</h3>
    <p>{body}</p>
  </div>
}

export function LoadingSkeleton() {
  return <div className="skeletons" aria-label="Loading videos"><span /><span /><span /></div>
}

export function StatusMessage({ tone, onDismiss, children }: { tone: 'error' | 'success'; onDismiss?: () => void; children: ReactNode }) {
  return <div className={`status-message ${tone}`} role={tone === 'error' ? 'alert' : 'status'} aria-live={tone === 'error' ? 'assertive' : 'polite'}>
    <span>{children}</span>
    {onDismiss && <button onClick={onDismiss} aria-label="Dismiss message">×</button>}
  </div>
}
