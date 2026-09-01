import { createFileRoute } from '@tanstack/react-router'
import { DocsDocumentList } from '@/pages/docs/DocsDocumentList'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/my')({
  component: () => (
    <DocsDocumentList
      title="My Documents"
      description="All documents you own."
      filterMode="my"
    />
  ),
})
