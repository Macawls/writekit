import { useState, useEffect } from 'react'
import { useStore } from '@nanostores/react'
import { $members, $invitations, $membersLoading, loadMembers, inviteMember, removeMember, updateRole, revokeInvitation, resendInvitation } from '../stores/team'
import { $user, $isOwner } from '../stores/auth'
import { confirmDialog } from '../components/ConfirmDialog'
import { Select } from '../components/Select'

export default function Team() {
  const members = useStore($members)
  const invitations = useStore($invitations)
  const loading = useStore($membersLoading)
  const user = useStore($user)
  const isOwner = useStore($isOwner)

  const [email, setEmail] = useState('')
  const [role, setRole] = useState('editor')
  const [inviting, setInviting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState<string | null>(null)

  useEffect(() => { loadMembers() }, [])

  const handleInvite = async (e: React.FormEvent) => {
    e.preventDefault()
    setError(null)
    setSuccess(null)
    setInviting(true)
    try {
      await inviteMember(email, role)
      setSuccess(`Invitation sent to ${email}`)
      setEmail('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to invite')
    } finally {
      setInviting(false)
    }
  }

  const handleRevoke = async (id: string, inviteeEmail: string) => {
    const ok = await confirmDialog({
      title: 'Revoke invitation?',
      body: <>Revoke the pending invitation for <strong>{inviteeEmail}</strong>?</>,
      confirmLabel: 'Revoke',
      destructive: true,
    })
    if (!ok) return
    try {
      await revokeInvitation(id)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to revoke invitation')
    }
  }

  const handleResend = async (id: string, inviteeEmail: string) => {
    try {
      await resendInvitation(id)
      setSuccess(`New invitation link sent to ${inviteeEmail}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to resend invitation')
    }
  }

  const handleRemove = async (userId: string, memberEmail: string) => {
    const ok = await confirmDialog({
      title: 'Remove team member?',
      body: <>Remove <strong>{memberEmail}</strong> from the team?</>,
      confirmLabel: 'Remove',
      destructive: true,
    })
    if (!ok) return
    try {
      await removeMember(userId)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove member')
    }
  }

  const handleRoleChange = async (userId: string, newRole: string) => {
    try {
      await updateRole(userId, newRole)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update role')
    }
  }

  return (
    <>
      <h2>Team</h2>
      <p className="muted" style={{ marginTop: '.25rem' }}>Manage who has access to your site.</p>

      {isOwner && (
        <div className="card" style={{ marginTop: '1.5rem' }}>
          <h3>Invite member</h3>
          <form onSubmit={handleInvite} className="invite-form">
            <div className="invite-row">
              <input
                type="email"
                placeholder="Email address"
                value={email}
                onChange={e => { setEmail(e.target.value); setError(null); setSuccess(null) }}
                required
              />
              <Select
                value={role}
                onChange={setRole}
                ariaLabel="Role"
                options={[
                  { value: 'editor', label: 'Editor' },
                  { value: 'viewer', label: 'Viewer' },
                ]}
              />
              <button type="submit" className="btn" disabled={inviting}>
                {inviting ? 'Inviting...' : 'Invite'}
              </button>
            </div>
            {error && <p className="error">{error}</p>}
            {success && <p className="success">{success}</p>}
          </form>
        </div>
      )}

      {invitations.length > 0 && (
        <div className="card" style={{ marginTop: '1rem' }}>
          <h3>Pending invitations</h3>
          <div className="member-list">
            {invitations.map(inv => (
              <div key={inv.id} className="member-row">
                <div className="member-avatar">
                  <span>{inv.email[0].toUpperCase()}</span>
                </div>
                <div className="member-info">
                  <span className="member-name">{inv.email}</span>
                  <span className="member-email">
                    Invited as {inv.role} · expires {new Date(inv.expires_at).toLocaleDateString()}
                  </span>
                </div>
                <div className="member-actions">
                  {isOwner ? (
                    <>
                      <button
                        className="btn btn-secondary"
                        onClick={() => handleResend(inv.id, inv.email)}
                        title="Send a fresh link"
                      >
                        Resend
                      </button>
                      <button
                        className="remove-btn"
                        onClick={() => handleRevoke(inv.id, inv.email)}
                        title="Revoke invitation"
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
                      </button>
                    </>
                  ) : (
                    <span className="role-badge">pending</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="card" style={{ marginTop: '1rem' }}>
        <h3>Members</h3>
        {loading ? (
          <p className="muted" style={{ marginTop: '.5rem' }}>Loading...</p>
        ) : members.length === 0 ? (
          <p className="muted" style={{ marginTop: '.5rem' }}>No team members yet.</p>
        ) : (
          <div className="member-list">
            {members.map(m => (
              <div key={m.user_id} className="member-row">
                <div className="member-avatar">
                  {m.avatar_url
                    ? <img src={m.avatar_url} alt="" />
                    : <span>{(m.name || m.email)[0].toUpperCase()}</span>
                  }
                </div>
                <div className="member-info">
                  <span className="member-name">
                    {m.name || m.email}
                    {m.user_id === user?.id && <span className="you-badge">you</span>}
                  </span>
                  <span className="member-email">{m.email}</span>
                </div>
                <div className="member-actions">
                  {isOwner && m.user_id !== user?.id ? (
                    <>
                      <Select
                        className="role-select"
                        value={m.role}
                        onChange={v => handleRoleChange(m.user_id, v)}
                        ariaLabel="Role"
                        options={[
                          { value: 'owner', label: 'Owner' },
                          { value: 'editor', label: 'Editor' },
                          { value: 'viewer', label: 'Viewer' },
                        ]}
                      />
                      <button
                        className="remove-btn"
                        onClick={() => handleRemove(m.user_id, m.email)}
                        title="Remove member"
                      >
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
                      </button>
                    </>
                  ) : (
                    <span className={`role-badge role-${m.role}`}>{m.role}</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </>
  )
}
