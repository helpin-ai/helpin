import type { SupportEmailRoute } from '@/lib/pmTypes';
export type ForwardingProvider = 'gmail' | 'outlook' | 'other';
export interface ForwardingProgress { step: number; provider: ForwardingProvider; source: string | null }
export function forwardingStep(route: SupportEmailRoute, acknowledgedStep: number) {
  if (route.forwarding_verified_at) return 5;
  if (route.verification_sent_at) return 4;
  return Math.max(1, Math.min(4, acknowledgedStep), route.confirmation_received_at ? 2 : 1);
}
export function forwardingTestState(route: SupportEmailRoute, now: number) {
  if (route.forwarding_verified_at) return 'verified';
  if (route.forwarding_last_error) return 'error';
  if (!route.verification_sent_at) return 'idle';
  const sent = Date.parse(route.verification_sent_at);
  return !Number.isFinite(sent) || now - sent >= 120_000 ? 'delayed' : 'waiting';
}
export function readForwardingProgress(key: string): ForwardingProgress {
  const fallback: ForwardingProgress = { step: 1, provider: 'gmail', source: null };
  try {
    const value = JSON.parse(localStorage.getItem(key) || 'null');
    if (!value) return fallback;
    return {
      step: Number.isInteger(value.step) ? Math.max(1, Math.min(4, value.step)) : 1,
      provider: ['gmail', 'outlook', 'other'].includes(value.provider) ? value.provider : 'gmail',
      source: typeof value.source === 'string' ? value.source : null,
    };
  } catch { return fallback; }
}
