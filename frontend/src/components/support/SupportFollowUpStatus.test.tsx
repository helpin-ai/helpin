// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SupportConversation } from '@/lib/pmTypes';
import { SupportFollowUpStatus } from './SupportFollowUpStatus';

vi.mock('@/hooks/queries/useSession', () => ({ useWorkspaceAccess: () => ({ data: {} }), usePermissions: () => ({ has: () => false }) }));
let container: HTMLDivElement;
let root: Root;
let client: QueryClient;
beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  client = new QueryClient();
});
afterEach(() => { act(() => root.unmount()); client.clear(); container.remove(); vi.unstubAllGlobals(); });
async function renderEpisode(episode: Partial<NonNullable<SupportConversation['ai_follow_up']>>) {
  const conversation = { id: 'c', workspace_id: 'ws', ai_follow_up: { status: 'waiting', sequence_version: 2, due_at: '2026-09-10T12:00:00Z', close_at: '2026-09-11T12:00:00Z', ...episode } } as SupportConversation;
  await act(async () => root.render(<QueryClientProvider client={client}><SupportFollowUpStatus conversation={conversation} /></QueryClientProvider>));
}
describe('Support follow-up status', () => {
  it('shows the second follow-up deadline before the final message is sent', async () => {
    await renderEpisode({});
    expect(container.textContent).toContain('Second follow-up scheduled for');
    expect(container.querySelector('time')?.dateTime).toBe('2026-09-10T12:00:00Z');
    expect(container.textContent).not.toContain('Closes if no reply by');
  });
  it('shows closure only after the second message is sent', async () => {
    await renderEpisode({ second_sent_at: '2026-09-10T12:00:00Z' });
    expect(container.textContent).toContain('Closes if no reply by');
    expect(container.querySelector('time')?.dateTime).toBe('2026-09-11T12:00:00Z');
  });
  it.each([1, undefined])('preserves closure for legacy version %s', async sequence_version => {
    await renderEpisode({ sequence_version });
    expect(container.textContent).toContain('Closes if no reply by');
    expect(container.querySelector('time')?.dateTime).toBe('2026-09-11T12:00:00Z');
  });
  it('keeps a genuine handoff distinct from technical failure', async () => {
    await renderEpisode({ status: 'handoff', reason: 'Unfinished work requires review' });
    expect(container.textContent).toContain('Follow-up requires a teammate');
    expect(container.textContent).toContain('Unfinished work requires review');
    expect(container.textContent).toContain('A teammate should review this conversation');
    expect(container.textContent).toContain('It will not close automatically');
    expect(container.textContent).not.toContain('technical issue');
    expect(container.querySelector('time')).toBeNull();
  });
  it.each([
    ['assessment_launch_timeout', 'The conversation review could not start in time.'],
    ['assessment_did_not_complete', 'The conversation review could not be completed.'],
    ['assessment_timeout', 'The conversation review took too long to complete.'],
    ['assessment_expired', 'The conversation review expired before it could finish.'],
    ['closing_notice_missing', 'The final reminder could not be prepared.'],
    ['follow_up_delivery_failed', 'The follow-up email could not be delivered.'],
    ['follow_up_delivery_unconfirmed', 'Delivery of the follow-up email could not be confirmed.'],
  ])('explains %s without implying customer escalation', async (reason, explanation) => {
    await renderEpisode({ status: 'failed', reason });
    expect(container.textContent).toContain('Follow-up stopped');
    expect(container.textContent).toContain(explanation);
    expect(container.textContent).toContain('Automatic follow-ups have stopped');
    expect(container.textContent).toContain('will not close automatically');
    expect(container.textContent).toContain('The customer was not notified about this failure');
    expect(container.textContent).not.toContain(reason);
    expect(container.textContent).not.toContain('teammate');
    expect(container.querySelector('time')).toBeNull();
  });
  it.each(['unknown_internal_error_code', undefined])('uses a readable fallback for an unrecognized reason %s', async reason => {
    await renderEpisode({ status: 'failed', reason });
    expect(container.textContent).toContain('A technical issue prevented this follow-up from continuing.');
    expect(container.textContent).not.toContain('unknown_internal_error_code');
    expect(container.textContent).not.toContain('teammate');
  });
});
