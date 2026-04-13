import { Badge } from '@/components/ui/badge';
import type { SupportConversation } from '@/lib/pmTypes';
import { getConversationStateBadges } from './helpers';

const BADGE_TONE_CLASSNAMES = {
  blue: 'border-blue-200/70 bg-blue-50 text-blue-700 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-300',
  emerald: 'border-emerald-200/70 bg-emerald-50 text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300',
  amber: 'border-amber-200/70 bg-amber-50 text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300',
  slate: 'border-slate-200/70 bg-slate-50 text-slate-700 dark:border-slate-800 dark:bg-slate-900/40 dark:text-slate-300',
} as const;

interface ConversationStateBadgesProps {
  conversation: Pick<SupportConversation, 'flow_state' | 'ai_state' | 'customer_requested_human_at'>;
  className?: string;
  limit?: number;
}

export function ConversationStateBadges({
  conversation,
  className,
  limit,
}: ConversationStateBadgesProps) {
  const badges = getConversationStateBadges(conversation);
  const visibleBadges = typeof limit === 'number' ? badges.slice(0, limit) : badges;

  if (visibleBadges.length === 0) {
    return null;
  }

  return (
    <div className={className ?? 'flex flex-wrap items-center gap-1.5'}>
      {visibleBadges.map((badge) => (
        <Badge
          key={badge.key}
          variant="outline"
          className={`h-5 rounded-full px-2 text-[10px] font-medium ${BADGE_TONE_CLASSNAMES[badge.tone]}`}
        >
          {badge.label}
        </Badge>
      ))}
    </div>
  );
}
