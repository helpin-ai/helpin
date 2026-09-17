/** Parse a task key string like "HLP-123" into its parts. Returns null if invalid. */
export function parseTaskKey(input: string): { workspaceKey: string; displayId: number } | null {
  const match = input.match(/^([A-Z]{2,5})-(\d+)$/);
  if (!match) return null;
  return { workspaceKey: match[1], displayId: parseInt(match[2], 10) };
}
