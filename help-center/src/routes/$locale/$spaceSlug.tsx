import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/$locale/$spaceSlug')({
  component: LocalizedSpaceRoute,
})

function LocalizedSpaceRoute() {
  return <Outlet />
}
