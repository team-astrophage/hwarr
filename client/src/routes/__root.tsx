import { createRootRoute, Outlet } from '@tanstack/react-router'
import { ErrorFallback } from '../components/ErrorFallback'
import { NotFound } from '../components/NotFound'

export const Route = createRootRoute({
  component: () => <Outlet />,
  errorComponent: ErrorFallback,
  notFoundComponent: NotFound,
})
