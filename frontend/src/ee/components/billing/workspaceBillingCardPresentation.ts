import type { WorkspaceBillingCard as WSCard } from '@/ee/lib/billingTypes';

export type WorkspaceBillingCardAction = 'manage' | 'portal' | 'upgrade';

export type WorkspaceBillingCardPresentation = {
  isTrial: boolean;
  isFounderPlan: boolean;
  isPaymentIssue: boolean;
  isLocked: boolean;
  isActivePaid: boolean;
  primaryCtaLabel: string;
  primaryCtaAction: WorkspaceBillingCardAction;
  billedToLabel: string;
  showBilledToChange: boolean;
  extraUsageLabel: string;
};

export function getWorkspaceBillingCardPresentation(card: WSCard): WorkspaceBillingCardPresentation {
  const isTrial = card.trialing;
  const isFounderPlan = card.plan === 'founder';
  const isPaymentIssue = card.status === 'past_due' || card.status === 'unpaid';
  const isTrialExpired = card.status === 'trial_expired';
  const isLocked = card.locked || isTrialExpired || card.status === 'canceled' || card.status === 'unpaid';
  const isActivePaid = !isTrial && !isLocked && !isPaymentIssue && !isFounderPlan;
  const needsUpgrade = isTrial || isTrialExpired || card.status === 'canceled';

  let primaryCtaLabel = 'Manage';
  let primaryCtaAction: WorkspaceBillingCardAction = 'manage';
  if (isPaymentIssue) {
    primaryCtaLabel = 'Update payment';
    primaryCtaAction = 'portal';
  } else if (needsUpgrade) {
    primaryCtaLabel = 'Upgrade';
    primaryCtaAction = 'upgrade';
  } else if (isFounderPlan) {
    primaryCtaLabel = 'Usage';
  }

  let billedToLabel = 'Organization default card';
  if (card.payment_method) {
    billedToLabel = `${card.payment_method.brand} ···· ${card.payment_method.last4}`;
  } else if (isFounderPlan) {
    billedToLabel = 'No payment required';
  } else if (needsUpgrade) {
    billedToLabel = 'Upgrade to add billing';
  }

  let extraUsageLabel = 'Add usage past your plan limit';
  if (isFounderPlan) {
    extraUsageLabel = 'Not needed on Founder';
  } else if (needsUpgrade) {
    extraUsageLabel = 'Available after upgrade';
  } else if (isLocked) {
    extraUsageLabel = 'Available after activation';
  }

  return {
    isTrial,
    isFounderPlan,
    isPaymentIssue,
    isLocked,
    isActivePaid,
    primaryCtaLabel,
    primaryCtaAction,
    billedToLabel,
    showBilledToChange: card.can_manage && !isFounderPlan && !needsUpgrade,
    extraUsageLabel,
  };
}
