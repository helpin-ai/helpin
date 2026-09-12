import { create } from "zustand";
import { useEffect } from "react";
import type { SupportReplyDeliveryMode } from "@/lib/pmTypes";

export const REPLY_DELIVERY_LABELS: Record<SupportReplyDeliveryMode, string> = {
  chat_only: "Chat only",
  chat_and_email: "Chat + email",
  email_only: "Email only",
};

export function isReplyDeliveryMode(
  value: unknown,
): value is SupportReplyDeliveryMode {
  return (
    value === "chat_only" ||
    value === "chat_and_email" ||
    value === "email_only"
  );
}

export function getReplyDeliveryMode(
  metadata?: string,
): SupportReplyDeliveryMode | undefined {
  try {
    const value: unknown = JSON.parse(metadata ?? "{}").delivery_mode;
    return isReplyDeliveryMode(value) ? value : undefined;
  } catch {
    return undefined;
  }
}

export function replyDeliveryChannels(
  mode: SupportReplyDeliveryMode,
): ("chat" | "email")[] {
  return mode === "chat_only"
    ? ["chat"]
    : mode === "email_only"
      ? ["email"]
      : ["chat", "email"];
}

export type ReplyPresence = 'unknown' | 'online' | 'offline';

export function nextAutomaticReplyDelivery(
  current: SupportReplyDeliveryMode | undefined,
  { source, presence, emailEligible }: { source?: string; presence: ReplyPresence; emailEligible: boolean },
): SupportReplyDeliveryMode {
  if (current && current !== 'chat_only') return current;
  if (source === 'email') return 'email_only';
  return emailEligible && presence === 'offline' ? 'chat_and_email' : 'chat_only';
}

export function getReplyEmailSubject(metadata?: string): string | undefined {
  try {
    const subject: unknown = JSON.parse(metadata ?? '{}').email_subject;
    return typeof subject === 'string' ? subject : undefined;
  } catch {
    return undefined;
  }
}

function draftKey(workspaceId: string, conversationId: string) {
  return `support_reply_delivery:${workspaceId}:${conversationId}`;
}

const useDeliveryDrafts = create<
  Record<string, string | undefined>
>(() => ({}));

// Desktop and mobile can mount separate composers for the same draft.
export function useReplyDelivery(workspaceId: string, conversationId: string) {
  const value = useDeliveryDrafts(
    (state) =>
      state[draftKey(workspaceId, conversationId)] ??
      loadReplyDelivery(workspaceId, conversationId),
  );
  return isReplyDeliveryMode(value) ? value : undefined;
}

function loadDraftValue(key: string) {
  try { return localStorage.getItem(key) ?? undefined; } catch { return undefined; }
}

function saveDraftValue(key: string, value?: string) {
  try {
    if (value !== undefined) localStorage.setItem(key, value);
    else localStorage.removeItem(key);
  } catch { /* Keep both mounted composers synchronized without browser storage. */ }
  useDeliveryDrafts.setState({ [key]: value });
}

function useDraftValue(key: string) {
  return useDeliveryDrafts((state) => key in state ? state[key] : loadDraftValue(key));
}

function automaticDraftKey(workspaceId: string, conversationId: string) {
  return `${draftKey(workspaceId, conversationId)}:automatic`;
}

export function useComposerDelivery(
  workspaceId: string,
  conversationId: string,
  context: { source?: string; presence: ReplyPresence; emailEligible: boolean; active: boolean },
) {
  const preference = useReplyDelivery(workspaceId, conversationId);
  const key = automaticDraftKey(workspaceId, conversationId);
  const stored = useDraftValue(key);
  const restored = useDraftValue(`${key}:restored`);
  const current = isReplyDeliveryMode(stored) ? stored : undefined;
  const mode = (isReplyDeliveryMode(restored) ? restored : undefined) ?? preference ?? nextAutomaticReplyDelivery(current, context);
  useEffect(() => {
    if (!restored && !preference && context.active && context.source && mode !== current) saveDraftValue(key, mode);
  }, [restored, preference, context.active, context.source, mode, current, key]);
  return mode;
}

// Undo restores only this reply's choice; it must not replace the teammate's preference.
export function restoreReplyDelivery(workspaceId: string, conversationId: string, mode?: SupportReplyDeliveryMode) {
  saveDraftValue(`${automaticDraftKey(workspaceId, conversationId)}:restored`, mode);
}

export function clearAutomaticReplyDelivery(workspaceId: string, conversationId: string) {
  saveDraftValue(automaticDraftKey(workspaceId, conversationId));
  saveDraftValue(`${automaticDraftKey(workspaceId, conversationId)}:restored`);
}

export function useReplySubject(workspaceId: string, conversationId: string, defaultSubject: string) {
  return useDraftValue(`${draftKey(workspaceId, conversationId)}:subject`) ?? defaultSubject;
}

export function saveReplySubject(workspaceId: string, conversationId: string, subject?: string) {
  saveDraftValue(`${draftKey(workspaceId, conversationId)}:subject`, subject);
}

export function loadReplyDelivery(
  workspaceId: string,
  conversationId: string,
): SupportReplyDeliveryMode | undefined {
  try {
    const value = localStorage.getItem(draftKey(workspaceId, conversationId));
    return isReplyDeliveryMode(value) ? value : undefined;
  } catch {
    return undefined;
  }
}

export function saveReplyDelivery(
  workspaceId: string,
  conversationId: string,
  mode?: SupportReplyDeliveryMode,
) {
  saveDraftValue(`${automaticDraftKey(workspaceId, conversationId)}:restored`);
  try {
    const key = draftKey(workspaceId, conversationId);
    if (mode) localStorage.setItem(key, mode);
    else localStorage.removeItem(key);
  } catch {
    // Keep the shared draft usable when browser storage is unavailable.
  }
  useDeliveryDrafts.setState({ [draftKey(workspaceId, conversationId)]: mode });
}
