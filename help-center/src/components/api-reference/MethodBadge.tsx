import type { HTTPMethod } from './openapi'

const methodClasses: Record<HTTPMethod, string> = {
  get: 'bg-emerald-500/10 text-emerald-700 ring-emerald-600/15 dark:text-emerald-400',
  post: 'bg-blue-500/10 text-blue-700 ring-blue-600/15 dark:text-blue-400',
  put: 'bg-amber-500/10 text-amber-700 ring-amber-600/15 dark:text-amber-400',
  patch: 'bg-orange-500/10 text-orange-700 ring-orange-600/15 dark:text-orange-400',
  delete: 'bg-red-500/10 text-red-700 ring-red-600/15 dark:text-red-400',
  options: 'bg-violet-500/10 text-violet-700 ring-violet-600/15 dark:text-violet-400',
  head: 'bg-slate-500/10 text-slate-700 ring-slate-600/15 dark:text-slate-400',
  trace: 'bg-fuchsia-500/10 text-fuchsia-700 ring-fuchsia-600/15 dark:text-fuchsia-400',
}

interface MethodBadgeProps {
  method: HTTPMethod
  compact?: boolean
}

export function MethodBadge({ method, compact = false }: MethodBadgeProps) {
  return (
    <span
      className={`inline-flex shrink-0 items-center justify-center rounded font-mono font-bold uppercase tracking-wide ring-1 ring-inset ${methodClasses[method]} ${
        compact ? 'min-w-10 px-1 py-0.5 text-[9px]' : 'min-w-14 px-2 py-1 text-[11px]'
      }`}
    >
      {method}
    </span>
  )
}
