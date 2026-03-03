import { createFileRoute } from '@tanstack/react-router'
import Settings from '@/pages/Settings'

export const Route = createFileRoute('/_authenticated/w/$slug/settings')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <Settings />
    </div>
  ),
})
