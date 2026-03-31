import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/healthz')({
  server: {
    handlers: {
      GET: async () =>
        new Response('ok', {
          headers: {
            'cache-control': 'no-store',
            'content-type': 'text/plain; charset=utf-8',
          },
        }),
    },
  },
})
