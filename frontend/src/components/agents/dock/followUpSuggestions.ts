const SUGGESTION_MARKER = /<!--\s*helpin_follow_up_suggestions\s*([\s\S]*?)\s*-->/i;

export function parseFollowUpSuggestions(content: string): string[] {
  const match = content.match(SUGGESTION_MARKER);
  if (!match) return [];
  try {
    const values = JSON.parse(match[1]) as unknown;
    if (!Array.isArray(values)) return [];
    const seen = new Set<string>();
    return values
      .filter((value): value is string => typeof value === 'string')
      .map((value) => value.replace(/\s+/g, ' ').trim())
      .filter((value) => value.length >= 4 && value.length <= 140)
      .filter((value) => {
        const key = value.toLocaleLowerCase();
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      })
      .slice(0, 3);
  } catch {
    return [];
  }
}
