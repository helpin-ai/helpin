import { ZapIcon, Alert01Icon, ChartIncreaseIcon, Shield01Icon, Clock01Icon, Award01Icon, UserGroupIcon } from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { formatDistanceToNow } from 'date-fns';
import { useContactSignals, useDealSignals } from '@/hooks/queries/useCRM';
import type { CRMBuyerSignal, CRMSignalType } from '@/lib/crmTypes';

const signalConfig: Record<CRMSignalType, { icon: React.ElementType; color: string; label: string }> = {
  buying_intent: { icon: ChartIncreaseIcon, color: 'text-green-500', label: 'Buying Intent' },
  objection: { icon: Alert01Icon, color: 'text-yellow-500', label: 'Objection' },
  competitor_mention: { icon: UserGroupIcon, color: 'text-orange-500', label: 'Competitor' },
  budget_signal: { icon: ZapIcon, color: 'text-blue-500', label: 'Budget' },
  timeline_signal: { icon: Clock01Icon, color: 'text-purple-500', label: 'Timeline' },
  champion_signal: { icon: Award01Icon, color: 'text-emerald-500', label: 'Champion' },
  risk_signal: { icon: Shield01Icon, color: 'text-red-500', label: 'Risk' },
};

interface BuyerSignalsProps {
  workspaceId: string;
  contactId?: string;
  dealId?: string;
}

function formatSource(signal: CRMBuyerSignal) {
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

export function BuyerSignals({ workspaceId, contactId, dealId }: BuyerSignalsProps) {
  const contactQuery = useContactSignals(workspaceId, contactId ?? '');
  const dealQuery = useDealSignals(workspaceId, dealId ?? '');

  const query = contactId ? contactQuery : dealQuery;
  const signals = (query.data?.data ?? []) as CRMBuyerSignal[];

  if (signals.length === 0) {
    return <p className="text-sm text-muted-foreground">No buyer signals detected yet.</p>;
  }

  return (
    <div className="space-y-2">
      {signals.map((signal) => {
        const config = signalConfig[signal.signal_type] || signalConfig.buying_intent;
        const Icon = config.icon;
        return (
          <div key={signal.id} className="flex items-start gap-2 rounded-md border p-2.5">
            <Icon className={`mt-0.5 h-4 w-4 shrink-0 ${config.color}`} />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <Badge variant="outline" className="text-xs">{config.label}</Badge>
                <Badge variant="secondary" className="text-[10px] uppercase tracking-wide">
                  {formatSource(signal)}
                </Badge>
                <span className="text-xs text-muted-foreground">{Math.round(signal.confidence * 100)}%</span>
                <span className="text-xs text-muted-foreground">
                  {formatDistanceToNow(new Date(signal.detected_at), { addSuffix: true })}
                </span>
              </div>
              <p className="mt-0.5 text-sm">{signal.summary}</p>
              {signal.evidence_excerpt ? (
                <p className="mt-1 rounded-sm border-l-2 border-border/70 pl-2 text-xs italic text-muted-foreground">
                  "{signal.evidence_excerpt}"
                </p>
              ) : null}
              {signal.metadata?.message_direction || signal.metadata?.participant_count ? (
                <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
                  {signal.metadata?.message_direction ? (
                    <span className="capitalize">{signal.metadata.message_direction}</span>
                  ) : null}
                  {typeof signal.metadata?.participant_count === 'number' ? (
                    <span>{signal.metadata.participant_count} participants</span>
                  ) : null}
                </div>
              ) : null}
            </div>
          </div>
        );
      })}
    </div>
  );
}
