import { useCallback, useEffect, useRef } from 'react';
import { useRouter } from '@tanstack/react-router';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { useAppRefreshGuard } from './useAppRefreshGuard';

/** Keep unsaved editors mounted, and refuse the release-recovery refresh. */
export function useUnsavedChangesGuard(
  dirty: boolean | (() => boolean),
  refreshMessage = 'Save or discard your changes before refreshing.',
) {
  const latest = useRef(dirty);
  latest.current = dirty;
  const router = useRouter({ warn: false });
  const confirm = useConfirm();
  const isDirty = useCallback(() => typeof latest.current === 'function' ? latest.current() : latest.current, []);

  useAppRefreshGuard(() => isDirty() ? refreshMessage : null);
  useEffect(() => {
    const protect = (event: BeforeUnloadEvent) => {
      if (!isDirty()) return;
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', protect);
    // Editors also run in standalone previews, without a router. Browser and
    // release-refresh protection still apply there.
    const unblock = router?.history.block({
      enableBeforeUnload: false,
      blockerFn: async () => {
        if (!isDirty()) return false;
        return !await confirm({
          title: 'Leave without saving?',
          description: 'Your latest changes may be lost. Stay here to save your work before leaving.',
          confirmText: 'Leave without saving',
          cancelText: 'Stay here',
          variant: 'destructive',
        });
      },
    });
    return () => { unblock?.(); window.removeEventListener('beforeunload', protect); };
  }, [router, confirm, isDirty]);
}
