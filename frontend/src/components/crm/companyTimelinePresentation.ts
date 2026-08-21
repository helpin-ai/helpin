import type { CRMCompanyTimelineItem } from '@/lib/crmTypes';

export interface CompanyTimelinePresentation {
  mode: 'compact' | 'content';
  label: string;
  emphasizedValues: string[];
  actorKind: 'human' | 'system' | 'unknown';
  attribution?: string;
  contentTitle?: string;
  contentBody?: string;
  contentFormat?: 'plain' | 'markdown';
}

function clean(value?: string) {
  return value?.replace(/\s+/g, ' ').trim() ?? '';
}

function actorName(item: CRMCompanyTimelineItem) {
  return clean(item.actor?.name);
}

function creditedActorName(item: CRMCompanyTimelineItem) {
  return clean(item.actor?.id) ? actorName(item) : '';
}

function entityName(item: CRMCompanyTimelineItem) {
  return clean(item.entity?.name) || clean(item.title);
}

function entityLabel(item: CRMCompanyTimelineItem, noun: string) {
  const displayId = clean(item.entity?.display_id);
  const name = entityName(item);
  const label = `${noun.charAt(0).toUpperCase()}${noun.slice(1)}`;
  if (displayId && name) return `${label} ${displayId} · ${name}`;
  if (displayId) return `${label} ${displayId}`;
  if (name) return `${label} · ${name}`;
  return label;
}

function dealLabel(item: CRMCompanyTimelineItem) {
  const displayId = clean(item.entity?.display_id);
  const name = entityName(item);
  if (displayId && name) return `${displayId} · ${name}`;
  if (displayId) return displayId;
  return name ? `Deal · ${name}` : 'Deal';
}

function actionWithoutEntity(description: string, noun: string, fallback: string) {
  if (!description) return fallback;
  return clean(description.replace(new RegExp(`\\bthis ${noun}\\b`, 'i'), ''));
}

function dealStageDestination(item: CRMCompanyTimelineItem) {
  if (item.kind !== 'deal' || item.event_type !== 'deal.stage_changed') return '';
  const description = clean(item.description);
  const match = description.match(/\bmoved(?: this deal)?(?: from .+)? to (.+)$/i);
  return clean(match?.[1]).toLocaleLowerCase();
}

function isDetailedDealStageChange(item: CRMCompanyTimelineItem) {
  return /\bmoved(?: this deal)? from .+ to .+$/i.test(clean(item.description));
}

function sameTimelineEntity(left: CRMCompanyTimelineItem, right: CRMCompanyTimelineItem) {
  const leftID = clean(left.entity?.id) || clean(left.entity?.display_id);
  const rightID = clean(right.entity?.id) || clean(right.entity?.display_id);
  if (leftID || rightID) return leftID !== '' && leftID === rightID;
  return clean(left.title).toLocaleLowerCase() === clean(right.title).toLocaleLowerCase();
}

/**
 * Deal stage changes can briefly arrive twice: the canonical “from X to Y”
 * audit event and a legacy “to Y” analytics backfill. Keep the richer event.
 * The tight time window prevents unrelated transitions to the same stage from
 * being collapsed.
 */
export function dedupeCompanyTimelineItems(items: CRMCompanyTimelineItem[]) {
  return items.filter((item) => {
    const destination = dealStageDestination(item);
    if (!destination || isDetailedDealStageChange(item)) return true;

    const occurredAt = Date.parse(item.occurred_at);
    if (!Number.isFinite(occurredAt)) return true;

    return !items.some((candidate) => {
      if (candidate.id === item.id || !isDetailedDealStageChange(candidate)) return false;
      if (!sameTimelineEntity(item, candidate)) return false;
      if (dealStageDestination(candidate) !== destination) return false;
      const candidateOccurredAt = Date.parse(candidate.occurred_at);
      return Number.isFinite(candidateOccurredAt) && Math.abs(candidateOccurredAt - occurredAt) <= 2_000;
    });
  });
}

function compactPresentation(item: CRMCompanyTimelineItem): CompanyTimelinePresentation {
  const actor = creditedActorName(item);
  const description = clean(item.description);
  const humanActor = Boolean(actor);
  const base = {
    mode: 'compact' as const,
    actorKind: humanActor ? 'human' as const : 'unknown' as const,
  };

  if (item.kind === 'task') {
    const task = entityLabel(item, 'task');
    const action = actionWithoutEntity(description, 'task', 'updated');
    return { ...base, label: `${task} ${action}`, attribution: actor, emphasizedValues: [item.entity?.display_id ?? '', entityName(item)] };
  }

  if (item.kind === 'deal') {
    const deal = dealLabel(item);
    const action = actionWithoutEntity(description, 'deal', 'updated');
    return { ...base, label: `${deal} ${action}`, attribution: actor, emphasizedValues: [item.entity?.display_id ?? '', entityName(item)] };
  }

  if (item.kind === 'support') {
    const subject = entityName(item);
    const actionLabels: Record<string, string> = {
      'support.conversation_created': 'opened',
      'support.customer_message_created': 'received a customer reply',
      'support.human_reply_sent': 'received a teammate reply',
      'support.ai_answer_sent': 'received an AI reply',
      'support.conversation_resolved': 'resolved',
      'support.conversation_status_changed': 'changed status',
    };
    const action = actionLabels[item.event_type] || description || 'updated';
    const support = subject ? `Support · ${subject}` : 'Support conversation';
    return { ...base, label: `${support} ${action}`, attribution: actor, emphasizedValues: [subject] };
  }

  if (item.kind === 'enrichment') {
    return { ...base, actorKind: 'system', label: 'Enrichment completed', attribution: 'System', emphasizedValues: ['Enrichment'] };
  }

  const entity = entityLabel(item, item.kind.replace(/_/g, ' '));
  const action = description || 'logged';
  return { ...base, label: `${entity} ${action}`, attribution: actor, emphasizedValues: [entityName(item)] };
}

export function companyTimelinePresentation(item: CRMCompanyTimelineItem): CompanyTimelinePresentation {
  const isAuthoredContent = ['crm_activity', 'crm_email_message', 'crm_calendar_event'].includes(item.source_type)
    && ['note', 'call', 'meeting', 'email'].includes(item.kind);
  if (!isAuthoredContent) return compactPresentation(item);

  const actor = creditedActorName(item);
  const contact = clean(item.contact?.name);
  const subject = clean(item.title);
  let label = 'Activity logged';
  let actorKind: CompanyTimelinePresentation['actorKind'] = actor ? 'human' : 'unknown';
  let contentTitle: string | undefined;
  let attribution = actor;

  if (item.source_type === 'crm_email_message') {
    attribution = '';
    const email = subject ? `Email · ${subject}` : 'Email';
    if (item.event_type === 'email.inbound') label = contact ? `${email} received from ${contact}` : `${email} received`;
    else if (item.event_type === 'email.outbound') label = contact ? `${email} sent to ${contact}` : `${email} sent`;
    else label = contact ? `${email} logged with ${contact}` : `${email} logged`;
  } else if (item.source_type === 'crm_calendar_event') {
    attribution = '';
    label = subject ? `Meeting · ${subject} occurred` : 'Meeting occurred';
  } else if (item.event_type === 'meeting.captured') {
    actorKind = 'system';
    attribution = 'System';
    label = subject ? `Meeting · ${subject} notes added` : 'Meeting notes added';
  } else {
    if (item.kind === 'note') {
      label = 'Note added';
      contentTitle = subject;
    } else {
      const entity = subject ? `${item.kind.charAt(0).toUpperCase()}${item.kind.slice(1)} · ${subject}` : `${item.kind.charAt(0).toUpperCase()}${item.kind.slice(1)}`;
      label = `${entity} logged`;
    }
  }

  return {
    mode: 'content',
    label,
    emphasizedValues: [subject, contact],
    actorKind,
    attribution: attribution || undefined,
    contentTitle,
    contentBody: item.description?.trim() || undefined,
    contentFormat: item.event_type === 'meeting.captured' ? 'markdown' : 'plain',
  };
}
