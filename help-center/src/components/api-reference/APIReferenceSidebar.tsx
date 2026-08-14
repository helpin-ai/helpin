import { Braces, KeyRound, LayoutDashboard } from 'lucide-react'
import type {
  OpenAPISpec,
  ParsedOperation,
  SchemaLike,
} from './openapi'
import { groupOperations } from './openapi'

interface APIReferenceSidebarProps {
  spec: OpenAPISpec
  operations: ParsedOperation[]
  activeOperationId: string
  onSelectOperation: (operation: ParsedOperation) => void
}

const methodClasses: Record<ParsedOperation['method'], string> = {
  get: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
  post: 'bg-blue-500/10 text-blue-700 dark:text-blue-400',
  put: 'bg-amber-500/10 text-amber-700 dark:text-amber-400',
  patch: 'bg-orange-500/10 text-orange-700 dark:text-orange-400',
  delete: 'bg-red-500/10 text-red-700 dark:text-red-400',
  options: 'bg-violet-500/10 text-violet-700 dark:text-violet-400',
  head: 'bg-slate-500/10 text-slate-700 dark:text-slate-400',
  trace: 'bg-fuchsia-500/10 text-fuchsia-700 dark:text-fuchsia-400',
}

function scrollToAnchor(anchor: string) {
  const target = document.getElementById(anchor)
  target?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.history.replaceState(null, '', `#${anchor}`)
}

export function APIReferenceSidebar({
  spec,
  operations,
  activeOperationId,
  onSelectOperation,
}: APIReferenceSidebarProps) {
  const groups = groupOperations(spec, operations)
  const schemas = Object.entries(spec.components?.schemas ?? {}) as Array<
    [string, SchemaLike]
  >
  const hasAuthentication =
    Object.keys(spec.components?.securitySchemes ?? {}).length > 0

  return (
    <aside className="sticky top-[var(--hc-header-height)] hidden h-[calc(100vh-var(--hc-header-height))] w-[260px] shrink-0 overflow-y-auto border-r border-border/70 bg-background px-3 py-5 lg:block">
      <nav aria-label="API reference navigation" className="space-y-6">
        <div className="space-y-1">
          <p className="px-2 pb-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70">
            Reference
          </p>
          <button
            type="button"
            onClick={() => scrollToAnchor('api-overview')}
            className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] font-medium text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
          >
            <LayoutDashboard size={14} />
            Overview
          </button>
          {hasAuthentication && (
            <button
              type="button"
              onClick={() => scrollToAnchor('api-authentication')}
              className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] font-medium text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
            >
              <KeyRound size={14} />
              Authentication
            </button>
          )}
        </div>

        {groups.map((group) => (
          <div key={group.name} className="space-y-1">
            <p className="truncate px-2 pb-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70">
              {group.name}
            </p>
            {group.operations.map((operation) => {
              const active = activeOperationId === operation.id
              return (
                <button
                  key={operation.anchor}
                  type="button"
                  aria-current={active ? 'location' : undefined}
                  onClick={() => {
                    onSelectOperation(operation)
                    scrollToAnchor(operation.anchor)
                  }}
                  className={`group flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors ${
                    active
                      ? 'bg-sidebar-active text-sidebar-active-foreground'
                      : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
                  }`}
                >
                  <span
                    className={`w-10 shrink-0 rounded px-1 py-0.5 text-center font-mono text-[9px] font-bold uppercase tracking-wide ${methodClasses[operation.method]}`}
                  >
                    {operation.method}
                  </span>
                  <span className="min-w-0 truncate text-[12.5px] font-medium">
                    {operation.summary}
                  </span>
                </button>
              )
            })}
          </div>
        ))}

        {schemas.length > 0 && (
          <div className="space-y-1">
            <p className="px-2 pb-1 text-[11px] font-semibold uppercase tracking-[0.12em] text-muted-foreground/70">
              Models
            </p>
            {schemas.map(([name]) => (
              <button
                key={name}
                type="button"
                onClick={() => scrollToAnchor(`schema-${name}`)}
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-muted-foreground transition-colors hover:bg-muted/60 hover:text-foreground"
              >
                <Braces size={13} className="shrink-0" />
                <span className="truncate font-mono text-[12px]">{name}</span>
              </button>
            ))}
          </div>
        )}
      </nav>
    </aside>
  )
}
