import { StrictMode, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { Toaster } from 'sonner'
import { configureSessionStorage, createBrowserSessionStorage } from '@helpin-ai/support-core'
import { getCurrent as getCurrentDeepLink, onOpenUrl } from '@tauri-apps/plugin-deep-link'
import { onPushTapped } from '@helpin/plugin-push'
import { setupVisibilityRefresh, startTokenRefreshTimer, stopTokenRefreshTimer } from '@mobile/lib/api'
import { isTauri } from '@mobile/lib/host'
import { queryClient } from '@mobile/lib/queryClient'
import { createTauriSessionStorage } from '@mobile/lib/session-storage'
import { router } from '@mobile/router'
import { routeColdStartUrls, routeDeepLinkUrl, routePushTap } from '@mobile/push/push-registration'
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

/**
 * Shared `navigate` callback for both tap sources below: queues the target
 * path and immediately attempts a flush (a no-op if auth bootstrap is still
 * `loading`, per {@link flushPendingTap}'s contract). One queue, not two —
 * a push tap and a deep-link open are the same "route me to a conversation
 * once we know who's signed in" problem.
 */
function queueTapNavigation(to: string) {
  pendingTapTo = to
  flushPendingTap()
}

if (isTauri()) {
  void onPushTapped((data) => {
    routePushTap(data, queueTapNavigation)
  })

  // `helpin://w/{slug}/support/{id}` opened from outside the app (OS deep
  // link, `adb`/`xcrun` device testing, a link pasted in Slack). Per the
  // plugin's docs, the callback receives an array of URLs "to be compatible
  // with the macOS API" even though in practice it's almost always one — we
  // route each through the same parser and let `routeDeepLinkUrl` no-op on
  // anything that isn't a valid `helpin://w/{slug}/support/{id}` URL.
  void onOpenUrl((urls) => {
    for (const url of urls) {
      routeDeepLinkUrl(url, queueTapNavigation)
    }
  })

  // Cold-start deep links: per the plugin's own .d.ts, `onOpenUrl` only
  // fires while the app is already running — a deep link that *launched*
  // the app must be picked up via `getCurrent()` once at startup. Feeds the
  // same auth-gated queue as the two listeners above. If the same launch
  // ever surfaced a target via both the push plugin's pending-tap drain and
  // `getCurrent()` (unlikely — a push tap is not a deep-link open), the
  // queue holds only one pending target and last write wins, which is fine.
  void getCurrentDeepLink().then((urls) => {
    routeColdStartUrls(urls, queueTapNavigation)
  })
}

// Intentionally never unsubscribed: this module-scope subscription lives for
// the app's lifetime, same as the onPushTapped registration above.
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
      {/* Offset below the notch/status bar + the app's ~52px top bar so toasts
          (e.g. "Resolved · Undo") float as a clean banner instead of rendering
          under the safe-area. `mobileOffset` is the one that applies in the
          webview's mobile viewport. */}
      <Toaster
        richColors
        position="top-center"
        offset={{ top: 'calc(env(safe-area-inset-top, 0px) + 60px)' }}
        mobileOffset={{ top: 'calc(env(safe-area-inset-top, 0px) + 60px)' }}
      />
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
