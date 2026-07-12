/**
 * Pure, JSX-free humanization of support **system events** (assigned, resolved,
 * routed, escalated, tag changes, task creation, ...).
 *
 * Extracted from MessageBubble.tsx so the SAME narration + emphasis + badge
 * rules can be reused by other surfaces (the mobile app's message list) without
 * pulling in MessageBubble's React/icon/store dependencies. MessageBubble
 * re-uses the text + segment helpers here; the badge helper is consumed by
 * mobile (web renders its badges inline). Keep this file free of React/JSX —
 * callers turn `SystemEventSegment[]` into their own markup.
 */

/** First whitespace-delimited part of a display name (e.g. "Ada Lovelace" -> "Ada"). */
export function firstDisplayNamePart(name?: string | null): string {
  return name?.trim().split(/\s+/)[0] ?? '';
}

/** Human-readable escalation labels keyed by system_event_type. */
export const ESCALATION_LABELS: Record<string, string> = {
  ai_escalated: 'AI escalated to a human',
  customer_requested_human: 'Customer requested a human',
};

/**
 * Humanized narration for a system event. Falls back to the raw `content` for
 * event types without a bespoke phrasing (the backend content is already
 * human-readable for those).
 */
export function supportSystemEventDisplayContent(
  eventType: string | undefined,
  content: string,
  senderName: string,
): string {
  const actor = firstDisplayNamePart(senderName);
  switch (eventType) {
    case 'assigned':
      return content.trim() || (actor ? `${actor} assigned this conversation.` : 'Conversation assigned.');
    case 'agent_assigned':
      return actor ? `${actor} assigned this conversation to an AI agent.` : 'Assigned to an AI agent.';
    case 'unassigned':
      return actor ? `${actor} moved this conversation to unassigned.` : 'Moved to unassigned.';
    case 'took':
      return actor ? `${actor} took this conversation.` : 'A teammate took this conversation.';
    default:
      return content;
  }
}

/**
 * The final narration string for a system event: an escalation label when the
 * event is an escalation, otherwise the humanized display content.
 */
export function getSupportSystemEventText(
  eventType: string | undefined,
  content: string,
  senderName: string,
): string {
  if (eventType && eventType in ESCALATION_LABELS) return ESCALATION_LABELS[eventType];
  return supportSystemEventDisplayContent(eventType, content, senderName);
}

export interface SystemEventSegment {
  text: string;
  bold?: boolean;
}

function boldedMatches(content: string, pattern: RegExp): SystemEventSegment[] {
  const segments: SystemEventSegment[] = [];
  let lastIndex = 0;
  for (const match of content.matchAll(pattern)) {
    if (match.index === undefined) continue;
    const [fullMatch, prefix, value, suffix] = match;
    const valueIndex = match.index + prefix.length;
    if (valueIndex > lastIndex) segments.push({ text: content.slice(lastIndex, valueIndex) });
    segments.push({ text: value, bold: true });
    lastIndex = match.index + fullMatch.length - suffix.length;
  }
  if (lastIndex < content.length) segments.push({ text: content.slice(lastIndex) });
  return segments.length > 1 ? segments : [{ text: content }];
}

/**
 * Splits a system event's narration into plain + emphasized (bold) segments so
 * the actor/target/tag/task-key stands out. Returns a single plain segment when
 * the event type has no emphasis rule.
 */
export function toSupportSystemEventSegments(eventType: string | undefined, content: string): SystemEventSegment[] {
  if (eventType === 'assigned') {
    const match = content.match(/^(.*\bassigned this conversation to\s+)([^.]+)(\.)$/);
    if (!match) return [{ text: content }];
    return [{ text: match[1] }, { text: match[2], bold: true }, { text: match[3] }];
  }

  if (eventType === 'email_recipients_updated') {
    return boldedMatches(
      content,
      /(\b(?:made|added|removed)\s+)(\S+@\S+?)(\s+(?:the primary recipient|to Cc|from Cc)\.)/g,
    );
  }

  if (eventType === 'tag_added' || eventType === 'tag_removed') {
    return boldedMatches(content, /(\b(?:added|removed) tag\s+)([^.]+)(\.)/g);
  }

  if (eventType === 'task_created') {
    const taskMatch = content.match(/^(.*\bcreated task\s+)(#[^:\s]+)(?::\s+(.+))?(\.)$/);
    if (taskMatch) {
      const [, prefix, taskKey, taskName, suffix] = taskMatch;
      const segments: SystemEventSegment[] = [{ text: prefix }, { text: taskKey, bold: true }];
      if (taskName) {
        segments.push({ text: ': ' });
        segments.push({ text: taskName, bold: true });
      }
      segments.push({ text: suffix });
      return segments;
    }
  }

  return [{ text: content }];
}

export type SystemEventBadgeKind = 'routing_rule' | 'ai_routing' | 'ai_escalated' | 'customer_requested_human';

export interface SystemEventBadge {
  kind: SystemEventBadgeKind;
  label: string;
}

/**
 * The small badge shown next to certain automated/escalation events, or null.
 * Callers map `kind` to their own icon + colour treatment.
 */
export function getSupportSystemEventBadge(eventType: string | undefined, content: string): SystemEventBadge | null {
  if (eventType === 'customer_requested_human') return { kind: 'customer_requested_human', label: 'Customer' };
  if (eventType === 'ai_escalated') return { kind: 'ai_escalated', label: 'AI' };
  if (eventType === 'triage_routed') {
    const isRuleRouting = content.trim().toLowerCase().startsWith('routing rule ');
    return isRuleRouting
      ? { kind: 'routing_rule', label: 'Routing rule' }
      : { kind: 'ai_routing', label: 'AI routing' };
  }
  return null;
}
