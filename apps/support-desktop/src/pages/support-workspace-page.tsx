import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from '@tanstack/react-router'
import {
  useSupportRealtime,
  useSupportRealtimeStore,
} from '@helpin-ai/support-core'
import { SupportInboxLayout } from '@/components/support/SupportInboxLayout'
import { API_BASE } from '@/lib/api'
import { workspacesService } from '@/lib/services/workspacesService'
import { useAuthStore } from '@/stores/authStore'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useQuery } from '@tanstack/react-query'
import { getDesktopShellInfo, isTauriDesktop, type DesktopShellInfo } from '@desktop/lib/desktopHost'

function timeLabel(value?: string) {
  if (!value) {
    return 'No activity'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString([], {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

export function SupportWorkspacePage() {
  const params = useParams({ strict: false }) as { slug: string; conversationId?: string }
  const slug = params.slug
  const signOut = useAuthStore((state) => state.signOut)
  const currentWorkspace = useWorkspaceStore((state) => state.currentWorkspace)
  const setCurrentWorkspace = useWorkspaceStore((state) => state.setCurrentWorkspace)
  const [shellInfo, setShellInfo] = useState<DesktopShellInfo | null>(null)
  const realtimeStatus = useSupportRealtimeStore((state) => state.status)
  const realtimeMessage = useSupportRealtimeStore((state) => state.message)
  const lastResyncAt = useSupportRealtimeStore((state) => state.lastResyncAt)

  const workspaceQuery = useQuery({
    queryKey: ['desktop-workspace', slug],
    queryFn: async () => {
      const response = await workspacesService.getBySlug(slug)
      if (response.error || !response.data) {
        throw new Error(response.error || 'Failed to load workspace')
      }
      return response.data
    },
  })

  const workspaceId = workspaceQuery.data?.id ?? ''

  useSupportRealtime({
    apiBase: API_BASE,
    workspaceId,
    selectedConversationId: params.conversationId ?? null,
  })

  useEffect(() => {
    if (workspaceQuery.data) {
      setCurrentWorkspace(workspaceQuery.data)
    }
  }, [setCurrentWorkspace, workspaceQuery.data])

  useEffect(() => {
    let cancelled = false

    void getDesktopShellInfo().then((info) => {
      if (!cancelled) {
        setShellInfo(info)
      }
    })

    return () => {
      cancelled = true
    }
  }, [])

  const connectionTone = useMemo(() => {
    switch (realtimeStatus) {
      case 'connected':
        return 'border-emerald-200 bg-emerald-50 text-emerald-900'
      case 'offline':
      case 'auth_expired':
        return 'border-amber-200 bg-amber-50 text-amber-900'
      case 'reconnecting':
      case 'stale':
      case 'connecting':
        return 'border-sky-200 bg-sky-50 text-sky-900'
      default:
        return 'border-border/70 bg-background/80 text-muted-foreground'
    }
  }, [realtimeStatus])

  const connectionLabel = useMemo(() => {
    switch (realtimeStatus) {
      case 'connected':
        return 'Live'
      case 'connecting':
        return 'Connecting'
      case 'reconnecting':
        return 'Reconnecting'
      case 'offline':
        return 'Offline'
      case 'stale':
        return 'Refreshing'
      case 'auth_expired':
        return 'Session expired'
      default:
        return 'Disconnected'
    }
  }, [realtimeStatus])

  if (workspaceQuery.isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
      </div>
    )
  }

  if (workspaceQuery.error || !workspaceQuery.data) {
    return (
      <div className="flex min-h-screen items-center justify-center px-6">
        <div className="w-full max-w-md rounded-3xl border border-border/70 bg-card/95 p-8 text-center shadow-lg shadow-black/5">
          <h1 className="text-xl font-semibold">Workspace not found</h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {workspaceQuery.error instanceof Error ? workspaceQuery.error.message : 'Failed to load workspace.'}
          </p>
        </div>
      </div>
    )
  }

  const workspaceReady = currentWorkspace?.id === workspaceQuery.data.id

  return (
    <div className="min-h-screen px-6 py-8">
      <div className="mx-auto flex h-[calc(100vh-4rem)] w-full max-w-[1600px] flex-col gap-6">
        <header className="flex flex-wrap items-center justify-between gap-4 rounded-3xl border border-border/70 bg-card/95 px-6 py-5 shadow-lg shadow-black/5">
          <div>
            <div className="text-xs font-semibold uppercase tracking-[0.24em] text-muted-foreground">
              Helpin Support Desktop
            </div>
            <h1 className="mt-2 text-3xl font-semibold tracking-tight">
              {workspaceQuery.data.name}
            </h1>
            <p className="mt-2 text-sm text-muted-foreground">
              The desktop app now mounts the real support inbox UI from the shared web codebase.
            </p>
            <div className="mt-3 flex flex-wrap gap-2 text-xs text-muted-foreground">
              <span className="rounded-full border border-border/70 bg-background/80 px-3 py-1">
                {isTauriDesktop() ? 'Native Tauri shell' : 'Browser preview'}
              </span>
              {shellInfo ? (
                <span className="rounded-full border border-border/70 bg-background/80 px-3 py-1">
                  {shellInfo.platform} · v{shellInfo.app_version}
                </span>
              ) : null}
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <Link
              to="/workspaces"
              className="inline-flex h-10 items-center justify-center rounded-xl border border-border/70 px-4 text-sm font-medium transition hover:border-primary/40"
            >
              Switch workspace
            </Link>
            <button
              className="inline-flex h-10 items-center justify-center rounded-xl bg-primary px-4 text-sm font-medium text-primary-foreground transition hover:opacity-95"
              type="button"
              onClick={signOut}
            >
              Sign out
            </button>
          </div>
        </header>

        <section
          className={`flex flex-wrap items-center justify-between gap-3 rounded-2xl border px-4 py-3 text-sm ${connectionTone}`}
        >
          <div className="flex items-center gap-3">
            <span className="inline-flex rounded-full border border-current/15 bg-white/70 px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em]">
              {connectionLabel}
            </span>
            <span>
              {realtimeMessage ??
                (realtimeStatus === 'connected'
                  ? 'Realtime sync is healthy.'
                  : 'Support data will resync automatically when the connection recovers.')}
            </span>
          </div>
          {lastResyncAt ? (
            <div className="text-xs opacity-80">Last resync {timeLabel(new Date(lastResyncAt).toISOString())}</div>
          ) : null}
        </section>

        <div className="min-h-0 flex-1 overflow-hidden rounded-3xl border border-border/70 bg-card/95 shadow-lg shadow-black/5">
          {workspaceReady ? (
            <SupportInboxLayout />
          ) : (
            <div className="flex h-full items-center justify-center">
              <div className="h-8 w-8 animate-spin rounded-full border-b-2 border-primary" />
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
