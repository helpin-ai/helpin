import type { BillingStatus } from './billingTypes';

type BillingStateLike = {
  status: BillingStatus | string;
  locked?: boolean;
};

export function isBillingLocked(state: BillingStateLike | null | undefined): boolean {
  if (!state) return false;
  return Boolean(state.locked) || state.status === 'trial_expired' || state.status === 'unpaid' || state.status === 'canceled';
}

export function billingLockedCopy(state: BillingStateLike): { title: string; description: string } {
  if (state.status === 'trial_expired') {
    return {
      title: 'Trial ended',
      description: 'Choose Starter or Growth to reactivate this workspace.',
    };
  }
  if (state.status === 'unpaid') {
    return {
      title: 'Payment overdue',
      description: 'Update payment in Stripe to reactivate this workspace.',
    };
  }
  return {
    title: 'Workspace locked',
    description: 'This workspace subscription ended. Choose Starter or Growth to reactivate it.',
  };
}
