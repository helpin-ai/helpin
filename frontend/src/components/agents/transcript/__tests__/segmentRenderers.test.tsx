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

describe('TranscriptSegmentView — tool', () => {
  it('renders the action in a one-line label and keeps the body collapsed until clicked', () => {
    render(toolSegment, true);

    // The command itself is conveyed by the header label.
    expect(container.textContent).toContain('Run go build ./...');
    const body = container.querySelector('div.hidden');
    expect(body).not.toBeNull();

    const toggle = container.querySelector('button');
    act(() => toggle?.click());
    expect(container.querySelector('div.hidden')).toBeNull();
  });

  it('opens a failed tool by default and reveals its args + error output', () => {
    render(failedToolSegment, true);

    expect(container.querySelector('div.hidden')).toBeNull();
    expect(container.textContent).toContain('go build ./...');
    expect(container.textContent).toContain('build failed: undefined symbol');
  });

  it('renders no expandable body and no toggle when expandable is false (dock)', () => {
    render(toolSegment, false);

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
