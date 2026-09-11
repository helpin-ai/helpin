import { create } from "zustand";
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

function draftKey(workspaceId: string, conversationId: string) {
  return `support_reply_delivery:${workspaceId}:${conversationId}`;
}

const useDeliveryDrafts = create<
  Record<string, SupportReplyDeliveryMode | undefined>
>(() => ({}));

// Desktop and mobile can mount separate composers for the same draft.
export function useReplyDelivery(workspaceId: string, conversationId: string) {
  return useDeliveryDrafts(
    (state) =>
      state[draftKey(workspaceId, conversationId)] ??
      loadReplyDelivery(workspaceId, conversationId),
  );
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
  try {
    const key = draftKey(workspaceId, conversationId);
    if (mode) localStorage.setItem(key, mode);
    else localStorage.removeItem(key);
  } catch {
    // Keep the shared draft usable when browser storage is unavailable.
  }
  useDeliveryDrafts.setState({ [draftKey(workspaceId, conversationId)]: mode });
}
