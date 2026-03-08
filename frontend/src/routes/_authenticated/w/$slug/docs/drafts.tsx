import { createFileRoute } from '@tanstack/react-router'
import { DocsDocumentList } from '@/pages/docs/DocsDocumentList'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/drafts')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <DocsDocumentList
        title="Drafts"
        description="Documents that haven't been published yet."
        filterMode="drafts"
      />
    </div>
  ),
})
