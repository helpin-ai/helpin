import { useMemo, useState } from 'react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ArrowDown01Icon } from '@/lib/icons';
import type { CRMEnrichmentResult } from '@/lib/crmTypes';
import { cn } from '@/lib/utils';
import { useApplyEnrichmentSuggestion } from '@/hooks/queries/useCRM';

type EnrichmentSummaryProps = {
  result: CRMEnrichmentResult;
  compact?: boolean;
  workspaceId?: string;
};

type EnrichmentFieldResult = {
  field?: unknown;
  new_value?: unknown;
  old_value?: unknown;
  source_url?: unknown;
  confidence?: unknown;
  reason?: unknown;
  proposed_value?: unknown;
};

export function isVisibleEnrichmentResult(result: CRMEnrichmentResult) {
  return result.data?.dry_run !== true;
}

export function EnrichmentHistoryList({
  results,
  compact = false,
  workspaceId,
}: {
  results: CRMEnrichmentResult[];
  compact?: boolean;
  workspaceId?: string;
}) {
  const [showHistory, setShowHistory] = useState(false);
  const visibleResults = useMemo(
    () => [...results].filter(isVisibleEnrichmentResult).sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at)),
    [results],
  );
  const latest = visibleResults[0];
  const older = visibleResults.slice(1);

  if (!latest) return null;

  return (
    <div className={cn('space-y-2', compact ? 'text-[11px]' : 'text-xs')}>
      <div className={cn('rounded-md border', compact ? 'border-border/60 p-2.5' : 'p-3')}>
        <EnrichmentSummary result={latest} compact={compact} workspaceId={workspaceId} />
      </div>

      {older.length > 0 ? (
        <div className="space-y-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 w-full justify-between px-2 text-xs text-muted-foreground"
            onClick={() => setShowHistory((open) => !open)}
          >
            <span>{older.length} older enrichment{older.length === 1 ? '' : 's'}</span>
            <ArrowDown01Icon className={cn('h-3.5 w-3.5 transition-transform', showHistory && 'rotate-180')} />
          </Button>
          {showHistory ? (
            <div className="space-y-2 border-l border-border/70 pl-2">
              {older.map((result) => (
                <div key={result.id} className={cn('rounded-md border border-border/50 bg-muted/20', compact ? 'p-2' : 'p-3')}>
                  <EnrichmentSummary result={result} compact={compact} workspaceId={workspaceId} />
                </div>
              ))}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

export function EnrichmentSummary({ result, compact = false, workspaceId }: EnrichmentSummaryProps) {
  const data = result.data ?? {};
  const applied = asArray<EnrichmentFieldResult>(data.applied);
  const skipped = asArray<EnrichmentFieldResult>(data.skipped);
  const acceptedSuggestions = asArray<EnrichmentFieldResult>(data.accepted_suggestions);
  const reviewItems = skipped.filter((item) => item.reason !== 'accepted_suggestion');
  const requested = asArray<EnrichmentFieldResult>(data.requested_fields);
  const status = valueToString(data.status) || (applied.length > 0 ? 'applied' : 'recorded');
  const evidenceSummary = valueToString(data.evidence_summary || data.evidence);
  const companyName = valueToString(data.company_name);
  const domain = valueToString(data.domain);
  const createdCompany = data.created_company === true;
  const createdLink = data.created_link === true;

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-1.5">
        <Badge variant="outline" className="text-[10px]">
          {result.source}
        </Badge>
        <Badge variant={status === 'skipped' ? 'secondary' : 'default'} className="text-[10px]">
          {humanizeKey(status)}
        </Badge>
        <span className="text-[10px] text-muted-foreground">
          {Math.round(result.confidence * 100)}%
        </span>
      </div>

      {companyName ? (
        <div className="rounded-md bg-muted/40 px-2 py-1.5 text-xs">
          <div className="font-medium">{companyName}</div>
          <div className="mt-0.5 flex flex-wrap gap-x-2 gap-y-0.5 text-[11px] text-muted-foreground">
            {domain ? <span>{domain}</span> : null}
            <span>{createdCompany ? 'company created' : 'company reused'}</span>
            <span>{createdLink ? 'linked to contact' : 'already linked'}</span>
          </div>
        </div>
      ) : null}

      {evidenceSummary ? (
        <p className={cn('text-muted-foreground', compact ? 'line-clamp-2 text-[11px]' : 'text-xs')}>
          {evidenceSummary}
        </p>
      ) : null}

      {applied.length > 0 ? (
        <EnrichmentFieldList title="Applied" items={applied} tone="applied" compact={compact} />
      ) : null}
      {acceptedSuggestions.length > 0 ? (
        <EnrichmentFieldList title="Accepted suggestions" items={acceptedSuggestions} tone="applied" compact={compact} />
      ) : null}
      {reviewItems.length > 0 ? (
        <EnrichmentFieldList title="Needs review" items={reviewItems} tone="skipped" compact={compact} result={result} workspaceId={workspaceId} />
      ) : null}

      {applied.length === 0 && skipped.length === 0 && requested.length > 0 ? (
        <EnrichmentFieldList title="Requested" items={requested} tone="neutral" compact={compact} />
      ) : null}

      {applied.length === 0 && skipped.length === 0 && requested.length === 0 && !companyName ? (
        <PrimitiveDataRows data={data} compact={compact} />
      ) : null}
    </div>
  );
}

function EnrichmentFieldList({
  title,
  items,
  tone,
  compact,
  result,
  workspaceId,
}: {
  title: string;
  items: EnrichmentFieldResult[];
  tone: 'applied' | 'skipped' | 'neutral';
  compact: boolean;
  result?: CRMEnrichmentResult;
  workspaceId?: string;
}) {
  const applySuggestion = useApplyEnrichmentSuggestion(workspaceId ?? '');

  const handleApply = async (field: string) => {
    if (!workspaceId || !result) return;
    try {
      await applySuggestion.mutateAsync({ enrichmentId: result.id, field });
      toast.success('Enrichment suggestion applied');
    } catch {
      toast.error('Could not apply enrichment suggestion');
    }
  };

  return (
    <div className="space-y-1">
      <div className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
        {title}
      </div>
      <div className="space-y-1">
        {items.map((item, index) => (
          <div
            key={`${valueToString(item.field)}-${index}`}
            className={cn(
              'rounded-md border px-2 py-1.5',
              tone === 'applied' && 'border-emerald-200 bg-emerald-50/60 text-emerald-950',
              tone === 'skipped' && 'border-amber-200 bg-amber-50/60 text-amber-950',
              tone === 'neutral' && 'border-border bg-muted/30',
            )}
          >
            <div className="flex items-start justify-between gap-2 text-xs">
              <span className="font-medium">{humanizeKey(valueToString(item.field) || 'field')}</span>
              <span className={cn('text-right', compact && 'max-w-[9rem] truncate')}>
                {fieldDisplayValue(item)}
              </span>
            </div>
            {item.reason ? (
              <div className="mt-0.5 text-[10px] text-muted-foreground">
                {humanizeKey(valueToString(item.reason))}
              </div>
            ) : null}
            {item.reason === 'existing_value_protected' && workspaceId && result ? (
              <div className="mt-1.5 flex items-center justify-between gap-2">
                <span className="text-[10px] text-muted-foreground">Current value kept</span>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="h-6 px-2 text-[10px]"
                  disabled={applySuggestion.isPending}
                  onClick={() => void handleApply(valueToString(item.field))}
                >
                  Use suggested value
                </Button>
              </div>
            ) : null}
            {item.source_url ? (
              <a className="mt-1 block truncate text-[10px] text-muted-foreground underline-offset-2 hover:underline" href={valueToString(item.source_url)} target="_blank" rel="noreferrer">
                View source
              </a>
            ) : null}
          </div>
        ))}
      </div>
    </div>
  );
}

function PrimitiveDataRows({ data, compact }: { data: Record<string, unknown>; compact: boolean }) {
  const rows = Object.entries(data).filter(([, value]) => value !== null && value !== undefined && !isEmptyObject(value));
  if (rows.length === 0) return null;
  return (
    <div className="space-y-1">
      {rows.map(([key, value]) => (
        <div key={key} className="flex items-start justify-between gap-2 text-xs">
          <span className="shrink-0 text-muted-foreground">{humanizeKey(key)}</span>
          <span className={cn('text-right font-medium', compact && 'max-w-[10rem] truncate')}>
            {valueToString(value)}
          </span>
        </div>
      ))}
    </div>
  );
}

function fieldDisplayValue(item: EnrichmentFieldResult) {
  if (item.new_value !== undefined && item.new_value !== null) return valueToString(item.new_value);
  if (item.proposed_value !== undefined && item.proposed_value !== null) return valueToString(item.proposed_value);
  return valueToString((item as Record<string, unknown>).value);
}

function asArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : [];
}

function valueToString(value: unknown): string {
  if (value === null || value === undefined) return '';
  if (typeof value === 'string') return value;
  if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  if (Array.isArray(value)) return value.map(valueToString).filter(Boolean).join(', ');
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function humanizeKey(value: string) {
  return value.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase());
}

function isEmptyObject(value: unknown) {
  return typeof value === 'object' && value !== null && !Array.isArray(value) && Object.keys(value).length === 0;
}
