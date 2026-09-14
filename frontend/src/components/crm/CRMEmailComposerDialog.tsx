import { useEffect, useMemo, useState } from 'react';
import { EditorContent, useEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  ArrowLeft02Icon,
  Mail01Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { EmojiPicker } from '@/components/support/EmojiPicker';
import { LinkInsertModal } from '@/components/support/LinkInsertModal';
import type { CRMEmailAccount } from '@/lib/crmTypes';
import { crmEmailService } from '@/lib/services/crmService';
import { unwrap } from '@/lib/queryUtils';
import {
  QuietComposerAITools,
  QuietComposerEditorSurface,
  QuietComposerToolbar,
  QuietConversationComposer,
  type ConversationRewriteOperation,
} from '@/components/design-system/quiet';
import { useCRMEmailAttachments } from '@/hooks/useCRMEmailAttachments';
import { CRMEmailAttachmentStrip } from './CRMEmailAttachmentStrip';
import { UpgradeRequiredDialog } from '@edition';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@edition/errors';

export interface EmailDraft {
  title?: string;
  to?: string[];
  cc?: string[];
  subject?: string;
  body?: string;
}

interface CRMEmailComposerDialogProps {
  workspaceId: string;
  accounts: CRMEmailAccount[];
  open: boolean;
  draft?: EmailDraft;
  onOpenChange: (open: boolean) => void;
}

function parseRecipients(value: string) {
  return value.split(/[;,\n]/).map((item) => item.trim()).filter(Boolean);
}

function plainTextToHTML(value: string) {
  const escaped = value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
  return escaped.replace(/\n/g, '<br>');
}

export function CRMEmailComposerDialog({
  workspaceId,
  accounts,
  open,
  draft,
  onOpenChange,
}: CRMEmailComposerDialogProps) {
  const queryClient = useQueryClient();
  const availableAccounts = useMemo(
    () => accounts.filter((account) => account.can_send !== false && account.is_active && account.status === 'connected'),
    [accounts],
  );
  const [accountId, setAccountId] = useState(availableAccounts[0]?.id ?? '');
  const [to, setTo] = useState(draft?.to?.join(', ') ?? '');
  const [cc, setCC] = useState(draft?.cc?.join(', ') ?? '');
  const [showCC, setShowCC] = useState(Boolean(draft?.cc?.length));
  const [subject, setSubject] = useState(draft?.subject ?? '');
  const [bodyHTML, setBodyHTML] = useState(() => plainTextToHTML(draft?.body ?? ''));
  const [bodyText, setBodyText] = useState(draft?.body ?? '');
  const [sending, setSending] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkInitial, setLinkInitial] = useState({ label: '', url: '' });
  const [focused, setFocused] = useState(false);
  const [rewriting, setRewriting] = useState(false);
  const emailAttachments = useCRMEmailAttachments(workspaceId);
  const [upgradeReason, setUpgradeReason] = useState<UpgradeRequiredReason | null>(null);

  const extensions = useMemo(() => [
    StarterKit.configure({
      heading: false,
      codeBlock: false,
      horizontalRule: false,
      link: {
        openOnClick: false,
        autolink: true,
        linkOnPaste: true,
        HTMLAttributes: {
          target: '_blank',
          rel: 'noopener noreferrer nofollow',
        },
      },
    }),
    Placeholder.configure({ placeholder: 'Write your message...' }),
  ], []);

  const editor = useEditor({
    extensions,
    content: plainTextToHTML(draft?.body ?? ''),
    editable: !sending,
    immediatelyRender: false,
    editorProps: {
      attributes: {
        class: 'rich-text-soft prose prose-sm dark:prose-invert max-w-none min-h-[220px] max-h-[45vh] overflow-y-auto px-4 py-3 text-sm leading-relaxed focus:outline-none',
      },
      handleKeyDown: (_view, event) => {
        if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
          event.preventDefault();
          document.getElementById('crm-email-send')?.click();
          return true;
        }
        return false;
      },
    },
    onUpdate: ({ editor: currentEditor }) => {
      setBodyHTML(currentEditor.getHTML());
      setBodyText(currentEditor.getText());
    },
    onFocus: () => setFocused(true),
    onBlur: () => setFocused(false),
  });

  useEffect(() => {
    editor?.setEditable(!sending);
  }, [editor, sending]);

  const effectiveAccountId = availableAccounts.some((account) => account.id === accountId)
    ? accountId
    : availableAccounts[0]?.id ?? '';

  const openLinkModal = () => {
    if (!editor) return;
    const attrs = editor.getAttributes('link') as { href?: string };
    const { from, to: selectionTo, empty } = editor.state.selection;
    let label = '';
    if (editor.isActive('link')) {
      editor.chain().focus().extendMarkRange('link').run();
      const expanded = editor.state.selection;
      label = editor.state.doc.textBetween(expanded.from, expanded.to, ' ');
    } else if (!empty) {
      label = editor.state.doc.textBetween(from, selectionTo, ' ');
    }
    setLinkInitial({ label, url: attrs.href ?? '' });
    setLinkOpen(true);
  };

  const insertLink = (label: string, url: string) => {
    if (!editor) return;
    if (editor.isActive('link')) {
      editor.chain().focus().extendMarkRange('link').unsetLink().run();
    }
    const { from, to: selectionTo, empty } = editor.state.selection;
    if (empty) {
      editor.chain().focus().insertContent({
        type: 'text',
        text: label,
        marks: [{ type: 'link', attrs: { href: url } }],
      }).run();
      return;
    }
    editor.chain().focus().insertContentAt(
      { from, to: selectionTo },
      { type: 'text', text: label, marks: [{ type: 'link', attrs: { href: url } }] },
    ).run();
  };

  const send = async () => {
    const recipients = parseRecipients(to);
    const trimmedSubject = subject.trim();
    if (!effectiveAccountId || recipients.length === 0 || !trimmedSubject || !bodyText.trim()) {
      toast.error('Choose a sender and add a recipient, subject, and message');
      return;
    }

    setSending(true);
    try {
      await unwrap(await crmEmailService.sendEmail(workspaceId, {
        account_id: effectiveAccountId,
        to: recipients,
        cc: parseRecipients(cc),
        subject: trimmedSubject,
        body_html: bodyHTML,
        draft_id: emailAttachments.draftId,
        attachment_ids: emailAttachments.attachmentIds,
      }));
      await queryClient.invalidateQueries({ queryKey: ['crm', workspaceId] });
      toast.success('Email sent');
      emailAttachments.reset();
      onOpenChange(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Email could not be sent');
    } finally {
      setSending(false);
    }
  };

  const canSend = Boolean(
    effectiveAccountId
    && parseRecipients(to).length > 0
    && subject.trim()
    && bodyText.trim()
    && !emailAttachments.uploading
    && !sending,
  );
  const closeComposer = () => {
    emailAttachments.reset();
    onOpenChange(false);
  };

  const rewrite = async (operation: ConversationRewriteOperation) => {
    if (!editor || !bodyText.trim() || rewriting) return;
    setRewriting(true);
    try {
      const result = unwrap(await crmEmailService.rewriteDraft(workspaceId, bodyHTML, operation));
      editor.commands.setContent(result.content);
      editor.commands.focus('end');
    } catch (error) {
      const reason = getUpgradeRequiredReason(error);
      if (reason) setUpgradeReason(reason);
      else toast.error(error instanceof Error ? error.message : 'Email could not be rewritten');
    } finally {
      setRewriting(false);
    }
  };

  return (
    <>
    <Dialog open={open} onOpenChange={(nextOpen) => {
      if (sending) return;
      if (!nextOpen) emailAttachments.reset();
      onOpenChange(nextOpen);
    }}>
      <DialogContent className="max-h-[92vh] w-[min(760px,calc(100vw-2rem))] max-w-none gap-0 overflow-hidden p-0 sm:max-w-none">
        <DialogHeader className="border-b border-border/60 px-5 py-4 text-left">
          <div className="flex items-start gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Mail01Icon className="h-4 w-4" />
            </div>
            <div className="min-w-0">
              <DialogTitle>{draft?.title ?? 'New email'}</DialogTitle>
              <DialogDescription className="mt-1">Send from your connected mailbox and keep the conversation in CRM.</DialogDescription>
            </div>
          </div>
        </DialogHeader>

        <div className="divide-y divide-border/60">
          <div className="grid grid-cols-[64px_minmax(0,1fr)] items-center gap-2 px-5 py-2.5">
            <span className="text-xs font-medium text-muted-foreground">From</span>
            <Select value={effectiveAccountId} onValueChange={setAccountId} disabled={sending}>
              <SelectTrigger className="h-8 border-0 px-0 shadow-none focus:ring-0">
                <SelectValue placeholder="Choose a connected mailbox" />
              </SelectTrigger>
              <SelectContent>
                {availableAccounts.map((account) => (
                  <SelectItem key={account.id} value={account.id}>{account.email_address}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="grid grid-cols-[64px_minmax(0,1fr)_auto] items-center gap-2 px-5 py-2.5">
            <span className="text-xs font-medium text-muted-foreground">To</span>
            <Input
              value={to}
              onChange={(event) => setTo(event.target.value)}
              placeholder="name@company.com"
              className="h-8 border-0 px-0 shadow-none focus-visible:ring-0"
              disabled={sending}
            />
            {!showCC && (
              <Button type="button" variant="ghost" size="sm" className="h-7 px-2 text-xs text-muted-foreground" onClick={() => setShowCC(true)}>
                Cc
              </Button>
            )}
          </div>

          {showCC && (
            <div className="grid grid-cols-[64px_minmax(0,1fr)] items-center gap-2 px-5 py-2.5">
              <span className="text-xs font-medium text-muted-foreground">Cc</span>
              <Input
                value={cc}
                onChange={(event) => setCC(event.target.value)}
                placeholder="Separate addresses with commas"
                className="h-8 border-0 px-0 shadow-none focus-visible:ring-0"
                disabled={sending}
              />
            </div>
          )}

          <div className="grid grid-cols-[64px_minmax(0,1fr)] items-center gap-2 px-5 py-2.5">
            <span className="text-xs font-medium text-muted-foreground">Subject</span>
            <Input
              value={subject}
              onChange={(event) => setSubject(event.target.value)}
              placeholder="Email subject"
              className="h-8 border-0 px-0 font-medium shadow-none focus-visible:ring-0"
              disabled={sending}
            />
          </div>
        </div>

        {availableAccounts.length === 0 ? (
          <div className="mx-5 mt-4 rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm text-muted-foreground">
            Connect an active Gmail account in CRM email settings before sending.
          </div>
        ) : null}

        {editor ? <QuietConversationComposer focused={focused} className="m-5">
          <div className="flex items-center px-3 pt-2">
            <QuietComposerAITools disabled={!bodyText.trim()} pending={rewriting} onSelect={rewrite} />
          </div>
          <QuietComposerEditorSurface><EditorContent editor={editor} /></QuietComposerEditorSurface>
          <CRMEmailAttachmentStrip attachments={emailAttachments.attachments} onRemove={(id) => void emailAttachments.remove(id)} />
          <QuietComposerToolbar
            editor={editor}
            emoji={<EmojiPicker
                onEmojiSelect={(emoji) => editor?.chain().focus().insertContent(emoji).run()}
                side="top"
              />}
            onLink={openLinkModal}
            onAttach={emailAttachments.pickFiles}
            trailing={<Button variant="ghost" size="sm" onClick={closeComposer} disabled={sending}>
                <ArrowLeft02Icon className="h-3.5 w-3.5" />
                Cancel
              </Button>}
            onSubmit={() => void send()}
            submitLabel="Send"
            submitDisabled={!canSend}
            submitting={sending}
          />
          <button id="crm-email-send" type="button" className="hidden" onClick={() => void send()} />
        </QuietConversationComposer> : null}

        <LinkInsertModal
          open={linkOpen}
          onOpenChange={setLinkOpen}
          workspaceId={workspaceId}
          initialLabel={linkInitial.label}
          initialUrl={linkInitial.url}
          onInsert={insertLink}
          onRemove={editor?.isActive('link') ? () => editor.chain().focus().extendMarkRange('link').unsetLink().run() : undefined}
        />
      </DialogContent>
    </Dialog>
    <UpgradeRequiredDialog open={upgradeReason !== null} onOpenChange={(dialogOpen) => { if (!dialogOpen) setUpgradeReason(null); }} reason={upgradeReason} />
    </>
  );
}
