import { StrictMode, startTransition } from 'react'
import { hydrateRoot } from 'react-dom/client'
import { Await, RouterProvider } from '@tanstack/react-router'
import { hydrate } from '@tanstack/router-core/ssr/client'
import { getRouter } from './router'
import type { AnyRouter } from '@tanstack/router-core'

declare global {
  interface Window {
    $_TSR?: {
      h?: () => void
    }
    __TSS_START_OPTIONS__?: {
      serializationAdapters: unknown[]
    }
  }
}

let hydrationPromise: Promise<AnyRouter> | undefined

async function hydrateHelpCenterStart(): Promise<AnyRouter> {
  const router = await getRouter()

  window.__TSS_START_OPTIONS__ = {
    serializationAdapters: [...(router.options.serializationAdapters ?? [])],
  }

  if (!router.stores.matchesId.state.length) {
    await hydrate(router)
  }

  window.$_TSR?.h?.()
  return router
}

function HelpCenterStartClient() {
  hydrationPromise ??= hydrateHelpCenterStart()

  return (
    <Await
      promise={hydrationPromise}
      children={(router) => <RouterProvider router={router} />}
    />
  )
}

startTransition(() => {
  hydrateRoot(
    document,
    <StrictMode>
      <HelpCenterStartClient />
    </StrictMode>,
  )
})
