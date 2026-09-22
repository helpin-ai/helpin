import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession';
import { useLoadSampleData, useRemoveSampleData, useSampleDataStatus } from '@/hooks/queries/useSampleData';
import { DatabaseIcon, Delete01Icon, Loading01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { describeSampleModules, sampleRemovalMessage, summarizeSampleCounts } from './sampleDataPresentation';

export type SampleDataCardProps = {
  workspaceId: string;
  /**
   * Whether the viewer may load or remove sample data (workspace.update).
   * When omitted it is read from the viewer's workspace permissions.
   */
  canManage?: boolean;
  className?: string;
};

type Result = { tone: 'positive' | 'negative'; message: string };

/**
 * Loads or removes the Northwind Outfitters sample workspace content so a new
 * workspace has something realistic to explore. Sample records never count
 * toward Setup guide progress.
 */
export function SampleDataCard({ workspaceId, canManage, className }: SampleDataCardProps) {
  const { data: access } = useWorkspaceAccess(canManage === undefined ? workspaceId : '');
  const { has } = usePermissions(access);
  const allowed = canManage ?? has('workspace.update');
  const statusQuery = useSampleDataStatus(workspaceId);
  const load = useLoadSampleData(workspaceId);
  const remove = useRemoveSampleData(workspaceId);
  const confirm = useConfirm();
  const [result, setResult] = useState<Result | null>(null);
  const status = statusQuery.data;
  const busy = load.isPending || remove.isPending;

  const onLoad = async () => {
    setResult(null);
    try {
      const loaded = await load.mutateAsync();
      setResult({ tone: 'positive', message: `Sample data loaded: ${summarizeSampleCounts(loaded) || 'nothing to add'}.` });
    } catch (error) {
      setResult({ tone: 'negative', message: `Couldn’t load sample data: ${errorMessage(error)}` });
    }
  };

  const onRemove = async () => {
    const confirmed = await confirm({
      title: 'Remove sample data?',
      description: 'This permanently deletes every sample record, including any changes you made to them. Your own records are not affected.',
      confirmText: 'Remove sample data',
      variant: 'destructive',
    });
    if (!confirmed) return;
    setResult(null);
    try {
      const removed = await remove.mutateAsync();
      setResult({ tone: 'positive', message: sampleRemovalMessage(removed) });
    } catch (error) {
      setResult({ tone: 'negative', message: `Couldn’t remove sample data: ${errorMessage(error)}` });
    }
  };

  const modules = status?.modules ?? [];
  const summary = status?.loaded ? summarizeSampleCounts(status) : '';
  const headingId = `sample-data-${workspaceId}`;

  return (
    <section aria-labelledby={headingId} className={cn('rounded-[10px] border border-quiet-field px-4 py-3.5', className)}>
      <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
        <div className="min-w-0 flex-1 space-y-1">
          <h3 id={headingId} className="text-[13.5px] font-semibold text-quiet-text-primary">
            Sample data
          </h3>
          {statusQuery.isLoading ? (
            <p className="text-[12.5px] text-quiet-text-tertiary">Checking for sample data…</p>
          ) : statusQuery.isError ? (
            <p className="text-[12.5px] text-quiet-text-tertiary">Sample data status is unavailable right now.</p>
          ) : status?.loaded ? (
            <p className="text-[12.5px] leading-5 text-quiet-text-secondary">
              Northwind Outfitters sample data is loaded{summary ? `: ${summary}` : ''}. It doesn’t count toward setup progress.
            </p>
          ) : modules.length > 0 ? (
            <p className="text-[12.5px] leading-5 text-quiet-text-secondary">
              Explore with a small fictional company, Northwind Outfitters: {describeSampleModules(modules)}. You can remove it in one step.
            </p>
          ) : (
            <p className="text-[12.5px] leading-5 text-quiet-text-tertiary">
              Sample data needs Projects, Docs, CRM or Support to be available to you.
            </p>
          )}
        </div>
        {status && allowed ? (
          status.loaded ? (
            <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void onRemove()}>
              {remove.isPending
                ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                : <Delete01Icon className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />}
              {remove.isPending ? 'Removing…' : 'Remove sample data'}
            </Button>
          ) : modules.length > 0 ? (
            <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void onLoad()}>
              {load.isPending
                ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" aria-hidden="true" />
                : <DatabaseIcon className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />}
              {load.isPending ? 'Loading…' : 'Load sample data'}
            </Button>
          ) : null
        ) : null}
      </div>
      {status && !allowed && (status.loaded || modules.length > 0) ? (
        <p className="mt-2 text-[12.5px] text-quiet-text-tertiary">
          A workspace admin can {status.loaded ? 'remove' : 'load'} sample data.
        </p>
      ) : null}
      <p
        role="status"
        aria-live="polite"
        className={cn(
          'mt-2 text-[12.5px] leading-5 empty:sr-only',
          result?.tone === 'positive' ? 'text-quiet-positive' : 'text-quiet-accent',
        )}
      >
        {result ? result.message : ''}
      </p>
    </section>
  );
}

function errorMessage(error: unknown): string {
  const text = error instanceof Error && error.message ? error.message : 'try again.';
  return /[.!?]$/.test(text) ? text : `${text}.`;
}
