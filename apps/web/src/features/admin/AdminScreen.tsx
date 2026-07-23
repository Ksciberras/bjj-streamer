import { useState, type FormEvent } from 'react'
import { Dialog, PageHeader, SectionHeading, TruncatedText, WorkspaceTabs } from '../../components/ui'
import { api, errorMessage } from '../../lib/api'
import type { CourseSummary, Organization, User, Video } from '../../types'
import { PlatformGyms } from './PlatformGyms'

type AdminScreenProps = {
  users: User[]
  videos: Video[]
  courses: CourseSummary[]
  organizations: Organization[]
  platformOwner: boolean
  onRefreshUsers: () => Promise<void>
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
  setError,
  setNotice,
}: AdminScreenProps) {
  const [workspace, setWorkspace] = useState<'members' | 'gyms' | 'access'>('members')
  const [editingUser, setEditingUser] = useState<User>()
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
          organization_id: platformOwner && !target.is_platform_owner ? data.get('organization_id') : undefined,
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
      setEditingUser(undefined)
    } catch (reason) {
      setError(errorMessage(reason, 'Unable to update user'))
    }
  }

  return (
    <div className="screen">
      <PageHeader title="Admin" description="Manage member access and the video catalog." />
      {platformOwner && <WorkspaceTabs
        label="Administration workspace"
        value={workspace}
        items={[
          { id: 'members', label: 'Members', count: users.length },
          { id: 'gyms', label: 'Gyms', count: organizations.length },
          { id: 'access', label: 'Content access' },
        ]}
        onChange={setWorkspace}
      />}
      {workspace === 'members' && <>
      <section className="surface admin-create">
        <div className="admin-create-copy">
          <h2>Create account</h2>
          <p>Add a known member and place them in the correct gym.</p>
        </div>
        <form className={platformOwner ? 'account-create-form platform' : 'account-create-form'} onSubmit={createUser}>
          <label>Email address<input name="email" type="email" required placeholder="member@example.com" /></label>
          <label>
            Role
            <select name="role" defaultValue="student">
              <option value="student">Student</option>
              <option value="instructor">Instructor</option>
              <option value="admin">Admin</option>
            </select>
          </label>
          {platformOwner && (
            <label>
              Gym
              <select name="organization_id" required>
                <option value="">Choose gym</option>
                {organizations.map((organization) => <option value={organization.id} key={organization.id}>{organization.name}</option>)}
              </select>
            </label>
          )}
          <label>
            Temporary password
            <input name="password" type="password" minLength={12} required autoComplete="new-password" />
          </label>
          <div className="admin-form-actions">
            <button type="submit">Create member</button>
          </div>
        </form>
      </section>
      <section className="section">
        <SectionHeading title="Members" action={<span className="admin-count">{users.length} accounts</span>} />
        <div className="responsive-table surface member-table">
          <table aria-label="Member accounts">
            <thead>
              <tr>
                <th>Email</th>
                <th>Role</th>
                {platformOwner && <th>Gym</th>}
                <th>Status</th>
                <th>Action</th>
              </tr>
            </thead>
            <tbody>
              {users.map((item) => (
                <tr key={item.id}>
                  <td>
                    <strong><TruncatedText text={item.email} /></strong>
                    {item.is_platform_owner && <small className="platform-owner-label">Platform owner</small>}
                  </td>
                  <td>
                    <span className="table-value">{item.role}</span>
                  </td>
                  {platformOwner && (
                    <td>
                      {item.is_platform_owner
                        ? <span className="platform-scope">All gyms</span>
                        : (
                          <TruncatedText className="table-value" text={organizations.find((organization) => organization.id === item.organization_id)?.name ?? 'Unassigned'} />
                        )}
                    </td>
                  )}
                  <td>
                    <span className={`status-label ${item.disabled ? 'disabled' : 'enabled'}`}>{item.disabled ? 'Disabled' : 'Enabled'}</span>
                  </td>
                  <td>
                    <button type="button" className="table-action" disabled={item.is_platform_owner} onClick={() => setEditingUser(item)}>{item.is_platform_owner ? 'Protected' : 'Edit'}</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
      </>}
      {platformOwner && workspace === 'gyms' && <PlatformGyms mode="gyms" organizations={organizations} videos={videos} courses={courses} setError={setError} setNotice={setNotice} />}
      {platformOwner && workspace === 'access' && <PlatformGyms mode="access" organizations={organizations} videos={videos} courses={courses} setError={setError} setNotice={setNotice} />}
      {editingUser && <Dialog title="Edit member" description={editingUser.email} onClose={() => setEditingUser(undefined)}>
        <form className="dialog-form" onSubmit={(event) => void updateUser(event, editingUser)}>
          <div className="dialog-field-grid">
            <label>Role<select name="role" defaultValue={editingUser.role}><option value="student">Student</option><option value="instructor">Instructor</option><option value="admin">Admin</option></select></label>
            {platformOwner && <label>Gym<select name="organization_id" defaultValue={editingUser.organization_id} required>{organizations.map((organization) => <option value={organization.id} key={organization.id}>{organization.name}</option>)}</select></label>}
            <label className="dialog-full">New password<input name="password" type="password" minLength={12} placeholder="Leave unchanged" autoComplete="new-password" /></label>
            <label className="dialog-check"><input name="disabled" type="checkbox" defaultChecked={editingUser.disabled} /><span>Disable this account</span></label>
          </div>
          <div className="dialog-actions"><button type="button" className="secondary-button" onClick={() => setEditingUser(undefined)}>Cancel</button><button type="submit">Save changes</button></div>
        </form>
      </Dialog>}
    </div>
  )
}
