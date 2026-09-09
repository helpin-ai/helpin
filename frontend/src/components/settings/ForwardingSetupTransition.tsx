import { useEffect, useState, type ReactNode } from 'react';
import { Tick01Icon } from '@/lib/icons';

export function ForwardingSetupTransition({ verified, children }: { verified: boolean; children: ReactNode }) {
  const [showInstructions, setShowInstructions] = useState(!verified);
  useEffect(() => {
    if (!verified) {
      setShowInstructions(true);
      return;
    }
    const reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
    const timer = setTimeout(() => setShowInstructions(false), reducedMotion ? 0 : 300);
    return () => clearTimeout(timer);
  }, [verified]);

  return (
    <>
      {showInstructions && (
        <div
          className="grid transition-[grid-template-rows,opacity] duration-300 ease-in-out motion-reduce:transition-none"
          style={{ gridTemplateRows: verified ? '0fr' : '1fr', opacity: verified ? 0 : 1 }}
          inert={verified}
          aria-hidden={verified || undefined}
        >
          <div className="min-h-0 overflow-hidden">{children}</div>
        </div>
      )}
      <div role="status" aria-live="polite" aria-atomic="true">
        {verified && (
          <p className="ml-11 mt-2 flex items-center gap-1.5 text-xs text-emerald-700 dark:text-emerald-400 motion-safe:animate-in motion-safe:fade-in motion-safe:duration-300">
            <Tick01Icon className="size-3.5" aria-hidden="true" />Forwarding verified
          </p>
        )}
      </div>
    </>
  );
}
