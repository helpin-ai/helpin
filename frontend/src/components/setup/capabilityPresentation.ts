import type { ElementType } from 'react';
import { AlertCircleIcon, CheckmarkCircle02Icon, DashedLineCircleIcon, HelpCircleIcon } from '@/lib/icons';
import type { CapabilitiesResponse, Capability, CapabilityKey, CapabilityStatus } from '@/lib/capabilityTypes';

export const CAPABILITY_TITLES: Record<CapabilityKey, string> = {
  ai_chat: 'AI chat',
  ai_embeddings: 'Knowledge search',
  email_outbound: 'Application email',
  support_widget: 'Chat widget',
  support_email_inbound: 'Inbound support email',
  github: 'GitHub App',
  object_storage: 'Object storage',
  workers: 'Background workers',
  meeting_capture: 'Meeting capture',
  google_workspace: 'Google (Gmail and Calendar)',
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

/**
 * Server-level services shown on Settings → System status, in reading order.
 * Workspace adoption steps (such as installing the chat widget) belong to the
 * Setup guide instead.
 */
export const SYSTEM_STATUS_ORDER: CapabilityKey[] = [
  'object_storage',
  'workers',
  'email_outbound',
  'support_email_inbound',
  'ai_chat',
  'ai_embeddings',
  'github',
  'google_workspace',
  'meeting_capture',
];

const WORKSPACE_ADOPTION_KEYS: CapabilityKey[] = ['support_widget'];

/** A required service (storage, workers) that is not ready. */
export function isBlocking(capability: Capability) {
  return capability.required && capability.status !== 'ready';
}

function attentionRank(capability: Capability) {
  if (isBlocking(capability)) return 0;
  if (capability.status === 'needs_setup') return 1;
  if (capability.status === 'unable_to_verify') return 2;
  return 3;
}

export interface SystemStatusGroups {
  /** Required services that are not ready; shown first and prominently. */
  blocking: Capability[];
  /** Everything else this server could provide, needing attention first. */
  services: Capability[];
  unavailable: Capability[];
}

/**
 * Orders server capabilities for System status: required services that are
 * not ready, then the remaining services with anything needing setup first,
 * then services this server does not offer.
 */
export function groupSystemCapabilities(capabilities: Capability[]): SystemStatusGroups {
  const order = (key: string) => {
    const index = SYSTEM_STATUS_ORDER.indexOf(key as CapabilityKey);
    return index === -1 ? SYSTEM_STATUS_ORDER.length : index;
  };
  const byAttention = (a: Capability, b: Capability) => attentionRank(a) - attentionRank(b) || order(a.key) - order(b.key);

  const groups: SystemStatusGroups = { blocking: [], services: [], unavailable: [] };
  for (const capability of capabilities) {
    if (WORKSPACE_ADOPTION_KEYS.includes(capability.key)) continue;
    if (capability.status === 'unavailable') groups.unavailable.push(capability);
    else if (isBlocking(capability)) groups.blocking.push(capability);
    else groups.services.push(capability);
  }
  groups.blocking.sort(byAttention);
  groups.services.sort(byAttention);
  groups.unavailable.sort((a, b) => order(a.key) - order(b.key));
  return groups;
}

export function findCapability(response: CapabilitiesResponse | undefined, key: CapabilityKey) {
  return response?.capabilities.find((capability) => capability.key === key);
}

/** Required server services that are not ready, on a Community server only. */
export function blockingServices(response: CapabilitiesResponse | undefined) {
  if (response?.edition !== 'community') return [];
  return response.capabilities.filter(isBlocking);
}
