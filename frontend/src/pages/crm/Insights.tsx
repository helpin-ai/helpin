import { useWorkspaceStore } from '@/stores/workspaceStore';
import { SuggestionsPanel } from '@/components/crm/SuggestionsPanel';
import { CRMSearchResults } from '@/components/crm/CRMSearchResults';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useBuyerSignals, useHealthScores } from '@/hooks/queries/useCRM';
import { Badge } from '@/components/ui/badge';
import { Activity, Heart } from 'lucide-react';
import { useTitle } from '@/hooks/useTitle';
import type { CRMBuyerSignal } from '@/lib/crmTypes';

function formatSignalSource(signal: CRMBuyerSignal) {
  switch (signal.source_type) {
    case 'email':
      return 'Email';
    case 'meeting':
      return 'Meeting';
    case 'support':
      return 'Support';
    case 'note':
      return 'Note';
    default:
      return 'Manual';
  }
}

export function InsightsPage() {
  useTitle('CRM Insights');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const { data: signalsData } = useBuyerSignals(wsId, {});
  const { data: healthData } = useHealthScores(wsId);

  const signals = signalsData?.data ?? [];
  const healthScores = healthData?.data ?? [];

  return (
    <div className="mx-auto max-w-5xl px-4 md:px-6">
      <div className="mb-6">
        <h1 className="text-xl font-medium">Insights</h1>
        <p className="text-sm text-muted-foreground">AI-powered intelligence across your CRM</p>
      </div>
      <div className="grid gap-6 md:grid-cols-2">
        <div className="space-y-6">
          <SuggestionsPanel workspaceId={wsId} />

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Recent Buyer Signals</CardTitle>
            </CardHeader>
            <CardContent>
              {signals.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-6 text-center">
                  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                    <Activity className="h-5 w-5 text-muted-foreground/50" />
                  </div>
                  <p className="mt-2 text-sm text-muted-foreground">No buyer signals detected yet</p>
                  <p className="text-xs text-muted-foreground/70">Signals will appear as CRM activity grows</p>
                </div>
              ) : (
                <div className="space-y-2">
                  {signals.slice(0, 10).map((signal) => (
                    <div key={signal.id} className="rounded-md border p-2.5">
                      <div className="flex items-start justify-between gap-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge variant="outline" className="text-xs capitalize">
                            {signal.signal_type.replace(/_/g, ' ')}
                          </Badge>
                          <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                            {formatSignalSource(signal)}
                          </Badge>
                          {signal.metadata?.message_direction ? (
                            <span className="text-xs capitalize text-muted-foreground">{signal.metadata.message_direction}</span>
                          ) : null}
                        </div>
                        <span className="text-xs text-muted-foreground">{Math.round(signal.confidence * 100)}%</span>
                      </div>
                      {signal.summary && (
                        <p className="mt-1 text-sm text-foreground">{signal.summary}</p>
                      )}
                      {signal.evidence_excerpt ? (
                        <p className="mt-1 line-clamp-3 rounded-sm border-l-2 border-border/70 pl-2 text-xs italic text-muted-foreground">
                          "{signal.evidence_excerpt}"
                        </p>
                      ) : null}
                      <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
                        <span>{new Date(signal.detected_at).toLocaleString()}</span>
                        {typeof signal.metadata?.participant_count === 'number' ? (
                          <span>{signal.metadata.participant_count} participants</span>
                        ) : null}
                        {signal.source_thread_id ? <span>Thread linked</span> : null}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        <div className="space-y-6">
          <CRMSearchResults workspaceId={wsId} />

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">Deal Health Overview</CardTitle>
            </CardHeader>
            <CardContent>
              {healthScores.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-6 text-center">
                  <div className="flex h-10 w-10 items-center justify-center rounded-full bg-muted">
                    <Heart className="h-5 w-5 text-muted-foreground/50" />
                  </div>
                  <p className="mt-2 text-sm text-muted-foreground">No deal health scores available</p>
                  <p className="text-xs text-muted-foreground/70">Health scores will be calculated as deals progress</p>
                </div>
              ) : (
                <div className="space-y-2">
                  {healthScores.slice(0, 10).map((hs) => (
                    <div
                      key={hs.id}
                      className={`rounded-md border-l-4 p-3 ${
                        hs.score >= 70
                          ? 'border-l-green-500'
                          : hs.score >= 40
                            ? 'border-l-yellow-500'
                            : 'border-l-red-500'
                      }`}
                    >
                      <div className="flex items-center justify-between">
                        <span className="text-sm">Deal {hs.deal_id.slice(0, 8)}</span>
                        <div className="flex items-center gap-2">
                          <div className="h-2 w-20 overflow-hidden rounded-full bg-muted">
                            <div
                              className={`h-full rounded-full ${
                                hs.score >= 70 ? 'bg-green-500' : hs.score >= 40 ? 'bg-yellow-500' : 'bg-red-500'
                              }`}
                              style={{ width: `${hs.score}%` }}
                            />
                          </div>
                          <span className="text-xs font-medium">{hs.score}</span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
