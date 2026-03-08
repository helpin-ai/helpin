import { createFileRoute } from '@tanstack/react-router'
import { DocsDocumentList } from '@/pages/docs/DocsDocumentList'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/recent')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <DocsDocumentList
        title="Recent Documents"
        description="Recently updated documents across all spaces."
        filterMode="recent"
      />
    </div>
  ),
})
