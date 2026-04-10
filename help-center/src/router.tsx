import { useEffect } from 'react'
import { createRouter, ErrorComponent } from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { LoadingState } from '@/components/LoadingState'
import { createHelpCenterQueryClient } from '@/lib/queryClient'
import { resolveHelpCenterContext } from '@/lib/utils'
import { routeTree } from './routeTree.gen'

function ChunkErrorHandler({ error }: { error: Error }) {
  const isChunkError =
    error.message?.includes('dynamically imported module') ||
    error.message?.includes('Failed to fetch') ||
    error.message?.includes('Loading chunk')

  useEffect(() => {
    if (isChunkError) {
      window.location.reload()
    }
  }, [isChunkError])

  if (isChunkError) {
    return <LoadingState message="Updating..." />
  }

  return <ErrorComponent error={error} />
}

interface ServerRequestSnapshot {
  hostname?: string
  pathname?: string
  basepath?: string
}

/**
 * Reads the per-request snapshot installed by `serve.mjs` via AsyncLocalStorage.
 * Returns null in dev (vite) or in any non-SSR context where the global getter
 * has not been registered.
 */
function readServerRequestSnapshot(): ServerRequestSnapshot | null {
  if (typeof window !== 'undefined') return null
  const getter = (
    globalThis as { __hcGetRequestContext__?: () => ServerRequestSnapshot | null }
  ).__hcGetRequestContext__
  if (typeof getter !== 'function') return null
  try {
    return getter() ?? null
  } catch {
    return null
  }
}

function resolveBasepath(): string | undefined {
  if (typeof window !== 'undefined') {
    const ctx = resolveHelpCenterContext(
      window.location.hostname,
      window.location.pathname,
      window.location.search,
    )
    return ctx.basepath || undefined
  }

  const snapshot = readServerRequestSnapshot()
  if (!snapshot) return undefined

  if (snapshot.basepath) return snapshot.basepath
  if (snapshot.hostname && snapshot.pathname) {
    const ctx = resolveHelpCenterContext(snapshot.hostname, snapshot.pathname)
    return ctx.basepath || undefined
  }
  return undefined
}

export function getRouter() {
  const queryClient = createHelpCenterQueryClient()
  const basepath = resolveBasepath()
  const router = createRouter({
    routeTree,
    context: { queryClient },
    basepath,
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPendingComponent: () => <LoadingState />,
    defaultPendingMinMs: 0,
    defaultErrorComponent: ChunkErrorHandler,
  })

  setupRouterSsrQueryIntegration({
    router,
    queryClient,
  })

  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
