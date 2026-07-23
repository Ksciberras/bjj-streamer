import { useEffect, useState } from 'react'
import { EmptyState, PageHeader, SectionHeading } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { Analytics, Organization } from '../../types'

type AnalyticsScreenProps = {
  organizations: Organization[]
  platformOwner: boolean
  setError: (message: string) => void
}

export function AnalyticsScreen({ organizations, platformOwner, setError }: AnalyticsScreenProps) {
  const [period, setPeriod] = useState<7 | 30>(30)
  const [organizationID, setOrganizationID] = useState('')
  const [analytics, setAnalytics] = useState<Analytics>()
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    const organization = organizationID ? `&organization_id=${encodeURIComponent(organizationID)}` : ''
    void api(`/api/analytics?period=${period}${organization}`)
      .then((body) => {
        if (!cancelled) setAnalytics(body.analytics)
      })
      .catch((reason) => {
        if (!cancelled) setError(errorMessage(reason, 'Unable to load analytics'))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [organizationID, period, setError])

  function changePeriod(value: 7 | 30) {
    setLoading(true)
    setPeriod(value)
  }

  function changeOrganization(value: string) {
    setLoading(true)
    setOrganizationID(value)
  }

  return (
    <div className="screen analytics-screen">
      <PageHeader title="Analytics" description="See how members are using your gym’s study library." />
      <div className="analytics-toolbar">
        {platformOwner ? (
          <label className="analytics-gym-filter">
            <span>Gym</span>
            <select value={organizationID} onChange={(event) => changeOrganization(event.target.value)}>
              <option value="">All gyms</option>
              {organizations.map((organization) => <option key={organization.id} value={organization.id}>{organization.name}</option>)}
            </select>
          </label>
        ) : <span>Study activity</span>}
        <div className="period-switcher" role="group" aria-label="Analytics period">
          <button type="button" className={period === 7 ? 'active' : ''} onClick={() => changePeriod(7)}>7 days</button>
          <button type="button" className={period === 30 ? 'active' : ''} onClick={() => changePeriod(30)}>30 days</button>
        </div>
      </div>
      {loading ? <p className="analytics-loading" role="status">Loading analytics…</p> : analytics && (
        <>
          <section className="analytics-overview" aria-label={`${period}-day overview`}>
            <Metric label="Active learners" value={analytics.overview.active_learners} />
            <Metric label="Videos studied" value={analytics.overview.videos_started} />
            <Metric label="Resumes" value={analytics.overview.resumes} />
            <Metric label="Notes created" value={analytics.overview.notes_created} />
          </section>
          <section className="section">
            <SectionHeading title="Content engagement" />
            {analytics.content.length ? (
              <div className="responsive-table surface analytics-table">
                <table>
                  <thead><tr><th>Video</th><th>Viewers</th><th>Starts</th><th>Resumes</th><th>Completed</th><th>Notes</th></tr></thead>
                  <tbody>{analytics.content.map((item) => (
                    <tr key={item.video_id}>
                      <td><strong>{item.title}</strong><small>{item.instructor_name}</small></td>
                      <td>{item.unique_viewers}</td><td>{item.starts}</td><td>{item.resumes}</td><td>{item.completions}</td><td>{item.notes}</td>
                    </tr>
                  ))}</tbody>
                </table>
              </div>
            ) : <EmptyState title="No content activity yet" body="Video starts, resumes, completions, and note counts will appear here." />}
          </section>
          <section className="section">
            <SectionHeading title="Member activity" />
            {analytics.members.length ? (
              <div className="responsive-table surface analytics-table">
                <table>
                  <thead><tr><th>Member</th>{platformOwner && !organizationID && <th>Gym</th>}<th>Last active</th><th>Videos studied</th><th>Notes</th></tr></thead>
                  <tbody>{analytics.members.map((item) => (
                    <tr key={item.user_id}>
                      <td><strong>{item.email}</strong></td>
                      {platformOwner && !organizationID && <td>{item.organization_name}</td>}
                      <td>{item.last_active_at ? new Date(item.last_active_at).toLocaleDateString() : 'No activity'}</td>
                      <td>{item.videos_started}</td><td>{item.notes}</td>
                    </tr>
                  ))}</tbody>
                </table>
              </div>
            ) : <EmptyState title="No members to show" body="Active gym members will appear here." />}
          </section>
          <p className="analytics-privacy">Private note text is never shown. Only note totals are included.</p>
        </>
      )}
    </div>
  )
}

function Metric({ label, value }: { label: string; value: number }) {
  return <div className="surface analytics-metric"><strong>{value}</strong><span>{label}</span></div>
}
