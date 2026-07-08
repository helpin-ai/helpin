import { useNavigate } from '@tanstack/react-router'
import { queryOptions, useQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { TopBar } from '@mobile/ui/top-bar'
import { Avatar } from '@mobile/ui/avatar'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { setLastWorkspaceSlug } from '@mobile/lib/prefs'
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

  const handleSelect = (slug: string) => {
    void setLastWorkspaceSlug(slug)
    navigate({ to: '/w/$slug/support', params: { slug } })
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <TopBar title="Workspaces" large={Boolean(workspaces)} />

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
        <div className="flex flex-1 flex-col items-center justify-center gap-1 px-6 text-center">
          <p className="text-body">No workspaces yet</p>
          <p className="text-footnote text-muted-foreground">Ask a teammate to invite you, then pull to refresh.</p>
        </div>
      )}

      {workspaces && workspaces.length > 0 && (
        <div className="flex-1 overflow-y-auto px-4 pb-[var(--safe-bottom)]">
          {workspaces.map((workspace, index) => (
            <motion.div
              key={workspace.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: Math.min(index, MAX_STAGGERED_ROWS) * 0.03, duration: 0.2, ease: 'easeOut' }}
            >
              <Pressable
                haptic="selection"
                onPress={() => handleSelect(workspace.slug)}
                className="flex w-full items-center gap-3 border-b border-border/70 py-3 text-left"
              >
                <Avatar name={workspace.name} src={workspace.logo_url} size={44} />
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
