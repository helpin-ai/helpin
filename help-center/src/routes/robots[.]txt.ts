import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/robots.txt')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        const url = new URL(request.url)
        const host = request.headers.get('x-forwarded-host') || request.headers.get('host') || url.host
        const protocol =
          request.headers.get('x-forwarded-proto') || url.protocol.replace(/:$/, '')

        const body = [
          'User-agent: *',
          'Disallow: /preview/',
          'Crawl-delay: 1',
          `Sitemap: ${protocol}://${host}/sitemap.xml`,
        ].join('\n')

        return new Response(body, {
          headers: {
            'cache-control': 'public, max-age=3600',
            'content-type': 'text/plain; charset=utf-8',
          },
        })
      },
    },
  },
})
