import type { ElementType } from 'react';
import { AlertCircleIcon, CheckmarkCircle02Icon, DashedLineCircleIcon, HelpCircleIcon } from '@/lib/icons';
import type { Capability, CapabilityKey, CapabilityStatus } from '@/lib/capabilityTypes';
import type { SetupGoalKey } from '@/lib/setupTypes';

export const CAPABILITY_TITLES: Record<CapabilityKey, string> = {
  ai_chat: 'AI provider',
  ai_embeddings: 'Knowledge search',
  email_outbound: 'Outgoing email',
  support_widget: 'Chat widget',
  support_email_inbound: 'Support email',
  github: 'GitHub',
  object_storage: 'File storage',
  workers: 'Background workers',
};

export function capabilityTitle(key: string) {
  return CAPABILITY_TITLES[key as CapabilityKey] ?? key.replace(/_/g, ' ');
}

type StatusPresentation = { label: string; icon: ElementType; className: string };

/**
 * Every status pairs an icon with a word, so it never depends on color.
 * "Couldn't verify" deliberately uses a question mark and neutral ink: it must
 * never read as success.
 */
export const CAPABILITY_STATUS: Record<CapabilityStatus, StatusPresentation> = {
  ready: { label: 'Ready', icon: CheckmarkCircle02Icon, className: 'text-quiet-positive' },
  needs_setup: { label: 'Needs setup', icon: AlertCircleIcon, className: 'text-quiet-accent' },
  unable_to_verify: { label: 'Couldn’t verify', icon: HelpCircleIcon, className: 'text-quiet-text-secondary' },
  unavailable: { label: 'Not available', icon: DashedLineCircleIcon, className: 'text-quiet-muted' },
};

export function capabilityStatus(status: string): StatusPresentation {
  return CAPABILITY_STATUS[status as CapabilityStatus] ?? CAPABILITY_STATUS.unable_to_verify;
}

/** Capabilities every workspace depends on, whatever its goals. */
const FOUNDATION: CapabilityKey[] = ['object_storage', 'workers', 'ai_chat', 'email_outbound'];

/** Capabilities that unlock each Setup goal, in the order they matter. */
export const GOAL_CAPABILITIES: Record<SetupGoalKey, CapabilityKey[]> = {
  customer_support: ['support_widget', 'support_email_inbound', 'ai_embeddings'],
  help_center_docs: ['support_widget', 'ai_embeddings'],
  internal_docs: ['ai_embeddings'],
  product_delivery: ['github'],
  team_project_management: ['github'],
  automation_mastery: ['github'],
  sales_crm: [],
};

export const GOAL_LABELS: Record<SetupGoalKey, string> = {
  product_delivery: 'Plan and ship team projects',
  team_project_management: 'Plan and ship team projects',
  customer_support: 'Scale customer support',
  help_center_docs: 'Publish help center docs',
  internal_docs: 'Build internal knowledge',
  sales_crm: 'Build a sales pipeline',
  automation_mastery: 'Automate repeatable work',
};

/** Goals (by label) that need a capability; empty for foundation capabilities. */
export function goalsNeeding(key: CapabilityKey, goals: SetupGoalKey[]) {
  if (FOUNDATION.includes(key)) return [];
  return [...new Set(goals.filter((goal) => GOAL_CAPABILITIES[goal]?.includes(key)).map((goal) => GOAL_LABELS[goal]))];
}

function isRelevant(key: CapabilityKey, goals: SetupGoalKey[]) {
  return FOUNDATION.includes(key) || goals.some((goal) => GOAL_CAPABILITIES[goal]?.includes(key));
}

function attentionRank(capability: Capability) {
  if (capability.required && capability.status !== 'ready') return 0;
  if (capability.status === 'needs_setup') return 1;
  if (capability.status === 'unable_to_verify') return 2;
  return 3;
}

export interface CapabilityGroups {
  /** Required services that are not ready; shown first and prominently. */
  blocking: Capability[];
  relevant: Capability[];
  other: Capability[];
  unavailable: Capability[];
}

/**
 * Orders capabilities so what the chosen goals need comes first: required
 * services that are not ready, then goal-relevant items needing attention,
 * then everything else, with unavailable items last.
 */
export function groupCapabilities(capabilities: Capability[], goals: SetupGoalKey[]): CapabilityGroups {
  const order = new Map<string, number>();
  const goalOrder = [...FOUNDATION, ...goals.flatMap((goal) => GOAL_CAPABILITIES[goal] ?? [])];
  goalOrder.forEach((key, index) => { if (!order.has(key)) order.set(key, index); });
  const byAttention = (a: Capability, b: Capability) =>
    attentionRank(a) - attentionRank(b) || (order.get(a.key) ?? 99) - (order.get(b.key) ?? 99);

  const groups: CapabilityGroups = { blocking: [], relevant: [], other: [], unavailable: [] };
  for (const capability of capabilities) {
    if (capability.status === 'unavailable') groups.unavailable.push(capability);
    else if (capability.required && capability.status !== 'ready') groups.blocking.push(capability);
    else if (isRelevant(capability.key, goals)) groups.relevant.push(capability);
    else groups.other.push(capability);
  }
  groups.blocking.sort(byAttention);
  groups.relevant.sort(byAttention);
  groups.other.sort(byAttention);
  return groups;
}
