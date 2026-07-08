import type { WorkspaceBillingSummary } from '@/lib/types';
import { formatDate } from '@/lib/billingUtils';

export type BillingNoticeAction = 'portal' | 'upgrade';

export type BillingNoticePresentation = {
  kind: 'payment' | 'trial';
  title: string;
  message: string;
  actionLabel: string;
  action: BillingNoticeAction;
};

export function getBillingNoticePresentation(billing: WorkspaceBillingSummary): BillingNoticePresentation | null {
  if (billing.billing_notice_type === 'payment_failed' || billing.status === 'past_due') {
    return {
      kind: 'payment',
      title: 'Payment needs attention',
      message: billing.billing_notice_message || 'Payment failed. Update your payment method to keep this workspace active.',
      actionLabel: 'Update payment',
      action: 'portal',
    };
  }

  const trialEnd = billing.trial_will_end_at || billing.trial_ends_at;
  if (billing.billing_notice_type === 'trial_will_end' && trialEnd) {
    return {
      kind: 'trial',
      title: 'Trial ending soon',
      message: `Your Growth plan trial ends on ${formatDate(trialEnd)}. Upgrade to keep Growth plan limits active.`,
      actionLabel: 'Upgrade',
      action: 'upgrade',
    };
  }

  return null;
}
