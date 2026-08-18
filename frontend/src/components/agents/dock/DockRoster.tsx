import { useMemo, useState } from 'react';
import { BotIcon, GlobeIcon, MoreHorizontalIcon, NotificationBubbleIcon, PlusSignIcon, UserGroupIcon } from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import type { DockChat, DockRunSummary } from '@/lib/dockTypes';
import { cn } from '@/lib/utils';
import { useAuthStore } from '@/stores/authStore';
import { AnimatedDockChatTitle } from './AnimatedDockChatTitle';
import {
  dockRunSubtitle,
  dockRunTitle,
  presentDockRun,
  relativeDockTime,
  type DockRunGroup,
} from './dockPresentation';
import { DOCK_DATE_GROUPS, dockDateGroup } from './dockDateGroups';

interface DockRosterProps {
  workspaceId: string;
  tab: 'agents' | 'chats';
  runs: DockRunSummary[];
  chats: DockChat[];
  selectedRunId: string | null;
  selectedChatId: string | null;
  loadingRuns?: boolean;
  loadingChats?: boolean;
  runsError?: string | null;
  chatsError?: string | null;
  onTabChange: (tab: 'agents' | 'chats') => void;
  onSelectRun: (runId: string) => void;
  onSelectChat: (chatId: string) => void;
  onNewChat: () => void;
  onRenameChat: (chatId: string, title: string) => Promise<boolean>;
  onArchiveChat: (chatId: string) => Promise<boolean>;
  hasMoreChats?: boolean;
  loadingMoreChats?: boolean;
  onLoadMoreChats?: () => void;
  hasMoreRuns?: boolean;
  loadingMoreRuns?: boolean;
  onLoadMoreRuns?: () => void;
  onRetryRuns: () => void;
  onRetryChats: () => void;
}

const ACTIVE_GROUPS: Array<{ key: DockRunGroup; label: string; dot: string }> = [
  { key: 'needs_you', label: 'Needs you', dot: '#d97706' },
  { key: 'running', label: 'Running', dot: '#059669' },
];

type DockChatPresence = 'running' | 'paused' | 'stopped';

const DOCK_CHAT_IDLE_TIMEOUT_MS = 72 * 60 * 60 * 1000;

const CHAT_PRESENCE: Record<DockChatPresence, { label: string; color: string }> = {
  running: { label: 'Running', color: '#16a34a' },
  paused: { label: 'Paused', color: '#d97706' },
  stopped: { label: 'Stopped', color: '#a5a29b' },
};

function dockChatPresence(chat: DockChat): DockChatPresence {
  if (chat.active_run_status === 'queued' || chat.active_run_status === 'running') return 'running';
  if (chat.active_run_status === 'paused') return 'paused';
  if (!chat.active_run_status && chat.active_run_id) {
    const lastActivity = Date.parse(chat.last_message_at ?? chat.updated_at);
    if (!Number.isFinite(lastActivity) || Date.now() - lastActivity < DOCK_CHAT_IDLE_TIMEOUT_MS) return 'paused';
  }
  return 'stopped';
}

export function DockRoster(props: DockRosterProps) {
  return (
    <aside className="agent-dock-roster flex min-h-0 w-[300px] shrink-0 flex-col border-e border-[#f1efea] bg-[#fbfaf8] dark:border-[#302f2b] dark:bg-[#1d1c1a]">
      <div className="agent-dock-roster-controls flex flex-col gap-2.5 px-3 pb-2.5 pt-3">
        <button
          type="button"
          aria-label="New chat or task"
          onClick={props.onNewChat}
          className="agent-dock-new-chat flex min-h-10 w-full items-center gap-2.5 rounded-[9px] border border-[#e6e3dd] bg-[#fffefa] px-3 py-2 text-start text-[12.5px] font-semibold text-[#4b4945] transition-colors hover:border-[#d8d3c9] hover:bg-[#f4f2ee] focus-visible:ring-2 focus-visible:ring-[#a855f7]/35 dark:border-[#37352f] dark:bg-[#242320] dark:text-[#eeeae1] dark:hover:bg-[#302f2b]"
        >
          <span className="agent-dock-new-chat-icon grid h-5 w-5 place-items-center rounded-[6px] bg-[#f0eee9] text-[#6f6b64] dark:bg-[#37352f] dark:text-[#d4d0c7]" aria-hidden><PlusSignIcon className="h-3.5 w-3.5" /></span>
          <span className="agent-dock-roster-copy truncate">New chat or task</span>
          <kbd className="agent-dock-roster-copy ms-auto rounded border border-[#d8d3c9] bg-[#f7f5f1] px-1 font-mono text-[10.5px] font-medium text-[#8a8781] dark:border-[#4a4842] dark:bg-[#302f2b] dark:text-[#aaa69e]">N</kbd>
        </button>
        <div className="agent-dock-tab-list flex rounded-[9px] bg-[#f0eee9] p-[3px] dark:bg-[#292824]" role="tablist" aria-label="Dock views">
          {(['agents', 'chats'] as const).map((tab) => (
            <button
              key={tab}
              type="button"
              role="tab"
              aria-selected={props.tab === tab}
              onClick={() => props.onTabChange(tab)}
              className={cn(
                'agent-dock-tab flex min-w-0 flex-1 items-center justify-center gap-1.5 rounded-[7px] py-1.5 text-[12.5px] font-semibold capitalize transition-colors',
                props.tab === tab
                  ? 'bg-[#fffefa] text-[#1c1b19] shadow-sm dark:bg-[#37352f] dark:text-[#f2efe8]'
                  : 'text-[#8a8781] hover:text-[#4b4945] dark:text-[#96928a] dark:hover:text-[#d4d0c7]',
              )}
            >
              <span className="agent-dock-tab-icon hidden" aria-hidden>
                {tab === 'agents' ? <BotIcon className="h-4 w-4" /> : <NotificationBubbleIcon className="h-4 w-4" />}
              </span>
              <span className="agent-dock-tab-copy">{tab === 'agents' ? 'Agent runs' : 'Chats'}</span>
            </button>
          ))}
        </div>
      </div>

      <div
        className="min-h-0 flex-1 overflow-y-auto pb-2"
        role="tabpanel"
        onScroll={(event) => {
          const target = event.currentTarget;
          if (target.scrollHeight - target.scrollTop - target.clientHeight >= 120) return;
          if (props.tab === 'chats' && props.hasMoreChats && !props.loadingMoreChats) props.onLoadMoreChats?.();
          if (props.tab === 'agents' && props.hasMoreRuns && !props.loadingMoreRuns) props.onLoadMoreRuns?.();
        }}
      >
        {props.tab === 'agents' ? <AgentRows {...props} /> : <ChatRows {...props} />}
        {props.tab === 'chats' && props.loadingMoreChats ? (
          <p className="agent-dock-roster-copy px-3 py-2 text-center text-[11px] text-[#8a8781]">Loading more…</p>
        ) : null}
        {props.tab === 'agents' && props.loadingMoreRuns ? (
          <p className="agent-dock-roster-copy px-3 py-2 text-center text-[11px] text-[#8a8781]">Loading more…</p>
        ) : null}
      </div>

    </aside>
  );
}

function AgentRows(props: DockRosterProps) {
  const grouped = useMemo(() => {
    const result = new Map<DockRunGroup, DockRunSummary[]>();
    result.set('needs_you', []);
    result.set('running', []);
    result.set('recent', []);
    for (const summary of props.runs) {
      const presentation = presentDockRun(summary.run.status, summary.run.pause_reason, summary.attention_kind);
      result.get(presentation.group)?.push(summary);
    }
    for (const rows of result.values()) {
      rows.sort((left, right) => Date.parse(right.last_activity_at) - Date.parse(left.last_activity_at));
    }
    return result;
  }, [props.runs]);

  if (props.loadingRuns && props.runs.length === 0) return <RosterMessage>Loading your agents…</RosterMessage>;
  if (props.runsError && props.runs.length === 0) {
    return <RosterError message={props.runsError} onRetry={props.onRetryRuns} />;
  }
  if (props.runs.length === 0) {
    return <RosterMessage>No agents running. Describe a task below.</RosterMessage>;
  }

  return (
    <>
      {ACTIVE_GROUPS.map((group) => {
        const rows = grouped.get(group.key) ?? [];
        if (rows.length === 0) return null;
        return (
          <section key={group.key} aria-labelledby={`agent-dock-group-${group.key}`}>
            <div className="agent-dock-group-header sticky top-0 z-[1] flex items-center gap-[7px] bg-[#fbfaf8]/95 px-3.5 pb-[5px] pt-2.5 backdrop-blur-sm dark:bg-[#1d1c1a]/95">
              <span className="h-1.5 w-1.5 shrink-0 rounded-full" style={{ backgroundColor: group.dot }} />
              <span id={`agent-dock-group-${group.key}`} className="agent-dock-roster-copy text-[10.5px] font-bold uppercase tracking-[.09em] text-[#8a8781] dark:text-[#96928a]">{group.label}</span>
              <span className="agent-dock-roster-copy text-[10.5px] font-semibold text-[#b3b0a9]">{rows.length}</span>
            </div>
            {rows.map((summary) => <RunRow key={summary.run.id} summary={summary} selected={props.selectedRunId === summary.run.id} onSelect={() => props.onSelectRun(summary.run.id)} />)}
          </section>
        );
      })}
      {DOCK_DATE_GROUPS.map((group) => {
        const rows = (grouped.get('recent') ?? []).filter((summary) => dockDateGroup(summary.last_activity_at) === group.key);
        if (rows.length === 0) return null;
        return (
          <section key={group.key} aria-labelledby={`agent-dock-run-date-${group.key}`}>
            <RosterGroupHeader id={`agent-dock-run-date-${group.key}`} label={group.label} count={rows.length} />
            {rows.map((summary) => <RunRow key={summary.run.id} summary={summary} selected={props.selectedRunId === summary.run.id} onSelect={() => props.onSelectRun(summary.run.id)} />)}
          </section>
        );
      })}
    </>
  );
}

function RunRow({ summary, selected, onSelect }: { summary: DockRunSummary; selected: boolean; onSelect: () => void }) {
  const presentation = presentDockRun(summary.run.status, summary.run.pause_reason, summary.attention_kind);
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-label={`${dockRunTitle(summary)}, ${presentation.label}`}
      aria-current={selected ? 'true' : undefined}
      title={`${dockRunTitle(summary)} — ${presentation.label}`}
      className={cn(
        'agent-dock-roster-row flex w-full min-w-0 items-center gap-[9px] px-3 py-2 text-start transition-colors hover:bg-[#f4f2ee] dark:hover:bg-[#292824]',
        selected && 'bg-[#f4f2ee] shadow-[inset_2px_0_0_var(--dock-row-state)] dark:bg-[#292824]',
      )}
      style={{ '--dock-row-state': presentation.dot } as React.CSSProperties}
    >
      <DockAgentAvatar summary={summary} dot={presentation.dot} />
      <span className="agent-dock-roster-copy min-w-0 flex-1">
        <span className="block truncate text-[12.5px] font-medium text-[#1c1b19] dark:text-[#eeeae1]">{dockRunTitle(summary)}</span>
        <span className="mt-px block truncate text-[11px] text-[#8a8781] dark:text-[#96928a]">{dockRunSubtitle(summary)}</span>
      </span>
      <time className="agent-dock-roster-copy shrink-0 font-mono text-[10.5px] text-[#b3b0a9]" dateTime={summary.last_activity_at}>
        {relativeDockTime(summary.last_activity_at)}
      </time>
    </button>
  );
}

function DockAgentAvatar({ summary, dot }: { summary: DockRunSummary; dot: string }) {
  return (
    <span className="relative flex h-[26px] w-[26px] shrink-0 leading-none">
      <AgentAvatar
        name={summary.agent.name}
        presetKey={summary.agent.preset_key}
        iconKey={summary.agent.icon_key}
        className="h-[26px] w-[26px] rounded-[9px] border-0 shadow-none"
      />
      <span
        className="absolute -bottom-0.5 -end-0.5 h-[11px] w-[11px] rounded-full border-2 border-[#fbfaf8] dark:border-[#1d1c1a]"
        style={{ backgroundColor: dot }}
        data-agent-dock-roster-status-dot
      />
    </span>
  );
}

function ChatRows(props: DockRosterProps) {
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const currentUserId = useAuthStore((state) => state.user?.id);

  const rename = async (chatId: string) => {
    const title = renameValue.trim();
    setRenamingId(null);
    if (!title) return;
    await props.onRenameChat(chatId, title);
  };
  const archive = async (chatId: string) => {
    await props.onArchiveChat(chatId);
  };

  if (props.loadingChats && props.chats.length === 0) return <RosterMessage>Loading conversations…</RosterMessage>;
  if (props.chatsError && props.chats.length === 0) return <RosterError message={props.chatsError} onRetry={props.onRetryChats} />;
  if (props.chats.length === 0) return <RosterMessage>No conversations yet. Start one above.</RosterMessage>;

  return DOCK_DATE_GROUPS.map((group) => {
    const chats = props.chats.filter((chat) => dockDateGroup(chat.last_message_at ?? chat.updated_at ?? chat.created_at) === group.key);
    if (chats.length === 0) return null;
    return <section key={group.key} aria-labelledby={`agent-dock-chat-date-${group.key}`}>
      <RosterGroupHeader id={`agent-dock-chat-date-${group.key}`} label={group.label} count={chats.length} />
      {chats.map((chat) => {
        const presence = dockChatPresence(chat);
        const presentation = CHAT_PRESENCE[presence];
        return (
          <div
            key={chat.id}
            className={cn(
              'agent-dock-roster-row group relative flex min-w-0 items-center px-3 py-2 hover:bg-[#f4f2ee] dark:hover:bg-[#292824]',
              props.selectedChatId === chat.id && 'bg-[#f4f2ee] shadow-[inset_2px_0_0_var(--dock-chat-state)] dark:bg-[#292824]',
            )}
            style={{ '--dock-chat-state': presentation.color } as React.CSSProperties}
          >
          {renamingId === chat.id ? (
            <div className="flex min-w-0 flex-1 items-center gap-[9px]">
            <ChatStatusIndicator presence={presence} className="agent-dock-chat-row-dot" />
            <input
              autoFocus
              aria-label="Conversation title"
              value={renameValue}
              onChange={(event) => setRenameValue(event.target.value)}
              onBlur={() => void rename(chat.id)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void rename(chat.id);
                if (event.key === 'Escape') setRenamingId(null);
              }}
              className="agent-dock-roster-copy min-w-0 flex-1 rounded border border-[#d8d3c9] bg-[#fffefa] px-1.5 py-1 text-[12px] outline-none focus:border-[#a5a29b] dark:bg-[#242320]"
              maxLength={120}
            />
            </div>
          ) : (
            <button
              type="button"
              aria-label={`${chat.title.trim() || 'Untitled chat'}, ${presentation.label}`}
              aria-current={props.selectedChatId === chat.id ? 'true' : undefined}
              className="flex min-w-0 flex-1 items-center gap-[9px] text-start"
              onClick={() => props.onSelectChat(chat.id)}
            >
              <ChatMarker chat={chat} />
              <ChatStatusIndicator presence={presence} className="agent-dock-chat-row-dot" />
              <span className="agent-dock-roster-copy min-w-0 flex-1">
                <span className="block truncate text-[12.5px] font-medium text-[#1c1b19] dark:text-[#eeeae1]">
                  <AnimatedDockChatTitle title={chat.title.trim() || 'Untitled chat'} />
                </span>
              </span>
              {chat.visibility === 'module' ? (
                <UserGroupIcon className="h-3.5 w-3.5 shrink-0 text-[#8a8781]" aria-label="Visible to module teammates" />
              ) : chat.visibility === 'workspace' ? (
                <GlobeIcon className="h-3.5 w-3.5 shrink-0 text-[#8a8781]" aria-label="Visible to everyone in the workspace" />
              ) : null}
              <time className="agent-dock-roster-copy ms-auto shrink-0 font-mono text-[10.5px] text-[#b3b0a9]" dateTime={chat.last_message_at ?? chat.updated_at} data-agent-dock-chat-time>
                {relativeDockTime(chat.last_message_at ?? chat.updated_at)}
              </time>
            </button>
          )}
          {chat.user_id === currentUserId ? <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button type="button" aria-label={`Actions for ${chat.title || 'Untitled chat'}`} className="agent-dock-chat-menu absolute end-2 top-1/2 z-[2] grid h-7 w-7 -translate-y-1/2 place-items-center rounded-md bg-[#f4f2ee] text-[#8a8781] opacity-0 transition-[opacity,color,background-color] hover:bg-[#eae7e0] hover:text-[#4b4945] group-hover:opacity-100 focus:opacity-100 dark:bg-[#292824] dark:text-[#a9a59d] dark:hover:bg-[#37352f] dark:hover:text-[#eeeae1]">
                <MoreHorizontalIcon className="h-3.5 w-3.5" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="z-[70]">
              <DropdownMenuItem onSelect={() => { setRenamingId(chat.id); setRenameValue(chat.title); }}>Rename</DropdownMenuItem>
              <DropdownMenuItem className="text-destructive" onSelect={() => void archive(chat.id)}>Archive</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu> : null}
          </div>
        );
      })}
    </section>;
  });
}

function RosterGroupHeader({ id, label, count }: { id: string; label: string; count: number }) {
  return <div className="agent-dock-group-header sticky top-0 z-[1] flex items-center gap-[7px] bg-[#fbfaf8]/95 px-3.5 pb-[5px] pt-2.5 backdrop-blur-sm dark:bg-[#1d1c1a]/95">
    <span className="h-1.5 w-1.5 rounded-full bg-[#a5a29b]" />
    <span id={id} className="agent-dock-roster-copy text-[10.5px] font-bold uppercase tracking-[.09em] text-[#8a8781] dark:text-[#96928a]">{label}</span>
    <span className="agent-dock-roster-copy text-[10.5px] font-semibold text-[#b3b0a9]">{count}</span>
  </div>;
}

function ChatMarker({ chat }: { chat: DockChat }) {
  const label = chat.title.trim() || 'Untitled chat';
  const initial = label.match(/[\p{L}\p{N}]/u)?.[0]?.toLocaleUpperCase() ?? '·';
  return (
    <span
      aria-hidden
      className="agent-dock-chat-marker relative hidden h-[28px] w-[28px] shrink-0 place-items-center rounded-[9px] bg-[#eee8df] text-[11px] font-bold text-[#8a6a43] dark:bg-[#38332b] dark:text-[#d0aa7b]"
    >
      {initial}
      <span className="absolute -bottom-0.5 -end-0.5 grid h-[12px] w-[12px] place-items-center rounded-[4px] bg-[#fbfaf8] dark:bg-[#1d1c1a]">
        <ChatStatusIndicator presence={dockChatPresence(chat)} compact />
      </span>
    </span>
  );
}

function ChatStatusIndicator({ presence, compact = false, className }: { presence: DockChatPresence; compact?: boolean; className?: string }) {
  const common = cn('relative inline-flex h-2 w-2 shrink-0 items-center justify-center', className);
  if (presence === 'running') {
    return (
      <span className={common} aria-hidden data-agent-dock-chat-status="running">
        <span className={cn('absolute rounded-full bg-[#16a34a]', compact ? 'h-1.5 w-1.5' : 'h-2 w-2')} />
        <span className={cn('agent-dock-chat-running-pulse absolute rounded-full bg-[#16a34a]', compact ? 'h-1.5 w-1.5' : 'h-2 w-2')} />
      </span>
    );
  }
  if (presence === 'paused') {
    return (
      <span className={cn(common, 'gap-[2px]')} aria-hidden data-agent-dock-chat-status="paused">
        <span className={cn('w-[2px] rounded-[1px] bg-[#d97706]', compact ? 'h-[6px]' : 'h-2')} />
        <span className={cn('w-[2px] rounded-[1px] bg-[#d97706]', compact ? 'h-[6px]' : 'h-2')} />
      </span>
    );
  }
  return (
    <span className={common} aria-hidden data-agent-dock-chat-status="stopped">
      <span className={cn('rounded-[2px] bg-[#a5a29b]', compact ? 'h-1.5 w-1.5' : 'h-2 w-2')} />
    </span>
  );
}

function RosterMessage({ children }: { children: React.ReactNode }) {
  return <p className="agent-dock-roster-copy px-5 py-10 text-center text-[12.5px] leading-5 text-[#8a8781] dark:text-[#96928a]">{children}</p>;
}

function RosterError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="agent-dock-roster-copy px-4 py-8 text-center text-[12px] text-[#8a8781]">
      <p className="line-clamp-3">{message}</p>
      <button type="button" onClick={onRetry} className="mt-2 font-semibold text-[#1c1b19] hover:underline dark:text-[#eeeae1]">Try again</button>
    </div>
  );
}
