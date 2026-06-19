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
}

function buildChart(usage: UsageResponse, mode: UsageMode) {
  // Feature keys ordered by total credits desc (matches table order).
  const featureKeys = usage.features.map((f) => f.feature_key);
  const labelByKey = new Map(usage.features.map((f) => [f.feature_key, f.label]));

  let running: Record<string, number> = {};
  const points = usage.series.map((pt) => {
    const segs = featureKeys.map((key, i) => {
      const dayVal = pt.features[key] ?? 0;
      const value = mode === 'cumulative' ? (running[key] ?? 0) + dayVal : dayVal;
      if (mode === 'cumulative') running = { ...running, [key]: value };
      return { key, label: labelByKey.get(key) ?? key, value, color: featureColor(i) };
    });
    const total = segs.reduce((s, x) => s + x.value, 0);
    return { date: pt.date, segs, total };
  });
  const max = Math.max(1, ...points.map((p) => p.total));
  return { points, max, featureKeys, labelByKey };
}

export function UsageDetail({ workspaceId }: Props) {
  const [mode, setMode] = useState<UsageMode>('daily');
  const period = useMemo(() => dayjs().format('YYYY-MM'), []);
  const { data: usage, isLoading } = useWorkspaceUsage(workspaceId, period, mode);

  if (isLoading || !usage) {
    return <Skeleton className="h-64 w-full" />;
  }

  const { points, max } = buildChart(usage, mode);
  const usedPct = usage.included_credits
    ? Math.min(100, Math.round((usage.credits_used / usage.included_credits) * 100))
    : 0;

  return (
    <div className="space-y-5">
      {/* Headline + mode switch */}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <div className="text-2xl font-semibold tabular-nums">
            {formatNumber(usage.credits_used)}
            <span className="text-base font-normal text-muted-foreground">
              {' '}
              / {formatNumber(usage.included_credits)} credits
            </span>
          </div>
          <div className="text-xs text-muted-foreground">{usedPct}% of included credits used</div>
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
              title={`${dayjs(p.date).format('MMM D')}: ${formatNumber(p.total)} credits`}
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
          {usage.features.map((f, i) => (
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
              <TableHead className="text-right">Cost</TableHead>
              <TableHead className="text-right">Usage</TableHead>
              <TableHead className="text-right">Total credits (%)</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {usage.features.map((f, i) => (
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
                  {f.cost > 0 ? `${f.cost} / action` : 'variable'}
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
