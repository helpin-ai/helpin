// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { TranscriptSegmentView } from '../segmentRenderers';
import type { TranscriptSegment } from '../segments';

const mocks = vi.hoisted(() => ({
  resolveTeamMemberAvatarSrc: vi.fn(),
}));

vi.mock('@/lib/teamMemberAvatar', () => ({
  resolveTeamMemberAvatarSrc: mocks.resolveTeamMemberAvatarSrc,
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  mocks.resolveTeamMemberAvatarSrc.mockReset();
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
  it('renders a successful tool as a static one-liner with its duration', () => {
    render(toolSegment, true);

    expect(container.textContent).toContain('Run go build ./...');
    expect(container.textContent).toContain('2s');
    expect(container.querySelector('button')).toBeNull();
    expect(container.querySelector('[aria-expanded]')).toBeNull();
    expect(container.textContent).not.toContain('▸');
    expect(container.textContent).not.toContain('▾');
  });

  it('renders a successful tool with no args or result as a flat, non-expandable one-liner', () => {
    const bareToolSegment: TranscriptSegment = {
      kind: 'tool',
      id: 'tool-bare',
      toolCall: {
        tool_call_id: 'tc-bare',
        tool_name: 'run_command',
        args_text: '',
        status: 'completed',
      },
    };
    render(bareToolSegment, true);

    expect(container.querySelector('button')).toBeNull();
    expect(container.querySelector('div.hidden')).toBeNull();
  });

  it('keeps failed tool output out of the transcript row', () => {
    render(failedToolSegment, true);

    expect(container.querySelector('button')).toBeNull();
    expect(container.textContent).not.toContain('build failed: undefined symbol');
  });

  it('keeps apply_patch as a static one-liner without an inline diff', () => {
    render(applyPatchSegment, true);

    expect(container.querySelector('button')).toBeNull();
    expect(container.querySelector('[aria-expanded]')).toBeNull();
    expect(container.textContent).not.toContain('*** Begin Patch');
  });

  it('still shows duration when the surrounding transcript is non-expandable', () => {
    render(toolSegment, false);

    expect(container.textContent).toContain('Run go build ./...');
    expect(container.textContent).toContain('2s');
    expect(container.querySelector('button')).toBeNull();
    expect(container.querySelector('div.hidden')).toBeNull();
  });
});

describe('TranscriptSegmentView — reasoning', () => {
  it('shows a collapsed one-line "Thought" row that reveals the reasoning text', () => {
    const segment: TranscriptSegment = {
      kind: 'reasoning',
      id: 'reasoning-1',
      reasoning: { message_id: 'r1', content: 'Considering the edge cases.', status: 'completed' },
    };
    render(segment, true);

    expect(container.textContent).toContain('Thought');
    const toggle = container.querySelector('button');
    expect(toggle).not.toBeNull();
    act(() => toggle?.click());
    expect(container.textContent).toContain('Considering the edge cases.');
  });

  it('labels completed reasoning with its duration when timestamps are present', () => {
    const segment: TranscriptSegment = {
      kind: 'reasoning',
      id: 'reasoning-2',
      reasoning: {
        message_id: 'r2',
        content: 'Weighing options.',
        status: 'completed',
        started_at: '2026-08-05T07:00:00Z',
        completed_at: '2026-08-05T07:00:12Z',
      },
    };
    render(segment, true);

    expect(container.textContent).toContain('Thought for 12s');
  });
});

describe('TranscriptSegmentView — user', () => {
  it('uses the Dock fallback label and renders the message bubble', () => {
    const segment: TranscriptSegment = {
      kind: 'user',
      id: 'user-1',
      message: {
        event_id: 'user-1',
        role: 'user',
        message_type: 'message',
        content: 'Inspect the repository and create a document.',
        timestamp: '2026-08-05T07:00:00Z',
        sequence_no: 1,
      },
    };
    act(() => {
      root.render(
        <TranscriptSegmentView
          segment={segment}
          options={{ expandable: false, fallbackUserLabel: 'You' }}
        />,
      );
    });

    expect(container.textContent).toContain('You');
    expect(container.textContent).toContain('Inspect the repository and create a document.');
  });

  it('renders the actor\'s configured avatar', () => {
    const segment: TranscriptSegment = {
      kind: 'user',
      id: 'user-avatar',
      message: {
        event_id: 'user-avatar',
        role: 'user',
        message_type: 'message',
        content: 'Use my saved avatar.',
        timestamp: '2026-08-05T07:00:00Z',
        sequence_no: 1,
      },
    };

    act(() => {
      root.render(
        <TranscriptSegmentView
          segment={segment}
          options={{
            expandable: false,
            resolveActor: () => ({
              id: 'user-1',
              email: 'alice@example.com',
              full_name: 'Alice Johnson',
              avatar_style: 'personas',
              avatar_seed: 'alice-seed',
              avatar_background_mode: 'color',
              avatar_background_color: '#fbbf24',
            }),
          }}
        />,
      );
    });

    expect(mocks.resolveTeamMemberAvatarSrc).toHaveBeenCalledWith({
      avatarUrl: undefined,
      avatarStyle: 'personas',
      avatarSeed: 'alice-seed',
      avatarBackgroundMode: 'color',
      avatarBackgroundColor: '#fbbf24',
      fallbackSeed: 'Alice Johnson',
    });
  });
});
