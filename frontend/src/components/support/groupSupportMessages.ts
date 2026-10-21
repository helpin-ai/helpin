import type { SupportMessage } from "@/lib/pmTypes";
import { getDayLabel, getEffectiveSenderType, isSameDay } from "./helpers";
export function groupSupportMessages(messages: SupportMessage[]) {
  const items: Array<
    | { type: "separator"; label: string }
    | {
        type: "message";
        message: SupportMessage;
        isConsecutive: boolean;
        isLastInGroup: boolean;
      }
  > = [];
  let lastDate: string | null = null;

  messages.forEach((msg, idx) => {
    // Insert day separator if new day
    if (!lastDate || !isSameDay(lastDate, msg.created_at)) {
      items.push({ type: "separator", label: getDayLabel(msg.created_at) });
      lastDate = msg.created_at;
    }

    // Check if consecutive (same sender within 2 minutes, same type)
    const prev = idx > 0 ? messages[idx - 1] : null;
    const currentSenderType = getEffectiveSenderType(msg);
    const prevSenderType = prev ? getEffectiveSenderType(prev) : null;
    const isConsecutive =
      prev !== null &&
      prev.message_type !== "system" &&
      msg.message_type !== "system" &&
      prevSenderType === currentSenderType &&
      (currentSenderType !== "user" ||
        (!!msg.sender_user_id && prev.sender_user_id === msg.sender_user_id)) &&
      prev.is_internal === msg.is_internal &&
      isSameDay(prev.created_at, msg.created_at) &&
      new Date(msg.created_at).getTime() - new Date(prev.created_at).getTime() <
        120000;

    // Check if this is the last message in a consecutive group
    const next = idx < messages.length - 1 ? messages[idx + 1] : null;
    const nextSenderType = next ? getEffectiveSenderType(next) : null;
    const isLastInGroup =
      next === null ||
      next.message_type === "system" ||
      msg.message_type === "system" ||
      nextSenderType !== currentSenderType ||
      (currentSenderType === "user" &&
        (!msg.sender_user_id || next.sender_user_id !== msg.sender_user_id)) ||
      next.is_internal !== msg.is_internal ||
      !isSameDay(msg.created_at, next.created_at) ||
      new Date(next.created_at).getTime() -
        new Date(msg.created_at).getTime() >=
        120000;

    items.push({ type: "message", message: msg, isConsecutive, isLastInGroup });
  });

  return items;
}
