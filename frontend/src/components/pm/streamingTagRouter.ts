/**
 * Streaming-aware XML tag router for agent run output.
 *
 * Incrementally parses LLM text deltas and splits them into structured
 * segments so that content inside known XML tags (spec_draft, story_plan,
 * questions, approval_request, thinking) is routed to the artifact panel
 * instead of appearing as raw markup in the chat bubble.
 */

const KNOWN_TAGS = ['thinking', 'spec_draft', 'story_plan', 'approval_request', 'questions'] as const;
export type KnownTag = (typeof KNOWN_TAGS)[number];

/** Opening tag strings (approval_request is prefix-matched due to attributes). */
const OPENING_TAGS: Record<KnownTag, string> = {
  thinking: '<thinking>',
  spec_draft: '<spec_draft>',
  story_plan: '<story_plan>',
  approval_request: '<approval_request',
  questions: '<questions>',
};

const CLOSING_TAGS: Record<KnownTag, string> = {
  thinking: '</thinking>',
  spec_draft: '</spec_draft>',
  story_plan: '</story_plan>',
  approval_request: '</approval_request>',
  questions: '</questions>',
};

/** Maximum chars we hold back while trying to match a tag. */
const MAX_PENDING = 22; // longer than "</approval_request>"

export interface StreamSegments {
  chatText: string;
  thinkingText: string;
  isThinking: boolean;
  specDraftText: string;
  isStreamingSpecDraft: boolean;
  storyPlanText: string;
  isStreamingStoryPlan: boolean;
  questionsText: string;
  isStreamingQuestions: boolean;
  approvalRequestText: string;
  isStreamingApproval: boolean;
}

export const INITIAL_SEGMENTS: StreamSegments = {
  chatText: '',
  thinkingText: '',
  isThinking: false,
  specDraftText: '',
  isStreamingSpecDraft: false,
  storyPlanText: '',
  isStreamingStoryPlan: false,
  questionsText: '',
  isStreamingQuestions: false,
  approvalRequestText: '',
  isStreamingApproval: false,
};

function segmentTextField(tag: KnownTag): keyof StreamSegments {
  switch (tag) {
    case 'thinking': return 'thinkingText';
    case 'spec_draft': return 'specDraftText';
    case 'story_plan': return 'storyPlanText';
    case 'questions': return 'questionsText';
    case 'approval_request': return 'approvalRequestText';
  }
}

function segmentStreamingField(tag: KnownTag): keyof StreamSegments {
  switch (tag) {
    case 'thinking': return 'isThinking';
    case 'spec_draft': return 'isStreamingSpecDraft';
    case 'story_plan': return 'isStreamingStoryPlan';
    case 'questions': return 'isStreamingQuestions';
    case 'approval_request': return 'isStreamingApproval';
  }
}

/**
 * Try to match the beginning of `buf` against known opening tags.
 * Returns:
 *   { tag, consumed } on definite match
 *   { prefix: true }  if buf is a prefix of a known tag (need more chars)
 *   { prefix: false }  if no known tag can match
 */
function tryMatchOpen(buf: string): { tag: KnownTag; consumed: number } | { prefix: boolean } {
  for (const tag of KNOWN_TAGS) {
    const opening = OPENING_TAGS[tag];
    if (tag === 'approval_request') {
      // <approval_request may be followed by attributes, then > or />
      if (buf.startsWith(opening)) {
        // We've matched "<approval_request" -- need to find > or />
        const rest = buf.slice(opening.length);
        const selfCloseIdx = rest.indexOf('/>');
        const closeIdx = rest.indexOf('>');
        if (selfCloseIdx >= 0 && (closeIdx < 0 || selfCloseIdx <= closeIdx)) {
          // Self-closing: capture attributes as content
          const attrContent = rest.slice(0, selfCloseIdx).trim();
          return { tag, consumed: opening.length + selfCloseIdx + 2 };
        }
        if (closeIdx >= 0) {
          return { tag, consumed: opening.length + closeIdx + 1 };
        }
        // Haven't seen > yet -- could still match
        return { prefix: true };
      }
      if (opening.startsWith(buf) && buf.length < opening.length) {
        return { prefix: true };
      }
    } else {
      if (buf.startsWith(opening)) {
        return { tag, consumed: opening.length };
      }
      if (opening.startsWith(buf) && buf.length < opening.length) {
        return { prefix: true };
      }
    }
  }
  return { prefix: false };
}

function tryMatchClose(buf: string, currentTag: KnownTag): { closed: boolean; consumed: number } | { prefix: boolean } {
  const closing = CLOSING_TAGS[currentTag];
  if (buf.startsWith(closing)) {
    return { closed: true, consumed: closing.length };
  }
  if (closing.startsWith(buf) && buf.length < closing.length) {
    return { prefix: true };
  }
  return { prefix: false };
}

export class StreamingTagRouter {
  private mode: 'text' | KnownTag = 'text';
  private pendingBuf = '';
  private segments: StreamSegments = { ...INITIAL_SEGMENTS };
  /** For self-closing approval_request, track whether we already captured it. */
  private approvalSelfClosed = false;

  feed(delta: string): StreamSegments {
    const work = this.pendingBuf + delta;
    this.pendingBuf = '';
    let i = 0;

    while (i < work.length) {
      if (this.mode === 'text') {
        if (work[i] === '<') {
          const rest = work.slice(i);
          const result = tryMatchOpen(rest);
          if ('tag' in result) {
            // Definite match
            this.mode = result.tag;
            (this.segments[segmentStreamingField(result.tag)] as boolean) = true;
            // For self-closing approval_request, handle specially
            if (result.tag === 'approval_request') {
              const tagContent = rest.slice(0, result.consumed);
              if (tagContent.endsWith('/>')) {
                // Self-closing: extract attributes as content
                const attrStart = '<approval_request'.length;
                const attrEnd = tagContent.length - 2; // before />
                this.segments.approvalRequestText += tagContent.slice(attrStart, attrEnd).trim();
                this.approvalSelfClosed = true;
                this.mode = 'text';
                (this.segments.isStreamingApproval as boolean) = false;
              }
            }
            i += result.consumed;
            continue;
          }
          if (result.prefix) {
            // Could still be a tag — hold in pending buffer
            if (rest.length >= MAX_PENDING) {
              // Too long to be a known tag, flush the '<'
              this.segments.chatText += '<';
              i += 1;
            } else {
              this.pendingBuf = rest;
              return this.snapshot();
            }
          } else {
            // Not a known tag
            this.segments.chatText += '<';
            i += 1;
          }
        } else {
          this.segments.chatText += work[i];
          i += 1;
        }
      } else {
        // Inside a known tag — look for closing tag
        if (work[i] === '<') {
          const rest = work.slice(i);
          const result = tryMatchClose(rest, this.mode);
          if ('closed' in result && result.closed) {
            (this.segments[segmentStreamingField(this.mode)] as boolean) = false;
            this.mode = 'text';
            i += result.consumed;
            continue;
          }
          if ('prefix' in result && result.prefix) {
            if (rest.length >= MAX_PENDING) {
              // Not actually the closing tag, flush '<' as content
              (this.segments[segmentTextField(this.mode)] as string) += '<';
              i += 1;
            } else {
              this.pendingBuf = rest;
              return this.snapshot();
            }
          } else {
            // Not the closing tag — just content
            (this.segments[segmentTextField(this.mode)] as string) += '<';
            i += 1;
          }
        } else {
          (this.segments[segmentTextField(this.mode)] as string) += work[i];
          i += 1;
        }
      }
    }

    return this.snapshot();
  }

  reset(): void {
    this.mode = 'text';
    this.pendingBuf = '';
    this.segments = { ...INITIAL_SEGMENTS };
    this.approvalSelfClosed = false;
  }

  finalize(fullText?: string): StreamSegments {
    // Flush any pending buffer
    if (this.pendingBuf) {
      if (this.mode === 'text') {
        this.segments.chatText += this.pendingBuf;
      } else {
        (this.segments[segmentTextField(this.mode)] as string) += this.pendingBuf;
      }
      this.pendingBuf = '';
    }
    // Mark all streaming as done
    this.segments.isThinking = false;
    this.segments.isStreamingSpecDraft = false;
    this.segments.isStreamingStoryPlan = false;
    this.segments.isStreamingQuestions = false;
    this.segments.isStreamingApproval = false;
    this.mode = 'text';
    return this.snapshot();
  }

  getSegments(): StreamSegments {
    return this.snapshot();
  }

  private snapshot(): StreamSegments {
    return { ...this.segments };
  }
}
