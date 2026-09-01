import { createFileRoute } from '@tanstack/react-router'
import { DocsDocumentList } from '@/pages/docs/DocsDocumentList'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/drafts')({
  component: () => (
    <DocsDocumentList
      title="Drafts"
      description="Documents that haven't been published yet."
      filterMode="drafts"
    />
  ),
})
