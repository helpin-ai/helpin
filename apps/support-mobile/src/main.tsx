import { StrictMode, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { Toaster } from 'sonner'
import { configureSessionStorage, createBrowserSessionStorage } from '@helpin-ai/support-core'
import { onPushTapped } from '@helpin/plugin-push'
import { setupVisibilityRefresh, startTokenRefreshTimer, stopTokenRefreshTimer } from '@mobile/lib/api'
import { isTauri } from '@mobile/lib/host'
import { queryClient } from '@mobile/lib/queryClient'
import { createTauriSessionStorage } from '@mobile/lib/session-storage'
import { router } from '@mobile/router'
import { routePushTap } from '@mobile/push/push-registration'
import { bootstrapAuth, useAuthStore } from '@mobile/stores/auth-store'
import './index.css'

configureSessionStorage(isTauri() ? createTauriSessionStorage() : createBrowserSessionStorage())

/**
 * Push-tap routing, wired once at module scope (never inside a component or
 * effect) — `onPushTapped` MUST be registered exactly once for the app's
 * lifetime, since a second registration can replay a stale warm tap (see
 * the plugin's own doc comment in
 * `src-tauri/tauri-plugin-helpin-push/guest-js/index.ts`). Module top-level
 * code runs once per module load regardless of React StrictMode's
 * double-invoked effects/renders, so this is naturally a singleton.
 *
 * Cold-start taps can arrive before `bootstrapAuth()` (kicked off inside
 * `InnerApp`'s effect, below) has resolved, so the resulting navigation is
 * queued until `useAuthStore`'s `loading` flips to `false`:
 * - if a signed-in user is present at that point, the queued navigation
 *   fires.
 * - if bootstrap concludes with no session (`user === null`), the queued
 *   tap is DROPPED with a debug log rather than replayed post-login —
 *   navigating to a conversation route unauthenticated just bounces to
 *   `/login` anyway (see `requireAuth` in `router.tsx`), so queuing across a
 *   login flow would land the user somewhere unrelated to what they tapped
 *   for no real benefit.
 */
let pendingTapTo: string | null = null

function flushPendingTap() {
  if (pendingTapTo === null) return
  const { loading, user } = useAuthStore.getState()
  if (loading) return
  const to = pendingTapTo
  pendingTapTo = null
  if (!user) {
    console.debug('[push] dropping queued tap navigation: no signed-in user after bootstrap', { to })
    return
  }
  router.navigate({ to })
}

if (isTauri()) {
  void onPushTapped((data) => {
    routePushTap(data, (to) => {
      pendingTapTo = to
      flushPendingTap()
    })
  })
}

useAuthStore.subscribe((state, prevState) => {
  if (prevState.loading && !state.loading) flushPendingTap()
})

function InnerApp() {
  const user = useAuthStore((state) => state.user)
  const loading = useAuthStore((state) => state.loading)
  const serverUnreachable = useAuthStore((state) => state.serverUnreachable)

  useEffect(() => {
    void bootstrapAuth()
  }, [])

  useEffect(() => {
    if (!user) {
      stopTokenRefreshTimer()
      return
    }

    startTokenRefreshTimer()
    const cleanupVisibilityRefresh = setupVisibilityRefresh()
    return () => {
      cleanupVisibilityRefresh()
      stopTokenRefreshTimer()
    }
  }, [user])

  useEffect(() => {
    router.invalidate()
  }, [user, loading, serverUnreachable])

  return (
    <>
      <RouterProvider
        router={router}
        context={{ auth: { user, loading, serverUnreachable } }}
      />
      <Toaster richColors position="top-center" />
    </>
  )
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <InnerApp />
    </QueryClientProvider>
  )
}

const rootEl = document.getElementById('root')
if (rootEl) {
  createRoot(rootEl).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}
