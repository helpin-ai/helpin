import { type ReactNode, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import {
  Delete01Icon,
  InboxIcon,
  Link01Icon,
  MailOpenIcon,
  Message01Icon,
  OctagonXIcon,
  PencilEdit01Icon,
} from '@/lib/icons';
import type { ConversationStatus, SupportConversation } from '@/lib/pmTypes';
import {
  useDeleteConversation,
  useMarkConversationRead,
  useMarkConversationUnread,
  useMoveConversation,
  useUpdateConversationStatus,
  useUpdateConversationSubject,
} from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export interface ConversationActionMoveOption {
  id: string;
  name: string;
}

interface ConversationActionsMenuProps {
  workspaceId: string;
  conversation: SupportConversation;
  moveOptions?: ConversationActionMoveOption[];
  trigger: ReactNode;
  align?: 'start' | 'center' | 'end';
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  onConversationDeleted?: () => void;
  onConversationMoved?: (option: ConversationActionMoveOption) => void;
}

function buildConversationLink(workspaceSlug: string | undefined, conversationId: string) {
  if (typeof window === 'undefined') return '';
  if (workspaceSlug) {
    return new URL(`/w/${workspaceSlug}/support/${conversationId}`, window.location.origin).toString();
  }
  return window.location.href;
}

export function ConversationActionsMenu({
  workspaceId,
  conversation,
  moveOptions = [],
  trigger,
  align = 'end',
  open,
  onOpenChange,
  onConversationDeleted,
  onConversationMoved,
}: ConversationActionsMenuProps) {
  const confirm = useConfirm();
  const markConversationRead = useMarkConversationRead(workspaceId);
  const markConversationUnread = useMarkConversationUnread(workspaceId);
  const updateSubject = useUpdateConversationSubject(workspaceId);
  const updateStatus = useUpdateConversationStatus(workspaceId);
  const deleteConversation = useDeleteConversation(workspaceId);
  const moveConversation = useMoveConversation(workspaceId);
  const workspaceSlug = useWorkspaceStore((s) => s.currentWorkspace?.slug);
  const selectedConversationId = useSupportInboxStore((s) => s.selectedConversationId);
  const selectConversation = useSupportInboxStore((s) => s.selectConversation);
  const isUnread = (conversation.unread_count ?? 0) > 0;
  const itemClassName = 'text-[13px]';
  const iconClassName = 'h-3.5 w-3.5';
  const [subjectDialogOpen, setSubjectDialogOpen] = useState(false);
  const [subjectDraft, setSubjectDraft] = useState(conversation.subject);

  useEffect(() => {
    if (!subjectDialogOpen) {
      setSubjectDraft(conversation.subject);
    }
  }, [conversation.subject, subjectDialogOpen]);

  const handleToggleReadState = () => {
    const mutation = isUnread ? markConversationRead : markConversationUnread;
    mutation.mutate(conversation.id, {
      onSuccess: () => {
        toast.success(isUnread ? 'Marked as read' : 'Marked as unread');
      },
    });
  };

  const handleCopyLink = async () => {
    try {
      await navigator.clipboard.writeText(buildConversationLink(workspaceSlug, conversation.id));
      toast.success('Link copied to clipboard');
    } catch (error) {
      toast.error('Failed to copy link', {
        description: error instanceof Error ? error.message : 'Clipboard access was not available.',
      });
    }
  };

  const handleUpdateSubject = () => {
    setSubjectDialogOpen(true);
  };

  const handleSaveSubject = () => {
    const trimmed = subjectDraft.trim();
    if (!trimmed) return;
    updateSubject.mutate({ conversationId: conversation.id, subject: trimmed }, {
      onSuccess: () => {
        setSubjectDialogOpen(false);
        toast.success('Conversation subject updated');
      },
    });
  };

  const handleToggleSpam = () => {
    const nextStatus: ConversationStatus = conversation.status === 'spam' ? 'open' : 'spam';
    updateStatus.mutate({ conversationId: conversation.id, status: nextStatus }, {
      onSuccess: () => {
        toast.success(nextStatus === 'spam' ? 'Marked as spam' : 'Restored to inbox');
      },
    });
  };

  const handleDelete = async () => {
    const ok = await confirm({
      title: 'Delete conversation?',
      description: 'This will permanently delete this conversation. This action cannot be undone.',
      confirmText: 'Delete',
      variant: 'destructive',
    });
    if (!ok) return;

    deleteConversation.mutate(conversation.id, {
      onSuccess: () => {
        if (selectedConversationId === conversation.id) {
          selectConversation(null);
        }
        onConversationDeleted?.();
        toast.success('Conversation deleted');
      },
    });
  };

  return (
    <>
      <DropdownMenu open={open} onOpenChange={onOpenChange}>
        <DropdownMenuTrigger asChild>
          {trigger}
        </DropdownMenuTrigger>
        <DropdownMenuContent align={align} className="w-52">
          <DropdownMenuItem onClick={handleToggleReadState} className={itemClassName}>
            <MailOpenIcon className={iconClassName} />
            {isUnread ? 'Mark as read' : 'Mark as unread'}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleCopyLink} className={itemClassName}>
            <Link01Icon className={iconClassName} />
            Copy link
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleUpdateSubject} className={itemClassName}>
            <PencilEdit01Icon className={iconClassName} />
            Set subject
          </DropdownMenuItem>
          {moveOptions.length > 0 && (
            <>
              <DropdownMenuSeparator />
              {moveOptions.map((option) => (
                <DropdownMenuItem
                  key={option.id}
                  className={itemClassName}
                  onClick={() => {
                    moveConversation.mutate({
                      conversationId: conversation.id,
                      mailboxId: option.id === 'shared' ? null : option.id,
                    }, {
                      onSuccess: () => {
                        onConversationMoved?.(option);
                        toast.success(`Moved to ${option.name}`);
                      },
                    });
                  }}
                >
                  <Message01Icon className={iconClassName} />
                  Move to {option.name}
                </DropdownMenuItem>
              ))}
            </>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem
            className={
              conversation.status === 'spam'
                ? itemClassName
                : `${itemClassName} text-destructive focus:text-destructive`
            }
            onClick={handleToggleSpam}
          >
            {conversation.status === 'spam' ? (
              <>
                <InboxIcon className={iconClassName} />
                Restore to inbox
              </>
            ) : (
              <>
                <OctagonXIcon className={iconClassName} />
                Mark as spam
              </>
            )}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            className={`${itemClassName} text-destructive focus:text-destructive`}
            onClick={() => { void handleDelete(); }}
          >
            <Delete01Icon className={iconClassName} />
            Delete conversation
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      {subjectDialogOpen && (
        <Dialog open={subjectDialogOpen} onOpenChange={setSubjectDialogOpen}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>Set conversation subject</DialogTitle>
              <DialogDescription>
                Update the conversation title shown in the inbox and thread header.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-2">
              <label htmlFor={`conversation-subject-${conversation.id}`} className="text-sm font-medium">
                Subject
              </label>
              <Input
                id={`conversation-subject-${conversation.id}`}
                value={subjectDraft}
                onChange={(event) => setSubjectDraft(event.target.value)}
                placeholder="Enter a conversation subject"
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => {
                  setSubjectDialogOpen(false);
                  setSubjectDraft(conversation.subject);
                }}
              >
                Cancel
              </Button>
              <Button
                onClick={handleSaveSubject}
                disabled={updateSubject.isPending || subjectDraft.trim().length === 0}
              >
                Save
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </>
  );
}
