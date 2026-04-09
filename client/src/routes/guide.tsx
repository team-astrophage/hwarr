import { createFileRoute } from '@tanstack/react-router'
import { GuidePage } from '../features/guide/GuidePage'

export const Route = createFileRoute('/guide')({
  component: GuidePage,
})
