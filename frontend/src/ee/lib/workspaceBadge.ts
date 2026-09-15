import type { WorkspaceBillingSummary } from "@/lib/types";
import { daysUntil, PLAN_LABEL } from "./billingUtils";

function planDisplayName(plan: string): string {
  return /\bplan\b/i.test(plan) ? plan : `${plan} plan`;
}

export function workspaceBillingBadge(
  billing?: WorkspaceBillingSummary | null,
): {
  label: string;
  className: string;
} {
  if (!billing) {
    return {
      label: "Trial pending",
      className: "bg-muted text-muted-foreground border-transparent",
    };
  }

  if (billing.status === "past_due") {
    return {
      label: "Past due",
      className: "bg-destructive/10 text-destructive border-destructive/20",
    };
  }

  if (
    billing.locked ||
    billing.status === "trial_expired" ||
    billing.status === "canceled"
  ) {
    return {
      label: billing.status === "trial_expired" ? "Trial ended" : "Locked",
      className: "bg-destructive/10 text-destructive border-destructive/20",
    };
  }

  const plan = PLAN_LABEL[billing.plan] ?? billing.plan;
  const planName = planDisplayName(plan);
  if (billing.trialing) {
    const days = daysUntil(billing.trial_ends_at);
    return {
      label:
        days > 0 ? `${planName} trial · ${days}d left` : `${planName} trial`,
      className:
        "bg-amber-500/10 text-amber-700 border-amber-500/20 dark:text-amber-300",
    };
  }

  return {
    label: planName,
    className: billing.locked
      ? "bg-destructive/10 text-destructive border-destructive/20"
      : "bg-primary/10 text-primary border-primary/15",
  };
}
