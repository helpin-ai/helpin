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
import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { featureColor, formatNumber } from '@/ee/lib/billingUtils';
import { formatAIUsagePercent } from '@/lib/aiUsage';
import type { UsageMode, UsageResponse } from '@/ee/lib/billingTypes';
import { useWorkspaceUsage } from '@/ee/hooks/queries/useBilling';

interface Props {
  workspaceId: string;
  periodStart?: string;
  periodEnd?: string;
  mockUsage?: UsageResponse;
}

export function buildUsageChart(usage: UsageResponse, mode: UsageMode) {
  const features = Array.isArray(usage.features) ? usage.features : [];
  const series = Array.isArray(usage.series) ? usage.series : [];
  // Feature keys ordered by total usage desc (matches table order).
  const featureKeys = features.map((f) => f.feature_key);
  const labelByKey = new Map(features.map((f) => [f.feature_key, f.label]));
  const apiAlreadyCumulative = usage.mode === 'cumulative';
  const allowance = Math.max(0, usage.ai_usage_allowance_microusd ?? 0);

  let running: Record<string, number> = {};
  const points = series.map((pt) => {
    const segs = featureKeys.map((key, i) => {
      const chargedMicrousd = pt.features?.[key] ?? 0;
      const dayVal = allowance > 0 ? (chargedMicrousd / allowance) * 100 : 0;
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

export function selectUsageDateTickIndexes(pointCount: number, maxTicks = 6) {
  if (pointCount <= 0) return [];
  if (pointCount <= maxTicks) return Array.from({ length: pointCount }, (_, index) => index);
  return Array.from({ length: maxTicks }, (_, index) => (
    Math.round((index * (pointCount - 1)) / (maxTicks - 1))
  ));
}

export function UsageDetail({ workspaceId, periodStart, periodEnd, mockUsage }: Props) {
  const [mode, setMode] = useState<UsageMode>('daily');
  const period = useMemo(() => {
    if (periodStart && periodEnd) return `${periodStart}..${periodEnd}`;
    return dayjs().format('YYYY-MM');
  }, [periodEnd, periodStart]);
  const { data: queriedUsage, isLoading, isError } = useWorkspaceUsage(
    workspaceId,
    period,
    mode,
    periodStart,
    periodEnd,
  );
  const usage = mockUsage ?? queriedUsage;

  if (isLoading && !mockUsage) {
    return <Skeleton className="h-64 w-full" />;
  }

  if ((isError && !mockUsage) || !usage) {
    return (
      <div className="rounded-xl border border-dashed p-8 text-center text-sm text-muted-foreground">
        Usage details couldn't be loaded. Only the billing owner can manage this
        workspace's billing, but usage should be visible to workspace admins — if
        this keeps happening, please refresh or contact support.
      </div>
    );
  }

  const { points, max } = buildUsageChart(usage, mode);
  const chartColumnCount = Math.max(1, points.length);
  const tickIndexes = new Set(selectUsageDateTickIndexes(points.length));
  const crossesYears = points.length > 1
    && dayjs(points[0]?.date).year() !== dayjs(points[points.length - 1]?.date).year();
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
        <div className="overflow-x-auto px-1 pb-1">
          <TooltipProvider delayDuration={100}>
            <div
              data-usage-chart-track
              className="grid h-40 w-full items-end justify-center gap-[3px]"
              style={{ gridTemplateColumns: `repeat(${chartColumnCount}, minmax(8px, 32px))` }}
            >
              {points.map((p) => {
                const activeSegments = p.segs.filter((segment) => segment.value > 0);
                const accessibleLabel = `${dayjs(p.date).format('MMMM D, YYYY')}: ${formatAIUsagePercent(p.total)} of allowance`;
                return (
                  <Tooltip key={p.date}>
                    <TooltipTrigger asChild>
                      <div
                        data-usage-bar-column
                        className="relative flex h-full min-w-0 flex-col justify-end outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
                        tabIndex={0}
                        aria-label={accessibleLabel}
                      >
                        <div className="flex w-full flex-col-reverse" style={{ height: `${(p.total / max) * 100}%` }}>
                          {activeSegments.map((segment) => (
                            <div
                              key={segment.key}
                              data-usage-bar-segment
                              style={{
                                height: `${(segment.value / Math.max(1, p.total)) * 100}%`,
                                backgroundColor: segment.color,
                              }}
                              className="w-full"
                            />
                          ))}
                        </div>
                      </div>
                    </TooltipTrigger>
                    <TooltipContent side="top" sideOffset={8} className="block min-w-44 space-y-2 p-3">
                      <div>
                        <p className="font-semibold">{dayjs(p.date).format('MMMM D, YYYY')}</p>
                        <p className="text-background/75">{formatAIUsagePercent(p.total)} of allowance</p>
                      </div>
                      {activeSegments.length > 0 ? (
                        <div className="space-y-1 border-t border-background/20 pt-2">
                          {activeSegments.map((segment) => (
                            <div key={segment.key} className="flex items-center justify-between gap-4">
                              <span className="flex min-w-0 items-center gap-1.5">
                                <span className="size-2 shrink-0" style={{ backgroundColor: segment.color }} />
                                <span className="truncate">{segment.label}</span>
                              </span>
                              <span className="tabular-nums">{formatAIUsagePercent(segment.value)}</span>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <p className="text-background/75">No usage</p>
                      )}
                    </TooltipContent>
                  </Tooltip>
                );
              })}
            </div>
          </TooltipProvider>
          <div
            aria-label="Usage period"
            className="mt-2 grid w-full justify-center gap-[3px] text-[10px] text-muted-foreground"
            style={{ gridTemplateColumns: `repeat(${chartColumnCount}, minmax(8px, 32px))` }}
          >
            {points.map((point, index) => (
              <div key={point.date} className="relative h-4 min-w-0">
                {tickIndexes.has(index) ? (
                  <span className="absolute left-1/2 -translate-x-1/2 whitespace-nowrap">
                    {dayjs(point.date).format(crossesYears ? 'MMM D, YY' : 'MMM D')}
                  </span>
                ) : null}
              </div>
            ))}
          </div>
        </div>
        {/* Legend */}
        <div className="mt-3 flex flex-wrap justify-center gap-x-4 gap-y-1 text-center">
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
              <TableHead>Model size</TableHead>
              <TableHead className="text-right">Actions</TableHead>
              <TableHead className="text-right">Share of allowance</TableHead>
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
                <TableCell>
                  <div className="flex flex-wrap gap-1">
                    {(f.model_tiers?.length ? f.model_tiers : f.model_tier ? [f.model_tier] : []).map((tier) => (
                      <Badge key={tier} variant="secondary" className="capitalize">
                        {tier}
                      </Badge>
                    ))}
                    {!f.model_tiers?.length && !f.model_tier && <span className="text-muted-foreground">—</span>}
                  </div>
                </TableCell>
                <TableCell className="text-right tabular-nums">{formatNumber(f.action_count ?? 0)}</TableCell>
                <TableCell className="text-right tabular-nums">
                  {formatAIUsagePercent(f.pct)}
                  {(f.estimated_count ?? 0) > 0 && <span className="ml-1 text-muted-foreground">estimated</span>}
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
