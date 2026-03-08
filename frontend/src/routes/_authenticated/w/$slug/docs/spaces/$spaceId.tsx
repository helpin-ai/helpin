import { createFileRoute } from '@tanstack/react-router'
import { DocsSpaceDetail } from '@/pages/docs/DocsSpaceDetail'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/spaces/$spaceId')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <DocsSpaceDetail />
    </div>
  ),
})
