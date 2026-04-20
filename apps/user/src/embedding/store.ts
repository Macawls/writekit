const DB_VERSION = 1
const STORE = 'embeddings'

export interface StoredVector {
  pageId: string
  modelId: string
  dims: number
  vec: Float32Array
  contentHash: string
  updatedAt: number
}

function dbName(tenantId: string): string {
  return `writekit:embeddings:${tenantId}`
}

function open(tenantId: string): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(dbName(tenantId), DB_VERSION)
    req.onupgradeneeded = () => {
      const db = req.result
      if (!db.objectStoreNames.contains(STORE)) {
        db.createObjectStore(STORE, { keyPath: 'pageId' })
      }
    }
    req.onsuccess = () => resolve(req.result)
    req.onerror = () => reject(req.error)
  })
}

export class VectorStore {
  private dbPromise: Promise<IDBDatabase>

  constructor(tenantId: string) {
    this.dbPromise = open(tenantId)
  }

  async get(pageId: string): Promise<StoredVector | null> {
    const db = await this.dbPromise
    return new Promise((resolve, reject) => {
      const req = db.transaction(STORE, 'readonly').objectStore(STORE).get(pageId)
      req.onsuccess = () => resolve((req.result as StoredVector) ?? null)
      req.onerror = () => reject(req.error)
    })
  }

  async put(v: StoredVector): Promise<void> {
    const db = await this.dbPromise
    return new Promise((resolve, reject) => {
      const req = db.transaction(STORE, 'readwrite').objectStore(STORE).put(v)
      req.onsuccess = () => resolve()
      req.onerror = () => reject(req.error)
    })
  }

  async delete(pageId: string): Promise<void> {
    const db = await this.dbPromise
    return new Promise((resolve, reject) => {
      const req = db.transaction(STORE, 'readwrite').objectStore(STORE).delete(pageId)
      req.onsuccess = () => resolve()
      req.onerror = () => reject(req.error)
    })
  }

  async listForModel(modelId: string): Promise<StoredVector[]> {
    const db = await this.dbPromise
    return new Promise((resolve, reject) => {
      const out: StoredVector[] = []
      const req = db.transaction(STORE, 'readonly').objectStore(STORE).openCursor()
      req.onsuccess = () => {
        const cursor = req.result
        if (cursor) {
          const v = cursor.value as StoredVector
          if (v.modelId === modelId) out.push(v)
          cursor.continue()
        } else {
          resolve(out)
        }
      }
      req.onerror = () => reject(req.error)
    })
  }

  async clear(): Promise<void> {
    const db = await this.dbPromise
    return new Promise((resolve, reject) => {
      const req = db.transaction(STORE, 'readwrite').objectStore(STORE).clear()
      req.onsuccess = () => resolve()
      req.onerror = () => reject(req.error)
    })
  }
}

export async function hashContent(text: string): Promise<string> {
  const enc = new TextEncoder().encode(text)
  const buf = await crypto.subtle.digest('SHA-256', enc)
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
}
