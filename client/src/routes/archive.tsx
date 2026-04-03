import { createFileRoute } from '@tanstack/react-router'
import { ArchivePage } from '../features/archive/components/ArchivePage'

export const Route = createFileRoute('/archive')({
  component: ArchivePage,
})
