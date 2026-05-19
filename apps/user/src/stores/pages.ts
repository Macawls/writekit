import { atom } from 'nanostores'
import { api, type PageListItem, type CollectionLight } from '../api'

export type StatusFilter = 'all' | 'published' | 'draft'
export type VisibilityFilter = 'all' | 'public' | 'unlisted' | 'private'
export type SortKey = 'recent' | 'title' | 'published' | 'created'

export interface PagesQuery {
  search: string
  status: StatusFilter
  visibility: VisibilityFilter
  collection: string[]
  tag: string[]
  sort: SortKey
  pageSize: number
  offset: number
}

export interface PagesResult {
  pages: PageListItem[]
  collections: CollectionLight[]
  tags: string[]
  total: number
}

const QUERY_KEY = 'writekit:pages:query'
const PAGE_SIZE_OPTIONS = [10, 15, 25, 50]
const DEFAULT_PAGE_SIZE = 15

const defaultQuery: PagesQuery = {
  search: '',
  status: 'all',
  visibility: 'all',
  collection: [],
  tag: [],
  sort: 'recent',
  pageSize: DEFAULT_PAGE_SIZE,
  offset: 0,
}

function loadStoredQuery(): PagesQuery {
  try {
    const raw = localStorage.getItem(QUERY_KEY)
    if (!raw) return defaultQuery
    const parsed = JSON.parse(raw) as Partial<PagesQuery>
    return {
      ...defaultQuery,
      ...parsed,
      pageSize: PAGE_SIZE_OPTIONS.includes(parsed.pageSize as number) ? (parsed.pageSize as number) : DEFAULT_PAGE_SIZE,
      offset: 0,
    }
  } catch {
    return defaultQuery
  }
}

export const $query = atom<PagesQuery>(loadStoredQuery())
export const $debouncedSearch = atom<string>(loadStoredQuery().search)
export const $result = atom<PagesResult | null>(null)
export const $loading = atom<boolean>(false)
export const $error = atom<string | null>(null)

let searchTimer: ReturnType<typeof setTimeout> | undefined
let fetchSeq = 0

function persistQuery(q: PagesQuery) {
  try {
    const { offset: _o, ...rest } = q
    localStorage.setItem(QUERY_KEY, JSON.stringify(rest))
  } catch {}
}

export function setQuery(partial: Partial<PagesQuery>) {
  const prev = $query.get()
  const filterKeys: (keyof PagesQuery)[] = ['search', 'status', 'visibility', 'collection', 'tag', 'pageSize']
  const filtersChanged = filterKeys.some(k => partial[k] !== undefined && partial[k] !== prev[k])
  const next: PagesQuery = { ...prev, ...partial }
  if (filtersChanged && partial.offset === undefined) next.offset = 0
  $query.set(next)
  persistQuery(next)

  if (partial.search !== undefined && partial.search !== prev.search) {
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => $debouncedSearch.set(next.search), 200)
  }
}

export function resetQuery() {
  if (searchTimer) clearTimeout(searchTimer)
  const next: PagesQuery = { ...defaultQuery, pageSize: $query.get().pageSize }
  $query.set(next)
  $debouncedSearch.set('')
  persistQuery(next)
}

export async function loadPages() {
  const q = $query.get()
  const search = $debouncedSearch.get()
  const seq = ++fetchSeq
  if (!$result.get()) $loading.set(true)
  $error.set(null)
  try {
    const r = await api.listPages({
      limit: q.pageSize,
      offset: q.offset,
      status: q.status,
      collection: q.collection,
      visibility: q.visibility,
      tag: q.tag,
      sort: q.sort,
      q: search,
    })
    if (seq !== fetchSeq) return
    $result.set({ pages: r.pages, collections: r.collections, tags: r.tags ?? [], total: r.total })
  } catch (e) {
    if (seq !== fetchSeq) return
    $error.set(e instanceof Error ? e.message : 'failed')
  } finally {
    if (seq === fetchSeq) $loading.set(false)
  }
}

$query.listen(() => { loadPages() })
$debouncedSearch.listen(() => { loadPages() })

let hydrated = false
export function ensurePagesLoaded() {
  if (hydrated) return
  hydrated = true
  loadPages()
}

export { PAGE_SIZE_OPTIONS }
