import { createFileRoute } from '@tanstack/react-router'
import { SharedDocumentView } from '@/pages/docs/SharedDocumentView'

export const Route = createFileRoute('/share/$shareToken')({
  component: SharedDocumentView,
})
