import type { SupportConversationMerge, SupportMessage } from "@/lib/pmTypes";
import { groupSupportMessages } from "./groupSupportMessages";
import { getDayLabel } from "./helpers";

type MessageItem = ReturnType<typeof groupSupportMessages>[number];
export type SupportTimelineItem =
  | MessageItem
  | {
      type: "merge";
      id: string;
      merges: SupportConversationMerge[];
      restored: boolean;
    };
export function groupSupportTimeline(
  messages: SupportMessage[],
  merges: SupportConversationMerge[],
): SupportTimelineItem[] {
  const operations = new Map<string, SupportConversationMerge[]>();
  for (const merge of merges)
    operations.set(merge.operation_id, [
      ...(operations.get(merge.operation_id) ?? []),
      merge,
    ]);
  const events = [...operations].map(([id, rows]) => ({
    at: rows[0].created_at,
    id,
    rows,
    restored: false,
  }));
  for (const merge of merges)
    if (merge.ended_at)
      events.push({
        at: merge.ended_at,
        id: `restore-${merge.id}`,
        rows: [merge],
        restored: true,
      });
  events.sort(
    (a, b) => Date.parse(a.at) - Date.parse(b.at) || a.id.localeCompare(b.id),
  );
  const result: SupportTimelineItem[] = [];
  let index = 0;
  let lastDay = "";
  const append = (chunk: SupportMessage[]) => {
    for (const item of groupSupportMessages(chunk)) {
      if (item.type === "separator") {
        if (item.label === lastDay) continue;
        lastDay = item.label;
      }
      result.push(item);
    }
  };
  for (const event of events) {
    const start = index;
    while (
      index < messages.length &&
      Date.parse(messages[index].created_at) <= Date.parse(event.at)
    )
      index++;
    append(messages.slice(start, index));
    const day = getDayLabel(event.at);
    if (day !== lastDay) {
      result.push({ type: "separator", label: day });
      lastDay = day;
    }
    result.push({
      type: "merge",
      id: event.id,
      merges: event.rows,
      restored: event.restored,
    });
  }
  append(messages.slice(index));
  return result;
}
