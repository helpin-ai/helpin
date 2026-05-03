import { searchService, type SearchResult } from '@/lib/services/searchService'
import { supportService } from '@/lib/services/supportService'
import { crmCompanyService, crmContactService, crmDealService } from '@/lib/services/crmService'
import type { CRMCompany, CRMContact, CRMDeal } from '@/lib/crmTypes'
import type { SupportConversation } from '@/lib/pmTypes'
import type { DocsEntityEmbedType } from './EntityEmbedExtension'

export type DocsEntitySearchType = DocsEntityEmbedType | 'document'

export type DocsEntitySearchItem = {
  entityType: DocsEntitySearchType
  entityId: string
  title: string
  displayId?: string | number | null
  meta: string
  status?: string | null
  href?: string
}

export const DOCS_ENTITY_TYPE_ALIASES: Record<string, DocsEntitySearchType> = {
  task: 'task',
  tasks: 'task',
  story: 'story',
  stories: 'story',
  epic: 'epic',
  epics: 'epic',
  deal: 'deal',
  deals: 'deal',
  contact: 'contact',
  contacts: 'contact',
  company: 'company',
  companies: 'company',
  conversation: 'support_conversation',
  conversations: 'support_conversation',
  support: 'support_conversation',
  doc: 'document',
  docs: 'document',
  document: 'document',
  documents: 'document',
}

export function entityTypeLabel(type: DocsEntitySearchType) {
  if (type === 'reference') return 'Reference'
  if (type === 'epic') return 'Epic'
  if (type === 'support_conversation') return 'Conversation'
  if (type === 'deal') return 'Deal'
  if (type === 'contact') return 'Contact'
  if (type === 'company') return 'Company'
  if (type === 'story') return 'Story'
  if (type === 'document') return 'Doc'
  return 'Task'
}

export function entityTypePluralLabel(type: DocsEntitySearchType) {
  if (type === 'company') return 'companies'
  if (type === 'story') return 'stories'
  return `${entityTypeLabel(type).toLowerCase()}s`
}

export function parseEntityMentionQuery(rawQuery: string): {
  entityType?: DocsEntitySearchType
  query: string
  typed: boolean
} {
  const trimmed = rawQuery.trimStart().replace(/\s+/g, ' ')
  const [first = '', ...rest] = trimmed.split(' ')
  const entityType = DOCS_ENTITY_TYPE_ALIASES[first.toLowerCase()]
  if (!entityType) {
    return { query: trimmed, typed: false }
  }
  return {
    entityType,
    query: rest.join(' ').trim(),
    typed: true,
  }
}

export function entityMentionHref(workspaceSlug: string | undefined, item: Pick<DocsEntitySearchItem, 'entityType' | 'entityId'>) {
  if (!workspaceSlug) return undefined
  if (item.entityType === 'task' || item.entityType === 'story') return `/w/${workspaceSlug}/pm/tasks/${item.entityId}`
  if (item.entityType === 'epic') return `/w/${workspaceSlug}/pm/epics/${item.entityId}`
  if (item.entityType === 'support_conversation') return `/w/${workspaceSlug}/support/${item.entityId}`
  if (item.entityType === 'deal') return `/w/${workspaceSlug}/crm/deals/${item.entityId}`
  if (item.entityType === 'contact') return `/w/${workspaceSlug}/crm/contacts/${item.entityId}`
  if (item.entityType === 'company') return `/w/${workspaceSlug}/crm/companies/${item.entityId}`
  if (item.entityType === 'document') return `/w/${workspaceSlug}/docs/documents/${item.entityId}`
  return undefined
}

function taskSearchItem(result: SearchResult, workspaceSlug?: string): DocsEntitySearchItem {
  const item: DocsEntitySearchItem = {
    entityType: 'task',
    entityId: result.id,
    title: result.name,
    displayId: result.task_key || result.display_id,
    meta: [result.task_key || (result.display_id ? `#${result.display_id}` : null), result.team_name].filter(Boolean).join(' · ') || 'Task',
  }
  item.href = entityMentionHref(workspaceSlug, item)
  return item
}

function epicSearchItem(result: SearchResult, workspaceSlug?: string): DocsEntitySearchItem {
  const item: DocsEntitySearchItem = {
    entityType: 'epic',
    entityId: result.id,
    title: result.name,
    displayId: result.display_id,
    meta: 'Epic',
  }
  item.href = entityMentionHref(workspaceSlug, item)
  return item
}

function documentSearchItem(result: SearchResult, workspaceSlug?: string): DocsEntitySearchItem {
  const item: DocsEntitySearchItem = {
    entityType: 'document',
    entityId: result.id,
    title: result.name,
    displayId: result.display_id,
    meta: 'Doc',
  }
  item.href = entityMentionHref(workspaceSlug, item)
  return item
}

function supportSearchItem(conversation: SupportConversation, workspaceSlug?: string): DocsEntitySearchItem {
  const item: DocsEntitySearchItem = {
    entityType: 'support_conversation',
    entityId: conversation.id,
    title: conversation.subject || `Conversation #${conversation.display_id}`,
    displayId: conversation.display_id,
    status: conversation.status,
    meta: [conversation.customer_email, conversation.mailbox_name].filter(Boolean).join(' · ') || `#${conversation.display_id}`,
  }
  item.href = entityMentionHref(workspaceSlug, item)
  return item
}

function contactName(contact: CRMContact) {
  return [contact.first_name, contact.last_name].filter(Boolean).join(' ') || contact.email || `Contact ${contact.display_id}`
}

function contactSearchItem(contact: CRMContact, workspaceSlug?: string): DocsEntitySearchItem {
  const item: DocsEntitySearchItem = {
    entityType: 'contact',
    entityId: contact.id,
    title: contactName(contact),
    displayId: contact.display_id,
    status: contact.lifecycle_stage,
    meta: [contact.email, contact.job_title].filter(Boolean).join(' · ') || 'Contact',
  }
  item.href = entityMentionHref(workspaceSlug, item)
  return item
}

function companySearchItem(company: CRMCompany, workspaceSlug?: string): DocsEntitySearchItem {
  const item: DocsEntitySearchItem = {
    entityType: 'company',
    entityId: company.id,
    title: company.name,
    displayId: company.display_id,
    meta: [company.domain, company.industry].filter(Boolean).join(' · ') || 'Company',
  }
  item.href = entityMentionHref(workspaceSlug, item)
  return item
}

function formatDealAmount(deal: CRMDeal) {
  if (deal.amount == null) return null
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: deal.currency || 'USD', maximumFractionDigits: 0 }).format(deal.amount)
  } catch {
    return `${deal.amount} ${deal.currency || ''}`.trim()
  }
}

function dealSearchItem(deal: CRMDeal, workspaceSlug?: string): DocsEntitySearchItem {
  const item: DocsEntitySearchItem = {
    entityType: 'deal',
    entityId: deal.id,
    title: deal.name,
    displayId: deal.display_id,
    status: deal.stage?.name,
    meta: [formatDealAmount(deal), deal.pipeline?.name, deal.stage?.name].filter(Boolean).join(' · ') || 'Deal',
  }
  item.href = entityMentionHref(workspaceSlug, item)
  return item
}

export async function searchDocsEntityItems({
  workspaceId,
  query,
  fixedEntityType,
  workspaceSlug,
  includeDocuments = false,
  fixedLimit = 10,
  broadLimit = 4,
}: {
  workspaceId: string
  query: string
  fixedEntityType?: DocsEntitySearchType
  workspaceSlug?: string
  includeDocuments?: boolean
  fixedLimit?: number
  broadLimit?: number
}): Promise<{ items: DocsEntitySearchItem[]; error: string | null }> {
  const trimmed = query.trim()
  const next: DocsEntitySearchItem[] = []
  let error: string | null = null
  const limit = fixedEntityType ? fixedLimit : broadLimit

  if (!fixedEntityType || fixedEntityType === 'task' || fixedEntityType === 'epic' || fixedEntityType === 'story' || fixedEntityType === 'document') {
    const searchRes = await searchService.search(workspaceId, trimmed)
    if (searchRes.data) {
      if (!fixedEntityType || fixedEntityType === 'task' || fixedEntityType === 'story') {
        next.push(...(searchRes.data.tasks ?? []).slice(0, fixedEntityType ? fixedLimit : 6).map((item) => taskSearchItem(item, workspaceSlug)))
      }
      if (!fixedEntityType || fixedEntityType === 'epic') {
        next.push(...(searchRes.data.epics ?? []).slice(0, fixedEntityType ? fixedLimit : broadLimit).map((item) => epicSearchItem(item, workspaceSlug)))
      }
      if (includeDocuments && (!fixedEntityType || fixedEntityType === 'document')) {
        next.push(...(searchRes.data.documents ?? []).slice(0, fixedEntityType ? fixedLimit : broadLimit).map((item) => documentSearchItem(item, workspaceSlug)))
      }
    }
    error = error || searchRes.error
  }

  if (!fixedEntityType || fixedEntityType === 'deal') {
    const dealRes = await crmDealService.list(workspaceId, { search: trimmed, per_page: limit })
    if (dealRes.data?.data) next.push(...dealRes.data.data.slice(0, limit).map((item) => dealSearchItem(item, workspaceSlug)))
    error = error || dealRes.error
  }

  if (!fixedEntityType || fixedEntityType === 'contact') {
    const contactRes = await crmContactService.list(workspaceId, { search: trimmed, per_page: limit })
    if (contactRes.data?.data) next.push(...contactRes.data.data.slice(0, limit).map((item) => contactSearchItem(item, workspaceSlug)))
    error = error || contactRes.error
  }

  if (!fixedEntityType || fixedEntityType === 'company') {
    const companyRes = await crmCompanyService.list(workspaceId, { search: trimmed, per_page: limit })
    if (companyRes.data?.data) next.push(...companyRes.data.data.slice(0, limit).map((item) => companySearchItem(item, workspaceSlug)))
    error = error || companyRes.error
  }

  if (!fixedEntityType || fixedEntityType === 'support_conversation') {
    const supportRes = await supportService.listConversations(workspaceId, { search: trimmed, per_page: fixedEntityType ? fixedLimit : 6 })
    if (supportRes.data?.data) next.push(...supportRes.data.data.slice(0, fixedEntityType ? fixedLimit : 6).map((item) => supportSearchItem(item, workspaceSlug)))
    error = error || supportRes.error
  }

  return { items: next, error }
}
