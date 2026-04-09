import { createRouter } from '@tanstack/react-router'
import { routeTree } from '../routeTree.gen'
import { ErrorFallback } from '../components/ErrorFallback'
import { NotFound } from '../components/NotFound'

export const router = createRouter({
  routeTree,
  defaultErrorComponent: ErrorFallback,
  defaultNotFoundComponent: NotFound,
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
