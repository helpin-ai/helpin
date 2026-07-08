import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { TopBar } from '@mobile/ui/top-bar'
import { Avatar } from '@mobile/ui/avatar'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'
import { workspacesService } from '@mobile/lib/services/workspaces-service'
import { getLastWorkspaceSlug, setLastWorkspaceSlug } from '@mobile/lib/prefs'
import type { Workspace } from '@mobile/lib/types'

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

export function WorkspacesScreen() {
  const navigate = useNavigate()
  // `undefined` = not loaded yet, distinct from `null` (loaded, nothing stored).
  const [storedSlug, setStoredSlug] = useState<string | null | undefined>(undefined)
  // Starts true so the list never paints while a redirect might still fire;
  // only the "you must choose" branch flips it off.
  const [awaitingRedirectDecision, setAwaitingRedirectDecision] = useState(true)

  const {
    data: workspaces,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ['workspaces'],
    queryFn: async () => {
      const { data, error } = await workspacesService.list()
      if (error || !data) throw new Error(error ?? 'Failed to load workspaces')
      return data
    },
  })

  useEffect(() => {
    let cancelled = false
    void getLastWorkspaceSlug().then((slug) => {
      if (!cancelled) setStoredSlug(slug)
    })
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!workspaces || storedSlug === undefined) return
    const redirectSlug = resolveWorkspaceRedirect(workspaces, storedSlug)
    if (redirectSlug) {
      void setLastWorkspaceSlug(redirectSlug)
      navigate({ to: '/w/$slug/support', params: { slug: redirectSlug } })
      return
    }
    setAwaitingRedirectDecision(false)
  }, [workspaces, storedSlug, navigate])

  const handleSelect = (slug: string) => {
    void setLastWorkspaceSlug(slug)
    navigate({ to: '/w/$slug/support', params: { slug } })
  }

  const showList = !isLoading && !isError && storedSlug !== undefined && !awaitingRedirectDecision

  return (
    <div className="flex min-h-dvh flex-col">
      <TopBar title="Workspaces" large={showList} />

      {!showList && !isError && (
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

      {showList && workspaces?.length === 0 && (
        <div className="flex flex-1 flex-col items-center justify-center gap-1 px-6 text-center">
          <p className="text-body">No workspaces yet</p>
          <p className="text-footnote text-muted-foreground">Ask a teammate to invite you, then pull to refresh.</p>
        </div>
      )}

      {showList && workspaces && workspaces.length > 0 && (
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
