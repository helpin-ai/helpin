import { useMemo } from 'react'
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { useLocation, useNavigate } from '@tanstack/react-router'
import { Briefcase01Icon, Building03Icon, HeadphonesIcon, KanbanIcon, LinkSquare01Icon, UserCheck01Icon } from '@/lib/icons'
import { useTasks } from '@/hooks/queries/useTasks'
import { useViews } from '@/hooks/queries/useViews'
import { useCompanies, useContacts, useDeals } from '@/hooks/queries/useCRM'
import { useConversations } from '@/hooks/queries/useSupport'
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation'
import { cn } from '@/lib/utils'

type SavedViewModule = 'pm' | 'crm' | 'support'
type CRMSavedViewEntity = 'deals' | 'contacts' | 'companies'
type SavedViewEntity = 'tasks' | CRMSavedViewEntity | 'conversations'

interface DocsSavedViewOption {
  id: string
  name: string
  module: SavedViewModule
  entity: SavedViewEntity
  filters: Record<string, unknown>
}

const CRM_SAVED_VIEWS: DocsSavedViewOption[] = [
  { id: 'crm.deals.all', name: 'All deals', module: 'crm', entity: 'deals', filters: { per_page: 5 } },
  { id: 'crm.contacts.all', name: 'All contacts', module: 'crm', entity: 'contacts', filters: { per_page: 5 } },
  { id: 'crm.companies.all', name: 'All companies', module: 'crm', entity: 'companies', filters: { per_page: 5 } },
]

const SUPPORT_SAVED_VIEWS: DocsSavedViewOption[] = [
  { id: 'support.open', name: 'Open conversations', module: 'support', entity: 'conversations', filters: { status: 'open', per_page: 5 } },
  { id: 'support.unassigned', name: 'Unassigned conversations', module: 'support', entity: 'conversations', filters: { filter: 'unassigned', per_page: 5 } },
  { id: 'support.ai-active', name: 'AI active conversations', module: 'support', entity: 'conversations', filters: { filter: 'ai_active', per_page: 5 } },
]

const MODULE_LABELS: Record<SavedViewModule, string> = {
  pm: 'PM',
  crm: 'CRM',
  support: 'Support',
}

function normalizeModule(value: unknown): SavedViewModule {
  return value === 'crm' || value === 'support' ? value : 'pm'
}

function filterSummary(filters: Record<string, unknown> | undefined) {
  const entries = Object.entries(filters ?? {}).filter(([, value]) => String(value ?? '').trim() !== '')
  if (entries.length === 0) return 'No filters'
  return entries
    .slice(0, 3)
    .map(([key, value]) => `${key.replaceAll('_', ' ')}: ${value}`)
    .join(' · ') + (entries.length > 3 ? ` · +${entries.length - 3}` : '')
}

function optionHref(option: DocsSavedViewOption | undefined, workspaceSlug: string | undefined, viewId: string) {
  if (!workspaceSlug || !option) return undefined
  if (option.module === 'pm') return `/w/${workspaceSlug}/pm/my-work${viewId ? `?view=${encodeURIComponent(viewId)}` : ''}`
  if (option.module === 'support') return `/w/${workspaceSlug}/support`
  if (option.entity === 'contacts') return `/w/${workspaceSlug}/crm/contacts`
  if (option.entity === 'companies') return `/w/${workspaceSlug}/crm/companies`
  return `/w/${workspaceSlug}/crm/deals`
}

function formatMoney(value: number | undefined, currency: string | undefined) {
  if (typeof value !== 'number') return null
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: currency || 'USD', maximumFractionDigits: 0 }).format(value)
  } catch {
    return `${currency || 'USD'} ${value.toLocaleString()}`
  }
}

export function SavedViewEmbedNodeView({ node, extension, editor, updateAttributes }: NodeViewProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const workspaceId = extension.options.workspaceId as string | undefined
  const workspaceSlug = extension.options.workspaceSlug as string | undefined
  const editable = editor.isEditable
  const module = normalizeModule(node.attrs.module)
  const viewId = typeof node.attrs.viewId === 'string' ? node.attrs.viewId : ''
  const fallbackName = typeof node.attrs.viewName === 'string' && node.attrs.viewName.trim() ? node.attrs.viewName.trim() : 'Saved view'
  const { data: views = [], isLoading } = useViews(module === 'pm' ? workspaceId || '' : '')
  const options = useMemo<DocsSavedViewOption[]>(() => {
    if (module === 'crm') return CRM_SAVED_VIEWS
    if (module === 'support') return SUPPORT_SAVED_VIEWS
    return views.map((view) => ({
      id: view.id,
      name: view.name,
      module: 'pm',
      entity: 'tasks',
      filters: view.filters ?? {},
    }))
  }, [module, views])
  const selected = useMemo(() => options.find((view) => view.id === viewId), [options, viewId])
  const taskFilters = useMemo<Parameters<typeof useTasks>[1]>(
    () => module === 'pm' && selected ? { ...selected.filters, per_page: 5, archived: false } as Parameters<typeof useTasks>[1] : undefined,
    [module, selected],
  )
  const { data: taskPage, isLoading: tasksLoading } = useTasks(module === 'pm' && selected ? workspaceId || '' : '', taskFilters)
  const crmFilters = useMemo(() => selected?.module === 'crm' ? { ...selected.filters, per_page: 5 } : undefined, [selected])
  const supportFilters = useMemo(() => selected?.module === 'support' ? { ...selected.filters, per_page: 5 } : undefined, [selected])
  const { data: dealPage, isLoading: dealsLoading } = useDeals(module === 'crm' && selected?.entity === 'deals' ? workspaceId || '' : '', crmFilters as Parameters<typeof useDeals>[1])
  const { data: contactPage, isLoading: contactsLoading } = useContacts(module === 'crm' && selected?.entity === 'contacts' ? workspaceId || '' : '', crmFilters as Parameters<typeof useContacts>[1])
  const { data: companyPage, isLoading: companiesLoading } = useCompanies(module === 'crm' && selected?.entity === 'companies' ? workspaceId || '' : '', crmFilters as Parameters<typeof useCompanies>[1])
  const { data: conversationPage, isLoading: conversationsLoading } = useConversations(module === 'support' && selected ? workspaceId || '' : '', supportFilters as Parameters<typeof useConversations>[1])
  const title = selected?.name || fallbackName
  const href = optionHref(selected, workspaceSlug, viewId)
  const tasks = taskPage?.data ?? []
  const deals = dealPage?.data ?? []
  const contacts = contactPage?.data ?? []
  const companies = companyPage?.data ?? []
  const conversations = conversationPage?.data ?? []
  const previewRows = selected?.entity === 'deals'
    ? deals
    : selected?.entity === 'contacts'
      ? contacts
      : selected?.entity === 'companies'
        ? companies
        : selected?.entity === 'conversations'
          ? conversations
          : tasks
  const previewLoading = tasksLoading || dealsLoading || contactsLoading || companiesLoading || conversationsLoading
  const total = selected?.entity === 'deals'
    ? dealPage?.total ?? deals.length
    : selected?.entity === 'contacts'
      ? contactPage?.total ?? contacts.length
      : selected?.entity === 'companies'
        ? companyPage?.total ?? companies.length
        : selected?.entity === 'conversations'
          ? conversationPage?.total ?? conversations.length
          : taskPage?.total ?? tasks.length
  const noun = selected?.entity === 'deals'
    ? 'deal'
    : selected?.entity === 'contacts'
      ? 'contact'
      : selected?.entity === 'companies'
        ? 'company'
        : selected?.entity === 'conversations'
          ? 'conversation'
          : 'task'
  const Icon = selected?.entity === 'deals'
    ? Briefcase01Icon
    : selected?.entity === 'contacts'
      ? UserCheck01Icon
      : selected?.entity === 'companies'
        ? Building03Icon
        : selected?.entity === 'conversations'
          ? HeadphonesIcon
          : KanbanIcon

  return (
    <NodeViewWrapper data-saved-view-embed-wrapper>
      <section
        className={cn(
          'not-prose my-4 rounded-md border border-border bg-muted/20 px-3 py-3 text-sm',
          editable && 'cursor-grab',
        )}
        contentEditable={false}
        data-drag-handle
      >
        <div className="flex min-w-0 items-start gap-3">
          <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground ring-1 ring-border/70">
            <Icon className="h-4 w-4" />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-[11px] font-medium uppercase text-muted-foreground">{MODULE_LABELS[module]} saved view</span>
            <span className="mt-0.5 block truncate font-medium text-foreground">{title}</span>
            <span className="mt-0.5 block truncate text-xs text-muted-foreground">
              {selected ? filterSummary(selected.filters) : isLoading ? 'Loading views...' : 'Select a view to embed'}
            </span>
          </span>
          {href && selected && (
            <a href={href} className="mt-1 text-muted-foreground hover:text-foreground">
              <LinkSquare01Icon className="h-4 w-4" />
            </a>
          )}
        </div>
        {editable && (
          <div className="mt-3 grid gap-2 sm:grid-cols-[120px_1fr]">
            <select
              className="h-8 rounded-md border border-input bg-background px-2 text-xs outline-none focus:ring-2 focus:ring-ring"
              value={module}
              onChange={(event) => {
                const nextModule = normalizeModule(event.target.value)
                const nextOptions = nextModule === 'crm' ? CRM_SAVED_VIEWS : nextModule === 'support' ? SUPPORT_SAVED_VIEWS : []
                const next = nextOptions[0]
                updateAttributes({
                  module: nextModule,
                  viewId: next?.id || null,
                  viewName: next?.name || null,
                })
              }}
            >
              <option value="pm">PM</option>
              <option value="crm">CRM</option>
              <option value="support">Support</option>
            </select>
            <select
              className="h-8 rounded-md border border-input bg-background px-2 text-xs outline-none focus:ring-2 focus:ring-ring"
              value={viewId}
              onChange={(event) => {
                const next = options.find((view) => view.id === event.target.value)
                updateAttributes({ viewId: next?.id || null, viewName: next?.name || null, module })
              }}
            >
              <option value="">Choose {MODULE_LABELS[module]} view...</option>
              {options.map((view) => (
                <option key={view.id} value={view.id}>
                  {view.name}
                </option>
              ))}
            </select>
          </div>
        )}
        {selected && (
          <div className="mt-3 overflow-hidden rounded-md border border-border/70 bg-background">
            <div className="flex items-center justify-between border-b border-border/70 px-3 py-2 text-xs">
              <span className="font-medium text-foreground">Live preview</span>
              <span className="text-muted-foreground">{previewLoading ? 'Loading...' : `${total} ${total === 1 ? noun : `${noun}s`}`}</span>
            </div>
            {previewLoading ? (
              <div className="px-3 py-3 text-xs text-muted-foreground">Loading matching {noun}s...</div>
            ) : previewRows.length === 0 ? (
              <div className="px-3 py-3 text-xs text-muted-foreground">No {noun}s match this view.</div>
            ) : (
              <div className="divide-y divide-border/60">
                {selected.entity === 'tasks' && tasks.map((task) => (
                  <button
                    key={task.id}
                    type="button"
                    className="flex w-full items-center gap-3 px-3 py-2 text-left text-xs hover:bg-muted/50"
                    onClick={() => {
                      if (!workspaceSlug) return
                      openTaskRoute(navigate as never, location as never, workspaceSlug, task.id)
                    }}
                  >
                    <span className="min-w-0 flex-1 truncate font-medium text-foreground">{task.name}</span>
                    <span className="shrink-0 text-muted-foreground">{task.task_key || `#${task.display_id}`}</span>
                    {task.state_name && <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">{task.state_name}</span>}
                  </button>
                ))}
                {selected.entity === 'deals' && deals.map((deal) => (
                  <a key={deal.id} href={workspaceSlug ? `/w/${workspaceSlug}/crm/deals/${deal.id}` : undefined} className="flex w-full items-center gap-3 px-3 py-2 text-left text-xs hover:bg-muted/50">
                    <span className="min-w-0 flex-1 truncate font-medium text-foreground">{deal.name}</span>
                    {formatMoney(deal.amount, deal.currency) && <span className="shrink-0 text-muted-foreground">{formatMoney(deal.amount, deal.currency)}</span>}
                    {deal.stage?.name && <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">{deal.stage.name}</span>}
                  </a>
                ))}
                {selected.entity === 'contacts' && contacts.map((contact) => {
                  const name = [contact.first_name, contact.last_name].filter(Boolean).join(' ') || contact.email || `Contact ${contact.display_id}`
                  return (
                    <a key={contact.id} href={workspaceSlug ? `/w/${workspaceSlug}/crm/contacts/${contact.id}` : undefined} className="flex w-full items-center gap-3 px-3 py-2 text-left text-xs hover:bg-muted/50">
                      <span className="min-w-0 flex-1 truncate font-medium text-foreground">{name}</span>
                      {contact.email && <span className="min-w-0 max-w-[40%] truncate text-muted-foreground">{contact.email}</span>}
                      <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">{contact.lifecycle_stage.replaceAll('_', ' ')}</span>
                    </a>
                  )
                })}
                {selected.entity === 'companies' && companies.map((company) => (
                  <a key={company.id} href={workspaceSlug ? `/w/${workspaceSlug}/crm/companies/${company.id}` : undefined} className="flex w-full items-center gap-3 px-3 py-2 text-left text-xs hover:bg-muted/50">
                    <span className="min-w-0 flex-1 truncate font-medium text-foreground">{company.name}</span>
                    {company.domain && <span className="min-w-0 max-w-[40%] truncate text-muted-foreground">{company.domain}</span>}
                    {company.industry && <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">{company.industry}</span>}
                  </a>
                ))}
                {selected.entity === 'conversations' && conversations.map((conversation) => (
                  <a key={conversation.id} href={workspaceSlug ? `/w/${workspaceSlug}/support/${conversation.id}` : undefined} className="flex w-full items-center gap-3 px-3 py-2 text-left text-xs hover:bg-muted/50">
                    <span className="min-w-0 flex-1 truncate font-medium text-foreground">{conversation.subject || `Conversation #${conversation.display_id}`}</span>
                    {conversation.customer_email && <span className="min-w-0 max-w-[35%] truncate text-muted-foreground">{conversation.customer_email}</span>}
                    <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">{conversation.status.replaceAll('_', ' ')}</span>
                  </a>
                ))}
              </div>
            )}
          </div>
        )}
      </section>
    </NodeViewWrapper>
  )
}
