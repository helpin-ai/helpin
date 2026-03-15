import { Badge } from '@/components/ui/badge';
import type { SupportConversation } from '@/lib/pmTypes';
import { STATUS_COLORS, STATUS_LABELS, PRIORITY_COLORS } from './constants';
import { timeAgo, getInitial } from './helpers';

interface ConversationRowProps {
  conversation: SupportConversation;
  isSelected: boolean;
  onSelect: () => void;
}

export function ConversationRow({ conversation, isSelected, onSelect }: ConversationRowProps) {
  const displayName = conversation.customer_name || conversation.customer_email || 'Anonymous';

  return (
    <button
      type="button"
      onClick={onSelect}
      className={`w-full text-left border-b px-3 py-2.5 transition-colors hover:bg-muted/50 ${
        isSelected ? 'bg-muted border-l-2 border-l-primary' : ''
      }`}
    >
      <div className="flex items-start gap-2.5">
        <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-medium text-primary">
          {getInitial(displayName)}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-sm font-medium">{displayName}</span>
            <span className="shrink-0 text-[10px] text-muted-foreground">{timeAgo(conversation.updated_at)}</span>
          </div>
          <p className="mt-0.5 truncate text-sm text-foreground/80">{conversation.subject}</p>
          <div className="mt-1 flex items-center gap-1.5">
            <span className="text-[10px] text-muted-foreground">#{conversation.display_id}</span>
            <Badge variant="secondary" className={`text-[10px] px-1.5 py-0 leading-tight ${STATUS_COLORS[conversation.status]}`}>
              {STATUS_LABELS[conversation.status]}
            </Badge>
            <Badge variant="secondary" className={`text-[10px] px-1.5 py-0 leading-tight ${PRIORITY_COLORS[conversation.priority]}`}>
              {conversation.priority}
            </Badge>
          </div>
        </div>
      </div>
    </button>
  );
}
