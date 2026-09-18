// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import type { TranscriptSegment } from '@/components/agents/transcript';
import { DockAnswerSegment } from '../DockAnswerSegment';
import { useDockAnswerAnimation } from '../useDockAnswerAnimation';

vi.mock('@/components/agents/transcript', () => ({
  TranscriptSegmentView: ({ segment }: { segment: TranscriptSegment }) =>
    <p>{segment.kind === 'assistant' ? segment.content : null}</p>,
}));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let node: HTMLDivElement;
let root: Root;
const answer = (content: string): TranscriptSegment => ({ kind: 'assistant', id: 'answer', messageId: 'answer', final: true, content });
function Conversation({ segments, active }: { segments: TranscriptSegment[]; active: boolean }) {
  const animated = useDockAnswerAnimation(segments, active);
  return segments.map(segment => <DockAnswerSegment key={segment.kind === 'assistant' ? segment.messageId ?? segment.id : segment.id}
    segment={segment} options={{ expandable: false }} animate={animated(segment)} />);
}
beforeEach(() => {
  vi.useFakeTimers();
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  node = document.createElement('div'); document.body.append(node); root = createRoot(node);
});
afterEach(() => { act(() => root.unmount()); node.remove(); vi.useRealTimers(); vi.unstubAllGlobals(); });

it('reveals a newly delivered final progressively and continues appended chunks without restarting', () => {
  const progress: TranscriptSegment = { kind: 'assistant', id: 'progress', content: 'Checking', progress: true };
  act(() => root.render(<Conversation segments={[progress]} active />));
  const content = 'Here is the final answer, with useful details and a clear next step.';
  act(() => root.render(<Conversation segments={[answer(content)]} active={false} />));
  expect(node.textContent).toBe('');
  act(() => vi.advanceTimersByTime(32));
  expect(node.textContent!.length).toBeGreaterThan(0);
  expect(node.textContent!.length).toBeLessThan(content.length);
  const prefix = node.textContent!;
  act(() => root.render(<Conversation segments={[{ ...answer(content + ' More details.'), id: 'saved-answer' }]} active={false} />));
  expect(node.textContent).toBe(prefix);
  for (let i = 0; i < 65; i++) act(() => vi.advanceTimersByTime(32));
  expect(node.textContent).toBe(content + ' More details.');
  expect(node.querySelector('[aria-busy="true"]')).toBeNull();
});

it('shows saved history immediately and does not replay it when the run becomes active', () => {
  act(() => root.render(<Conversation segments={[answer('Saved answer')]} active={false} />));
  expect(node.textContent).toBe('Saved answer');
  act(() => root.render(<Conversation segments={[answer('Saved answer')]} active />));
  expect(node.textContent).toBe('Saved answer');
  expect(node.querySelector('[aria-busy="true"]')).toBeNull();
});

it('honors reduced motion and displays authoritative corrections without stale text', () => {
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
  act(() => root.render(<DockAnswerSegment segment={answer('Full answer')} options={{ expandable: false }} animate />));
  expect(node.textContent).toBe('Full answer');
  act(() => root.render(<DockAnswerSegment segment={answer('Corrected answer')} options={{ expandable: false }} animate />));
  expect(node.textContent).toBe('Corrected answer');
});

it('does not animate older answers paged in while the current turn is running', () => {
  const user: TranscriptSegment = { kind: 'user', id: 'new-question', message: {
    event_id: 'new-question', role: 'user', content: 'Continue?', timestamp: '', sequence_no: 2,
  } };
  const progress: TranscriptSegment = { kind: 'assistant', id: 'progress', content: 'Checking', progress: true };
  act(() => root.render(<Conversation segments={[user, progress]} active />));
  act(() => root.render(<Conversation segments={[answer('Earlier answer'), user, progress]} active />));
  expect(node.textContent).toContain('Earlier answer');
  expect(node.querySelector('[aria-busy="true"]')).toBeNull();
});
