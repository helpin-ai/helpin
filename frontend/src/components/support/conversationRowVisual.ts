import type { CSSProperties } from 'react';

/**
 * Pure, dependency-free visual helpers for a support conversation row.
 *
 * Extracted from ConversationRow.tsx so the SAME rules can be reused by other
 * surfaces (e.g. the mobile app's conversation cell) without pulling in
 * ConversationRow's React/store/icon dependencies. ConversationRow re-exports
 * these for backwards compatibility. Keep this file free of React component,
 * store, and icon imports — type-only imports are fine.
 */

/** Prefix the backend uses on `last_message` when the latest activity is an internal note. */
export const NOTE_PREFIX = 'Note: ';

/** True when a conversation's last message is an internal note preview. */
export function isNotePreview(text: string | null | undefined): boolean {
  return !!text && text.startsWith(NOTE_PREFIX);
}

/** The note text with the `Note: ` prefix removed (returns input unchanged if not a note). */
export function stripNotePrefix(text: string): string {
  return text.startsWith(NOTE_PREFIX) ? text.slice(NOTE_PREFIX.length) : text;
}

export type ConversationRowVisualState = {
  isUnread: boolean;
  needsTeamAction: boolean;
  usesActionBackground: boolean;
  usesUnreadTypography: boolean;
  usesSelectionBar: boolean;
};

/**
 * Derives the row's visual state from a conversation. Structurally typed (not
 * tied to a specific SupportConversation union) so both web and mobile
 * conversation objects satisfy it.
 */
export function getConversationRowVisualState(
  conversation: { unread_count?: number | null; awaiting_reply?: boolean; status: string },
  isSelected = false,
): ConversationRowVisualState {
  const isUnread = (conversation.unread_count ?? 0) > 0;
  const needsTeamAction = conversation.status === 'open' && (isUnread || Boolean(conversation.awaiting_reply));
  return {
    isUnread,
    needsTeamAction,
    usesActionBackground: !isSelected && needsTeamAction,
    usesUnreadTypography: isUnread,
    usesSelectionBar: isSelected,
  };
}

export function getVisibleSupportTagCount(tagWidths: number[], availableWidth: number, gap = 4) {
  if (tagWidths.length === 0) return 0;
  if (availableWidth <= 0) return tagWidths.length;

  let usedWidth = 0;
  let visibleCount = 0;
  for (const width of tagWidths) {
    const nextWidth = usedWidth + (visibleCount > 0 ? gap : 0) + Math.max(0, width);
    if (nextWidth > availableWidth + 0.5) break;
    usedWidth = nextWidth;
    visibleCount += 1;
  }

  return visibleCount > 0 ? visibleCount : 1;
}

function parseHexColor(color: string | null | undefined) {
  const normalized = color?.trim();
  if (!normalized) return null;
  const shortMatch = normalized.match(/^#([0-9a-f]{3})$/i);
  const longMatch = normalized.match(/^#([0-9a-f]{6})$/i);
  const hex = shortMatch
    ? shortMatch[1].split('').map((char) => `${char}${char}`).join('')
    : longMatch?.[1];
  if (!hex) return null;
  return {
    r: Number.parseInt(hex.slice(0, 2), 16),
    g: Number.parseInt(hex.slice(2, 4), 16),
    b: Number.parseInt(hex.slice(4, 6), 16),
  };
}

export function getSupportTagPillStyle(color: string | null | undefined): CSSProperties | undefined {
  const rgb = parseHexColor(color);
  if (!rgb) return undefined;
  const value = `${rgb.r}, ${rgb.g}, ${rgb.b}`;
  return {
    backgroundColor: `rgba(${value}, 0.08)`,
    borderColor: `rgba(${value}, 0.22)`,
    color: `rgba(${value}, 0.82)`,
  };
}
