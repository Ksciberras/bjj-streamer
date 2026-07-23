import { useEffect, useState, type FormEvent } from 'react'
import { SectionHeading, TruncatedText } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { CourseSummary, Organization, Video } from '../../types'

type Assignment = { asset_id: string; organization_id: string }
type Availability = { videos: Assignment[]; courses: Assignment[] }
type AssetKind = keyof Availability

type PlatformGymsProps = {
  mode: 'gyms' | 'access'
  organizations: Organization[]
  videos: Video[]
  courses: CourseSummary[]
  setError: (value: string) => void
  setNotice: (value: string) => void
}

export function PlatformGyms(props: PlatformGymsProps) {
  const { mode, organizations, videos, courses, setError, setNotice } = props
  const [availability, setAvailability] = useState<Availability>({ videos: [], courses: [] })

  useEffect(() => {
    if (mode !== 'access') return
    void api('/api/platform/availability')
      .then(setAvailability)
      .catch((reason) => setError(errorMessage(reason, 'Unable to load availability')))
  }, [mode, setError])

  async function createGym(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = event.currentTarget
    const data = new FormData(form)
    try {
      await api('/api/platform/organizations', {
        method: 'POST',
        body: JSON.stringify({ name: data.get('name'), slug: data.get('slug') }),
      })
      form.reset()
      setNotice('Gym created. Refresh to assign content or create its administrator.')
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to create gym'))
    }
  }

  async function toggle(kind: AssetKind, assetID: string, organizationID: string, enabled: boolean) {
    try {
      await api(`/api/platform/${kind}/${assetID}/organizations/${organizationID}`, {
        method: enabled ? 'PUT' : 'DELETE',
        body: '{}',
      })
      setAvailability((current) => ({
        ...current,
        [kind]: enabled
          ? [...current[kind], { asset_id: assetID, organization_id: organizationID }]
          : current[kind].filter((item) =>
              item.asset_id !== assetID || item.organization_id !== organizationID),
      }))
      setNotice('Gym availability updated.')
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to update availability'))
    }
  }

  function assigned(kind: AssetKind, assetID: string, organizationID: string) {
    return availability[kind].some((item) =>
      item.asset_id === assetID && item.organization_id === organizationID)
  }

  return (
    <section className="section platform-gyms">
      <SectionHeading title={mode === 'gyms' ? 'Gyms' : 'Content access'} action={<span className="admin-count">{organizations.length} gyms</span>} />
      {mode === 'gyms' ? <>
      <form className="gym-create-form surface" onSubmit={createGym}>
        <div>
          <strong>Add a gym</strong>
          <span>Create the gym first, then add its administrator above.</span>
        </div>
        <label>Gym name<input name="name" required maxLength={120} placeholder="e.g. BJJ Cork" /></label>
        <label>URL slug<input name="slug" required pattern="[a-z0-9]+(?:-[a-z0-9]+)*" placeholder="e.g. bjj-cork" /></label>
        <div className="admin-form-actions">
          <button>Create gym</button>
        </div>
      </form>
      <div className="gym-directory">
        {organizations.map((organization) => <article className="surface" key={organization.id}>
          <strong><TruncatedText text={organization.name} /></strong>
          <TruncatedText text={organization.slug} />
        </article>)}
      </div>
      </> : <>
      <div className="gym-availability-heading">
        <div>
          <h3>Content availability</h3>
          <p>Choose which videos and courses each gym can access.</p>
        </div>
      </div>
      {organizations.map((organization) => (
        <details className="surface gym-access" key={organization.id}>
          <summary>
            <span><strong><TruncatedText text={organization.name} /></strong><small><TruncatedText text={organization.slug} /></small></span>
            <span>Manage access</span>
          </summary>
          <div className="availability-grid">
            <div className="availability-list">
              <div className="availability-list-heading"><h4>Videos</h4><span>{videos.length}</span></div>
              {videos.length ? videos.map((video) => (
                <label className="check" key={video.id}>
                  <input
                    type="checkbox"
                    checked={assigned('videos', video.id, organization.id)}
                    onChange={(event) =>
                      void toggle('videos', video.id, organization.id, event.target.checked)}
                  />
                  <TruncatedText text={video.title} />
                </label>
              )) : <p>No videos uploaded.</p>}
            </div>
            <div className="availability-list">
              <div className="availability-list-heading"><h4>Courses</h4><span>{courses.length}</span></div>
              {courses.length ? courses.map((course) => (
                <label className="check" key={course.id}>
                  <input
                    type="checkbox"
                    checked={assigned('courses', course.id, organization.id)}
                    onChange={(event) =>
                      void toggle('courses', course.id, organization.id, event.target.checked)}
                  />
                  <TruncatedText text={course.title} />
                </label>
              )) : <p>No courses created.</p>}
            </div>
          </div>
        </details>
      ))}
      </>}
    </section>
  )
}
