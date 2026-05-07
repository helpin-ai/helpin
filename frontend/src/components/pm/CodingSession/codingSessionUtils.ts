import { formatDistanceToNow, parseISO } from 'date-fns';

import type { CodingSessionEvent, CodingSessionInteraction } from '@/lib/pmTypes';
import { normalizeCodingSessionPreviewPanelKey } from './previewPanelKeys';

export function formatCodingSessionRelative(value?: string) {
  if (!value) return 'Unknown time';
  try {
    return formatDistanceToNow(parseISO(value), { addSuffix: true });
  } catch {
    return value;
  }
}

export function prettyCodingSessionEventType(value: string) {
  return value.replaceAll('.', ' ');
}

export function codingSessionEventContent(payload: Record<string, unknown>) {
  const content = payload.content;
  if (typeof content === 'string') return content;
  if (typeof payload.text === 'string') return payload.text;
  return '';
}

export function sortCodingSessionEvents(events: CodingSessionEvent[]) {
  return [...events].sort((a, b) => a.sequence_no - b.sequence_no);
}

export function isPersistedCodingSessionEvent(event: CodingSessionEvent) {
  const source = typeof event.runtime_metadata?.source === 'string' ? event.runtime_metadata.source.trim() : '';
  if (source.length > 0) return true;
  return (
    event.id.startsWith('msg:')
    || event.id.startsWith('artifact:')
    || event.id.startsWith('interaction:')
    || event.id.startsWith('run:')
  );
}

export function maxPersistedCodingSessionSequence(events: CodingSessionEvent[]) {
  return events.reduce((max, event) => (
    isPersistedCodingSessionEvent(event) ? Math.max(max, event.sequence_no) : max
  ), 0);
}

export function upsertCodingSessionEvents(current: CodingSessionEvent[], incoming: CodingSessionEvent[]) {
  const byId = new Map(current.map((event) => [event.id, event]));
  for (const event of incoming) {
    byId.set(event.id, event);
  }
  return sortCodingSessionEvents([...byId.values()]);
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asString(value: unknown) {
  return typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;
}

function asNumber(value: unknown) {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

export function parseCodingSessionInteraction(event: CodingSessionEvent): CodingSessionInteraction | null {
  if (!event.type.startsWith('interaction.')) return null;
  const payload = asRecord(event.payload);
  const requestPayload = asRecord(payload?.request_payload) ?? {};
  const responsePayload = asRecord(payload?.response_payload) ?? undefined;
  const interactionId = asString(payload?.interaction_id);
  const interactionKind = asString(payload?.interaction_kind);
  const status = asString(payload?.status);
  const requestSchemaVersion = asString(payload?.request_schema_version);
  if (!interactionId || !interactionKind || !status || !requestSchemaVersion) return null;

  return {
    interaction_id: interactionId,
    interaction_kind: interactionKind as CodingSessionInteraction['interaction_kind'],
    status: status as CodingSessionInteraction['status'],
    request_schema_version: requestSchemaVersion,
    response_schema_version: asString(payload?.response_schema_version),
    request_payload: requestPayload,
    response_payload: responsePayload,
    title: asString(payload?.title),
    summary: asString(payload?.summary),
    request_id: asString(payload?.request_id),
    thread_id: asString(payload?.thread_id),
    turn_id: asString(payload?.turn_id),
    item_id: asString(payload?.item_id),
    approval_id: asString(payload?.approval_id),
    assistant_message_sequence_no: asNumber(payload?.assistant_message_sequence_no),
    resolved_at: asString(payload?.resolved_at),
    resolved_by: asString(payload?.resolved_by),
  };
}

export type CodingSessionPreviewApprovalStatus = 'pending' | 'approved' | 'changes_requested';

export interface CodingSessionPreviewApprovalState {
  interaction: CodingSessionInteraction;
  status: CodingSessionPreviewApprovalStatus;
  title?: string;
  summary?: string;
  phase?: string;
  previewPanelKey: string;
  resolvedAt?: string;
  resolvedBy?: string;
  note?: string;
}

function approvalStateFromInteraction(
  interaction: CodingSessionInteraction,
): CodingSessionPreviewApprovalState | null {
  if (interaction.interaction_kind !== 'approval_request') return null;
  const previewPanelKey = normalizeCodingSessionPreviewPanelKey(
    typeof interaction.request_payload?.preview_panel_key === 'string'
      ? interaction.request_payload.preview_panel_key
      : '',
  );
  if (!previewPanelKey) return null;

  const responsePayload = asRecord(interaction.response_payload);
  const decision = asString(responsePayload?.decision);
  const status: CodingSessionPreviewApprovalStatus = interaction.status === 'pending'
    ? 'pending'
    : decision === 'request_changes'
      ? 'changes_requested'
      : 'approved';

  return {
    interaction,
    status,
    title: asString(interaction.request_payload?.title) ?? interaction.title,
    summary: asString(interaction.request_payload?.summary) ?? interaction.summary,
    phase: asString(interaction.request_payload?.phase),
    previewPanelKey,
    resolvedAt: interaction.resolved_at ?? asString(responsePayload?.resolved_at),
    resolvedBy: interaction.resolved_by ?? asString(responsePayload?.resolved_by),
    note: asString(responsePayload?.message) ?? asString(responsePayload?.note),
  };
}

export function codingSessionApprovalStatesByPreviewKey(events: CodingSessionEvent[]) {
  const latestByPreviewKey = new Map<string, { sequence: number; state: CodingSessionPreviewApprovalState }>();
  for (const event of events) {
    const interaction = parseCodingSessionInteraction(event);
    if (!interaction) continue;
    const state = approvalStateFromInteraction(interaction);
    if (!state) continue;
    const existing = latestByPreviewKey.get(state.previewPanelKey);
    if (!existing || event.sequence_no >= existing.sequence) {
      latestByPreviewKey.set(state.previewPanelKey, { sequence: event.sequence_no, state });
    }
  }
  return new Map([...latestByPreviewKey].map(([previewPanelKey, entry]) => [previewPanelKey, entry.state]));
}

export function latestPendingCodingSessionInteraction(events: CodingSessionEvent[]) {
  const latestByInteraction = new Map<string, { sequence: number; interaction: CodingSessionInteraction }>();
  for (const event of events) {
    const interaction = parseCodingSessionInteraction(event);
    if (!interaction) continue;
    const existing = latestByInteraction.get(interaction.interaction_id);
    if (!existing || event.sequence_no >= existing.sequence) {
      latestByInteraction.set(interaction.interaction_id, { sequence: event.sequence_no, interaction });
    }
  }
  return [...latestByInteraction.values()]
    .map((entry) => entry.interaction)
    .filter((interaction) => interaction.status === 'pending')
    .sort((a, b) => (b.assistant_message_sequence_no ?? 0) - (a.assistant_message_sequence_no ?? 0))[0] ?? null;
}
