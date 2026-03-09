import { createFileRoute, Outlet } from '@tanstack/react-router'
import { useSpaceNavigation } from '@/hooks/queries'
import { Sidebar } from '@/components/layout/Sidebar'
import { LoadingState } from '@/components/LoadingState'

export const Route = createFileRoute('/$spaceSlug')({
  component: SpaceLayout,
})

function SpaceLayout() {
  const { spaceSlug } = Route.useParams()
  const subdomain = Route.useRouteContext({ select: (s) => s.subdomain })
  const { data: navigation, isLoading } = useSpaceNavigation(
    subdomain,
    spaceSlug,
  )

  if (isLoading) {
    return <LoadingState message="Loading space..." />
  }

  return (
    <div className="flex">
      <Sidebar navigation={navigation ?? []} spaceSlug={spaceSlug} />
      <main className="flex-1 min-w-0">
        <Outlet />
      </main>
    </div>
  )
}
