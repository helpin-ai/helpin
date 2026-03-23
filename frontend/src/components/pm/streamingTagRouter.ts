/**
 * Streaming-aware tag router for agent run output.
 *
 * Incrementally parses LLM text deltas and splits them into structured
 * segments so that content inside known tags (currently thinking)
 * is routed separately instead of appearing as raw markup in the chat bubble.
 */

const KNOWN_TAGS = ['thinking'] as const;
export type KnownTag = (typeof KNOWN_TAGS)[number];

const OPENING_TAGS: Record<KnownTag, string> = {
  thinking: '<thinking>',
};

const CLOSING_TAGS: Record<KnownTag, string> = {
  thinking: '</thinking>',
};

/** Maximum chars we hold back while trying to match a tag. */
const MAX_PENDING = 12; // longer than "</thinking>"

export interface StreamSegments {
  chatText: string;
  thinkingText: string;
  isThinking: boolean;
}

export const INITIAL_SEGMENTS: StreamSegments = {
  chatText: '',
  thinkingText: '',
  isThinking: false,
};

function segmentTextField(tag: KnownTag): keyof StreamSegments {
  switch (tag) {
    case 'thinking': return 'thinkingText';
  }
}

function segmentStreamingField(tag: KnownTag): keyof StreamSegments {
  switch (tag) {
    case 'thinking': return 'isThinking';
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
    if (buf.startsWith(opening)) {
      return { tag, consumed: opening.length };
    }
    if (opening.startsWith(buf) && buf.length < opening.length) {
      return { prefix: true };
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
  }

  finalize(): StreamSegments {
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
