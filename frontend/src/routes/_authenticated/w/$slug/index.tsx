import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/_authenticated/w/$slug/')({
  beforeLoad: ({ params }) => {
    // Keep shared-chat links intact until the Ask dock consumes ask_chat.
    throw redirect({ to: '/w/$slug/pm/my-work', params, search: true })
  },
})
