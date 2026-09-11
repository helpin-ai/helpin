import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { Editor as TiptapEditor } from '@tiptap/react'
import {
  Cancel01Icon,
  Loading01Icon,
  MagicWand01Icon,
  Tick01Icon,
} from '@/lib/icons'
import { Button } from '@/components/ui/button'
import { NwdiagBlock } from '@/components/editor/NwdiagBlock'
import { MermaidBlock } from '@/components/editor/MermaidBlock'
import { cn, timeAgo } from '@/lib/utils'
import { buildMarkdownDiff } from '@/lib/markdownDiff'
import { parseHelpinReference } from '@/lib/helpinReferences'
import { automationService } from '@/lib/services/automationService'
import type { DocsBlock, DocsChangeProposal } from '@/lib/docsTypes'
import type { DocsProposalAnchor } from './ProposalAnchorExtension'
import {
  asProposalBlockNode,
  proposalNodeText,
  proposalPreviewKind,
  type ProposalBlockNode,
  type ProposalPreviewKind,
} from './proposalPreview'

/**
 * Resolves an image src that may be a private `helpin://artifacts/<id>`
 * reference into a fetchable URL, mirroring what the image nodeview does.
 */
function useResolvedImageSrc(src: string | undefined, artifactId: string | undefined, workspaceId: string | undefined) {
  const reference = parseHelpinReference(src)
  const resolvedArtifactId = artifactId || (reference?.type === 'artifacts' ? reference.id : null)
  const [resolved, setResolved] = useState<string | null>(resolvedArtifactId ? null : (src ?? null))
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let cancelled = false
    if (!resolvedArtifactId) {
      setResolved(src ?? null)
      setFailed(false)
      return () => {
        cancelled = true
      }
    }
    if (!workspaceId) {
      setResolved(null)
      setFailed(true)
      return () => {
        cancelled = true
      }
    }
    setResolved(null)
    setFailed(false)
    void automationService.getArtifactContentURL(workspaceId, resolvedArtifactId).then((response) => {
      if (cancelled) return
      if (response.error || !response.data?.url) {
        setFailed(true)
        return
      }
      setResolved(response.data.url)
    })
    return () => {
      cancelled = true
    }
  }, [resolvedArtifactId, src, workspaceId])

  return { src: resolved, failed }
}

function ImagePane({ node, workspaceId }: { node: ProposalBlockNode | null; workspaceId?: string }) {
  const attrs = node?.attrs ?? {}
  const { src, failed } = useResolvedImageSrc(
    typeof attrs.src === 'string' ? attrs.src : undefined,
    typeof attrs.artifactId === 'string' ? attrs.artifactId : undefined,
    workspaceId,
  )
  if (!node) return <p className="text-xs text-muted-foreground">No content</p>
  if (failed || !src) {
    return (
      <p className="text-xs text-muted-foreground">
        {failed ? 'Preview unavailable' : 'Loading preview…'}
      </p>
    )
  }
  return (
    <img
      src={src}
      alt={typeof attrs.alt === 'string' ? attrs.alt : ''}
      className="max-h-64 w-full rounded border border-border object-contain"
    />
  )
}

function SideBySide({
  kind,
  current,
  proposed,
  workspaceId,
}: {
  kind: Exclude<ProposalPreviewKind, 'text'>
  current: ProposalBlockNode | null
  proposed: ProposalBlockNode | null
  workspaceId?: string
}) {
  const panes: { label: string; node: ProposalBlockNode | null; tone: string }[] = [
    { label: 'Current', node: current, tone: 'border-rose-500/40' },
    { label: 'Proposed', node: proposed, tone: 'border-emerald-500/40' },
  ]
  return (
    <div className="grid gap-3 p-3 sm:grid-cols-2">
      {panes.map((pane) => (
        <div key={pane.label} className={cn('min-w-0 overflow-hidden rounded border bg-background/60', pane.tone)}>
          <div className="border-b border-border/60 px-3 py-1.5 text-xs font-medium text-muted-foreground">
            {pane.label}
          </div>
          <div className="overflow-x-auto p-3">
            {kind === 'mermaid' || kind === 'nwdiag' ? (
              proposalNodeText(pane.node).trim()
                ? kind === 'nwdiag'
                  ? <NwdiagBlock source={proposalNodeText(pane.node)} />
                  : <MermaidBlock source={proposalNodeText(pane.node)} />
                : <p className="text-xs text-muted-foreground">Empty diagram</p>
            ) : (
              <ImagePane node={pane.node} workspaceId={workspaceId} />
            )}
          </div>
        </div>
      ))}
    </div>
  )
}

function TextDiff({ baseText, proposedText }: { baseText: string; proposedText: string }) {
  const rows = useMemo(() => buildMarkdownDiff(baseText, proposedText), [baseText, proposedText])
  if (rows.length === 0) {
    return <p className="px-4 py-3 text-sm text-muted-foreground">No text changes detected.</p>
  }
  return (
    <div className="divide-y divide-border/60 font-mono text-sm leading-6">
      {rows.map((row, index) => (
        <div
          key={`${row.kind}-${index}`}
          className={cn(
            'grid grid-cols-[1.5rem_1fr] gap-2 px-3 py-1',
            row.kind === 'added' && 'bg-emerald-500/10 text-emerald-950 dark:text-emerald-100',
            row.kind === 'deleted' && 'bg-rose-500/10 text-rose-950 dark:text-rose-100',
            row.kind === 'kept' && 'text-muted-foreground',
          )}
        >
          <span className="select-none text-muted-foreground">
            {row.kind === 'added' ? '+' : row.kind === 'deleted' ? '-' : ''}
          </span>
          <span className="whitespace-pre-wrap break-words">
            {row.kind === 'kept' ? row.text : row.segments.map((segment, segmentIndex) => (
              <span
                key={segmentIndex}
                className={cn(
                  segment.changed && row.kind === 'added' && 'rounded-sm bg-emerald-500/30',
                  segment.changed && row.kind === 'deleted' && 'rounded-sm bg-rose-500/30 line-through',
                )}
              >
                {segment.text}
              </span>
            ))}
          </span>
        </div>
      ))}
    </div>
  )
}

function InlineProposalCard({
  proposal,
  block,
  canEdit,
  applying,
  discarding,
  workspaceId,
  onApply,
  onDiscard,
}: {
  proposal: DocsChangeProposal
  block: DocsBlock | undefined
  canEdit: boolean
  applying: boolean
  discarding: boolean
  workspaceId?: string
  onApply: (proposalId: string) => void
  onDiscard: (proposalId: string) => void
}) {
  const currentNode = asProposalBlockNode(block?.content)
  const proposedNode = asProposalBlockNode(proposal.content)
  const kind = proposalPreviewKind(currentNode, proposedNode)
  const busy = applying || discarding

  return (
    <div
      className="docs-proposal-card my-3 overflow-hidden rounded-lg border border-primary/40 bg-muted/20 shadow-sm"
      data-proposal-id={proposal.id}
    >
      <div className="flex flex-wrap items-start justify-between gap-2 border-b border-border/60 bg-background/70 px-3 py-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
            <MagicWand01Icon className="h-3.5 w-3.5 text-primary" />
            <span>Quill proposed an update</span>
            <span>·</span>
            <span>{timeAgo(proposal.created_at)}</span>
          </div>
          <p className="mt-1 text-sm font-medium text-foreground">{proposal.summary}</p>
        </div>
        {canEdit && (
          <div className="flex shrink-0 items-center gap-1.5">
            <Button
              variant="outline"
              size="sm"
              className="h-7 gap-1 px-2 text-xs"
              disabled={busy}
              onClick={() => onDiscard(proposal.id)}
              aria-label="Reject proposed change"
            >
              {discarding ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <Cancel01Icon className="h-3.5 w-3.5" />}
              Reject
            </Button>
            <Button
              size="sm"
              className="h-7 gap-1 px-2 text-xs"
              disabled={busy}
              onClick={() => onApply(proposal.id)}
              aria-label="Accept proposed change"
            >
              {applying ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <Tick01Icon className="h-3.5 w-3.5" />}
              Accept
            </Button>
          </div>
        )}
      </div>
      {kind === 'text' ? (
        <TextDiff
          baseText={proposal.base_markdown || block?.content_text || ''}
          proposedText={proposal.content_markdown ?? ''}
        />
      ) : (
        <SideBySide kind={kind} current={currentNode} proposed={proposedNode} workspaceId={workspaceId} />
      )}
    </div>
  )
}

/**
 * Renders each pending block-scoped proposal as a card anchored directly under
 * the block it changes. The cards are React portals into DOM nodes that
 * ProposalAnchorExtension places in the document flow as widget decorations.
 */
export function InlineProposalReview({
  editor,
  proposals,
  blocks,
  canEdit,
  applyingProposalId,
  discardingProposalId,
  workspaceId,
  onApply,
  onDiscard,
}: {
  editor: TiptapEditor | null
  proposals: DocsChangeProposal[]
  blocks: DocsBlock[]
  canEdit: boolean
  applyingProposalId: string | null
  discardingProposalId: string | null
  workspaceId?: string
  onApply: (proposalId: string) => void
  onDiscard: (proposalId: string) => void
}) {
  const containersRef = useRef<Map<string, HTMLElement>>(new Map())

  const anchored = useMemo(() => {
    const containers = containersRef.current
    const live = new Set<string>()
    const result: { proposal: DocsChangeProposal; anchor: DocsProposalAnchor }[] = []
    for (const proposal of proposals) {
      const blockId = proposal.block_id?.trim()
      if (proposal.scope !== 'block' || proposal.status !== 'pending' || !blockId) continue
      live.add(proposal.id)
      let dom = containers.get(proposal.id)
      if (!dom) {
        dom = document.createElement('div')
        dom.contentEditable = 'false'
        dom.className = 'docs-proposal-widget'
        containers.set(proposal.id, dom)
      }
      result.push({ proposal, anchor: { id: proposal.id, blockId, dom } })
    }
    for (const id of [...containers.keys()]) {
      if (!live.has(id)) containers.delete(id)
    }
    return result
  }, [proposals])

  useEffect(() => {
    if (!editor || editor.isDestroyed) return
    if (typeof editor.commands.setProposalAnchors !== 'function') return
    editor.commands.setProposalAnchors(anchored.map((item) => item.anchor))
  }, [editor, anchored])

  useEffect(() => {
    return () => {
      containersRef.current.clear()
    }
  }, [])

  if (!editor || editor.isDestroyed) return null

  const blocksById = new Map(blocks.map((block) => [block.id, block]))

  return (
    <>
      {anchored.map(({ proposal, anchor }) =>
        createPortal(
          <InlineProposalCard
            proposal={proposal}
            block={blocksById.get(anchor.blockId)}
            canEdit={canEdit}
            applying={applyingProposalId === proposal.id}
            discarding={discardingProposalId === proposal.id}
            workspaceId={workspaceId}
            onApply={onApply}
            onDiscard={onDiscard}
          />,
          anchor.dom,
          proposal.id,
        ),
      )}
    </>
  )
}
