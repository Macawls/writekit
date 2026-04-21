import { atom } from 'nanostores'
import { api, type TeamInvitation, type TeamMember } from '../api'

export const $members = atom<TeamMember[]>([])
export const $invitations = atom<TeamInvitation[]>([])
export const $membersLoading = atom(false)

export async function loadMembers() {
  $membersLoading.set(true)
  try {
    const [members, invitations] = await Promise.all([
      api.listTeamMembers(),
      api.listInvitations().catch(() => []),
    ])
    $members.set(members)
    $invitations.set(invitations)
  } catch {
    $members.set([])
    $invitations.set([])
  } finally {
    $membersLoading.set(false)
  }
}

export async function inviteMember(email: string, role: string) {
  await api.inviteTeamMember(email, role)
  await loadMembers()
}

export async function removeMember(userId: string) {
  await api.removeTeamMember(userId)
  await loadMembers()
}

export async function updateRole(userId: string, role: string) {
  await api.updateTeamMemberRole(userId, role)
  await loadMembers()
}

export async function revokeInvitation(id: string) {
  await api.revokeInvitation(id)
  await loadMembers()
}

export async function resendInvitation(id: string) {
  await api.resendInvitation(id)
  await loadMembers()
}
