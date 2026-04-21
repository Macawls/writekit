import { atom } from 'nanostores'
import { DEFAULT_MODEL_ID } from './models'

export interface EmbeddingPrefs {
  enabled: boolean
  modelId: string
}

const KEY = 'writekit:embedding:prefs'

function load(): EmbeddingPrefs {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { enabled: true, modelId: DEFAULT_MODEL_ID }
    const parsed = JSON.parse(raw)
    return {
      enabled: typeof parsed.enabled === 'boolean' ? parsed.enabled : true,
      modelId: typeof parsed.modelId === 'string' ? parsed.modelId : DEFAULT_MODEL_ID,
    }
  } catch {
    return { enabled: true, modelId: DEFAULT_MODEL_ID }
  }
}

export const $embeddingPrefs = atom<EmbeddingPrefs>(load())

export function setEmbeddingPrefs(next: EmbeddingPrefs) {
  $embeddingPrefs.set(next)
  try {
    localStorage.setItem(KEY, JSON.stringify(next))
  } catch {}
}
