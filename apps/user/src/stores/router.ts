import { atom } from 'nanostores'

export type Route = 'site' | 'team' | 'settings' | 'billing' | 'graph' | 'connect'

const pathToRoute: Record<string, Route> = {
  '/': 'site',
  '/team': 'team',
  '/settings': 'settings',
  '/billing': 'billing',
  '/graph': 'graph',
  '/connect': 'connect',
}

const routeToPath: Record<Route, string> = {
  site: '/',
  team: '/team',
  settings: '/settings',
  billing: '/billing',
  graph: '/graph',
  connect: '/connect',
}

function getRouteFromPath(): Route {
  return pathToRoute[window.location.pathname] ?? 'site'
}

export const $route = atom<Route>(getRouteFromPath())

export function navigate(route: Route) {
  $route.set(route)
  window.history.pushState(null, '', routeToPath[route])
}

// Handle browser back/forward
window.addEventListener('popstate', () => {
  $route.set(getRouteFromPath())
})
