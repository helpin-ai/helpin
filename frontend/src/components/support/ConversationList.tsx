import { useMemo, useCallback, useState, memo, useEffect } from 'react';
import { Message01Icon, Search01Icon, Cancel01Icon, ArrowDown01Icon } from '@/lib/icons';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
} from '@/components/ui/dropdown-menu';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useConversations, useInboxScopes, useMarkConversationRead } from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { ConversationRow } from './ConversationRow';
import { filterSupportConversations } from '@/lib/supportInboxFilters';
import { supportStatusOptions } from '@/components/layout/sidebar/config';
import type { SupportInboxScope } from '@/lib/pmTypes';

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
    statusFilter, setStatusFilter,
    searchQuery, setSearchQuery,
    selectedConversationId, selectConversation,
    navFilter,
    selectedMailboxId,
  } = useSupportInboxStore();
  const [searchExpanded, setSearchExpanded] = useState(false);
  const wsSend = useSupportPresenceStore((s) => s.wsSend);
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected);
  const markConversationRead = useMarkConversationRead(workspaceId);
  const { data: inboxScopes } = useInboxScopes(workspaceId);

  const handleSelect = useCallback((id: string) => {
    selectConversation(id);
    markConversationRead.mutate(id);
  }, [markConversationRead, selectConversation]);

  const filters = useMemo(() => {
    const f: Record<string, string> = {};
    if (selectedMailboxId !== 'all') {
      f.mailbox_id = selectedMailboxId;
    }
    if (statusFilter !== 'all') f.status = statusFilter;
    if (navFilter === 'mentions') f.filter = 'mentions';
    if (navFilter === 'ai_active') f.flow_state = 'ai_handling';
    if (navFilter === 'resolved_by_ai') f.flow_state = 'resolved_by_ai';
    return Object.keys(f).length > 0 ? f : undefined;
  }, [statusFilter, navFilter, selectedMailboxId]);
  const { data: response, isLoading, error } = useConversations(workspaceId, filters);
  const conversations = response?.data ?? [];

  const filteredConversations = useMemo(() => {
    return filterSupportConversations(conversations, {
      navFilter,
      mailboxScope: selectedMailboxId,
      statusFilter,
      userId,
      searchQuery,
    });
  }, [conversations, navFilter, selectedMailboxId, statusFilter, userId, searchQuery]);
  const mailboxMoveOptions = useMemo(
    () => [inboxScopes?.shared_inbox, ...(inboxScopes?.mailboxes ?? [])].filter(Boolean) as SupportInboxScope[],
    [inboxScopes]
  );

  useEffect(() => {
    if (!wsSend || !wsConnected || filteredConversations.length === 0) return;
    wsSend('support:presence:sync', {
      conversation_ids: filteredConversations.map((conversation) => conversation.id),
    });
  }, [filteredConversations, wsConnected, wsSend]);

  return (
    <div className="flex h-full w-[300px] flex-col border-r bg-background dark:border-sidebar-border dark:bg-sidebar">
      {/* Filter toolbar */}
      <div
        className="relative z-10 flex items-center gap-1.5 border-b border-border/60 bg-background/85 px-2 py-1.5 supports-[backdrop-filter]:bg-background/75 dark:border-sidebar-border dark:bg-sidebar/90 dark:supports-[backdrop-filter]:bg-sidebar/80"
        style={{ backdropFilter: 'blur(8px) saturate(160%)' }}
      >
        {searchExpanded ? (
          <div className="flex flex-1 items-center gap-1">
            <Search01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <Input
              autoFocus
              className="h-7 flex-1 border-0 bg-transparent px-1 text-sm shadow-none focus-visible:ring-0 dark:text-sidebar-foreground dark:placeholder:text-sidebar-foreground/60"
              placeholder="Search conversations..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  setSearchQuery('');
                  setSearchExpanded(false);
                }
              }}
            />
            <Button
              variant="ghost"
              size="icon"
              className="h-6 w-6 shrink-0"
              onClick={() => { setSearchQuery(''); setSearchExpanded(false); }}
            >
              <Cancel01Icon className="h-3.5 w-3.5" />
            </Button>
          </div>
        ) : (
          <>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm" className="h-7 gap-1.5 px-2 text-xs font-medium">
                  {(() => {
                    const active = supportStatusOptions.find((o) => o.value === statusFilter);
                    const Icon = active?.icon;
                    return Icon ? <Icon className={`h-3.5 w-3.5 ${active.color}`} /> : null;
                  })()}
                  {supportStatusOptions.find((o) => o.value === statusFilter)?.label ?? 'All statuses'}
                  <ArrowDown01Icon className="h-3 w-3 opacity-50" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-44">
                {supportStatusOptions.map((option) => (
                  <DropdownMenuCheckboxItem
                    key={option.value}
                    checked={statusFilter === option.value}
                    onCheckedChange={() => setStatusFilter(option.value)}
                    className="gap-2"
                  >
                    <option.icon className={`h-3.5 w-3.5 ${option.color}`} />
                    {option.label}
                  </DropdownMenuCheckboxItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
            <div className="flex-1" />
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7 shrink-0"
              onClick={() => setSearchExpanded(true)}
            >
              <Search01Icon className="h-3.5 w-3.5" />
            </Button>
          </>
        )}
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
            <Message01Icon className="h-10 w-10 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">No conversations found.</p>
          </div>
        )}
        {filteredConversations.map((conversation) => (
          <ConversationRow
            key={conversation.id}
            workspaceId={workspaceId}
            conversation={conversation}
            moveOptions={mailboxMoveOptions}
            isSelected={selectedConversationId === conversation.id}
            onSelect={() => handleSelect(conversation.id)}
          />
        ))}
      </ScrollArea>
    </div>
  );
}
