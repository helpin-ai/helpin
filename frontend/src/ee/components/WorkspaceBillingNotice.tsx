import type { ReactNode } from "react";
import { Link, Navigate, useLocation } from "@tanstack/react-router";
import { CalendarClock, CircleAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { WorkspaceBillingSummary } from "@/lib/types";
import {
  BILLING_CHOOSE_PLAN_SEARCH,
  BILLING_OVERVIEW_SEARCH,
} from "@/ee/lib/billingNavigation";

export function WorkspaceBillingGate({
  billing,
  slug,
  children,
}: {
  children: ReactNode;
  billing?: WorkspaceBillingSummary | null;
  slug: string;
}) {
  const location = useLocation();
  return billing?.locked && !location.pathname.endsWith("/settings/billing") ? (
    <Navigate
      to="/w/$slug/settings/billing"
      params={{ slug }}
      search={BILLING_OVERVIEW_SEARCH}
      replace
    />
  ) : (
    children
  );
}

export function WorkspaceBillingNotice({
  billing,
  slug,
  isOwner,
}: {
  billing?: WorkspaceBillingSummary | null;
  slug: string;
  isOwner: boolean;
}) {
  const location = useLocation();
  if (!billing || location.pathname.endsWith("/settings/billing")) return null;

  const isPaymentIssue =
    billing.status === "past_due" ||
    billing.status === "unpaid" ||
    billing.billing_notice_type === "payment_failed";
  const isTrialEnding = billing.billing_notice_type === "trial_will_end";
  if (!isPaymentIssue && !isTrialEnding) return null;

  const Icon = isPaymentIssue ? CircleAlert : CalendarClock;
  const title = isPaymentIssue
    ? "Payment needs attention"
    : "Trial ending soon";
  const message = isPaymentIssue
    ? billing.billing_notice_message ||
      "Update your payment method to keep this workspace active."
    : billing.billing_notice_message ||
      "Choose a plan to keep this workspace active after the trial.";
  const ownerCTA = isPaymentIssue ? "Update" : "Upgrade";
  const bannerClassName = isPaymentIssue
    ? "border-red-200 bg-red-50 text-red-900 dark:border-red-500/40 dark:bg-red-500/10 dark:text-red-200"
    : "border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200";

  return (
    <div className={`border-b px-4 py-2 text-sm ${bannerClassName}`}>
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-start gap-2">
          <Icon className="mt-0.5 h-4 w-4 shrink-0" />
          <div className="min-w-0">
            <p className="font-medium">{title}</p>
            <p className="mt-0.5 text-xs sm:text-sm">{message}</p>
          </div>
        </div>
        {isOwner ? (
          <Button
            asChild
            size="sm"
            variant="destructive"
            className="h-7 shrink-0 px-3 text-xs"
          >
            <Link
              to="/w/$slug/settings/billing"
              params={{ slug }}
              search={
                isPaymentIssue
                  ? BILLING_OVERVIEW_SEARCH
                  : BILLING_CHOOSE_PLAN_SEARCH
              }
            >
              {ownerCTA}
            </Link>
          </Button>
        ) : (
          <Button
            size="sm"
            variant="outline"
            className="h-7 shrink-0 bg-background px-3 text-xs"
            disabled
          >
            Ask owner
          </Button>
        )}
      </div>
    </div>
  );
}
