import { atom } from 'nanostores'
import { api, type TeamMember } from '../api'

export const $members = atom<TeamMember[]>([])
export const $membersLoading = atom(false)

export async function loadMembers() {
  $membersLoading.set(true)
  try {
    const members = await api.listTeamMembers()
    $members.set(members)
  } catch {
    $members.set([])
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
