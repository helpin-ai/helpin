import { lazy } from "react";
export { useWorkspaceBilling } from "@/ee/hooks/queries/useBilling";
export {
  WorkspaceBillingGate,
  WorkspaceBillingNotice,
} from "@/ee/components/WorkspaceBillingNotice";
export { TrialBanner } from "@/ee/components/layout/TrialBanner";
export { UpgradeRequiredDialog } from "@/ee/components/billing/UpgradeRequiredDialog";
export {
  getUpgradeRequiredReason,
  isUpgradeRequiredError,
} from "@/ee/lib/upgradeRequired";
export type {
  UpgradeRequiredKind,
  UpgradeRequiredReason,
} from "@/edition/contracts";
export {
  BILLING_CHOOSE_PLAN_SEARCH,
  BILLING_OVERVIEW_SEARCH,
  shouldOpenBillingPlanChooser,
} from "@/ee/lib/billingNavigation";
export {
  canRemoveHelpinBranding,
  brandingDescription,
} from "@/ee/lib/branding";
export const BillingSettingsPage = lazy(() =>
  import("@/ee/pages/settings/BillingSettingsPage").then((m) => ({
    default: m.BillingSettingsPage,
  })),
);
export const BillingPreviewRoute = lazy(() =>
  import("@/ee/pages/BillingPreviewRoute").then((m) => ({
    default: m.BillingPreviewRoute,
  })),
);

export { workspaceBillingBadge } from "@/ee/lib/workspaceBadge";
export {
  ContactLimitNotice,
  isContactLimitError,
} from "@/ee/components/ContactLimitNotice";
