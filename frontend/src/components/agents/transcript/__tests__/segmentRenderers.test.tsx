// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { TranscriptSegmentView } from '../segmentRenderers';
import type { TranscriptSegment } from '../segments';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

function render(segment: TranscriptSegment, expandable: boolean) {
  act(() => {
    root.render(<TranscriptSegmentView segment={segment} options={{ expandable }} />);
  });
}

const toolSegment: TranscriptSegment = {
  kind: 'tool',
  id: 'tool-1',
  toolCall: {
    tool_call_id: 'tc-1',
    tool_name: 'run_command',
    args_text: '{"command":"go build ./..."}',
    status: 'completed',
    duration_ms: 1500,
  },
};

const failedToolSegment: TranscriptSegment = {
  kind: 'tool',
  id: 'tool-2',
  toolCall: {
    tool_call_id: 'tc-2',
    tool_name: 'run_command',
    args_text: '{"command":"go build ./..."}',
    status: 'failed',
    result: { content: 'build failed: undefined symbol' },
  },
};

const applyPatchSegment: TranscriptSegment = {
  kind: 'tool',
  id: 'tool-3',
  toolCall: {
    tool_call_id: 'tc-3',
    tool_name: 'apply_patch',
    args_text: ['*** Begin Patch', '*** Update File: a.ts', '@@', '-old', '+new', '*** End Patch'].join('\n'),
    status: 'completed',
  },
};

describe('TranscriptSegmentView — tool', () => {
  it('renders a successful tool with no further info as a flat, non-expandable one-liner', () => {
    render(toolSegment, true);

    expect(container.textContent).toContain('Run go build ./...');
    expect(container.querySelector('button')).toBeNull();
    expect(container.querySelector('div.hidden')).toBeNull();
  });

  it('makes a failed tool expandable but collapsed by default', () => {
    render(failedToolSegment, true);

    // Has a chevron/toggle, but the body is hidden until opened.
    const toggle = container.querySelector('button');
    expect(toggle).not.toBeNull();
    expect(container.querySelector('div.hidden')).not.toBeNull();

    act(() => toggle?.click());
    expect(container.querySelector('div.hidden')).toBeNull();
    expect(container.textContent).toContain('build failed: undefined symbol');
  });

  it('expands a successful apply_patch by default and shows the diff', () => {
    render(applyPatchSegment, true);

    expect(container.querySelector('button')).not.toBeNull();
    expect(container.querySelector('div.hidden')).toBeNull();
    expect(container.textContent).toContain('a.ts');
  });

  it('renders no expandable body and no toggle when expandable is false (dock)', () => {
    render(failedToolSegment, false);

    expect(container.textContent).toContain('Run go build ./...');
    expect(container.querySelector('button')).toBeNull();
    expect(container.querySelector('div.hidden')).toBeNull();
  });
});

describe('TranscriptSegmentView — reasoning', () => {
  it('shows a collapsed one-line "Thinking" row that reveals the reasoning text', () => {
    const segment: TranscriptSegment = {
      kind: 'reasoning',
      id: 'reasoning-1',
      reasoning: { message_id: 'r1', content: 'Considering the edge cases.', status: 'completed' },
    };
    render(segment, true);

    expect(container.textContent).toContain('Thinking');
    const toggle = container.querySelector('button');
    expect(toggle).not.toBeNull();
    act(() => toggle?.click());
    expect(container.textContent).toContain('Considering the edge cases.');
  });
});
