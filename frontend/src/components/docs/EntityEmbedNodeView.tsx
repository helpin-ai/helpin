import { useEffect, useMemo, useState } from 'react'
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { useLocation, useNavigate } from '@tanstack/react-router'
import {
  AlertCircleIcon,
  Building03Icon,
  CheckListIcon,
  DollarCircleIcon,
  FolderKanbanIcon,
  LinkSquare01Icon,
  Message01Icon,
  UserIcon,
} from '@/lib/icons'
import { openEpicRoute } from '@/components/pm/epic-detail/epicRouteNavigation'
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation'
import { openDealRoute } from '@/components/crm/deal-detail/dealRouteNavigation'
import { docsService } from '@/lib/services/docsService'
import { cn } from '@/lib/utils'

type EntityEmbedState =
  | { status: 'idle' | 'loading' }
  | { status: 'loaded'; title: string; label: string; meta: string; stateLabel?: string; href?: string }
  | { status: 'error'; title: string; label: string; reason: string }

function entityLabel(entityType: string) {
  if (entityType === 'reference') return 'Reference'
  switch (entityType) {
    case 'epic':
      return 'Epic'
    case 'support_conversation':
      return 'Support'
    case 'deal':
      return 'Deal'
    case 'contact':
      return 'Contact'
    case 'company':
      return 'Company'
    default:
      return 'Task'
  }
}

function EntityIcon({ entityType, className }: { entityType: string; className?: string }) {
  if (entityType === 'reference') return <LinkSquare01Icon className={className} />
  if (entityType === 'epic') return <FolderKanbanIcon className={className} />
  if (entityType === 'support_conversation') return <Message01Icon className={className} />
  if (entityType === 'deal') return <DollarCircleIcon className={className} />
  if (entityType === 'contact') return <UserIcon className={className} />
  if (entityType === 'company') return <Building03Icon className={className} />
  return <CheckListIcon className={className} />
}

function fallbackTitle(attrs: Record<string, unknown>) {
  return typeof attrs.title === 'string' && attrs.title.trim() ? attrs.title : 'Linked entity'
}

function taskHref(slug: string | undefined, id: string) {
  return slug ? `/w/${slug}/pm/tasks/${id}` : undefined
}

function epicHref(slug: string | undefined, id: string) {
  return slug ? `/w/${slug}/pm/epics/${id}` : undefined
}

function supportHref(slug: string | undefined, id: string) {
  return slug ? `/w/${slug}/support/${id}` : undefined
}

function crmHref(slug: string | undefined, entityType: string, id: string) {
  if (!slug) return undefined
  if (entityType === 'deal') return `/w/${slug}/crm/deals/${id}`
  if (entityType === 'contact') return `/w/${slug}/crm/contacts/${id}`
  if (entityType === 'company') return `/w/${slug}/crm/companies/${id}`
  return undefined
}

function entityHref(slug: string | undefined, entityType: string, id: string) {
  if (entityType === 'task') return taskHref(slug, id)
  if (entityType === 'epic') return epicHref(slug, id)
  if (entityType === 'support_conversation') return supportHref(slug, id)
  return crmHref(slug, entityType, id)
}

function entityErrorReason(error: unknown) {
  const message = error instanceof Error ? error.message : 'Unable to load entity'
  const normalized = message.toLowerCase()
  if (
    normalized.includes('forbidden') ||
    normalized.includes('permission') ||
    normalized.includes('unauthorized') ||
    normalized.includes('access')
  ) {
    return 'You do not have access to this entity'
  }
  return message
}

export function EntityEmbedNodeView(props: NodeViewProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const attrs = props.node.attrs as Record<string, unknown>
  const entityType = String(attrs.entityType || 'task')
  const entityId = String(attrs.entityId || '')
  const workspaceId = props.extension.options.workspaceId as string | undefined
  const workspaceSlug = props.extension.options.workspaceSlug as string | undefined
  const editable = props.editor.isEditable
  const access = String(attrs.access || '')
  const [state, setState] = useState<EntityEmbedState>({ status: 'idle' })

  const initialTitle = fallbackTitle(attrs)
  const label = entityLabel(entityType)

  useEffect(() => {
    let cancelled = false

    async function load() {
      if (access === 'redacted') {
        setState({
          status: 'loaded',
          title: initialTitle,
          label: 'Reference',
          meta: 'Hidden on public link',
        })
        return
      }

      if (!workspaceId || !entityId) {
        setState({
          status: 'error',
          title: initialTitle,
          label,
          reason: 'Missing workspace or entity reference',
        })
        return
      }

      setState({ status: 'loading' })
      try {
        const res = await docsService.resolveEntityRefs(workspaceId, [{
          entity_type: entityType,
          entity_id: entityId,
          label: initialTitle,
          display_id: typeof attrs.displayId === 'string' || typeof attrs.displayId === 'number' ? attrs.displayId : null,
        }])
        if (res.error || !res.data?.refs?.[0]) throw new Error(res.error || 'Unable to load entity')
        const data = res.data.refs[0]
        if (cancelled) return
        if (data.status === 'unavailable') {
          setState({
            status: 'error',
            title: data.title || initialTitle,
            label,
            reason: data.meta || 'Reference unavailable',
          })
          return
        }
        setState({
          status: 'loaded',
          title: data.title,
          label: entityLabel(entityType),
          meta: data.meta || entityLabel(entityType),
          stateLabel: data.state_label,
          href: entityHref(workspaceSlug, entityType, entityId),
        })
      } catch (error) {
        if (cancelled) return
        setState({
          status: 'error',
          title: initialTitle,
          label,
          reason: entityErrorReason(error),
        })
      }
    }

    void load()
    return () => {
      cancelled = true
    }
  }, [access, entityId, entityType, initialTitle, label, workspaceId, workspaceSlug])

  const content = useMemo(() => {
    if (state.status === 'loaded') {
      return {
        title: state.title,
        label: state.label,
        meta: state.meta,
        stateLabel: state.stateLabel,
        href: state.href,
        error: null as string | null,
      }
    }
    if (state.status === 'error') {
      return {
        title: state.title,
        label: state.label,
        meta: 'Reference unavailable',
        stateLabel: undefined,
        href: undefined,
        error: state.reason,
      }
    }
    return {
      title: initialTitle,
      label,
      meta: 'Loading...',
      stateLabel: undefined,
      href: undefined,
      error: null as string | null,
    }
  }, [initialTitle, label, state])

  const card = (
    <div
      className={cn(
        'my-3 flex items-start gap-3 rounded-lg border border-border/70 bg-muted/20 px-3 py-2.5 text-sm shadow-sm transition-colors',
        editable && 'cursor-grab hover:border-border hover:bg-muted/30',
        content.error && 'border-destructive/30 bg-destructive/5',
      )}
      data-drag-handle
    >
      <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground ring-1 ring-border/70">
        {content.error ? (
          <AlertCircleIcon className="h-4 w-4 text-destructive" />
        ) : (
          <EntityIcon entityType={entityType} className="h-4 w-4" />
        )}
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-[11px] font-medium uppercase text-muted-foreground">
            {content.label}
          </span>
          {content.stateLabel && (
            <span className="shrink-0 rounded bg-background px-1.5 py-0.5 text-[11px] text-muted-foreground ring-1 ring-border/60">
              {content.stateLabel}
            </span>
          )}
        </span>
        <span className="mt-0.5 block truncate font-medium text-foreground">{content.title}</span>
        <span className="mt-0.5 block truncate text-xs text-muted-foreground">{content.error || content.meta}</span>
      </span>
      {content.href && (
        <span className="mt-1 text-muted-foreground">
          <LinkSquare01Icon className="h-4 w-4" />
        </span>
      )}
    </div>
  )

  const openEntity = () => {
    if (!workspaceSlug || !entityId) return
    if (entityType === 'task') {
      openTaskRoute(navigate as never, location as never, workspaceSlug, entityId)
      return
    }
    if (entityType === 'epic') {
      openEpicRoute(navigate as never, location as never, workspaceSlug, entityId)
      return
    }
    if (entityType === 'support_conversation') {
      navigate({ to: '/w/$slug/support/$conversationId' as string, params: { slug: workspaceSlug, conversationId: entityId } })
      return
    }
    if (entityType === 'deal') {
      openDealRoute(navigate as never, location, workspaceSlug, entityId)
      return
    }
    if (entityType === 'contact') {
      navigate({ to: '/w/$slug/crm/contacts/$contactId' as string, params: { slug: workspaceSlug, contactId: entityId } })
      return
    }
    if (entityType === 'company') {
      navigate({ to: '/w/$slug/crm/companies/$companyId' as string, params: { slug: workspaceSlug, companyId: entityId } })
    }
  }

  return (
    <NodeViewWrapper data-entity-embed-wrapper>
      {content.href ? (
        <button
          type="button"
          className="not-prose block w-full p-0 text-left"
          contentEditable={false}
          onClick={openEntity}
        >
          {card}
        </button>
      ) : (
        <div className="not-prose" contentEditable={false}>
          {card}
        </div>
      )}
    </NodeViewWrapper>
  )
}
