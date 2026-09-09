import { describe, expect, it } from 'vitest';
import { forwardingStep, forwardingTestState } from '../emailForwardingProgress';
import type { SupportEmailRoute } from '@/lib/pmTypes';
const route = {} as SupportEmailRoute;
describe('forwarding progress evidence', () => {
  it('advances for confirmation receipt without assuming provider approval', () => {
    expect(forwardingStep({ ...route, confirmation_received_at: '2026-09-09' }, 1)).toBe(2);
    expect(forwardingStep(route, 3)).toBe(3);
    expect(forwardingStep(route, 4)).toBe(4);
  });
  it('restores the test stage from server evidence and only completes when verified', () => {
    expect(forwardingStep({ ...route, verification_sent_at: '2026-09-09' }, 1)).toBe(4);
    expect(forwardingStep({ ...route, forwarding_verified_at: '2026-09-09' }, 1)).toBe(5);
  });
  it('shows a delayed test after two minutes and prioritizes errors and verification', () => {
    const sent = { ...route, verification_sent_at: new Date(1000).toISOString() };
    expect(forwardingTestState(sent, 2000)).toBe('waiting');
    expect(forwardingTestState(sent, 121000)).toBe('delayed');
    expect(forwardingTestState({ ...sent, forwarding_last_error: 'failed' }, 2000)).toBe('error');
    expect(forwardingTestState({ ...sent, forwarding_verified_at: '2026-09-09' }, 121000)).toBe('verified');
  });
});
