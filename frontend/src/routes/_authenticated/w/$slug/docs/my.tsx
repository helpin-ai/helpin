import { createFileRoute } from '@tanstack/react-router'
import { DocsDocumentList } from '@/pages/docs/DocsDocumentList'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/my')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <DocsDocumentList
        title="My Documents"
        description="All documents you own."
        filterMode="my"
      />
    </div>
  ),
})
