import { Button } from '@/components/ui/button';
import { Loading01Icon, Tick01Icon } from '@/lib/icons';

export type SettingsSaveState = 'idle' | 'pending' | 'saving' | 'saved' | 'error';

export function SettingsSaveStatus({ status, error, onRetry }: {
  status: SettingsSaveState;
  error?: string;
  onRetry?: () => void;
}) {
  if (status === 'error') {
    return (
      <div className="flex min-w-0 flex-wrap items-center gap-2 text-sm text-destructive">
        <span role="alert" className="min-w-0 break-words">{error || 'Changes could not be saved.'}</span>
        {onRetry && <Button type="button" variant="ghost" size="sm" onClick={onRetry}>Retry</Button>}
      </div>
    );
  }
  return (
    <span role="status" aria-live="polite" aria-atomic="true" className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
      {status === 'saving' && <Loading01Icon aria-hidden="true" className="size-3.5 animate-spin" />}
      {status === 'saved' && <Tick01Icon aria-hidden="true" className="size-3.5" />}
      {status === 'pending' ? 'Unsaved changes' : status === 'saving' ? 'Saving…' : status === 'saved' ? 'Saved' : null}
    </span>
  );
}
