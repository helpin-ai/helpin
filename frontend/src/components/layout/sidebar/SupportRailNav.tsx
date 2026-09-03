import { useState } from 'react';
import { InboxIcon, PlusSignIcon, BookOpen01Icon, LifebuoyIcon, MoreVerticalIcon, PencilEdit01Icon, ArchiveIcon, FileSearchIcon, Delete01Icon, UserGroupIcon, BotIcon } from '@/lib/icons';
import { StoredIcon } from '@/components/ui/icon-picker';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
} from '@/components/ui/sidebar';
import { supportAiItems, supportFilterItems } from './config';
import { SidebarSectionAction } from './SidebarSectionAction';
import type { SupportInboxView, SupportInboxViewCount, UnreadStats } from '@/lib/pmTypes';

type SupportNavFilter =
  | 'inbox'
  | 'mine'
  | 'waiting'
  | 'resolved'
  | 'spam'
  | 'ai_active'
  | 'resolved_by_ai';

type InboxScopes = {
  mailboxes?: Array<{
    id: string;
    name: string;
    icon?: string | null;
    total_count?: number;
    unread_count: number;
    needs_human_reply_count?: number;
  }>;
};

type SupportRailNavProps = {
  navFilter: SupportNavFilter;
  unreadStats?: UnreadStats | null;
  inboxUnreadStats?: UnreadStats | null;
  globalUnreadStats?: UnreadStats | null;
  inboxScopes?: InboxScopes | null;
  selectedMailboxId: string;
  activeCustomViewId?: string | null;
  currentUserId?: string;
  customViews?: SupportInboxView[];
  customViewCounts?: Record<string, SupportInboxViewCount>;
  canManageSettings: boolean;
  wsSlug: string;
  pathname: string;
  onNavFilterChange: (value: SupportNavFilter) => void;
  onMailboxSelect: (mailboxId: string) => void;
  onCustomViewSelect: (view: SupportInboxView) => void;
  onEditCustomView: (view: SupportInboxView) => void;
  onDeleteCustomView: (view: SupportInboxView) => void;
  onCreateMailbox: () => void;
  onEditMailbox: (mailboxId: string) => void;
  onArchiveMailbox: (mailboxId: string) => void;
  onNavigate: (to: string) => void;
};

export function SupportRailNav({
  navFilter,
  unreadStats,
  inboxUnreadStats,
  globalUnreadStats,
  inboxScopes,
  selectedMailboxId,
  activeCustomViewId,
  currentUserId,
  customViews = [],
  customViewCounts = {},
  canManageSettings,
  wsSlug,
  pathname,
  onNavFilterChange,
  onMailboxSelect,
  onCustomViewSelect,
  onEditCustomView,
  onDeleteCustomView,
  onCreateMailbox,
  onEditMailbox,
  onArchiveMailbox,
  onNavigate,
}: SupportRailNavProps) {
  const isOnCoverage = pathname.startsWith(`/w/${wsSlug}/support/coverage`);
  const isOnSearch = pathname.startsWith(`/w/${wsSlug}/support/search`);
  const mailboxes = inboxScopes?.mailboxes ?? [];
  const aiStats = selectedMailboxId === 'all' ? (globalUnreadStats ?? unreadStats) : globalUnreadStats;
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);

  const handleSettingsNavigate = (section: string) => {
    onNavigate(`/w/${wsSlug}/settings/${section}`);
  };

  const renderUnreadDot = (unread?: number | null) => {
    const unreadValue = unread ?? 0;
    if (unreadValue <= 0) return null;

    return (
      <span
        data-slot="support-unread-dot"
        className="h-1.5 w-1.5 shrink-0 rounded-full bg-red-500"
        title={`${unreadValue > 99 ? '99+' : unreadValue} unread`}
      />
    );
  };

  const renderCounts = (total?: number | null, className = '', alignRight = true) => {
    const totalValue = total ?? 0;
    if (totalValue <= 0) return null;

    return (
      <span className={`${alignRight ? 'ml-auto' : ''} flex h-5 min-w-4 shrink-0 items-center justify-center ${className}`}>
        <span
          data-slot="support-total-count"
          className="min-w-[1ch] text-right text-xs font-medium tabular-nums text-muted-foreground"
        >
          {totalValue > 99 ? '99+' : totalValue}
        </span>
      </span>
    );
  };

  const supportMenuRowClassName = 'grid h-8 grid-cols-[1rem_minmax(0,1fr)_1rem] items-center gap-2 rounded-md px-2 text-sm';
  const trailingSlotClassName = 'relative flex h-5 min-w-4 shrink-0 items-center justify-center';
  const trailingMenuButtonClassName = 'absolute inset-0 inline-flex h-5 min-w-4 items-center justify-center rounded-sm text-muted-foreground transition-opacity hover:text-foreground';

  return (
    <>
      <SidebarGroup className="p-0 pb-3">
        <SidebarMenu>
          {supportFilterItems.map((item) => {
            const unread =
              item.key === 'inbox'
                ? inboxUnreadStats?.inbox
                : item.key === 'mine'
                  ? (globalUnreadStats ?? unreadStats)?.mine
                  : item.key === 'waiting'
                    ? (globalUnreadStats ?? unreadStats)?.waiting
                    : undefined;
            const total =
              item.key === 'inbox'
                ? inboxUnreadStats?.inbox_total
                : item.key === 'mine'
                  ? (globalUnreadStats ?? unreadStats)?.mine_total
                  : item.key === 'waiting'
                    ? (globalUnreadStats ?? unreadStats)?.waiting_total
                    : undefined;

            return (
              <SidebarMenuItem key={item.key}>
                <SidebarMenuButton
                  isActive={!isOnSearch && !isOnCoverage && !activeCustomViewId && navFilter === item.key && (item.key !== 'inbox' || selectedMailboxId === 'all')}
                  className={supportMenuRowClassName}
                  onClick={() => onNavFilterChange(item.key)}
                >
                  <item.icon className="h-4 w-4" />
                  <span className="flex min-w-0 flex-1 items-center gap-1.5">
                    <span className="truncate">{item.label}</span>
                    {renderUnreadDot(unread)}
                  </span>
                  {renderCounts(total)}
                </SidebarMenuButton>
              </SidebarMenuItem>
            );
          })}
        </SidebarMenu>
      </SidebarGroup>

      <SidebarGroup className="p-0 pb-3">
        <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
          Helpin AI
        </SidebarGroupLabel>
        <SidebarMenu>
          {supportAiItems.map((item) => {
            const total =
              item.key === 'ai_active'
                ? aiStats?.ai_active_total
                : undefined;

            return (
              <SidebarMenuItem key={item.key}>
                <SidebarMenuButton
                  isActive={!isOnSearch && !isOnCoverage && !activeCustomViewId && navFilter === item.key}
                  className={supportMenuRowClassName}
                  onClick={() => onNavFilterChange(item.key)}
                >
                  <item.icon className="h-4 w-4" />
                  <span className="flex min-w-0 flex-1 items-center gap-1.5">
                    <span className="truncate">{item.label}</span>
                  </span>
                  {renderCounts(total)}
                </SidebarMenuButton>
              </SidebarMenuItem>
            );
          })}
        </SidebarMenu>
      </SidebarGroup>

      <SidebarSeparator className="mx-2 my-1" />

      <SidebarGroup className="p-0 pb-3">
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              isActive={isOnCoverage}
              className={supportMenuRowClassName}
              onClick={() => onNavigate(`/w/${wsSlug}/support/coverage`)}
            >
              <FileSearchIcon className="h-4 w-4" />
              <span className="min-w-0 truncate">Coverage Gaps</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarGroup>

      <SidebarGroup className="p-0 pb-3">
        <SidebarGroupLabel className="flex h-7 items-center justify-between px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
          <span>Team Inboxes</span>
          {canManageSettings && mailboxes.length > 0 && (
            <SidebarSectionAction
              label="Create Team Inbox"
              dataSlot="support-create-team-inbox-action"
              onClick={onCreateMailbox}
            />
          )}
        </SidebarGroupLabel>
        <SidebarMenu>
          {mailboxes.map((mailbox) => {
            const isActiveMailbox = !isOnSearch && !isOnCoverage && !activeCustomViewId && selectedMailboxId === mailbox.id;
            const isMenuOpen = openMenuId === mailbox.id;

            return (
              <SidebarMenuItem key={mailbox.id} className="group/mailbox">
                <SidebarMenuButton
                  isActive={isActiveMailbox}
                  className={supportMenuRowClassName}
                  onClick={() => onMailboxSelect(mailbox.id)}
                >
                  <StoredIcon
                    name={mailbox.icon}
                    className="h-4 w-4"
                    fallback={<InboxIcon className="h-4 w-4" />}
                  />
                  <span className="flex min-w-0 flex-1 items-center gap-1.5">
                    <span className="truncate">{mailbox.name}</span>
                    {renderUnreadDot(mailbox.unread_count)}
                  </span>
                  <span className={trailingSlotClassName}>
                    {renderCounts(
                      mailbox.total_count,
                      `transition-opacity ${canManageSettings && !isMenuOpen ? 'group-hover/mailbox:opacity-0' : ''} ${isMenuOpen ? 'opacity-0' : 'opacity-100'}`,
                      false,
                    )}
                    {canManageSettings && (
                      <DropdownMenu onOpenChange={(open) => setOpenMenuId(open ? mailbox.id : null)}>
                        <DropdownMenuTrigger asChild>
                          <span
                            role="button"
                            onPointerDown={(event) => event.stopPropagation()}
                            onClick={(event) => event.stopPropagation()}
                            className={`${trailingMenuButtonClassName} ${
                              isMenuOpen
                                ? 'opacity-100'
                                : (mailbox.needs_human_reply_count ?? 0) > 0
                                  ? 'opacity-0 group-hover/mailbox:opacity-100'
                                  : 'opacity-0 group-hover/mailbox:opacity-100'
                            }`}
                          >
                            <MoreVerticalIcon className="h-3.5 w-3.5" />
                          </span>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent side="right" align="start" className="w-36">
                          <DropdownMenuItem
                            onClick={(event) => {
                              event.stopPropagation();
                              onEditMailbox(mailbox.id);
                            }}
                          >
                            <PencilEdit01Icon className="mr-2 h-3.5 w-3.5" />
                            Edit
                          </DropdownMenuItem>
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={(event) => {
                              event.stopPropagation();
                              onArchiveMailbox(mailbox.id);
                            }}
                          >
                            <ArchiveIcon className="mr-2 h-3.5 w-3.5" />
                            Archive
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    )}
                  </span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            );
          })}
          {mailboxes.length === 0 && (
            <SidebarMenuItem>
              <SidebarMenuButton
                className={supportMenuRowClassName}
                onClick={onCreateMailbox}
              >
                <PlusSignIcon className="h-4 w-4" />
                <span className="min-w-0 truncate">Create Inbox</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          )}
        </SidebarMenu>
      </SidebarGroup>

      {customViews.length > 0 && (
        <SidebarGroup className="p-0 pb-3">
          <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
            Custom views
          </SidebarGroupLabel>
          <SidebarMenu>
            {customViews.map((view) => {
              const isActive = !isOnSearch && !isOnCoverage && activeCustomViewId === view.id;
              const isMenuOpen = openMenuId === view.id;
              const canModifyView = view.created_by === currentUserId || (view.is_shared && canManageSettings);
              const count = customViewCounts[view.id];

              return (
                <SidebarMenuItem key={view.id} className="group/custom-view">
                  <SidebarMenuButton
                    isActive={isActive}
                    className={supportMenuRowClassName}
                    onClick={() => onCustomViewSelect(view)}
                  >
                    <FileSearchIcon className="h-4 w-4" />
                    <span className="flex min-w-0 flex-1 items-center gap-1.5">
                      <span className="truncate">{view.name}</span>
                      {view.is_shared && <UserGroupIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />}
                      {renderUnreadDot(count?.unread_count)}
                    </span>
                    <span className={trailingSlotClassName}>
                      {renderCounts(
                        count?.total_count,
                        `transition-opacity ${canModifyView && !isMenuOpen ? 'group-hover/custom-view:opacity-0' : ''} ${isMenuOpen ? 'opacity-0' : 'opacity-100'}`,
                        false,
                      )}
                      {canModifyView && (
                        <DropdownMenu onOpenChange={(open) => setOpenMenuId(open ? view.id : null)}>
                          <DropdownMenuTrigger asChild>
                            <span
                              role="button"
                              onPointerDown={(event) => event.stopPropagation()}
                              onClick={(event) => event.stopPropagation()}
                              className={`${trailingMenuButtonClassName} ${
                                isMenuOpen
                                  ? 'opacity-100'
                                  : view.is_shared
                                    ? 'opacity-0 group-hover/custom-view:opacity-100'
                                    : 'opacity-0 group-hover/custom-view:opacity-100'
                              }`}
                            >
                              <MoreVerticalIcon className="h-3.5 w-3.5" />
                            </span>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent side="right" align="start" className="w-36">
                            <DropdownMenuItem
                              onClick={(event) => {
                                event.stopPropagation();
                                onEditCustomView(view);
                              }}
                            >
                              <PencilEdit01Icon className="mr-2 h-3.5 w-3.5" />
                              Edit
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              className="text-destructive focus:text-destructive"
                              onClick={(event) => {
                                event.stopPropagation();
                                onDeleteCustomView(view);
                              }}
                            >
                              <Delete01Icon className="mr-2 h-3.5 w-3.5" />
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      )}
                    </span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              );
            })}
          </SidebarMenu>
        </SidebarGroup>
      )}

      <div className="support-bottom-bar sticky bottom-0 z-10 -mx-2 mt-auto border-t border-border/70 bg-[#fafafa] px-2 py-2 dark:bg-sidebar">
        <div className="flex items-center justify-around">
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => handleSettingsNavigate('support-ai-assistant')}
              >
                <BotIcon className="h-4 w-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top">AI Assistant</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => handleSettingsNavigate('knowledge')}
              >
                <BookOpen01Icon className="h-4 w-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top">Brand Knowledge</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => handleSettingsNavigate('inboxes-routing')}
              >
                <InboxIcon className="h-4 w-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top">Inboxes & Routing</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => handleSettingsNavigate('chat-general')}
              >
                <LifebuoyIcon className="h-4 w-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="top">Chat Widget</TooltipContent>
          </Tooltip>
        </div>
      </div>
    </>
  );
}
