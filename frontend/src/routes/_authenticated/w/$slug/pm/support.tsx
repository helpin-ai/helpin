import { createFileRoute, redirect } from '@tanstack/react-router';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/support')({
  beforeLoad: ({ params }) => {
    throw redirect({ to: '/w/$slug/support', params: { slug: params.slug } });
  },
});
