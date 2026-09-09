import { useEffect, useState, type ReactNode } from 'react';

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
          <span className="sr-only">Forwarding verified</span>
        )}
      </div>
    </>
  );
}
