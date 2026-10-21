import { useEffect, useState } from "react";

// Keep the reviewed audience with the draft across navigation/reloads. Existing
// drafts predate merging and therefore start at revision zero.
export function useDraftMergeRevision(
  workspaceId: string,
  conversationId: string,
  current: number | undefined,
  hasDraft: boolean,
) {
  const key = `support-draft-audience:${workspaceId}:${conversationId}`;
  const read = () => {
    try {
      return Number(localStorage.getItem(key) ?? 0);
    } catch {
      return 0;
    }
  };
  const [review, setReview] = useState(() => ({ key, revision: read() }));
  const revision = review.key === key ? review.revision : read();
  const acknowledge = () => {
    if (current === undefined) return;
    try {
      localStorage.setItem(key, String(current));
    } catch {
      /* Current tab still retains the choice. */
    }
    setReview({ key, revision: current });
  };
  useEffect(() => {
    if (current !== undefined && !hasDraft && current !== revision)
      acknowledge();
  }, [key, current, hasDraft, revision]);
  return {
    revision,
    changed: current !== undefined && hasDraft && revision !== current,
    acknowledge,
  };
}
