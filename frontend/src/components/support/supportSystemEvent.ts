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

type Range = [start: number, end: number];

/** Events whose content starts with an actor's name (a person we should emphasize). */
const ACTOR_LED_EVENTS = new Set([
  'teammate_joined',
  'assigned',
  'agent_assigned',
  'unassigned',
  'took',
  'mailbox_moved',
  'resolved',
  'reopened',
  'closed',
  'task_created',
]);

/** Verbs that follow the leading actor name across the event phrasings. */
const ACTOR_VERB = /^(.+?)\s+(?:joined|assigned|took|moved|resolved|reopened|closed|created)\b/;

function pushGroup(ranges: Range[], match: RegExpMatchArray | null, group: number): void {
  const indices = match?.indices?.[group];
  if (indices) ranges.push([indices[0], indices[1]]);
}

/** Turns a content string + a set of emphasis ranges into ordered plain/bold segments. */
function rangesToSegments(content: string, ranges: Range[]): SystemEventSegment[] {
  const valid = ranges.filter(([start, end]) => end > start).sort((a, b) => a[0] - b[0]);
  if (valid.length === 0) return [{ text: content }];

  const merged: Range[] = [];
  for (const range of valid) {
    const last = merged[merged.length - 1];
    if (last && range[0] <= last[1]) last[1] = Math.max(last[1], range[1]);
    else merged.push([range[0], range[1]]);
  }

  const segments: SystemEventSegment[] = [];
  let pos = 0;
  for (const [start, end] of merged) {
    if (start > pos) segments.push({ text: content.slice(pos, start) });
    segments.push({ text: content.slice(start, end), bold: true });
    pos = end;
  }
  if (pos < content.length) segments.push({ text: content.slice(pos) });
  return segments.length > 1 ? segments : [{ text: content }];
}

export interface SupportSystemEventSegmentOptions {
  /**
   * When true, also emphasize the **leading actor name** and the **inbox name**
   * (stripping its quotes) — the fuller "bold every meaningful token" treatment.
   * Defaults to false so existing callers (the web thread) keep their exact
   * rendering (assignee / tag / task / email only). The mobile app opts in.
   */
  extended?: boolean;
}

/**
 * Splits a system event's narration into plain + emphasized (bold) segments so
 * the meaningful tokens stand out: the person assigned, tags, task keys/names,
 * and email recipients — plus, with `extended`, the acting person and the inbox
 * moved to. The `bold` flag only marks *which* tokens matter — each surface
 * decides how to render them.
 */
export function toSupportSystemEventSegments(
  eventType: string | undefined,
  rawContent: string,
  options: SupportSystemEventSegmentOptions = {},
): SystemEventSegment[] {
  let content = rawContent;
  const ranges: Range[] = [];

  // Inbox name — strip the surrounding straight quotes and emphasize the name.
  // (`moved to inbox 'Sales'.` → `moved to inbox Sales.` with "Sales" bold.)
  if (options.extended) {
    const inboxMatch = content.match(/(inbox\s+)'([^']+)'/);
    if (inboxMatch && inboxMatch.index !== undefined) {
      const nameStart = inboxMatch.index + inboxMatch[1].length;
      content =
        content.slice(0, inboxMatch.index) +
        inboxMatch[1] +
        inboxMatch[2] +
        content.slice(inboxMatch.index + inboxMatch[0].length);
      ranges.push([nameStart, nameStart + inboxMatch[2].length]);
    }
  }

  // Leading actor name (person events). Anchored at 0, so no indices flag needed.
  if (options.extended && eventType && ACTOR_LED_EVENTS.has(eventType)) {
    const actor = content.match(ACTOR_VERB);
    if (actor) ranges.push([0, actor[1].length]);
  }

  // Assigned target person: "... to Jarek".
  if (eventType === 'assigned') {
    pushGroup(ranges, content.match(/\bto\s+([^.]+?)\s*\.?\s*$/d), 1);
  }

  // Tags: "added/removed tag VIP.".
  if (eventType === 'tag_added' || eventType === 'tag_removed') {
    pushGroup(ranges, content.match(/\b(?:added|removed) tag\s+([^.]+?)\s*\.?\s*$/d), 1);
  }

  // Task: "created task #KEY: Name.".
  if (eventType === 'task_created') {
    const task = content.match(/\bcreated task\s+(#[^:\s]+)(?::\s+(.+?))?\s*\.?\s*$/d);
    pushGroup(ranges, task, 1);
    pushGroup(ranges, task, 2);
  }

  // Email recipients: the address before its "… the primary recipient / to Cc /
  // from Cc" suffix. Bounded by that lookahead so the whole address (dots and
  // all) is captured, not truncated at the first ".".
  if (eventType === 'email_recipients_updated') {
    for (const match of content.matchAll(
      /\b(?:made|added|removed)\s+(\S+@\S+?)(?=\s+(?:the primary recipient|to Cc|from Cc))/dg,
    )) {
      pushGroup(ranges, match, 1);
    }
  }

  return rangesToSegments(content, ranges);
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
