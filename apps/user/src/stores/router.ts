import { atom } from 'nanostores'

export type Route = 'site' | 'pages' | 'pageView' | 'team' | 'settings' | 'billing' | 'graph' | 'connect' | 'database'

const pathToRoute: Record<string, Route> = {
  '/': 'site',
  '/pages': 'pages',
  '/team': 'team',
  '/settings': 'settings',
  '/billing': 'billing',
  '/graph': 'graph',
  '/connect': 'connect',
  '/database': 'database',
}

const routeToPath: Record<Exclude<Route, 'pageView'>, string> = {
  site: '/',
  pages: '/pages',
  team: '/team',
  settings: '/settings',
  billing: '/billing',
  graph: '/graph',
  connect: '/connect',
  database: '/database',
}

const PAGE_VIEW_PREFIX = '/pages/view/'

function parsePath(): { route: Route; slug: string } {
  const p = window.location.pathname
  if (p.startsWith(PAGE_VIEW_PREFIX)) {
    return { route: 'pageView', slug: decodeURIComponent(p.slice(PAGE_VIEW_PREFIX.length)) }
  }
  return { route: pathToRoute[p] ?? 'site', slug: '' }
}

const initial = parsePath()
export const $route = atom<Route>(initial.route)
export const $pageSlug = atom<string>(initial.slug)

export function navigate(route: Route, slug?: string) {
  $route.set(route)
  if (route === 'pageView') {
    const s = slug ?? $pageSlug.get()
    $pageSlug.set(s)
    window.history.pushState(null, '', PAGE_VIEW_PREFIX + s.split('/').map(encodeURIComponent).join('/'))
    return
  }
  $pageSlug.set('')
  window.history.pushState(null, '', routeToPath[route as Exclude<Route, 'pageView'>])
}

window.addEventListener('popstate', () => {
  const { route, slug } = parsePath()
  $route.set(route)
  $pageSlug.set(slug)
})
