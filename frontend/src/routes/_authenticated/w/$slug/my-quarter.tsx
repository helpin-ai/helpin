import { createFileRoute } from '@tanstack/react-router'
import MyQuarter from '@/pages/MyQuarter'

export const Route = createFileRoute('/_authenticated/w/$slug/my-quarter')({
  component: () => (
    <div className="h-full overflow-auto p-4 md:p-6">
      <MyQuarter />
    </div>
  ),
})
