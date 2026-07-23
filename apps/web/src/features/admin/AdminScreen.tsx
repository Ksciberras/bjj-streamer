import type { FormEvent } from 'react'
import { PageHeader, SectionHeading } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { CourseSummary, Organization, User, Video } from '../../types'
import { ManageVideos } from '../videos/ManageVideos'
import { PlatformGyms } from './PlatformGyms'

type AdminScreenProps = {
  users: User[]
  videos: Video[]
  courses: CourseSummary[]
  organizations: Organization[]
  platformOwner: boolean
  onRefreshUsers: () => Promise<void>
  onRefreshVideos: () => Promise<void>
  setError: (value: string) => void
  setNotice: (value: string) => void
}

export function AdminScreen({
  users,
  videos,
  courses,
  organizations,
  platformOwner,
  onRefreshUsers,
  onRefreshVideos,
  setError,
  setNotice,
}: AdminScreenProps) {
  async function createUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const form = event.currentTarget
    const data = new FormData(form)

    try {
      await api('/api/admin/users', {
        method: 'POST',
        body: JSON.stringify({
          email: data.get('email'),
          role: data.get('role'),
          password: data.get('password'),
          organization_id: platformOwner ? data.get('organization_id') : undefined,
        }),
      })
      form.reset()
      await onRefreshUsers()
      setNotice('Account created.')
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to create user'))
    }
  }

  async function updateUser(event: FormEvent<HTMLFormElement>, target: User) {
    event.preventDefault()
    const data = new FormData(event.currentTarget)

    try {
      await api(`/api/admin/users/${target.id}`, {
        method: 'PATCH',
        body: JSON.stringify({
          role: data.get('role'),
          disabled: data.get('disabled') === 'on',
        }),
      })

      const password = data.get('password')
      if (typeof password === 'string' && password) {
        await api(`/api/admin/users/${target.id}/password`, {
          method: 'POST',
          body: JSON.stringify({ password }),
        })
      }

      await onRefreshUsers()
      setNotice(`Updated ${target.email}.`)
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to update user'))
    }
  }

  async function moveUser(target: User, organizationID: string) {
    try {
      await api(`/api/admin/users/${target.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ organization_id: organizationID }),
      })
      await onRefreshUsers()
      setNotice(`Moved ${target.email} to its new gym. Their active sessions were signed out.`)
    } catch (reason) {
      await onRefreshUsers()
      setError(errorMessage(reason, 'Unable to move account'))
    }
  }

  return (
    <div className="screen">
      <PageHeader title="Admin" description="Manage member access and the video catalog." />
      <section className="surface admin-create">
        <div>
          <h2>Create account</h2>
          <p>Create private access for a known member.</p>
        </div>
        <form onSubmit={createUser}>
          <label>Email<input name="email" type="email" required /></label>
          <label>
            Role
            <select name="role" defaultValue="student">
              <option value="student">Student</option>
              <option value="instructor">Instructor</option>
              <option value="admin">Admin</option>
            </select>
          </label>
          {platformOwner && <label>Gym<select name="organization_id" required><option value="">Choose gym</option>{organizations.map((organization) => <option value={organization.id} key={organization.id}>{organization.name}</option>)}</select></label>}
          <label>
            Temporary password
            <input name="password" type="password" minLength={12} required autoComplete="new-password" />
          </label>
          <button type="submit">Create account</button>
        </form>
      </section>
      {platformOwner && <PlatformGyms organizations={organizations} videos={videos} courses={courses} setError={setError} setNotice={setNotice} />}
      <section className="section">
        <SectionHeading title="Members" />
        <div className="responsive-table surface">
          <table>
            <thead>
              <tr>
                <th>Email</th>
                <th>Role</th>
                {platformOwner && <th>Gym</th>}
                <th>Status</th>
                <th>Password reset</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {users.map((item) => (
                <tr key={item.id}>
                  <td><strong>{item.email}</strong></td>
                  <td colSpan={platformOwner ? 5 : 4}>
                    <form className="table-form" onSubmit={(event) => void updateUser(event, item)}>
                      <select name="role" defaultValue={item.role} aria-label={`Role for ${item.email}`}>
                        <option value="admin">Admin</option>
                        <option value="instructor">Instructor</option>
                        <option value="student">Student</option>
                      </select>
                      {platformOwner && !item.is_platform_owner && (
                        <select key={`${item.id}:${item.organization_id}`} defaultValue={item.organization_id} aria-label={`Gym for ${item.email}`} required onChange={(event) => void moveUser(item, event.target.value)}>
                          {organizations.map((organization) => <option value={organization.id} key={organization.id}>{organization.name}</option>)}
                        </select>
                      )}
                      <label className="check">
                        <input name="disabled" type="checkbox" defaultChecked={item.disabled} /> Disabled
                      </label>
                      <input
                        name="password"
                        type="password"
                        minLength={12}
                        placeholder="New password (optional)"
                        aria-label={`New password for ${item.email}`}
                        autoComplete="new-password"
                      />
                      <button type="submit">Save</button>
                    </form>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
      <ManageVideos videos={videos} onUpdate={onRefreshVideos} onError={setError} />
    </div>
  )
}
