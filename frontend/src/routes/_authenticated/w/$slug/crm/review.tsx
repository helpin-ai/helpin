import { createFileRoute } from '@tanstack/react-router'
import { ReviewFeed } from '@/pages/crm/ReviewFeed'

export const Route = createFileRoute('/_authenticated/w/$slug/crm/review')({
  component: ReviewRoute,
})

function ReviewRoute() {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <ReviewFeed />
    </div>
  )
}
