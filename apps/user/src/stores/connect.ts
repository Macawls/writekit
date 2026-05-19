import { atom } from 'nanostores'
import { api, type ClientInfo, type LocalInfo } from '../api'

export const $info = atom<LocalInfo | null>(null)
export const $clients = atom<ClientInfo[] | null>(null)
export const $error = atom<string | null>(null)

let loaded = false

export async function loadConnect(opts?: { force?: boolean }) {
  if (!opts?.force && loaded) return
  try {
    const [i, cs] = await Promise.all([api.localInfo(), api.listClients()])
    $info.set(i)
    $clients.set(cs)
    $error.set(null)
    loaded = true
  } catch (e) {
    $error.set(e instanceof Error ? e.message : 'failed to load')
  }
}

export async function connectClient(id: string) {
  await api.connectClient(id)
  await loadConnect({ force: true })
}

export async function disconnectClient(id: string) {
  await api.disconnectClient(id)
  await loadConnect({ force: true })
}
