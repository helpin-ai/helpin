import { useEffect, useRef, useState } from 'react';
import { Tick01Icon, Loading01Icon } from '@/lib/icons';

interface SaveIndicatorProps {
  saving: boolean;
  error?: string | null;
  presentation?: 'default' | 'quiet';
}

export function SaveIndicator({ saving, error, presentation = 'default' }: SaveIndicatorProps) {
  const [justSaved, setJustSaved] = useState(false);
  const wasSaving = useRef(false);

  useEffect(() => {
    if (wasSaving.current && !saving) {
      setJustSaved(true);
      const timer = setTimeout(() => setJustSaved(false), 2500);
      return () => clearTimeout(timer);
    }
    wasSaving.current = saving;
  }, [saving]);

  const quiet = presentation === 'quiet';

  if (error) {
    return (
      <div className="flex min-w-0 items-center text-[11.5px]" aria-live="polite">
        <span className={quiet ? 'text-quiet-accent' : 'text-destructive'}>{error}</span>
      </div>
    );
  }

  return (
    <div className="flex min-w-0 items-center gap-1 text-[11.5px]" aria-live="polite">
      {saving ? (
        <span className={quiet ? 'inline-flex items-center gap-1.5 text-quiet-text-tertiary' : 'inline-flex items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-muted-foreground'}>
          <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
          Saving...
        </span>
      ) : justSaved ? (
        <span className={quiet ? 'inline-flex items-center gap-1.5 text-quiet-positive' : 'inline-flex items-center gap-1.5 rounded-full bg-emerald-100 px-2.5 py-1 text-emerald-700 animate-in fade-in zoom-in-95 duration-300 dark:bg-emerald-900/40 dark:text-emerald-400'}>
          <Tick01Icon className="h-3.5 w-3.5" />
          Saved
        </span>
      ) : (
        <span className={quiet ? 'inline-flex items-center gap-1 text-quiet-muted' : 'inline-flex items-center gap-1 text-muted-foreground'}>
          <Tick01Icon className="h-3 w-3" />
          All changes saved
        </span>
      )}
    </div>
  );
}
