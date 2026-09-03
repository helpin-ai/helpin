import { useEffect, useMemo, useState } from 'react'
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { useLocation, useNavigate } from '@tanstack/react-router'
import {
  AlertCircleIcon,
  Building03Icon,
  CheckListIcon,
  DollarCircleIcon,
  FolderKanbanIcon,
  File01Icon,
  LinkSquare01Icon,
  Message01Icon,
  UserIcon,
} from '@/lib/icons'
import { openEpicRoute } from '@/components/pm/epic-detail/epicRouteNavigation'
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation'
import { docsService } from '@/lib/services/docsService'
import type { DocsResolvedEntityRef } from '@/lib/docsTypes'
import { cn } from '@/lib/utils'
import { entityMentionHref, entityTypeLabel, type DocsEntitySearchType } from './entitySearch'

function EntityMentionIcon({ entityType, className }: { entityType: string; className?: string }) {
  if (entityType === 'reference') return <LinkSquare01Icon className={className} />
  if (entityType === 'epic') return <FolderKanbanIcon className={className} />
  if (entityType === 'support_conversation') return <Message01Icon className={className} />
  if (entityType === 'deal') return <DollarCircleIcon className={className} />
  if (entityType === 'contact') return <UserIcon className={className} />
  if (entityType === 'company') return <Building03Icon className={className} />
  if (entityType === 'document') return <File01Icon className={className} />
  return <CheckListIcon className={className} />
}

export function EntityMentionNodeView({ node, extension }: NodeViewProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const entityType = String(node.attrs.entityType || 'task') as DocsEntitySearchType
  const entityId = String(node.attrs.entityId || '')
  const label = String(node.attrs.label || entityTypeLabel(entityType))
  const access = String(node.attrs.access || '')
  const displayId = node.attrs.displayId as string | number | null | undefined
  const storedHref = typeof node.attrs.href === 'string' ? node.attrs.href : ''
  const workspaceId = extension.options.workspaceId as string | undefined
  const workspaceSlug = useMemo(() => {
    const optionSlug = extension.options.workspaceSlug as string | undefined
    if (optionSlug) return optionSlug
    const parts = storedHref.match(/^\/w\/([^/]+)\//)
    return parts?.[1]
  }, [extension.options.workspaceSlug, storedHref])
  const [resolved, setResolved] = useState<DocsResolvedEntityRef | null>(null)

  useEffect(() => {
    let cancelled = false
    setResolved(null)
    if (!workspaceId || !entityId) return
    docsService.resolveEntityRefs(workspaceId, [{
      entity_type: entityType,
      entity_id: entityId,
      label,
      display_id: displayId,
    }]).then((res) => {
      if (cancelled) return
      setResolved(res.data?.refs?.[0] ?? {
        entity_type: entityType,
        entity_id: entityId,
        status: 'unavailable',
        access: 'unavailable',
        title: label,
        display_id: displayId,
        meta: 'Reference unavailable',
      })
    }).catch(() => {
      if (!cancelled) {
        setResolved({
          entity_type: entityType,
          entity_id: entityId,
          status: 'unavailable',
          access: 'unavailable',
          title: label,
          display_id: displayId,
          meta: 'Reference unavailable',
        })
      }
    })
    return () => {
      cancelled = true
    }
  }, [displayId, entityId, entityType, label, workspaceId])

  const openEntity = () => {
    if (!entityId) return
    if (resolved?.status === 'unavailable') return
    if (workspaceSlug && entityType === 'task') {
      openTaskRoute(navigate as never, location as never, workspaceSlug, entityId)
      return
    }
    if (workspaceSlug && entityType === 'epic') {
      openEpicRoute(navigate as never, location as never, workspaceSlug, entityId)
      return
    }
    const href = storedHref || entityMentionHref(workspaceSlug, { entityType, entityId })
    if (href) {
      navigate({ to: href as string })
    }
  }

  const redacted = access === 'redacted'
  const unavailable = redacted || resolved?.status === 'unavailable'
  const title = resolved?.title || label
  const href = storedHref || entityMentionHref(workspaceSlug, { entityType, entityId })
  const titlePrefix = redacted ? 'Reference' : entityTypeLabel(entityType)

  return (
    <NodeViewWrapper as="span" data-entity-mention-wrapper>
      <button
        type="button"
        contentEditable={false}
        className={cn(
          'mx-0.5 inline-flex max-w-[18rem] items-center gap-1 rounded bg-muted px-1.5 py-0.5 align-baseline text-[0.9em] font-medium text-foreground ring-1 ring-border/70',
          unavailable
            ? 'cursor-default text-muted-foreground ring-destructive/30'
            : 'hover:bg-accent hover:text-accent-foreground',
        )}
        title={unavailable ? `${titlePrefix}: Reference unavailable` : `${titlePrefix}: ${title}`}
        onClick={openEntity}
        disabled={unavailable || !href}
      >
        {unavailable ? (
          <AlertCircleIcon className="h-3 w-3 shrink-0 text-destructive" />
        ) : (
          <EntityMentionIcon entityType={entityType} className="h-3 w-3 shrink-0 text-muted-foreground" />
        )}
        <span className="truncate">@{title}</span>
      </button>
    </NodeViewWrapper>
  )
}
