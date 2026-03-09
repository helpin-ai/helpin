import { Zap, AlertTriangle, TrendingUp, Shield, Clock, Trophy, Users } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { formatDistanceToNow } from 'date-fns';
import { useContactSignals, useDealSignals } from '@/hooks/queries/useCRM';
import type { CRMBuyerSignal, CRMSignalType } from '@/lib/crmTypes';

const signalConfig: Record<CRMSignalType, { icon: React.ElementType; color: string; label: string }> = {
  buying_intent: { icon: TrendingUp, color: 'text-green-500', label: 'Buying Intent' },
  objection: { icon: AlertTriangle, color: 'text-yellow-500', label: 'Objection' },
  competitor_mention: { icon: Users, color: 'text-orange-500', label: 'Competitor' },
  budget_signal: { icon: Zap, color: 'text-blue-500', label: 'Budget' },
  timeline_signal: { icon: Clock, color: 'text-purple-500', label: 'Timeline' },
  champion_signal: { icon: Trophy, color: 'text-emerald-500', label: 'Champion' },
  risk_signal: { icon: Shield, color: 'text-red-500', label: 'Risk' },
};

interface BuyerSignalsProps {
  workspaceId: string;
  contactId?: string;
  dealId?: string;
}

export function BuyerSignals({ workspaceId, contactId, dealId }: BuyerSignalsProps) {
  const contactQuery = useContactSignals(workspaceId, contactId ?? '');
  const dealQuery = useDealSignals(workspaceId, dealId ?? '');

  const query = contactId ? contactQuery : dealQuery;
  const signals = (query.data?.data ?? []) as CRMBuyerSignal[];

  if (signals.length === 0) return null;

  return (
    <div className="space-y-2">
      {signals.map((signal) => {
        const config = signalConfig[signal.signal_type] || signalConfig.buying_intent;
        const Icon = config.icon;
        return (
          <div key={signal.id} className="flex items-start gap-2 rounded-md border p-2.5">
            <Icon className={`mt-0.5 h-4 w-4 shrink-0 ${config.color}`} />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="text-xs">{config.label}</Badge>
                <span className="text-xs text-muted-foreground">
                  {formatDistanceToNow(new Date(signal.detected_at), { addSuffix: true })}
                </span>
              </div>
              <p className="mt-0.5 text-sm">{signal.summary}</p>
            </div>
          </div>
        );
      })}
    </div>
  );
}
