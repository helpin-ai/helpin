import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/w/$slug/')({
  beforeLoad: ({ params }) => {
    throw redirect({ to: '/w/$slug/pm/my-work', params })
  },
})
