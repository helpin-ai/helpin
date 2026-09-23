import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { useLoadSampleData, useSampleDataStatus } from '@/hooks/queries/useSampleData';
import { DatabaseIcon, Loading01Icon } from '@/lib/icons';
import { OnboardingActions } from './OnboardingShell';

type FinishStepProps = {
  workspaceId: string;
  /** Whether the person may load sample data (workspace.update). */
  canLoadSampleData: boolean;
  onGoToWorkspace: () => void;
};

/**
 * Last step. The workspace is ready; people who can manage it may first load
 * the Northwind Outfitters sample data to explore with.
 */
export function FinishStep({ workspaceId, canLoadSampleData, onGoToWorkspace }: FinishStepProps) {
  const status = useSampleDataStatus(canLoadSampleData ? workspaceId : undefined);
  const load = useLoadSampleData(workspaceId);
  const [error, setError] = useState<string | null>(null);
  const offerSampleData = canLoadSampleData && Boolean(status.data) && !status.data?.loaded && (status.data?.modules.length ?? 0) > 0;

  const exploreWithSampleData = async () => {
    setError(null);
    try {
      await load.mutateAsync();
      onGoToWorkspace();
    } catch (caught) {
      const message = caught instanceof Error && caught.message ? caught.message : 'try again';
      setError(`Sample data couldn’t be loaded: ${message.replace(/[.!?]$/, '')}. You can load it later from the Setup guide.`);
    }
  };

  return (
    <div className="space-y-7">
      {offerSampleData && (
        <p className="text-sm leading-relaxed text-muted-foreground">
          Want something to click around first? Sample data adds a small fictional company, Northwind Outfitters. You can remove it in one step.
        </p>
      )}
      <p role="status" aria-live="polite" className="text-sm text-destructive empty:sr-only">{error ?? ''}</p>
      <OnboardingActions>
        {offerSampleData && (
          <Button type="button" variant="outline" className="w-full sm:w-auto" disabled={load.isPending} onClick={() => void exploreWithSampleData()}>
            {load.isPending
              ? <Loading01Icon className="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
              : <DatabaseIcon className="mr-2 h-4 w-4" aria-hidden="true" />}
            {load.isPending ? 'Loading sample data…' : 'Explore with sample data'}
          </Button>
        )}
        <Button type="button" className="w-full sm:w-auto sm:min-w-40" disabled={load.isPending} onClick={onGoToWorkspace}>
          Go to your workspace
        </Button>
      </OnboardingActions>
    </div>
  );
}
