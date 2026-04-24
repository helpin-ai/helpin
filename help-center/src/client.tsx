import { StrictMode, startTransition } from 'react'
import { hydrateRoot } from 'react-dom/client'
import { Await, RouterProvider, type AnyRouter } from '@tanstack/react-router'
import { hydrate } from '@tanstack/react-router/ssr/client'
import { getRouter } from './router'

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
