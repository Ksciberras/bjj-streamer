import { useEffect, useMemo, useState } from 'react'
import { EmptyState, PageHeader, TruncatedText } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { Analytics, Organization } from '../../types'

type AnalyticsScreenProps = {
  organizations: Organization[]
  platformOwner: boolean
  setError: (message: string) => void
}

type DetailView = 'videos' | 'members'

export function AnalyticsScreen({ organizations, platformOwner, setError }: AnalyticsScreenProps) {
  const [period, setPeriod] = useState<7 | 30>(30)
  const [organizationID, setOrganizationID] = useState('')
  const [detailView, setDetailView] = useState<DetailView>('videos')
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
        if (!cancelled) setError(errorMessage(reason, 'Couldn’t load analytics'))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [organizationID, period, setError])

  const scopeName = useMemo(() => {
    if (!platformOwner) return 'Your gym'
    if (!organizationID) return 'All gyms'
    return organizations.find((organization) => organization.id === organizationID)?.name ?? 'Selected gym'
  }, [organizationID, organizations, platformOwner])

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
      <PageHeader title="Analytics" description="Understand what members study and where they return." />
      <div className="analytics-toolbar surface">
        <div className="analytics-scope">
          <span>Viewing</span>
          {platformOwner ? (
            <label className="analytics-gym-filter">
              <span className="sr-only">Gym</span>
              <select value={organizationID} onChange={(event) => changeOrganization(event.target.value)}>
                <option value="">All gyms</option>
                {organizations.map((organization) => <option key={organization.id} value={organization.id}>{organization.name}</option>)}
              </select>
            </label>
          ) : <strong>{scopeName}</strong>}
        </div>
        <div className="period-switcher" role="group" aria-label="Analytics period">
          <button type="button" aria-pressed={period === 7} className={period === 7 ? 'active' : ''} onClick={() => changePeriod(7)}>7 days</button>
          <button type="button" aria-pressed={period === 30} className={period === 30 ? 'active' : ''} onClick={() => changePeriod(30)}>30 days</button>
        </div>
      </div>

      {loading ? <AnalyticsLoading /> : analytics && (
        <>
          <AnalyticsOverview analytics={analytics} period={period} scopeName={scopeName} />
          <section className="analytics-visual-grid" aria-label={`${period}-day study activity`}>
            <ActivityTimeline analytics={analytics} period={period} />
            <MostStudied analytics={analytics} period={period} />
          </section>
          <AnalyticsDetails
            analytics={analytics}
            detailView={detailView}
            setDetailView={setDetailView}
            showOrganization={platformOwner && !organizationID}
          />
          <p className="analytics-privacy">Note contents stay private. Analytics include totals only.</p>
        </>
      )}
    </div>
  )
}

function AnalyticsOverview({ analytics, period, scopeName }: { analytics: Analytics; period: number; scopeName: string }) {
  return (
    <section className="analytics-overview" aria-label={`${period}-day overview`}>
      <div className="analytics-primary-metric surface">
        <span className="analytics-kicker">{scopeName} · Last {period} days</span>
        <strong>{analytics.overview.active_learners}</strong>
        <div>
          <h2>Active learners</h2>
          <p>{analytics.overview.active_learners
            ? `${analytics.overview.videos_started} learner–video sessions recorded in this period.`
            : 'Study activity will appear when members begin watching.'}</p>
        </div>
      </div>
      <div className="analytics-supporting-metrics surface">
        <Metric label="Videos studied" value={analytics.overview.videos_started} hint="Unique learner and video pairs" />
        <Metric label="Returned to study" value={analytics.overview.resumes} hint="Resume actions" />
        <Metric label="Notes captured" value={analytics.overview.notes_created} hint="Private notes created" />
      </div>
    </section>
  )
}

function Metric({ label, value, hint }: { label: string; value: number; hint: string }) {
  return (
    <div className="analytics-metric">
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{hint}</small>
    </div>
  )
}

function ActivityTimeline({ analytics, period }: { analytics: Analytics; period: number }) {
  const activity = analytics.activity ?? []
  const maxActions = Math.max(1, ...activity.map((item) => item.study_actions + item.notes_created))
  const hasActivity = activity.some((item) => item.study_actions > 0 || item.notes_created > 0)

  return (
    <div className="surface analytics-chart analytics-timeline-card">
      <ChartHeading title="Daily activity" description="Study actions and notes by day" meta={`${period} days`} />
      {hasActivity ? (
        <>
          <div className="timeline-legend" aria-hidden="true"><span>Study actions</span><span>Notes</span></div>
          <div className={`activity-timeline days-${period}`} role="img" aria-label={`Daily activity over the last ${period} days`}>
            {activity.map((item, index) => {
              const total = item.study_actions + item.notes_created
              const date = new Date(item.date)
              const label = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
              const showLabel = period === 7 || index === 0 || index === activity.length - 1 || index % 7 === 0
              return (
                <div className="activity-day" key={item.date} title={`${label}: ${item.study_actions} study actions, ${item.notes_created} notes, ${item.active_learners} learners`}>
                  <span className="activity-bar" style={{ height: `${Math.max(3, (total / maxActions) * 100)}%` }}>
                    <i className="activity-bar-notes" style={{ height: total ? `${(item.notes_created / total) * 100}%` : '0' }} />
                  </span>
                  <small>{showLabel ? label : ''}</small>
                </div>
              )
            })}
          </div>
        </>
      ) : <ChartEmpty body="A daily view will build as members watch and take notes." />}
    </div>
  )
}

function MostStudied({ analytics, period }: { analytics: Analytics; period: number }) {
  const content = analytics.content.filter((item) => item.unique_viewers > 0).slice(0, 5)
  const maxViewers = Math.max(1, ...content.map((item) => item.unique_viewers))

  return (
    <div className="surface analytics-chart">
      <ChartHeading title="Most studied" description="Videos ranked by unique learners" meta={`${period} days`} />
      {content.length ? (
        <ol className="ranked-content">
          {content.map((item, index) => (
            <li key={item.video_id}>
              <span className="rank-number">{String(index + 1).padStart(2, '0')}</span>
              <div>
                <strong><TruncatedText text={item.title} /></strong>
                <TruncatedText text={item.instructor_name} />
                <span className="chart-track"><i style={{ width: `${(item.unique_viewers / maxViewers) * 100}%` }} /></span>
              </div>
              <span className="rank-value"><strong>{item.unique_viewers}</strong> {item.unique_viewers === 1 ? 'learner' : 'learners'}</span>
            </li>
          ))}
        </ol>
      ) : <ChartEmpty body="Popular videos will appear after members begin studying." />}
    </div>
  )
}

function ChartHeading({ title, description, meta }: { title: string; description: string; meta?: string }) {
  return (
    <div className="analytics-chart-heading">
      <div><h2>{title}</h2><p>{description}</p></div>
      {meta && <span>{meta}</span>}
    </div>
  )
}

function AnalyticsDetails({
  analytics,
  detailView,
  setDetailView,
  showOrganization,
}: {
  analytics: Analytics
  detailView: DetailView
  setDetailView: (view: DetailView) => void
  showOrganization: boolean
}) {
  return (
    <section className="section analytics-details">
      <div className="analytics-details-heading">
        <div><h2>Activity details</h2><p>Review exact totals by video or member.</p></div>
        <div className="analytics-detail-tabs" role="tablist" aria-label="Analytics details">
          <button type="button" role="tab" aria-selected={detailView === 'videos'} onClick={() => setDetailView('videos')}>Videos <span>{analytics.content.length}</span></button>
          <button type="button" role="tab" aria-selected={detailView === 'members'} onClick={() => setDetailView('members')}>Members <span>{analytics.members.length}</span></button>
        </div>
      </div>
      {detailView === 'videos'
        ? <VideoDetails analytics={analytics} />
        : <MemberDetails analytics={analytics} showOrganization={showOrganization} />}
    </section>
  )
}

function VideoDetails({ analytics }: { analytics: Analytics }) {
  if (!analytics.content.length) return <EmptyState title="No videos to show" body="Ready videos and their study totals will appear here." />
  return (
    <div className="responsive-table surface analytics-table">
      <table>
        <thead><tr><th>Video</th><th>Viewers</th><th>Starts</th><th>Resumes</th><th>Completed</th><th>Notes</th></tr></thead>
        <tbody>{analytics.content.map((item) => (
          <tr key={item.video_id}>
            <td><strong><TruncatedText text={item.title} /></strong><small><TruncatedText text={item.instructor_name} /></small></td>
            <td>{item.unique_viewers}</td><td>{item.starts}</td><td>{item.resumes}</td><td>{item.completions}</td><td>{item.notes}</td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  )
}

function MemberDetails({ analytics, showOrganization }: { analytics: Analytics; showOrganization: boolean }) {
  if (!analytics.members.length) return <EmptyState title="No members to show" body="Enabled gym members will appear here." />
  return (
    <div className="responsive-table surface analytics-table">
      <table>
        <thead><tr><th>Member</th>{showOrganization && <th>Gym</th>}<th>Last active</th><th>Videos studied</th><th>Notes</th></tr></thead>
        <tbody>{analytics.members.map((item) => (
          <tr key={item.user_id}>
            <td><strong><TruncatedText text={item.email} /></strong></td>
            {showOrganization && <td><TruncatedText text={item.organization_name} /></td>}
            <td>{item.last_active_at ? formatActivityDate(item.last_active_at) : 'No activity'}</td>
            <td>{item.videos_started}</td><td>{item.notes}</td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  )
}

function formatActivityDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(new Date(value))
}

function ChartEmpty({ body }: { body: string }) {
  return <div className="chart-empty"><span aria-hidden="true" /><p>{body}</p></div>
}

function AnalyticsLoading() {
  return (
    <div className="analytics-loading" role="status" aria-label="Loading analytics">
      <div className="analytics-overview"><span /><span /></div>
      <div className="analytics-visual-grid"><span /><span /></div>
    </div>
  )
}
