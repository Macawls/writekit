import { atom, computed } from 'nanostores'
import { api, type User, type Site, type Subscription } from '../api'

export const $user = atom<User | null>(null)
export const $site = atom<Site | null>(null)
export const $subscription = atom<Subscription | null>(null)
export const $role = atom<string>('')
export const $loading = atom(true)
export const $error = atom<string | null>(null)

export const $isOwner = computed($role, role => role === 'owner')
export const $isLoaded = computed([$user, $loading], (user, loading) => !loading && user !== null)

export async function loadAuth() {
  $loading.set(true)
  $error.set(null)
  try {
    const me = await api.me()
    $user.set(me.user)
    $site.set(me.site ?? null)
    $subscription.set(me.subscription ?? null)
    $role.set(me.role ?? '')
  } catch (e) {
    if (e instanceof Error && e.message !== 'unauthorized') {
      $error.set(e.message)
    }
  } finally {
    $loading.set(false)
  }
}

export function logout() {
  const host = window.location.hostname.replace(/^app\./, '')
  document.cookie = 'session=; path=/; max-age=0; domain=.' + host
  window.location.href = `${window.location.protocol}//${host}`
}
