import { atom } from 'nanostores'
import { VectorStore, hashContent, type StoredVector } from './store'
import { findModel, DEFAULT_MODEL_ID } from './models'
import { $embeddingPrefs } from './settings'

export interface EmbeddingStatus {
  state: 'idle' | 'loading' | 'ready' | 'error'
  message?: string
  loaded?: number
  total?: number
  pending: number
  total_pages: number
  embedded: number
}

export const $embeddingStatus = atom<EmbeddingStatus>({
  state: 'idle',
  pending: 0,
  total_pages: 0,
  embedded: 0,
})

export const $vectorsTick = atom(0)

interface PageInfo {
  id: string
  text: string
}

class EmbeddingController {
  private worker: Worker | null = null
  private store: VectorStore | null = null
  private tenantId: string | null = null
  private modelId: string | null = null
  private pendingJobs = new Map<string, { pageId: string; contentHash: string; resolve: (v: StoredVector | null) => void }>()
  private queue: PageInfo[] = []
  private inFlight = false
  private nextJobId = 1

  async start(tenantId: string, modelId: string) {
    if (this.worker && this.tenantId === tenantId && this.modelId === modelId) return
    await this.stop()
    this.tenantId = tenantId
    this.modelId = modelId
    this.store = new VectorStore(tenantId)

    const w = new Worker(new URL('./worker.ts', import.meta.url), { type: 'module' })
    this.worker = w
    w.addEventListener('message', e => this.onMessage(e.data))

    $embeddingStatus.set({ ...$embeddingStatus.get(), state: 'loading', message: 'Loading model…', loaded: 0, total: 0 })
    w.postMessage({ type: 'init', modelId })
  }

  async stop() {
    if (this.worker) {
      this.worker.terminate()
      this.worker = null
    }
    this.queue = []
    this.pendingJobs.clear()
    this.inFlight = false
    $embeddingStatus.set({ state: 'idle', pending: 0, total_pages: 0, embedded: 0 })
  }

  async clear() {
    if (this.store) await this.store.clear()
    $vectorsTick.set($vectorsTick.get() + 1)
    this.refreshCounts()
  }

  async refreshCounts() {
    if (!this.store || !this.modelId) return
    const list = await this.store.listForModel(this.modelId)
    const cur = $embeddingStatus.get()
    $embeddingStatus.set({ ...cur, embedded: list.length })
  }

  async getVectors(): Promise<StoredVector[]> {
    if (!this.store || !this.modelId) return []
    return this.store.listForModel(this.modelId)
  }

  async syncPages(pages: PageInfo[]) {
    if (!this.store || !this.modelId) return
    const cur = $embeddingStatus.get()
    $embeddingStatus.set({ ...cur, total_pages: pages.length })

    const livePageIds = new Set(pages.map(p => p.id))
    const existing = await this.store.listForModel(this.modelId)
    for (const v of existing) {
      if (!livePageIds.has(v.pageId)) {
        await this.store.delete(v.pageId)
      }
    }

    const toEmbed: PageInfo[] = []
    for (const page of pages) {
      const hash = await hashContent(page.text)
      const existing = await this.store.get(page.id)
      if (existing && existing.modelId === this.modelId && existing.contentHash === hash) continue
      toEmbed.push(page)
    }
    this.enqueue(toEmbed)
    this.refreshCounts()
  }

  async embedPage(page: PageInfo) {
    if (!this.store || !this.modelId) return
    const hash = await hashContent(page.text)
    const existing = await this.store.get(page.id)
    if (existing && existing.modelId === this.modelId && existing.contentHash === hash) return
    this.enqueue([page])
  }

  async deletePage(pageId: string) {
    if (!this.store) return
    await this.store.delete(pageId)
    $vectorsTick.set($vectorsTick.get() + 1)
    this.refreshCounts()
  }

  private enqueue(pages: PageInfo[]) {
    if (pages.length === 0) return
    const queueIds = new Set(this.queue.map(p => p.id))
    for (const p of pages) {
      if (!queueIds.has(p.id)) this.queue.push(p)
    }
    const cur = $embeddingStatus.get()
    $embeddingStatus.set({ ...cur, pending: this.queue.length })
    this.drain()
  }

  private async drain() {
    if (this.inFlight) return
    if (!this.worker || !this.store || !this.modelId) return
    if ($embeddingStatus.get().state !== 'ready') return
    const next = this.queue.shift()
    if (!next) {
      this.refreshCounts()
      return
    }
    this.inFlight = true
    const cur = $embeddingStatus.get()
    $embeddingStatus.set({ ...cur, pending: this.queue.length })

    const hash = await hashContent(next.text)
    const jobId = String(this.nextJobId++)
    const modelDef = findModel(this.modelId)
    this.pendingJobs.set(jobId, {
      pageId: next.id,
      contentHash: hash,
      resolve: () => {
        this.inFlight = false
        this.drain()
      },
    })
    this.worker.postMessage({ type: 'embed', jobId, text: next.text, prefix: modelDef?.docPrefix })
  }

  private async onMessage(msg: any) {
    if (msg.type === 'ready') {
      $embeddingStatus.set({ ...$embeddingStatus.get(), state: 'ready', message: undefined, loaded: undefined, total: undefined })
      this.drain()
    } else if (msg.type === 'progress') {
      const cur = $embeddingStatus.get()
      $embeddingStatus.set({
        ...cur,
        state: 'loading',
        message: msg.file ? `${msg.status} ${msg.file}` : msg.status,
        loaded: msg.loaded,
        total: msg.total,
      })
    } else if (msg.type === 'error') {
      console.error('[embedding] worker error', msg)
      const job = msg.jobId ? this.pendingJobs.get(msg.jobId) : null
      if (job) {
        this.pendingJobs.delete(msg.jobId)
        $embeddingStatus.set({ ...$embeddingStatus.get(), state: 'error', message: msg.message })
        job.resolve(null)
      } else {
        $embeddingStatus.set({ ...$embeddingStatus.get(), state: 'error', message: msg.message })
      }
    } else if (msg.type === 'embedded') {
      const job = this.pendingJobs.get(msg.jobId)
      if (!job || !this.store || !this.modelId) return
      this.pendingJobs.delete(msg.jobId)
      const stored: StoredVector = {
        pageId: job.pageId,
        modelId: this.modelId,
        dims: msg.dims,
        vec: new Float32Array(msg.vec),
        contentHash: job.contentHash,
        updatedAt: Date.now(),
      }
      await this.store.put(stored)
      $vectorsTick.set($vectorsTick.get() + 1)
      this.refreshCounts()
      job.resolve(stored)
    }
  }
}

export const embeddingController = new EmbeddingController()

let lastEnabled = false
let lastModel = ''
$embeddingPrefs.subscribe(prefs => {
  if (prefs.enabled === lastEnabled && prefs.modelId === lastModel) return
  lastEnabled = prefs.enabled
  lastModel = prefs.modelId
  if (!prefs.enabled) {
    embeddingController.stop()
  }
})

export { DEFAULT_MODEL_ID }
