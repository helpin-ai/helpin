import { createFileRoute } from '@tanstack/react-router'
import { DocsHome } from '@/pages/docs/DocsHome'

export const Route = createFileRoute('/_authenticated/w/$slug/docs/')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <DocsHome />
    </div>
  ),
})
