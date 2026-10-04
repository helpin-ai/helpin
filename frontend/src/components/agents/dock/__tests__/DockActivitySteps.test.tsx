// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import type { TranscriptSegment } from '@/components/agents/transcript';
import { DockActivitySteps } from '../DockActivityTimeline';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const tool = (id: string, name = 'search_workspace', status: 'running' | 'completed' | 'failed' = 'completed'): TranscriptSegment => ({
  kind: 'tool', id, toolCall: { tool_call_id: id, tool_name: name, status, args_text: JSON.stringify({ query: id }),
    result: status === 'running' ? undefined : { content: `Result ${id}`, ...(status === 'failed' ? { error: `Failed ${id}` } : {}) } },
});
let container: HTMLDivElement;
let root: Root;
const render = (segments: TranscriptSegment[]) => act(() => root.render(<DockActivitySteps segments={segments} active />));
const rows = () => container.querySelectorAll('ol[aria-label="Activity steps"] > li');

beforeEach(() => {
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});
afterEach(() => { act(() => root.unmount()); container.remove(); });

describe('counted activity steps', () => {
  it('increments the same row while preserving its disclosure and each call’s details', () => {
    render([tool('first')]);
    const row = rows()[0];
    act(() => row.querySelector('button')?.click());
    render([tool('first'), tool('second', 'mcp__helpin__search_workspace', 'running')]);
    expect(rows()).toHaveLength(1);
    expect(rows()[0]).toBe(row);
    expect(row.querySelector('button')?.getAttribute('aria-expanded')).toBe('true');
    expect(row.textContent).toContain('×2');
    expect(row.getAttribute('data-state')).toBe('running');
    expect(row.textContent).toContain('Result first');
    expect(row.textContent).toContain('"query":"second"');
    render([tool('first'), tool('second'), tool('third')]);
    expect(rows()[0]).toBe(row);
    expect(row.textContent).toContain('×3');
    expect(row.textContent).toContain('Result third');
    expect(row.getAttribute('data-state')).toBe('completed');
  });

  it('starts a new row after another action, narration, or conversation boundary', () => {
    const final: TranscriptSegment = { kind: 'assistant', id: 'final', content: 'Done', final: true };
    const narration: TranscriptSegment = { kind: 'assistant', id: 'progress', content: 'Checking another source.', progress: true };
    const user: TranscriptSegment = { kind: 'user', id: 'next-request', message: { event_id: 'next-request', role: 'user', content: 'Search again', timestamp: '', sequence_no: 2 } };
    render([tool('one'), tool('two'), tool('read', 'read_document'), tool('three'), narration, tool('four'), final, tool('five'), user, tool('six')]);
    expect(rows()).toHaveLength(7);
    expect(Array.from(rows()).filter(row => row.textContent?.includes('×2'))).toHaveLength(1);
    expect(container.textContent).not.toContain('×3');
  });

  it('does not group different MCP servers with identically named actions', () => {
    render([tool('one', 'mcp__logs__search'), tool('two', 'mcp__crm__search')]);
    expect(rows()).toHaveLength(2);
  });

  it('makes failures visible and opens their details when a repeated call fails', () => {
    render([tool('one'), tool('two', 'search_workspace', 'running')]);
    expect(rows()).toHaveLength(1);
    expect(rows()[0].querySelector('button')?.getAttribute('aria-expanded')).toBe('false');
    render([tool('one'), tool('two', 'search_workspace', 'failed')]);
    expect(rows()[0].getAttribute('data-state')).toBe('failed');
    expect(rows()[0].textContent).toContain('1 failed');
    expect(rows()[0].textContent).toContain('Failed two');
    act(() => rows()[0].querySelector('button')?.click());
    render([tool('one'), tool('two', 'search_workspace', 'failed'), tool('three', 'search_workspace', 'failed')]);
    expect(rows()[0].textContent).toContain('2 failed');
    expect(rows()[0].querySelector('button')?.getAttribute('aria-expanded')).toBe('true');
  });
});
