import { useMemo, useState } from 'react';
import { AiMagicIcon, BotIcon, MoreHorizontalIcon, NotificationBubbleIcon, PlusSignIcon } from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import type { DockChat, DockRunSummary } from '@/lib/dockTypes';
import { cn } from '@/lib/utils';
import {
  dockRunSubtitle,
  dockRunTitle,
  presentDockRun,
  relativeDockTime,
  type DockRunGroup,
} from './dockPresentation';

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
  onRetryRuns: () => void;
  onRetryChats: () => void;
}

const GROUPS: Array<{ key: DockRunGroup; label: string; dot: string }> = [
  { key: 'needs_you', label: 'Needs you', dot: '#d97706' },
  { key: 'running', label: 'Running', dot: '#059669' },
  { key: 'recent', label: 'Paused & recent', dot: '#a5a29b' },
];

export function DockRoster(props: DockRosterProps) {
  return (
    <aside className="agent-dock-roster flex min-h-0 w-[300px] shrink-0 flex-col border-e border-[#f1efea] bg-[#fbfaf8] dark:border-[#302f2b] dark:bg-[#1d1c1a]">
      <div className="agent-dock-roster-controls flex flex-col gap-2.5 px-3 pb-2.5 pt-3">
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
              <span className="agent-dock-tab-copy">{tab === 'agents' ? 'Agents' : 'Chats'}</span>
            </button>
          ))}
        </div>
        <button
          type="button"
          aria-label="New chat or task"
          onClick={props.onNewChat}
          className="agent-dock-new-chat flex min-h-8 items-center gap-2 rounded-[9px] border border-[#eae7e0] bg-[#fffefa] px-2.5 py-1.5 text-start text-[12.5px] text-[#8a8781] transition hover:border-[#d8d3c9] hover:text-[#4b4945] dark:border-[#34322d] dark:bg-[#242320] dark:text-[#a9a59d]"
        >
          <span className="agent-dock-new-chat-icon hidden" aria-hidden><PlusSignIcon className="h-4 w-4" /></span>
          <span className="agent-dock-sparkle grid h-[11px] w-[11px] shrink-0 place-items-center rounded-[4px]" aria-hidden><AiMagicIcon className="h-2.5 w-2.5 text-white" /></span>
          <span className="agent-dock-roster-copy truncate">New chat or task</span>
          <kbd className="agent-dock-roster-copy ms-auto rounded border border-[#eeece7] px-1 font-mono text-[10.5px] text-[#b3b0a9] dark:border-[#3a3832]">N</kbd>
        </button>
      </div>

      <div
        className="min-h-0 flex-1 overflow-y-auto pb-2"
        role="tabpanel"
        onScroll={(event) => {
          if (!props.hasMoreChats || props.loadingMoreChats || props.tab !== 'chats') return;
          const target = event.currentTarget;
          if (target.scrollHeight - target.scrollTop - target.clientHeight < 120) props.onLoadMoreChats?.();
        }}
      >
        {props.tab === 'agents' ? <AgentRows {...props} /> : <ChatRows {...props} />}
        {props.tab === 'chats' && props.loadingMoreChats ? (
          <p className="agent-dock-roster-copy px-3 py-2 text-center text-[11px] text-[#8a8781]">Loading more…</p>
        ) : null}
      </div>
    </aside>
  );
}

function AgentRows(props: DockRosterProps) {
  const grouped = useMemo(() => {
    const result = new Map<DockRunGroup, DockRunSummary[]>();
    for (const group of GROUPS) result.set(group.key, []);
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

  return GROUPS.map((group) => {
    const rows = grouped.get(group.key) ?? [];
    if (rows.length === 0) return null;
    return (
      <section key={group.key} aria-labelledby={`agent-dock-group-${group.key}`}>
        <div className="agent-dock-group-header sticky top-0 z-[1] flex items-center gap-[7px] bg-[#fbfaf8]/95 px-3.5 pb-[5px] pt-2.5 backdrop-blur-sm dark:bg-[#1d1c1a]/95">
          <span className="h-1.5 w-1.5 shrink-0 rounded-full" style={{ backgroundColor: group.dot }} />
          <span id={`agent-dock-group-${group.key}`} className="agent-dock-roster-copy text-[10.5px] font-bold uppercase tracking-[.09em] text-[#8a8781] dark:text-[#96928a]">
            {group.label}
          </span>
          <span className="agent-dock-roster-copy text-[10.5px] font-semibold text-[#b3b0a9]">{rows.length}</span>
        </div>
        {rows.map((summary) => (
          <RunRow
            key={summary.run.id}
            summary={summary}
            selected={props.selectedRunId === summary.run.id}
            onSelect={() => props.onSelectRun(summary.run.id)}
          />
        ))}
      </section>
    );
  });
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
      <span className="absolute -bottom-0.5 -end-0.5 h-[9px] w-[9px] rounded-full border-2 border-[#fbfaf8] dark:border-[#1d1c1a]" style={{ backgroundColor: dot }} />
    </span>
  );
}

function ChatRows(props: DockRosterProps) {
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');

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

  return (
    <section aria-labelledby="agent-dock-chats-group">
      <div className="agent-dock-group-header sticky top-0 z-[1] flex items-center gap-[7px] bg-[#fbfaf8]/95 px-3.5 pb-[5px] pt-2.5 backdrop-blur-sm dark:bg-[#1d1c1a]/95">
        <span className="h-1.5 w-1.5 rounded-full bg-[#a5a29b]" />
        <span id="agent-dock-chats-group" className="agent-dock-roster-copy text-[10.5px] font-bold uppercase tracking-[.09em] text-[#8a8781]">Today & earlier</span>
        <span className="agent-dock-roster-copy text-[10.5px] font-semibold text-[#b3b0a9]">{props.chats.length}</span>
      </div>
      {props.chats.map((chat) => (
        <div
          key={chat.id}
          className={cn(
            'agent-dock-roster-row group flex min-w-0 items-center gap-[9px] px-3 py-2 hover:bg-[#f4f2ee] dark:hover:bg-[#292824]',
            props.selectedChatId === chat.id && 'bg-[#f4f2ee] shadow-[inset_2px_0_0_#a5a29b] dark:bg-[#292824]',
          )}
        >
          {renamingId === chat.id ? (
            <div className="flex min-w-0 flex-1 items-center gap-[9px]">
            <span className="agent-dock-chat-row-dot h-1.5 w-1.5 shrink-0 rounded-full bg-[#a5a29b]" aria-hidden />
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
              aria-label={chat.title.trim() || 'Untitled chat'}
              aria-current={props.selectedChatId === chat.id ? 'true' : undefined}
              className="flex min-w-0 flex-1 items-center gap-[9px] text-start"
              onClick={() => props.onSelectChat(chat.id)}
            >
              <ChatMarker title={chat.title} />
              <span className="agent-dock-chat-row-dot h-1.5 w-1.5 shrink-0 rounded-full bg-[#a5a29b]" aria-hidden />
              <span className="agent-dock-roster-copy min-w-0 flex-1">
                <span className="block truncate text-[12.5px] font-medium text-[#1c1b19] dark:text-[#eeeae1]">{chat.title.trim() || 'Untitled chat'}</span>
              </span>
              <time className="agent-dock-roster-copy shrink-0 font-mono text-[10.5px] text-[#b3b0a9]" dateTime={chat.last_message_at ?? chat.updated_at}>
                {relativeDockTime(chat.last_message_at ?? chat.updated_at)}
              </time>
            </button>
          )}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button type="button" aria-label={`Actions for ${chat.title || 'Untitled chat'}`} className="agent-dock-chat-menu -me-1 grid h-7 w-7 shrink-0 place-items-center rounded-md text-[#8a8781] opacity-0 transition hover:bg-[#eae7e0] group-hover:opacity-100 focus:opacity-100 dark:hover:bg-[#37352f]">
                <MoreHorizontalIcon className="h-3.5 w-3.5" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="z-[70]">
              <DropdownMenuItem onSelect={() => { setRenamingId(chat.id); setRenameValue(chat.title); }}>Rename</DropdownMenuItem>
              <DropdownMenuItem className="text-destructive" onSelect={() => void archive(chat.id)}>Archive</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      ))}
    </section>
  );
}

function ChatMarker({ title }: { title: string }) {
  const label = title.trim() || 'Untitled chat';
  const initial = label.match(/[\p{L}\p{N}]/u)?.[0]?.toLocaleUpperCase() ?? '·';
  return (
    <span
      aria-hidden
      className="agent-dock-chat-marker relative hidden h-[28px] w-[28px] shrink-0 place-items-center rounded-[9px] bg-[#eee8df] text-[11px] font-bold text-[#8a6a43] dark:bg-[#38332b] dark:text-[#d0aa7b]"
    >
      {initial}
      <span className="absolute -bottom-0.5 -end-0.5 h-2 w-2 rounded-full border-2 border-[#fbfaf8] bg-[#a5a29b] dark:border-[#1d1c1a]" />
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
