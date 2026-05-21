import { useEffect, useMemo, useRef } from 'react'
import {
  ArrowLeft02Icon,
  Cancel01Icon,
  Link01Icon,
  Loading01Icon,
  MagicWand01Icon,
  Tick01Icon,
} from '@/lib/icons'
import { Button } from '@/components/ui/button'
import { cn, timeAgo } from '@/lib/utils'
import type { DocsChangeProposal } from '@/lib/docsTypes'

type ProposalDiffRow = { kind: 'kept' | 'added' | 'deleted'; text: string }

function buildSimpleMarkdownDiff(currentText: string, proposedText: string): ProposalDiffRow[] {
  const current = currentText.split(/\r?\n/).map((line) => line.trimEnd())
  const proposed = proposedText.split(/\r?\n/).map((line) => line.trimEnd())
  const max = Math.max(current.length, proposed.length)
  const rows: ProposalDiffRow[] = []
  for (let i = 0; i < max; i += 1) {
    const before = current[i] ?? ''
    const after = proposed[i] ?? ''
    if (before === after) {
      if (before) rows.push({ kind: 'kept', text: before })
      continue
    }
    if (before) rows.push({ kind: 'deleted', text: before })
    if (after) rows.push({ kind: 'added', text: after })
  }
  return rows
}

function proposalSourceHref(source: NonNullable<DocsChangeProposal['sources']>[number], workspaceSlug: string): string | null {
  if (source.url) return source.url
  if (!source.id) return null
  if (source.type === 'document') return `/w/${workspaceSlug}/docs/documents/${source.id}`
  if (source.type === 'coverage_gap') return `/w/${workspaceSlug}/support/coverage?gap=${source.id}`
  return null
}

export function ProposalReviewView({
  proposal,
  pendingProposals,
  currentText,
  canEdit,
  applying,
  discarding,
  workspaceSlug,
  onBack,
  onApply,
  onDiscard,
  onSelectProposal,
}: {
  proposal: DocsChangeProposal
  pendingProposals: DocsChangeProposal[]
  currentText: string
  canEdit: boolean
  applying: boolean
  discarding: boolean
  workspaceSlug: string
  onBack: () => void
  onApply: () => void
  onDiscard: () => void
  onSelectProposal: (proposalId: string) => void
}) {
  const titleRef = useRef<HTMLHeadingElement | null>(null)
  const rows = useMemo(
    () => buildSimpleMarkdownDiff(currentText, proposal.content_markdown ?? ''),
    [currentText, proposal.content_markdown],
  )
  const pendingIndex = pendingProposals.findIndex((item) => item.id === proposal.id)
  const isPending = proposal.status === 'pending'
  const sources = proposal.sources ?? []

  useEffect(() => {
    titleRef.current?.focus()
  }, [proposal.id])

  return (
    <div className="min-h-0 flex-1 overflow-y-auto bg-background">
      <div className="sr-only" aria-live="polite" aria-atomic="true">
        Reviewing proposal: {proposal.summary}
      </div>
      <div className="mx-auto flex w-full max-w-4xl flex-col gap-5 px-5 py-6">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border/70 pb-4">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
              <MagicWand01Icon className="h-4 w-4 text-primary" />
              <span>Quill proposed an update</span>
              <span>·</span>
              <span>{timeAgo(proposal.created_at)}</span>
            </div>
            <h1 ref={titleRef} tabIndex={-1} className="mt-2 text-xl font-semibold leading-tight text-foreground outline-none">
              {proposal.summary}
            </h1>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              <span className="rounded border border-border px-2 py-0.5 capitalize">{proposal.scope} proposal</span>
              <span className="rounded border border-border px-2 py-0.5 capitalize">{proposal.status}</span>
              {pendingProposals.length > 1 && pendingIndex >= 0 && (
                <span className="rounded border border-border px-2 py-0.5">{pendingIndex + 1} of {pendingProposals.length}</span>
              )}
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button variant="outline" size="sm" onClick={onBack}>
              <ArrowLeft02Icon className="h-4 w-4" />
              Back
            </Button>
            {canEdit && isPending && (
              <>
                <Button variant="outline" size="sm" onClick={onDiscard} disabled={applying || discarding}>
                  {discarding ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <Cancel01Icon className="h-4 w-4" />}
                  Discard
                </Button>
                <Button size="sm" onClick={onApply} disabled={applying || discarding}>
                  {applying ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <Tick01Icon className="h-4 w-4" />}
                  Apply
                </Button>
              </>
            )}
          </div>
        </div>

        {sources.length > 0 && (
          <div className="flex flex-wrap gap-2">
            {sources.map((source, index) => {
              const href = proposalSourceHref(source, workspaceSlug)
              const content = (
                <span className="inline-flex items-center gap-1.5 rounded border border-border bg-muted/30 px-2 py-1 text-xs text-muted-foreground">
                  <Link01Icon className="h-3 w-3" />
                  {source.label}
                </span>
              )
              return href ? (
                <a key={`${source.type}-${source.id ?? source.url ?? index}`} href={href} target={href.startsWith('http') ? '_blank' : undefined} rel="noreferrer">
                  {content}
                </a>
              ) : (
                <span key={`${source.type}-${source.id ?? index}`}>{content}</span>
              )
            })}
          </div>
        )}

        {pendingProposals.length > 1 && pendingIndex >= 0 && (
          <div className="flex items-center justify-between rounded border border-border bg-muted/20 px-3 py-2 text-sm">
            <span className="text-muted-foreground">Pending proposals for this document</span>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={pendingIndex <= 0}
                onClick={() => onSelectProposal(pendingProposals[pendingIndex - 1]!.id)}
              >
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={pendingIndex >= pendingProposals.length - 1}
                onClick={() => onSelectProposal(pendingProposals[pendingIndex + 1]!.id)}
              >
                Next
              </Button>
            </div>
          </div>
        )}

        <div className="overflow-hidden rounded border border-border">
          <div className="border-b border-border bg-muted/30 px-4 py-2 text-sm font-medium">Proposed changes</div>
          <div className="divide-y divide-border/70 font-mono text-sm leading-6">
            {rows.length > 0 ? rows.map((row, index) => (
              <div
                key={`${row.kind}-${index}`}
                className={cn(
                  'grid grid-cols-[2.25rem_1fr] gap-2 px-4 py-1',
                  row.kind === 'added' && 'border-l-4 border-l-emerald-500 bg-emerald-500/10 text-emerald-950 dark:text-emerald-100',
                  row.kind === 'deleted' && 'border-l-4 border-l-rose-500 bg-rose-500/10 text-rose-950 line-through dark:text-rose-100',
                  row.kind === 'kept' && 'text-muted-foreground',
                )}
              >
                <span className="select-none text-muted-foreground">
                  {row.kind === 'added' ? '+' : row.kind === 'deleted' ? '-' : ''}
                </span>
                <span className="whitespace-pre-wrap break-words">{row.text}</span>
              </div>
            )) : (
              <div className="px-4 py-6 text-sm text-muted-foreground">No text changes detected.</div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
