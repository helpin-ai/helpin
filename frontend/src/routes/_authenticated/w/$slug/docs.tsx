import { createFileRoute, Outlet } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/w/$slug/docs')({
  component: () => <Outlet />,
})
