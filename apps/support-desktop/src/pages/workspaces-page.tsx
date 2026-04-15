import { useMemo } from 'react'
import { Link } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { workspacesService } from '@/lib/services/workspacesService'

export function WorkspacesPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['desktop-workspaces'],
    queryFn: async () => {
      const response = await workspacesService.list()
      if (response.error || !response.data) {
        throw new Error(response.error || 'Failed to load workspaces')
      }
      return response.data
    },
  })

  const workspaces = useMemo(() => data ?? [], [data])

  return (
    <div className="mx-auto flex min-h-full w-full max-w-5xl flex-col px-6 py-10">
      <div className="mb-8">
        <div className="text-xs font-semibold uppercase tracking-[0.24em] text-muted-foreground">
          Workspace Picker
        </div>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight">Choose a support workspace</h1>
        <p className="mt-2 text-sm text-muted-foreground">
          This host is ready for the support-first desktop flow. The full inbox UI will mount here once
          support logic extraction moves further.
        </p>
      </div>

      {isLoading ? <div className="text-sm text-muted-foreground">Loading workspaces...</div> : null}
      {error ? (
        <div className="text-sm text-destructive">
          {error instanceof Error ? error.message : 'Failed to load workspaces'}
        </div>
      ) : null}

      <div className="grid gap-4 md:grid-cols-2">
        {workspaces.map((workspace) => (
          <Link
            key={workspace.id}
            to="/w/$slug/support"
            params={{ slug: workspace.slug }}
            className="rounded-3xl border border-border/70 bg-card/95 p-6 shadow-lg shadow-black/5 transition hover:border-primary/40 hover:shadow-xl"
          >
            <div className="text-lg font-semibold">{workspace.name}</div>
            <div className="mt-1 text-sm text-muted-foreground">`{workspace.slug}`</div>
            <div className="mt-6 inline-flex rounded-full border border-border/70 px-3 py-1 text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">
              Open Support
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}
