import { createFileRoute } from '@tanstack/react-router'
import { MapPage } from '../features/fire-map/components/MapPage'

export const Route = createFileRoute('/map')({
  component: MapPage,
})
