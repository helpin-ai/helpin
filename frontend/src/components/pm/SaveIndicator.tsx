import { useEffect, useRef, useState } from 'react';
import { Check, Loader2 } from 'lucide-react';

interface SaveIndicatorProps {
  saving: boolean;
  error?: string | null;
}

export function SaveIndicator({ saving, error }: SaveIndicatorProps) {
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

  return (
    <div className="flex items-center gap-1 text-xs">
      {saving ? (
        <span className="inline-flex items-center gap-1.5 rounded-full bg-muted px-2.5 py-1 text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          Saving...
        </span>
      ) : justSaved ? (
        <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-100 px-2.5 py-1 text-emerald-700 animate-in fade-in zoom-in-95 duration-300 dark:bg-emerald-900/40 dark:text-emerald-400">
          <Check className="h-3.5 w-3.5" />
          Saved
        </span>
      ) : (
        <span className="inline-flex items-center gap-1 text-muted-foreground">
          <Check className="h-3 w-3" />
          All changes saved
        </span>
      )}
      {error && <span className="ml-2 text-destructive">{error}</span>}
    </div>
  );
}
