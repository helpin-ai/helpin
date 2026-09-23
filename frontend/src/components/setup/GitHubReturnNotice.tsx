import { useEffect, useState } from 'react';
import type { GitHubReturnResult } from '@/lib/githubReturn';
import { SetupResultMessage, type SetupResult } from './CapabilityActions';

/**
 * Inline, announced result of a GitHub App create/install flow that returned
 * to this page. The live region mounts empty and is filled a tick later so
 * screen readers announce it.
 */
export function GitHubReturnNotice({ result, className }: { result: GitHubReturnResult | null; className?: string }) {
  const [shown, setShown] = useState<SetupResult | null>(null);

  useEffect(() => {
    if (!result) return;
    const timer = window.setTimeout(() => {
      setShown({ tone: result.status === 'error' ? 'negative' : 'positive', message: result.message });
    }, 0);
    return () => window.clearTimeout(timer);
  }, [result]);

  return <SetupResultMessage result={shown} className={className} />;
}
