import type { ReactNode } from "react";
import type { WorkspaceBillingSummary } from "@/lib/types";
import type { UpgradeRequiredReason } from "@/edition/contracts";
export type {
  UpgradeRequiredKind,
  UpgradeRequiredReason,
} from "@/edition/contracts";

// These extension points have no HTTP requests, prices, or payment UI in CE.
export function useWorkspaceBilling(_workspace?: string): {
  data: WorkspaceBillingSummary | undefined;
  isLoading: boolean;
} {
  return { data: undefined, isLoading: false };
}
export function WorkspaceBillingGate({
  children,
}: {
  children: ReactNode;
  billing?: WorkspaceBillingSummary | null;
  slug: string;
}) {
  return children;
}
export function WorkspaceBillingNotice(_props: {
  billing?: WorkspaceBillingSummary | null;
  slug: string;
  isOwner: boolean;
}) {
  return null;
}
export function TrialBanner(_props: { collapsed?: boolean }) {
  return null;
}
export function UpgradeRequiredDialog(_props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUpgrade?: () => void;
  reason: UpgradeRequiredReason | string | null;
}) {
  return null;
}
export { getUpgradeRequiredReason, isUpgradeRequiredError } from "./errors";
export function BillingSettingsPage(_props: {
  openPlanChooser?: boolean;
  onPlanChooserChange?: (open: boolean) => void;
}) {
  return null;
}
export function BillingPreviewRoute() {
  return null;
}
export function canRemoveHelpinBranding(
  _billing: { plan?: string; locked?: boolean } | null | undefined,
): boolean {
  return true;
}

// Route compatibility carries no commercial behavior; the CE route rejects entry.
export const BILLING_CHOOSE_PLAN_SEARCH = { choose_plan: true } as const;
export const BILLING_OVERVIEW_SEARCH = { choose_plan: undefined } as const;
export function shouldOpenBillingPlanChooser(
  search: { choose_plan?: unknown } | null | undefined,
): boolean {
  return search?.choose_plan === true || search?.choose_plan === "true";
}

export function workspaceBillingBadge(
  _billing?: WorkspaceBillingSummary | null,
): { label: string; className: string } | null {
  return null;
}

export function brandingDescription(_billing: unknown): string {
  return "Display branding in the widget footer.";
}
export function ContactLimitNotice(_props: {
  workspaceSlug: string;
  onBack?: () => void;
}) {
  return null;
}
export function isContactLimitError(_error: unknown): boolean {
  return false;
}
