import { KeyRound, SquareTerminal } from 'lucide-react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { CodingSession, CodingSessionInteraction } from '@/lib/pmTypes';
import { CodingInteractionCard } from './CodingInteractionCard';

export function CodingInterruptionPanel({
  session,
  activeInteraction,
  acting,
  onAuthStart,
  onAuthCancel,
  onResolveInteraction,
  onCancelRun,
}: {
  session: CodingSession | null;
  activeInteraction?: CodingSessionInteraction | null;
  acting: string | null;
  onAuthStart: () => void;
  onAuthCancel: () => void;
  onResolveInteraction: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
  onCancelRun: () => void;
}) {
  return (
    <section className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-border bg-card shadow-sm">
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <SquareTerminal className="h-3.5 w-3.5" />
          Session cockpit
        </div>
        {session ? (
          <Badge variant="outline" className="text-[10px]">
            {session.status}
          </Badge>
        ) : null}
      </div>

      <div className="min-h-0 overflow-auto space-y-4 px-4 py-4">
        {session?.pause_reason === 'authentication' ? (
          <div className="rounded-lg border border-border bg-amber-50 p-4 dark:bg-amber-950/20">
            <div className="mb-2 flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              <KeyRound className="h-4 w-4" />
              Authentication gate
            </div>
            <div className="text-sm font-semibold">
              ChatGPT sign-in required
            </div>
            <p className="mt-1.5 text-sm text-muted-foreground">
              {session.auth_state?.verification_url ? 'Complete device sign-in to continue this session.' : 'Start sign-in to continue this session.'}
            </p>
            {session.auth_state?.user_code ? (
              <div className="mt-3 rounded-lg border border-border bg-background px-3 py-2 font-mono text-sm tracking-widest">
                {session.auth_state.user_code}
              </div>
            ) : null}
            <div className="mt-3 flex flex-wrap gap-2">
              <Button size="sm" onClick={onAuthStart} disabled={acting !== null}>
                Start sign-in
              </Button>
              {session.auth_state?.verification_url ? (
                <Button asChild variant="outline" size="sm">
                  <a href={session.auth_state.verification_url} target="_blank" rel="noreferrer">Open verification page</a>
                </Button>
              ) : null}
              {session.auth_state?.state === 'pending' ? (
                <Button variant="outline" size="sm" onClick={onAuthCancel} disabled={acting !== null}>
                  Cancel sign-in
                </Button>
              ) : null}
            </div>
          </div>
        ) : null}

        {activeInteraction ? (
          <CodingInteractionCard
            interaction={activeInteraction}
            acting={acting}
            onResolve={onResolveInteraction}
          />
        ) : null}

        {session ? (
          <div className="rounded-lg border border-border bg-muted/25 p-4">
            <div className="mb-3 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              Session snapshot
            </div>
            <div className="grid gap-3 sm:grid-cols-3">
              <SnapshotItem label="Status" value={session.status} />
              <SnapshotItem label="Pause reason" value={session.pause_reason} />
              <SnapshotItem label="Target" value={session.target_type} />
            </div>
            <div className="mt-3">
              <Button variant="outline" size="sm" onClick={onCancelRun} disabled={acting !== null || session.status === 'completed' || session.status === 'cancelled' || session.status === 'failed'}>
                Cancel run
              </Button>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function SnapshotItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="text-[11px] text-muted-foreground">{label}</div>
      <div className="mt-0.5 text-sm font-medium capitalize">{value.replaceAll('_', ' ')}</div>
    </div>
  );
}
