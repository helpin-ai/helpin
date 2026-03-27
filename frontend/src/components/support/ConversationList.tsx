import { useMemo, useCallback, memo, useEffect } from 'react';
import { MessageSquare, Search } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useConversations } from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { supportService } from '@/lib/services/supportService';
import { ConversationRow } from './ConversationRow';
import { filterSupportConversations } from '@/lib/supportInboxFilters';

const SkeletonRow = memo(function SkeletonRow() {
  return (
    <div className="flex items-center gap-3 px-3 py-2.5">
      <div className="h-9 w-9 shrink-0 animate-pulse rounded-full bg-muted" />
      <div className="flex-1 space-y-1.5">
        <div className="flex items-center justify-between gap-2">
          <div className="h-3.5 w-28 animate-pulse rounded bg-muted" />
          <div className="h-3 w-10 animate-pulse rounded bg-muted" />
        </div>
        <div className="h-3 w-40 animate-pulse rounded bg-muted" />
      </div>
    </div>
  );
});

interface ConversationListProps {
  workspaceId: string;
  userId?: string;
}

export function ConversationList({ workspaceId, userId }: ConversationListProps) {
  const {
    statusFilter,
    searchQuery, setSearchQuery,
    selectedConversationId, selectConversation,
    navFilter,
  } = useSupportInboxStore();
  const wsSend = useSupportPresenceStore((s) => s.wsSend);
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected);

  const handleSelect = useCallback((id: string) => {
    selectConversation(id);
    if (wsSend) {
      wsSend('support:conversation:read', { conversation_id: id });
    } else {
      supportService.markConversationRead(workspaceId, id).catch(() => {});
    }
  }, [selectConversation, wsSend, workspaceId]);

  const filters = useMemo(() => {
    const f: Record<string, string> = {};
    if (statusFilter !== 'all') f.status = statusFilter;
    if (navFilter === 'mentions') f.filter = 'mentions';
    return Object.keys(f).length > 0 ? f : undefined;
  }, [statusFilter, navFilter]);
  const { data: response, isLoading, error } = useConversations(workspaceId, filters);
  const conversations = response?.data ?? [];

  const filteredConversations = useMemo(() => {
    return filterSupportConversations(conversations, { navFilter, userId, searchQuery });
  }, [conversations, navFilter, userId, searchQuery]);

  useEffect(() => {
    if (!wsSend || !wsConnected || filteredConversations.length === 0) return;
    wsSend('support:presence:sync', {
      conversation_ids: filteredConversations.map((conversation) => conversation.id),
    });
  }, [filteredConversations, wsConnected, wsSend]);

  return (
    <div className="flex h-full w-[300px] flex-col border-r bg-background dark:border-sidebar-border dark:bg-sidebar">
      {/* Frosted glass search header */}
      <div
        className="relative z-10 border-b border-border/60 bg-background/85 px-3 py-2 supports-[backdrop-filter]:bg-background/75 dark:border-sidebar-border dark:bg-sidebar/90 dark:supports-[backdrop-filter]:bg-sidebar/80"
        style={{
          backdropFilter: 'blur(8px) saturate(160%)',
        }}
      >
        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            className="h-8 border-border/60 bg-background/80 pl-8 text-sm shadow-none dark:border-sidebar-border dark:bg-sidebar-accent/60 dark:text-sidebar-foreground dark:placeholder:text-sidebar-foreground/60"
            placeholder="Search conversations..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
      </div>

      {/* Conversation list */}
      <ScrollArea className="flex-1 min-h-0">
        {isLoading && (
          <div>
            <SkeletonRow />
            <SkeletonRow />
            <SkeletonRow />
            <SkeletonRow />
            <SkeletonRow />
          </div>
        )}
        {error && (
          <p className="p-4 text-sm text-destructive">{String(error)}</p>
        )}
        {!isLoading && filteredConversations.length === 0 && !error && (
          <div className="flex flex-col items-center justify-center gap-3 py-16">
            <MessageSquare className="h-10 w-10 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">No conversations found.</p>
          </div>
        )}
        {filteredConversations.map((conversation) => (
          <ConversationRow
            key={conversation.id}
            conversation={conversation}
            isSelected={selectedConversationId === conversation.id}
            onSelect={() => handleSelect(conversation.id)}
          />
        ))}
      </ScrollArea>
    </div>
  );
}
