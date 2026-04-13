import { createFileRoute, lazyRouteComponent } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/w/$slug/crm/review')({
  component: ReviewRoute,
})

const ReviewFeed = lazyRouteComponent(() => import('@/pages/crm/ReviewFeed'), 'ReviewFeed')

function ReviewRoute() {
  return (
    <div className="h-full overflow-hidden">
      <ReviewFeed />
    </div>
  )
}
