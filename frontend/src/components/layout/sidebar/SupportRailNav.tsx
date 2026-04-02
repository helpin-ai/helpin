import { Inbox, Plus } from 'lucide-react';
import { ICON_MAP } from '@/components/ui/icon-picker';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from '@/components/ui/sidebar';
import { supportAiItems, supportFilterItems } from './config';

type SupportNavFilter =
  | 'my_inbox'
  | 'unassigned'
  | 'mentions'
  | 'all'
  | 'ai_all'
  | 'ai_pending'
  | 'ai_resolved'
  | 'ai_escalated';

type InboxScopes = {
  mailboxes?: Array<{
    id: string;
    name: string;
    icon?: string | null;
    unread_count: number;
  }>;
};

type SupportRailNavProps = {
  navFilter: SupportNavFilter;
  activeContext: 'nav' | 'mailbox';
  unreadStats?: {
    my_inbox?: number;
    unassigned?: number;
    total?: number;
  } | null;
  inboxScopes?: InboxScopes | null;
  selectedMailboxId: string;
  canManageSettings: boolean;
  onNavFilterChange: (value: SupportNavFilter) => void;
  onMailboxSelect: (mailboxId: string) => void;
  onCreateMailbox: () => void;
};

export function SupportRailNav({
  navFilter,
  activeContext,
  unreadStats,
  inboxScopes,
  selectedMailboxId,
  canManageSettings,
  onNavFilterChange,
  onMailboxSelect,
  onCreateMailbox,
}: SupportRailNavProps) {
  const mailboxes = inboxScopes?.mailboxes ?? [];

  return (
    <>
      <SidebarMenu className="p-0 pb-3">
        {supportFilterItems.map((item) => {
          const badge =
            item.key === 'my_inbox'
              ? unreadStats?.my_inbox
              : item.key === 'unassigned'
                ? unreadStats?.unassigned
                : item.key === 'all'
                  ? unreadStats?.total
                  : undefined;

          return (
            <SidebarMenuItem key={item.key}>
              <SidebarMenuButton
                isActive={navFilter === item.key && activeContext === 'nav'}
                className="h-8 rounded-md px-2 text-sm"
                onClick={() => onNavFilterChange(item.key)}
              >
                <item.icon />
                <span className="flex-1">{item.label}</span>
                {badge != null && badge > 0 && (
                  <span className="ml-auto inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-blue-600 px-1 text-[10px] font-semibold text-white">
                    {badge > 99 ? '99+' : badge}
                  </span>
                )}
              </SidebarMenuButton>
            </SidebarMenuItem>
          );
        })}
      </SidebarMenu>

      <SidebarGroup className="p-0 pb-3">
        <SidebarGroupLabel className="h-7 px-2 text-[11px] uppercase tracking-wide text-muted-foreground/90">
          Helpin AI Agent
        </SidebarGroupLabel>
        <SidebarMenu>
          {supportAiItems.map((item) => (
            <SidebarMenuItem key={item.key}>
              <SidebarMenuButton
                isActive={navFilter === item.key && activeContext === 'nav'}
                className="h-8 rounded-md px-2 text-sm"
                onClick={() => onNavFilterChange(item.key)}
              >
                <item.icon />
                <span>{item.label}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          ))}
        </SidebarMenu>
      </SidebarGroup>

      <SidebarGroup className="p-0 pb-3">
        <div className="flex items-center justify-between px-2 pr-3">
          <SidebarGroupLabel className="h-7 px-0 text-[11px] uppercase tracking-wide text-muted-foreground/90">
            Team Inboxes
          </SidebarGroupLabel>
          {canManageSettings && mailboxes.length > 0 && (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-5 w-5 rounded-sm text-muted-foreground hover:text-foreground"
                  onClick={onCreateMailbox}
                >
                  <Plus className="h-3.5 w-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right">Create Team Inbox</TooltipContent>
            </Tooltip>
          )}
        </div>
        <SidebarMenu>
          {mailboxes.map((mailbox) => {
            const MailboxIcon = mailbox.icon ? (ICON_MAP[mailbox.icon] ?? Inbox) : Inbox;
            const isActiveMailbox = selectedMailboxId === mailbox.id;

            return (
              <SidebarMenuItem key={mailbox.id}>
                <SidebarMenuButton
                  isActive={isActiveMailbox}
                  className="h-8 rounded-md px-2 text-sm"
                  onClick={() => onMailboxSelect(isActiveMailbox ? 'shared' : mailbox.id)}
                >
                  <MailboxIcon className="h-4 w-4" />
                  <span className="flex-1 truncate">{mailbox.name}</span>
                  {mailbox.unread_count > 0 && (
                    <span className="ml-auto inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-blue-600 px-1 text-[10px] font-semibold text-white">
                      {mailbox.unread_count > 99 ? '99+' : mailbox.unread_count}
                    </span>
                  )}
                </SidebarMenuButton>
              </SidebarMenuItem>
            );
          })}
          {mailboxes.length === 0 && (
            <SidebarMenuItem>
              <SidebarMenuButton
                className="h-8 rounded-md px-2 text-sm"
                onClick={onCreateMailbox}
              >
                <Plus className="h-4 w-4" />
                <span>Create Inbox</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          )}
        </SidebarMenu>
      </SidebarGroup>
    </>
  );
}
