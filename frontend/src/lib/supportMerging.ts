export function mergeSourceIds(selected: string[], target: string): string[] {
  return [...new Set(selected)].filter((id) => id !== target);
}

export function canMergeSelection(
  conversations: {
    id: string;
    merged_into_id?: string | null;
    anonymized_at?: string | null;
  }[],
): boolean {
  return (
    conversations.length >= 2 &&
    conversations.length <= 50 &&
    conversations.every((c) => !c.merged_into_id && !c.anonymized_at)
  );
}
