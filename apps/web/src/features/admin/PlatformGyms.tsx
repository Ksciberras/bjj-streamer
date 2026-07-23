import { useEffect, useState, type FormEvent } from 'react'
import { SectionHeading } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { CourseSummary, Organization, Video } from '../../types'

type Assignment = { asset_id: string; organization_id: string }
type Availability = { videos: Assignment[]; courses: Assignment[] }
type AssetKind = keyof Availability

type PlatformGymsProps = {
  organizations: Organization[]
  videos: Video[]
  courses: CourseSummary[]
  setError: (value: string) => void
  setNotice: (value: string) => void
}

export function PlatformGyms(props: PlatformGymsProps) {
  const { organizations, videos, courses, setError, setNotice } = props
  const [availability, setAvailability] = useState<Availability>({ videos: [], courses: [] })

  useEffect(() => {
    void api('/api/platform/availability')
      .then(setAvailability)
      .catch((reason) => setError(errorMessage(reason, 'Unable to load availability')))
  }, [setError])

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
      <SectionHeading title="Gyms and availability" />
      <form className="admin-create surface" onSubmit={createGym}>
        <label>Gym name<input name="name" required maxLength={120} /></label>
        <label>Slug<input name="slug" required pattern="[a-z0-9]+(?:-[a-z0-9]+)*" /></label>
        <button>Create gym</button>
      </form>
      {organizations.map((organization) => (
        <details className="surface" key={organization.id}>
          <summary>{organization.name}</summary>
          <div className="availability-grid">
            <div>
              <h3>Videos</h3>
              {videos.map((video) => (
                <label className="check" key={video.id}>
                  <input
                    type="checkbox"
                    checked={assigned('videos', video.id, organization.id)}
                    onChange={(event) =>
                      void toggle('videos', video.id, organization.id, event.target.checked)}
                  />
                  {video.title}
                </label>
              ))}
            </div>
            <div>
              <h3>Courses</h3>
              {courses.map((course) => (
                <label className="check" key={course.id}>
                  <input
                    type="checkbox"
                    checked={assigned('courses', course.id, organization.id)}
                    onChange={(event) =>
                      void toggle('courses', course.id, organization.id, event.target.checked)}
                  />
                  {course.title}
                </label>
              ))}
            </div>
          </div>
        </details>
      ))}
    </section>
  )
}
