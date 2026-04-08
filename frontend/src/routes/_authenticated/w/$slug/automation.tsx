import { createFileRoute, Outlet } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/automation')({
  component: () => <Outlet />,
});
