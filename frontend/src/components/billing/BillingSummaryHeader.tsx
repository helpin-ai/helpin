import { Badge } from '@/components/ui/badge';
import { formatCents, formatNumber } from '@/lib/billingUtils';
import type { OrgBillingSummary } from '@/lib/billingTypes';

function Stat({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="min-w-0">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-xl font-semibold tabular-nums">{value}</div>
      {hint && <div className="text-[11px] text-muted-foreground">{hint}</div>}
    </div>
  );
}

export function BillingSummaryHeader({ summary }: { summary: OrgBillingSummary }) {
  return (
    <div className="rounded-xl border bg-card p-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="grid flex-1 grid-cols-2 gap-x-8 gap-y-4 sm:grid-cols-4">
          <Stat
            label="Monthly spend"
            value={formatCents(summary.total_monthly_spend_cents)}
            hint="active paid subscriptions"
          />
          <Stat label="Paid workspaces" value={formatNumber(summary.paid_count)} />
          <Stat label="On trial" value={formatNumber(summary.trialing_count)} />
          <Stat
            label="Credits used"
            value={formatNumber(summary.credits_used)}
            hint={`of ${formatNumber(summary.included_credits_total)} included`}
          />
        </div>
        {!summary.setup_complete && (
          <Badge
            variant="outline"
            className="border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-400"
          >
            Finish billing setup
          </Badge>
        )}
      </div>
    </div>
  );
}
