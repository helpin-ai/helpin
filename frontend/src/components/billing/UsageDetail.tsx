import { useMemo, useState } from 'react';
import dayjs from 'dayjs';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { featureColor, formatNumber } from '@/lib/billingUtils';
import type { UsageMode, UsageResponse } from '@/lib/billingTypes';
import { useWorkspaceUsage } from '@/hooks/queries';

interface Props {
  workspaceId: string;
  periodStart?: string;
  periodEnd?: string;
}

export function buildUsageChart(usage: UsageResponse, mode: UsageMode) {
  const features = Array.isArray(usage.features) ? usage.features : [];
  const series = Array.isArray(usage.series) ? usage.series : [];
  // Feature keys ordered by total usage desc (matches table order).
  const featureKeys = features.map((f) => f.feature_key);
  const labelByKey = new Map(features.map((f) => [f.feature_key, f.label]));
  const apiAlreadyCumulative = usage.mode === 'cumulative';

  let running: Record<string, number> = {};
  const points = series.map((pt) => {
    const segs = featureKeys.map((key, i) => {
      const dayVal = pt.features?.[key] ?? 0;
      const value = mode === 'cumulative' && !apiAlreadyCumulative ? (running[key] ?? 0) + dayVal : dayVal;
      if (mode === 'cumulative' && !apiAlreadyCumulative) running = { ...running, [key]: value };
      return { key, label: labelByKey.get(key) ?? key, value, color: featureColor(i) };
    });
    const total = segs.reduce((s, x) => s + x.value, 0);
    return { date: pt.date, segs, total };
  });
  const max = Math.max(1, ...points.map((p) => p.total));
  return { points, max, featureKeys, labelByKey };
}

export function UsageDetail({ workspaceId, periodStart, periodEnd }: Props) {
  const [mode, setMode] = useState<UsageMode>('daily');
  const period = useMemo(() => {
    if (periodStart && periodEnd) return `${periodStart}..${periodEnd}`;
    return dayjs().format('YYYY-MM');
  }, [periodEnd, periodStart]);
  const { data: usage, isLoading, isError } = useWorkspaceUsage(
    workspaceId,
    period,
    mode,
    periodStart,
    periodEnd,
  );

  if (isLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  if (isError || !usage) {
    return (
      <div className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">
        Usage details couldn't be loaded. Only the billing owner can manage this
        workspace's billing, but usage should be visible to workspace admins — if
        this keeps happening, please refresh or contact support.
      </div>
    );
  }

  const { points, max } = buildUsageChart(usage, mode);
  const features = Array.isArray(usage.features) ? usage.features : [];
  const periodLabel = formatUsagePeriod(usage.period_start, usage.period_end);

  return (
    <div className="space-y-5">
      {/* Headline + mode switch */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="space-y-1">
          <div className="text-sm font-semibold">
            {mode === 'cumulative' ? 'Cumulative AI usage' : 'Daily AI usage'}
          </div>
          <div className="text-xs text-muted-foreground">{periodLabel}</div>
        </div>
        <div className="flex items-center gap-1 rounded-lg bg-muted p-1 text-xs">
          {(['daily', 'cumulative'] as UsageMode[]).map((m) => (
            <button
              key={m}
              type="button"
              onClick={() => setMode(m)}
              className={cn(
                'rounded-md px-3 py-1 font-medium capitalize transition-colors',
                mode === m ? 'bg-background shadow-sm' : 'text-muted-foreground',
              )}
            >
              {m}
            </button>
          ))}
        </div>
      </div>

      {/* Stacked bar chart (CSS/flex) */}
      <div className="rounded-xl border bg-card p-4">
        <div className="flex h-40 items-end gap-[3px]">
          {points.map((p) => (
            <div
              key={p.date}
              className="group relative flex flex-1 flex-col justify-end"
              title={`${dayjs(p.date).format('MMM D')}: ${formatNumber(p.total)} usage units`}
            >
              <div className="flex w-full flex-col-reverse" style={{ height: `${(p.total / max) * 100}%` }}>
                {p.segs
                  .filter((s) => s.value > 0)
                  .map((s) => (
                    <div
                      key={s.key}
                      style={{
                        height: `${(s.value / Math.max(1, p.total)) * 100}%`,
                        backgroundColor: s.color,
                      }}
                      className="w-full first:rounded-t-sm"
                    />
                  ))}
              </div>
            </div>
          ))}
        </div>
        {/* Legend */}
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
          {features.length === 0 && (
            <span className="text-[11px] text-muted-foreground">No AI usage recorded for this period.</span>
          )}
          {features.map((f, i) => (
            <div key={f.feature_key} className="flex items-center gap-1.5 text-[11px]">
              <span
                className="inline-block size-2.5 rounded-sm"
                style={{ backgroundColor: featureColor(i) }}
              />
              <span className="text-muted-foreground">{f.label}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Per-feature table */}
      <div className="overflow-hidden rounded-xl border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Feature</TableHead>
              <TableHead className="text-right">Typical minimum</TableHead>
              <TableHead className="text-right">Actions</TableHead>
              <TableHead className="text-right">AI usage (%)</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {features.length === 0 && (
              <TableRow>
                <TableCell colSpan={4} className="py-8 text-center text-sm text-muted-foreground">
                  No feature usage yet.
                </TableCell>
              </TableRow>
            )}
            {features.map((f, i) => (
              <TableRow key={f.feature_key}>
                <TableCell>
                  <span className="inline-flex items-center gap-2">
                    <span
                      className="inline-block size-2.5 rounded-sm"
                      style={{ backgroundColor: featureColor(i) }}
                    />
                    {f.label}
                  </span>
                </TableCell>
                <TableCell className="text-right tabular-nums text-muted-foreground">
                  {f.cost > 0 ? `${f.cost} units` : 'variable'}
                </TableCell>
                <TableCell className="text-right tabular-nums">{formatNumber(f.usage)}</TableCell>
                <TableCell className="text-right tabular-nums">
                  {formatNumber(f.credits)}{' '}
                  <span className="text-muted-foreground">({Math.round(f.pct)}%)</span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  );
}

function formatUsagePeriod(start?: string, end?: string) {
  if (!start || !end) return 'Current billing period';
  const startDate = dayjs(start);
  const endDate = dayjs(end).subtract(1, 'day');
  if (!startDate.isValid() || !endDate.isValid()) return 'Current billing period';
  return `${startDate.format('MMM D')} - ${endDate.format('MMM D, YYYY')}`;
}
