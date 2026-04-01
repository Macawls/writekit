import { atom } from 'nanostores'

export type Route = 'site' | 'team' | 'settings' | 'billing'

const pathToRoute: Record<string, Route> = {
  '/': 'site',
  '/team': 'team',
  '/settings': 'settings',
  '/billing': 'billing',
}

const routeToPath: Record<Route, string> = {
  site: '/',
  team: '/team',
  settings: '/settings',
  billing: '/billing',
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
