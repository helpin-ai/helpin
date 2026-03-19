import type { PlanningSessionMessage } from '@/lib/pmTypes';

export interface ChatArtifact {
  type: 'spec_draft' | 'story_plan';
  label: string;
  content: string;
  messageId: string;
}

const ARTIFACT_PATTERNS: { type: ChatArtifact['type']; label: string; regex: RegExp }[] = [
  { type: 'spec_draft', label: 'Spec Draft', regex: /<spec_draft>([\s\S]*?)<\/spec_draft>/g },
  { type: 'story_plan', label: 'Story Plan', regex: /<story_plan>([\s\S]*?)<\/story_plan>/g },
];

/**
 * Scan messages and return the latest version of each artifact type.
 * Iterates in order so the last match wins (same as backend extractSpecDraft).
 */
export function extractArtifacts(messages: PlanningSessionMessage[]): ChatArtifact[] {
  const latest = new Map<ChatArtifact['type'], ChatArtifact>();

  for (const msg of messages) {
    if (msg.role !== 'assistant') continue;

    for (const { type, label, regex } of ARTIFACT_PATTERNS) {
      // Reset regex lastIndex since we reuse the same RegExp objects
      regex.lastIndex = 0;
      let match: RegExpExecArray | null;
      while ((match = regex.exec(msg.content)) !== null) {
        latest.set(type, {
          type,
          label,
          content: match[1].trim(),
          messageId: msg.id,
        });
      }
    }
  }

  return Array.from(latest.values());
}
