/**
 * NodeSkeleton is the loading placeholder rendered in the space
 * detail main area while collections / documents are in flight but
 * the space itself is already loaded. Shape matches the eventual
 * layout (header + card grid + table) so the page does not visibly
 * reflow on resolution.
 */
export function NodeSkeleton() {
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <div className="h-4 w-56 animate-pulse rounded bg-muted/60" />
        <div className="h-3 w-40 animate-pulse rounded bg-muted/40" />
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {[0, 1, 2, 3].map((i) => (
          <div key={i} className="h-24 animate-pulse rounded-lg bg-muted/60" />
        ))}
      </div>
      <div className="space-y-2">
        {[0, 1, 2].map((i) => (
          <div key={i} className="h-10 animate-pulse rounded-lg bg-muted/40" />
        ))}
      </div>
    </div>
  )
}
