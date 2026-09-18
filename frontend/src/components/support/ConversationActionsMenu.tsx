import { useSupportAIControl } from './SupportAIControl';
import { type ReactNode, useEffect, useState } from 'react';
import { toast } from 'sonner';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
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
import { Checkbox } from '@/components/ui/checkbox';
import {
  Delete01Icon,
  FolderInputIcon,
  InboxIcon,
  Link01Icon,
  Mail01Icon,
  MailOpenIcon,
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
  useSendConversationTranscript,
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
  onSubjectDialogOpenChange?: (open: boolean) => void;
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
  onSubjectDialogOpenChange,
}: ConversationActionsMenuProps) {
  const confirm = useConfirm();
  const aiControl = useSupportAIControl(conversation);
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
  const [transcriptDialogOpen, setTranscriptDialogOpen] = useState(false);
  const transcriptRecipients = Array.from(new Map([
    conversation.customer_email,
    conversation.suggested_primary_recipient_email,
    ...(conversation.email_cc ?? []),
    ...(conversation.email_thread_participants ?? []),
  ].filter((email): email is string => !!email?.trim()).map((email) => [email.trim().toLowerCase(), email.trim()])).values());
  const [transcriptEmail, setTranscriptEmail] = useState('');
  const [updateCustomerEmail, setUpdateCustomerEmail] = useState(false);
  const sendTranscript = useSendConversationTranscript(workspaceId);

  useEffect(() => {
    if (transcriptDialogOpen) {
      setTranscriptEmail(transcriptRecipients[0] ?? '');
      setUpdateCustomerEmail(!conversation.customer_email);
    }
  }, [transcriptDialogOpen, conversation.customer_email, transcriptRecipients.join('|')]);

  useEffect(() => {
    if (!subjectDialogOpen) {
      setSubjectDraft(conversation.subject);
    }
    onSubjectDialogOpenChange?.(subjectDialogOpen);
  }, [conversation.subject, onSubjectDialogOpenChange, subjectDialogOpen]);

  const handleToggleReadState = () => {
    if (isUnread) {
      markConversationRead.mutate({
        conversationId: conversation.id,
        throughMessageId: conversation.last_customer_message_id ?? undefined,
      }, {
        onSuccess: () => toast.success('Marked as read'),
      });
      return;
    }
    markConversationUnread.mutate(conversation.id, {
      onSuccess: () => toast.success('Marked as unread'),
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
        <DropdownMenuContent align={align} className="w-64">
          {aiControl.item && <>{aiControl.item}<DropdownMenuSeparator /></>}
          <DropdownMenuItem onClick={handleToggleReadState} className={itemClassName}>
            <MailOpenIcon className={iconClassName} />
            {isUnread ? 'Mark as read' : 'Mark as unread'}
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleCopyLink} className={itemClassName}>
            <Link01Icon className={iconClassName} />
            Copy link
          </DropdownMenuItem>
          <DropdownMenuItem onClick={() => setTranscriptDialogOpen(true)} className={itemClassName}>
            <Mail01Icon className={iconClassName} />
            Email transcript
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleUpdateSubject} className={itemClassName}>
            <PencilEdit01Icon className={iconClassName} />
            Set subject
          </DropdownMenuItem>
          {moveOptions.length > 0 && (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuSub>
                <DropdownMenuSubTrigger className={itemClassName}>
                  <FolderInputIcon className={iconClassName} />
                  Move to inbox
                </DropdownMenuSubTrigger>
                <DropdownMenuSubContent className="w-48">
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
                      <InboxIcon className={iconClassName} />
                      {option.name}
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuSubContent>
              </DropdownMenuSub>
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
      {aiControl.confirmation}

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
      <Dialog open={transcriptDialogOpen} onOpenChange={setTranscriptDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Email conversation transcript</DialogTitle>
            <DialogDescription>Choose where to send a copy of this conversation.</DialogDescription>
          </DialogHeader>
          <div className="space-y-3">
            {transcriptRecipients.length > 0 && (
              <div className="space-y-2">
                <div className="text-sm font-medium">Recipient</div>
                {transcriptRecipients.map((email) => (
                  <label key={email} className="flex cursor-pointer items-center gap-2 text-sm">
                    <input type="radio" name={`transcript-recipient-${conversation.id}`} value={email} checked={transcriptEmail === email} onChange={() => setTranscriptEmail(email)} />
                    <span className="truncate">{email}</span>
                  </label>
                ))}
                <Input value={transcriptRecipients.includes(transcriptEmail) ? '' : transcriptEmail} onChange={(event) => setTranscriptEmail(event.target.value)} placeholder="Or enter another email" />
              </div>
            )}
            {transcriptRecipients.length === 0 && (
              <Input value={transcriptEmail} onChange={(event) => setTranscriptEmail(event.target.value)} placeholder="recipient@example.com" autoFocus />
            )}
            {(!conversation.customer_email || transcriptEmail.trim().toLowerCase() !== conversation.customer_email.trim().toLowerCase()) && (
              <label className="flex items-start gap-2 text-sm">
                <Checkbox checked={updateCustomerEmail} onCheckedChange={(checked) => setUpdateCustomerEmail(checked === true)} />
                <span>Also save this email to the visitor profile</span>
              </label>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setTranscriptDialogOpen(false)}>Cancel</Button>
            <Button disabled={sendTranscript.isPending || !transcriptEmail.trim()} onClick={() => sendTranscript.mutate({ conversationId: conversation.id, email: transcriptEmail.trim(), updateCustomerEmail }, { onSuccess: (data) => { setTranscriptDialogOpen(false); toast.success(data.message || 'Transcript sent'); }, onError: (error: Error) => toast.error('Failed to send transcript', { description: error.message }) })}>
              {sendTranscript.isPending ? 'Sending…' : 'Send transcript'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
