import { createFileRoute, redirect } from '@tanstack/react-router'

export const Route = createFileRoute('/$spaceSlug/')({
  beforeLoad: ({ params }) => {
    throw redirect({
      statusCode: 301,
      to: '/$spaceSlug',
      params: { spaceSlug: params.spaceSlug },
    })
  },
})
