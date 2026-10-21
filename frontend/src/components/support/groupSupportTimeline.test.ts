import { expect, test } from "vitest";
import type { SupportMessage, SupportConversationMerge } from "@/lib/pmTypes";
import { groupSupportTimeline } from "./groupSupportTimeline";

test("places one bulk operation and its unmerge between native message groups", () => {
  const messages = [1, 3, 5].map(
    (n) =>
      ({
        id: `m${n}`,
        sender_type: "customer",
        content: "Reply",
        message_type: "reply",
        created_at: `2026-10-11T10:0${n}:00Z`,
      }) as SupportMessage,
  );
  const merges = [1, 2].map(
    (n) =>
      ({
        id: `e${n}`,
        operation_id: "operation",
        source_id: `source${n}`,
        target_id: "target",
        created_at: "2026-10-11T10:02:00Z",
        ...(n === 1 ? { ended_at: "2026-10-11T10:04:00Z" } : {}),
      }) as SupportConversationMerge,
  );
  const rows = groupSupportTimeline(messages, merges).filter(
    (row) => row.type !== "separator",
  );
  expect(
    rows.map((row) =>
      row.type === "message"
        ? row.message.id
        : row.restored
          ? "unmerged"
          : "merged",
    ),
  ).toEqual(["m1", "merged", "m3", "unmerged", "m5"]);
  expect(rows[1].type === "merge" && rows[1].merges.length).toBe(2);
});
