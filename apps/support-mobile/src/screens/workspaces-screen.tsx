import { useNavigate } from '@tanstack/react-router'
import { queryOptions, useQuery } from '@tanstack/react-query'
import { motion, useReducedMotion } from 'motion/react'
import { TopBar } from '@mobile/ui/top-bar'
import { WorkspaceAvatar } from '@mobile/ui/workspace-avatar'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { setLastWorkspaceSlug } from '@mobile/lib/prefs'
import { useConfirmPress } from '@mobile/lib/use-confirm-press'
import { signOut } from '@mobile/stores/auth-store'
import type { Workspace } from '@mobile/lib/types'

/**
 * Shared between this component (useQuery) and the /workspaces route's
 * beforeLoad (queryClient.ensureQueryData in src/router.tsx) so the redirect
 * decision and the picker render off the same cache entry — after a
 * successful beforeLoad the list is already cached and paints instantly.
 */
export const workspacesQueryOptions = queryOptions({
  queryKey: ['workspaces'],
  queryFn: async () => {
    const { data, error } = await workspacesService.list()
    if (error || !data) throw new Error(error ?? 'Failed to load workspaces')
    return data
  },
})

/**
 * Whether the picker should skip straight to a workspace instead of showing
 * the list: exactly one workspace available, or a previously-chosen slug
 * that still matches one of the available workspaces. Returns `null` when
 * the user has to choose.
 */
export function resolveWorkspaceRedirect(workspaces: Workspace[], storedSlug: string | null): string | null {
  if (workspaces.length === 1) return workspaces[0].slug
  if (storedSlug && workspaces.some((workspace) => workspace.slug === storedSlug)) return storedSlug
  return null
}

const MAX_STAGGERED_ROWS = 10

export function EmptyWorkspacesState({ onSignOut }: { onSignOut: () => void }) {
  const { armed, trigger } = useConfirmPress(3000)

  return (
    <div className="flex flex-1 flex-col items-center justify-center px-6 text-center">
      <p className="text-body">No workspaces yet</p>
      <p className="mt-1 max-w-64 text-footnote text-muted-foreground">
        Ask a teammate to invite you, then reopen the app.
      </p>
      <Pressable
        haptic="selection"
        onPress={() => trigger(onSignOut)}
        className="mt-6 rounded-full px-5 py-2.5 text-body font-medium text-muted-foreground active:bg-muted"
      >
        {armed ? 'Tap again to confirm' : 'Sign out'}
      </Pressable>
    </div>
  )
}

/**
 * The auto-redirect (single workspace / stored last_workspace_slug) runs in
 * the route's beforeLoad BEFORE this component ever mounts, so rendering
 * here means the user genuinely has to pick. The loading/error branches only
 * occur when beforeLoad's ensureQueryData failed (it falls through to the
 * picker rather than bricking the route) and useQuery is retrying.
 */
export function WorkspacesScreen() {
  const navigate = useNavigate()
  const { data: workspaces, isLoading, isError } = useQuery(workspacesQueryOptions)
  // The global CSS `prefers-reduced-motion` clamp (index.css) only catches
  // CSS transitions/animations — this staggered fade+rise is a JS-driven
  // `motion` value, which the clamp can't touch, so it needs its own guard
  // (Task 22 reduced-motion audit; same idiom as ScreenStack's `crossfade`).
  const reduced = useReducedMotion()

  const handleSelect = (slug: string) => {
    void setLastWorkspaceSlug(slug)
    navigate({ to: '/w/$slug/support', params: { slug } })
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <TopBar title="Workspaces" />

      {isLoading && (
        <div className="flex flex-1 items-center justify-center">
          <Spinner />
        </div>
      )}

      {isError && (
        <div className="flex flex-1 flex-col items-center justify-center gap-1 px-6 text-center">
          <p className="text-body">Couldn't load your workspaces.</p>
          <p className="text-footnote text-muted-foreground">Check your connection and try again.</p>
        </div>
      )}

      {workspaces?.length === 0 && (
        <EmptyWorkspacesState onSignOut={() => void signOut()} />
      )}

      {workspaces && workspaces.length > 0 && (
        <div className="flex-1 overflow-y-auto px-4 pb-[var(--safe-bottom)]">
          {workspaces.map((workspace, index) => (
            <motion.div
              key={workspace.id}
              initial={reduced ? false : { opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={
                reduced
                  ? { duration: 0 }
                  : { delay: Math.min(index, MAX_STAGGERED_ROWS) * 0.03, duration: 0.2, ease: 'easeOut' }
              }
            >
              <Pressable
                haptic="selection"
                onPress={() => handleSelect(workspace.slug)}
                className="flex w-full items-center gap-3 border-b border-border/70 py-3 text-left"
              >
                <WorkspaceAvatar workspace={workspace} size={44} />
                <span className="flex flex-1 flex-col">
                  <span className="text-headline">{workspace.name}</span>
                  {workspace.role && (
                    <span className="text-footnote capitalize text-muted-foreground">{workspace.role}</span>
                  )}
                </span>
              </Pressable>
            </motion.div>
          ))}
        </div>
      )}
    </div>
  )
}
