import { useEffect, useRef, useState } from 'react';
import { Button } from '@/components/ui/button';
import { externalMCPService } from '@/lib/services/externalMCPService';

type CallbackQuery = { state: string; code: string; error: string };

export function ExternalMCPCallbackPage({ query }: { query: CallbackQuery }) {
  const [failed, setFailed] = useState(false);
  // Strict Mode may repeat effects. Reuse the request so single-use OAuth state
  // is exchanged once, while the active effect still handles its result.
  const pending = useRef<ReturnType<typeof externalMCPService.completeOAuth> | null>(null);

  useEffect(() => {
    if (!query.state) return;
    let active = true;
    pending.current ??= externalMCPService.completeOAuth(query);
    void pending.current.then((response) => {
      if (!active) return;
      if (response.error || !response.data?.redirect_url) {
        setFailed(true);
        return;
      }
      const destination = new URL(response.data.redirect_url, window.location.origin);
      if (destination.origin !== window.location.origin) {
        setFailed(true);
        return;
      }
      window.location.replace(destination.href);
    }).catch(() => { if (active) setFailed(true); });
    return () => { active = false; };
  }, [query]);

  const invalid = failed || !query.state;
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 px-4 text-center">
      <h1 className="text-lg font-semibold">{invalid ? 'Could not finish connecting' : 'Finishing connection…'}</h1>
      {invalid ? (
        <>
          <p className="text-sm text-muted-foreground">Return to your workspace and connect again using the account that started the connection.</p>
          <Button asChild variant="outline"><a href="/workspaces">Back to workspaces</a></Button>
        </>
      ) : <p role="status" className="text-sm text-muted-foreground">You’ll be returned to your workspace shortly.</p>}
    </main>
  );
}
