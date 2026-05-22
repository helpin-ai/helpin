export function PendingProposalBadge({ count }: { count: number }) {
  if (count <= 0) return null

  return (
    <span className="shrink-0 rounded border border-amber-500/30 bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">
      {count} proposal{count === 1 ? '' : 's'}
    </span>
  )
}
