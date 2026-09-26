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

/** Chat presentation only: keep machine hints out of rendered and copied answers. */
export function stripFollowUpSuggestions(content: string): string {
  const cleaned = content.replace(/<!--\s*helpin_follow_up_suggestions\b[\s\S]*?(?:-->|$)/gi, '');
  // Streaming can stop halfway through the marker name. Hold that suffix until
  // it is either recognized as metadata or becomes ordinary comment text.
  const partial = cleaned.match(/<!--[^<>]*$/);
  const result = partial && '<!--helpin_follow_up_suggestions'.startsWith(partial[0].replace(/\s/g, '').toLowerCase())
    ? cleaned.slice(0, partial.index)
    : cleaned;
  return result === content ? content : result.trimEnd();
}

/** An answered chat remains resumable; ordinary paused agent runs are not done. */
export function chatFollowUpSuggestions(
  run: { status: string; pause_reason?: string } | null | undefined,
  messages: ReadonlyArray<{ role: string; content: string; message_type?: string }>,
): string[] {
  if (run?.status !== 'completed' && !(run?.status === 'paused' && run.pause_reason === 'awaiting_user_message')) return [];
  for (let i = messages.length - 1; i >= 0; i--) {
    const message = messages[i];
    if (message.role === 'user' || message.message_type === 'review_checkpoint_resolution' || message.message_type === 'approval_request_resolution') return [];
    if (message.role === 'assistant' && message.message_type !== 'assistant_progress' && message.content.trim()) {
      return parseFollowUpSuggestions(message.content);
    }
  }
  return [];
}
