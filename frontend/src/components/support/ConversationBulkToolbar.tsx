import { useEffect, useRef, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { ArrowTurnBackwardIcon, Cancel01Icon, CheckmarkCircle02Icon, Delete01Icon, FolderInputIcon, MailOpenIcon, Mail01Icon, MoreHorizontalIcon, OctagonXIcon } from '@/lib/icons';
import type { SupportConversation } from '@/lib/pmTypes';
import { queryKeys } from '@/lib/queryKeys';
import { runConversationBulkAction, type ConversationBulkAction } from '@/lib/supportBulkActions';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { QuietDropdown } from '@/components/design-system/quiet';
import type { ConversationActionMoveOption } from './ConversationActionsMenu';

interface Props {
  workspaceId: string;
  conversations: SupportConversation[];
  loadedCount: number;
  allLoadedSelected: boolean;
  hasMore: boolean;
  loadingSelection: boolean;
  canEdit: boolean;
  moveOptions: ConversationActionMoveOption[];
  onSelectLoaded: () => void;
  onClear: () => void;
  onBusyChange: (busy: boolean) => void;
  onCompleted: (ids: string[]) => void;
}

export function ConversationBulkToolbar({ workspaceId, conversations, loadedCount, allLoadedSelected, hasMore, loadingSelection, canEdit, moveOptions, onSelectLoaded, onClear, onBusyChange, onCompleted }: Props) {
  const queryClient = useQueryClient();
  const confirm = useConfirm();
  const running = useRef(false);
  const mounted = useRef(true);
  const [progress, setProgress] = useState<number | null>(null);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const busy = progress !== null;
  const disabled = busy || loadingSelection;
  const count = conversations.length;
  const reopen = conversations.every((conversation) => conversation.status === 'resolved' || conversation.status === 'spam');
  const primaryLabel = reopen ? 'Reopen conversations' : 'Resolve conversations';
  const PrimaryIcon = reopen ? ArrowTurnBackwardIcon : CheckmarkCircle02Icon;

  async function apply(action: ConversationBulkAction, verb: string) {
    if (running.current || loadingSelection || count === 0 || (!canEdit && action.type !== 'read' && action.type !== 'unread')) return;
    running.current = true;
    setProgress(0);
    onBusyChange(true);
    try {
      if (action.type === 'delete') {
        const approved = await confirm({
          title: `Delete ${count} conversation${count === 1 ? '' : 's'}?`,
          description: 'This permanently deletes the selected conversations and their messages. This cannot be undone.',
          confirmText: 'Delete',
          variant: 'destructive',
        });
        if (!approved || !mounted.current) return;
      }
      const result = await runConversationBulkAction(workspaceId, conversations, action, (completed) => {
        if (mounted.current) setProgress(completed);
      });
      const inbox = useSupportInboxStore.getState();
      if (mounted.current && inbox.selectedConversationId && result.succeeded.includes(inbox.selectedConversationId)
        && (action.type === 'delete' || action.type === 'move' || (action.type === 'status' && action.status !== 'open'))) {
        inbox.selectConversation(null);
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['support', workspaceId] }),
        queryClient.invalidateQueries({ queryKey: queryKeys.support.workspaceUnread() }),
      ]);
      if (mounted.current) onCompleted(result.succeeded);
      if (result.failed.length > 0) {
        toast.error(`${result.failed.length} conversation${result.failed.length === 1 ? '' : 's'} could not be updated`, {
          description: `${result.succeeded.length} updated.${mounted.current ? ' Failed conversations remain selected.' : ''} ${result.failed[0].error}`,
        });
      } else {
        toast.success(`${verb} ${result.succeeded.length} conversation${result.succeeded.length === 1 ? '' : 's'}`);
      }
    } catch (error) {
      toast.error('Could not complete bulk action', { description: error instanceof Error ? error.message : 'Please try again.' });
    } finally {
      running.current = false;
      if (mounted.current) {
        setProgress(null);
        onBusyChange(false);
      }
    }
  }

  return (
    <>
      <label className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 pl-1.5">
        <Checkbox
          className="border-neutral-400 bg-white data-[state=checked]:border-neutral-400 data-[state=checked]:bg-white data-[state=checked]:text-neutral-900 dark:data-[state=checked]:bg-white"
          aria-label={allLoadedSelected ? 'Clear selection' : `Select all ${loadedCount} loaded conversations`}
          checked={allLoadedSelected}
          disabled={disabled}
          onCheckedChange={() => allLoadedSelected ? onClear() : onSelectLoaded()}
        />
        <span className="min-w-0 text-xs leading-tight">
          {busy ? (
            <span role="status">Updating {progress}/{count}</span>
          ) : allLoadedSelected ? (
            <span role="status">{count} selected</span>
          ) : (
            <>
              <span className="block truncate font-medium">{hasMore ? 'Select loaded' : 'Select all'} ({loadedCount})</span>
              <span className="block text-[11px] text-muted-foreground" role="status">{count} selected</span>
            </>
          )}
        </span>
      </label>
      {canEdit && <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" aria-label={primaryLabel} disabled={disabled} onClick={() => void apply({ type: 'status', status: reopen ? 'open' : 'resolved' }, reopen ? 'Reopened' : 'Resolved')}>
            <PrimaryIcon className="h-3.5 w-3.5" />
          </Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">{primaryLabel}</TooltipContent>
      </Tooltip>}
      {canEdit && moveOptions.length > 0 && <QuietDropdown
        options={moveOptions.map((option) => ({ value: option.id, label: option.name }))}
        onSelect={(value) => { void apply({ type: 'move', mailboxId: value === 'shared' ? null : value }, 'Moved'); }}
        label="Move to inbox"
        disabled={disabled}
        triggerWrapper={(trigger) => <Tooltip><TooltipTrigger asChild>{trigger}</TooltipTrigger><TooltipContent side="bottom">Move to inbox</TooltipContent></Tooltip>}
        trigger={<Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" aria-label="Move conversations to inbox" disabled={disabled}><FolderInputIcon className="h-3.5 w-3.5" /></Button>}
      />}
      <DropdownMenu>
        <Tooltip>
          <TooltipTrigger asChild>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" aria-label="More bulk actions" disabled={disabled}><MoreHorizontalIcon className="h-3.5 w-3.5" /></Button>
            </DropdownMenuTrigger>
          </TooltipTrigger>
          <TooltipContent side="bottom">More actions</TooltipContent>
        </Tooltip>
        <DropdownMenuContent align="end">
          <DropdownMenuItem onSelect={() => void apply({ type: 'read' }, 'Marked as read:')}><MailOpenIcon className="h-3.5 w-3.5" />Mark as read</DropdownMenuItem>
          <DropdownMenuItem onSelect={() => void apply({ type: 'unread' }, 'Marked as unread:')}><Mail01Icon className="h-3.5 w-3.5" />Mark as unread</DropdownMenuItem>
          {canEdit && <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => void apply({ type: 'status', status: reopen ? 'resolved' : 'open' }, reopen ? 'Resolved' : 'Reopened')}>
              {reopen ? <CheckmarkCircle02Icon className="h-3.5 w-3.5" /> : <ArrowTurnBackwardIcon className="h-3.5 w-3.5" />}{reopen ? 'Resolve' : 'Reopen'}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => void apply({ type: 'status', status: 'spam' }, 'Marked as spam:')}><OctagonXIcon className="h-3.5 w-3.5" />Mark as spam</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive focus:text-destructive" onSelect={() => void apply({ type: 'delete' }, 'Deleted')}><Delete01Icon className="h-3.5 w-3.5" />Delete</DropdownMenuItem>
          </>}
        </DropdownMenuContent>
      </DropdownMenu>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" aria-label="Clear selection" disabled={busy} onClick={onClear}><Cancel01Icon className="h-3.5 w-3.5" /></Button>
        </TooltipTrigger>
        <TooltipContent side="bottom">Clear selection</TooltipContent>
      </Tooltip>
    </>
  );
}
