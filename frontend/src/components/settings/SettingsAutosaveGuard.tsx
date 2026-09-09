import { useEffect } from 'react';
import { useBlocker } from '@tanstack/react-router';
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter } from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';

/** Keep the editor mounted until its latest draft has finished saving. */
export function SettingsAutosaveGuard({ isDirty, error, onRetry }: {
  isDirty: boolean;
  error?: string;
  onRetry: () => void;
}) {
  const blocker = useBlocker({
    shouldBlockFn: () => isDirty,
    enableBeforeUnload: isDirty,
    withResolver: true,
  });
  useEffect(() => {
    if (blocker.status === 'blocked' && !isDirty) blocker.proceed();
  }, [blocker, isDirty]);

  return (
    <AlertDialog open={blocker.status === 'blocked'} onOpenChange={(open) => {
      if (!open && blocker.status === 'blocked') blocker.reset();
    }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{error ? 'Your changes haven’t been saved' : 'Saving your changes…'}</AlertDialogTitle>
          <AlertDialogDescription>
            {error ? 'Retry saving before leaving this page, or leave without saving your latest changes.' : 'You’ll continue automatically when saving is complete.'}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <Button variant="outline" onClick={() => blocker.status === 'blocked' && blocker.reset()}>Stay here</Button>
          {error && <Button variant="ghost" onClick={() => blocker.status === 'blocked' && blocker.proceed()}>Leave without saving</Button>}
          {error && <Button onClick={onRetry}>Retry</Button>}
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
