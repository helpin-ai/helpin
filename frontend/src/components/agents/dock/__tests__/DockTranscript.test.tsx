// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DockTranscript } from '../DockTranscript';
import type { CodingSessionStreamState } from '@/lib/pmTypes';
import { useAuthStore } from '@/stores/authStore';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;
const originalClipboard = navigator.clipboard;
const originalExecCommand = document.execCommand;

function userStream(): CodingSessionStreamState {
  return {
    transcript_messages: [
      {
        event_id: 'user-message-1',
        message_id: 'message-1',
        role: 'user',
        message_type: 'message',
        content: 'Which tasks are stale?',
        timestamp: '2026-08-05T10:00:00Z',
        sequence_no: 1,
      },
    ],
    live_turn_segments: [],
    live_reasoning_message: null,
  } as CodingSessionStreamState;
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  useAuthStore.setState({
    user: {
      id: 'user-1',
      email: 'yolanda@example.com',
      full_name: 'Yolanda Ortiz',
      avatar_url: 'https://cdn.example.com/yolanda.png',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
    loading: false,
    serverUnreachable: false,
  });
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  useAuthStore.setState({ user: null, loading: false, serverUnreachable: false });
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: originalClipboard,
  });
  document.execCommand = originalExecCommand;
  vi.restoreAllMocks();
});

describe('DockTranscript user messages', () => {
  it('places the current-user avatar to the right of the message bubble', () => {
    act(() => {
      root.render(<DockTranscript stream={userStream()} active={false} />);
    });

    const bubbleColumn = container.querySelector('[data-dock-user-bubble]');
    const avatar = container.querySelector('[data-dock-user-avatar]');

    expect(container.textContent).toContain('Which tasks are stale?');
    expect(avatar?.getAttribute('aria-label')).toBe('Yolanda Ortiz');
    expect(bubbleColumn?.nextElementSibling).toBe(avatar);
  });

  it('keeps right-aligned time and copy actions hidden until hover or keyboard focus', () => {
    act(() => {
      root.render(<DockTranscript stream={userStream()} active={false} />);
    });

    const actions = container.querySelector('[data-dock-message-actions]');
    const copyButton = container.querySelector('button[aria-label="Copy message"]');

    expect(actions?.className).toContain('justify-end');
    expect(actions?.className).toContain('opacity-0');
    expect(actions?.className).toContain('group-hover:opacity-100');
    expect(actions?.className).toContain('group-focus-within:opacity-100');
    expect(actions?.querySelector('time')?.getAttribute('datetime')).toBe('2026-08-05T10:00:00Z');
    expect(copyButton).not.toBeNull();
  });

  it('copies the message and exposes confirmation to assistive technology', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });

    act(() => {
      root.render(<DockTranscript stream={userStream()} active={false} />);
    });

    const copyButton = container.querySelector('button[aria-label="Copy message"]');
    await act(async () => {
      copyButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(writeText).toHaveBeenCalledWith('Which tasks are stale?');
    expect(container.querySelector('button[aria-label="Message copied"]')).not.toBeNull();
    expect(container.querySelector('[role="status"]')?.textContent).toBe('Copied');
  });

  it('announces when copying fails', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('clipboard denied'));
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
    document.execCommand = vi.fn(() => false);

    act(() => {
      root.render(<DockTranscript stream={userStream()} active={false} />);
    });

    const copyButton = container.querySelector('button[aria-label="Copy message"]');
    await act(async () => {
      copyButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(container.querySelector('[role="status"]')?.textContent).toBe('Copy failed');
  });
});
