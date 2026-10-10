import { createSupportComposerExtensions } from './supportComposerExtensions';
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { EditorContent, useEditor } from '@tiptap/react';
import Placeholder from '@tiptap/extension-placeholder';
import UnderlineExtension from '@tiptap/extension-underline';
import {
  ArrowReloadHorizontalIcon,
  ArrowUp01Icon,
  ArrowUpDownIcon,
  Briefcase01Icon,
  Cancel01Icon,
  CodeIcon,
  LeftToRightListBulletIcon,
  LeftToRightListNumberIcon,
  Link01Icon,
  Loading01Icon,
  Mail01Icon,
  Message01Icon,
  PlusSignIcon,
  QuoteDownIcon,
  SmileIcon,
  SparklesIcon,
  TextBoldIcon,
  TextItalicIcon,
  TextStrikethroughIcon,
  TextUnderlineIcon,
  TickDouble01Icon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { EmailChipInput } from '@/components/ui/email-chip-input';
import { Input } from '@/components/ui/input';
import { QuietSearchInput } from '@/components/design-system/quiet';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useContacts } from '@/hooks/queries/useCRM';
import { useCannedResponses, useCreateConversationWithMessage, useInboxScopes, useRewriteSupportDraft, useSupportTags } from '@/hooks/queries/useSupport';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import type { CRMContact } from '@/lib/crmTypes';
import type { SupportAIRewriteOperation, SupportCannedResponse, SupportInboxScope } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { EmojiPicker } from './EmojiPicker';
import { LinkInsertModal } from './LinkInsertModal';
import { SupportTagPicker } from './SupportTagPicker';
import { filterShortcuts, stripShortcutContent } from './shortcutFiltering';
import { resolveShortcutVariables } from './shortcutVariables';

interface NewConversationDialogProps {
  workspaceId: string;
  workspaceSlug: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

function contactName(contact: CRMContact) {
  return [contact.first_name, contact.last_name].filter(Boolean).join(' ').trim() || contact.email || 'Unnamed contact';
}

function mailboxValue(value: string) {
  return value === 'shared' || value === 'all' ? null : value;
}

function isMailboxOption(value: SupportInboxScope | null | undefined): value is SupportInboxScope {
  return !!value;
}

function getEditorMarkdown(editorInstance: ReturnType<typeof useEditor> | null | undefined): string {
  if (!editorInstance) return '';
  const storage = (editorInstance.storage as { markdown?: { getMarkdown(): string } }).markdown;
  if (storage?.getMarkdown) return storage.getMarkdown();
  return editorInstance.getText();
}

function FormatButton({
  active = false,
  title,
  onClick,
  children,
}: {
  active?: boolean;
  title: string;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={title}
      aria-pressed={active}
      title={title}
      onClick={(event) => {
        event.preventDefault();
        onClick();
      }}
      onMouseDown={(event) => event.preventDefault()}
      className={cn(
        'inline-flex h-6 w-6 items-center justify-center rounded-md transition-colors',
        active ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
      )}
    >
      {children}
    </button>
  );
}

function detectShortcuts(
  editorInstance: ReturnType<typeof useEditor>,
  shortcuts: SupportCannedResponse[],
): { from: number; to: number; query: string; items: SupportCannedResponse[]; selectedIndex: number } | null {
  if (!editorInstance) return null;
  const { selection } = editorInstance.state;
  if (!selection.empty) return null;
  const textBefore = selection.$from.parent.textBetween(0, selection.$from.parentOffset, undefined, '\ufffc');
  const match = textBefore.match(/(^|\s)!([^\s!]*)$/);
  if (!match) return null;
  const query = match[2].toLowerCase();
  return {
    from: selection.from - (query.length + 1),
    to: selection.from,
    query,
    items: filterShortcuts(shortcuts, query),
    selectedIndex: 0,
  };
}

function ShortcutList({
  items,
  selectedIndex,
  onSelect,
}: {
  items: SupportCannedResponse[];
  selectedIndex?: number;
  onSelect: (shortcut: SupportCannedResponse) => void;
}) {
  return (
    <div className="max-h-64 overflow-y-auto p-1">
      {items.length === 0 ? (
        <div className="px-3 py-6 text-center text-xs text-muted-foreground">No shortcuts found.</div>
      ) : (
        items.map((shortcut, index) => (
          <button
            key={shortcut.id}
            type="button"
            className={cn(
              'flex w-full items-start gap-3 rounded-md px-2 py-2 text-left hover:bg-muted',
              selectedIndex === index && 'bg-accent text-accent-foreground',
            )}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onSelect(shortcut)}
          >
            <span className="mt-0.5 shrink-0 font-mono text-xs font-semibold text-primary">{shortcut.short_code}</span>
            <span className="min-w-0 flex-1 truncate text-sm">{stripShortcutContent(shortcut.content) || shortcut.short_code}</span>
          </button>
        ))
      )}
    </div>
  );
}

function NewConversationMessageEditor({
  workspaceId,
  value,
  onChange,
  selectedContact,
  subject,
  onSend,
}: {
  workspaceId: string;
  value: string;
  onChange: (value: string) => void;
  selectedContact: CRMContact | null;
  subject: string;
  onSend: () => void;
}) {
  const { data: shortcuts = [] } = useCannedResponses(workspaceId);
  const rewriteMutation = useRewriteSupportDraft(workspaceId, null);
  const shortcutsRef = useRef(shortcuts);
  shortcutsRef.current = shortcuts;
  const [shortcutState, setShortcutState] = useState<ReturnType<typeof detectShortcuts>>(null);
  const shortcutStateRef = useRef(shortcutState);
  shortcutStateRef.current = shortcutState;
  const [manualShortcutsOpen, setManualShortcutsOpen] = useState(false);
  const [manualShortcutQuery, setManualShortcutQuery] = useState('');
  const [linkModalOpen, setLinkModalOpen] = useState(false);
  const [linkInitial, setLinkInitial] = useState({ label: '', url: '' });
  const editorRef = useRef<ReturnType<typeof useEditor>>(null);

  const resolveShortcutContent = useCallback((content: string) => resolveShortcutVariables(content, {
    customer: {
      fullName: selectedContact ? contactName(selectedContact) : null,
      email: selectedContact?.email ?? null,
    },
    conversationSubject: subject,
  }), [selectedContact, subject]);

  const insertShortcut = useCallback((shortcut: SupportCannedResponse, range?: { from: number; to: number }) => {
    const editor = editorRef.current;
    if (!editor) return;
    const content = resolveShortcutContent(shortcut.content);
    const chain = editor.chain().focus();
    if (range) {
      chain.deleteRange(range);
    }
    chain.insertContent(content).run();
    setShortcutState(null);
    setManualShortcutsOpen(false);
    setManualShortcutQuery('');
  }, [resolveShortcutContent]);

  const handleRewrite = useCallback(async (operation: SupportAIRewriteOperation) => {
    const editor = editorRef.current;
    if (!editor) return;
    const content = getEditorMarkdown(editor);
    if (!content.trim()) return;
    const response = await rewriteMutation.mutateAsync({ content, operation });
    editor.commands.setContent(response.content, { emitUpdate: true });
    onChange(response.content);
  }, [onChange, rewriteMutation]);

  const editor = useEditor({
    extensions: [
      ...createSupportComposerExtensions(),
      UnderlineExtension,
      Placeholder.configure({ placeholder: 'Write your message...' }),
    ],
    editorProps: {
      attributes: {
        class: 'rich-text-soft prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[96px] max-h-[220px] overflow-y-auto text-sm leading-relaxed',
      },
      handleKeyDown: (_view, event) => {
        const currentShortcut = shortcutStateRef.current;
        if (currentShortcut) {
          if (event.key === 'ArrowDown' && currentShortcut.items.length > 0) {
            event.preventDefault();
            setShortcutState({
              ...currentShortcut,
              selectedIndex: (currentShortcut.selectedIndex + 1) % currentShortcut.items.length,
            });
            return true;
          }
          if (event.key === 'ArrowUp' && currentShortcut.items.length > 0) {
            event.preventDefault();
            setShortcutState({
              ...currentShortcut,
              selectedIndex: (currentShortcut.selectedIndex - 1 + currentShortcut.items.length) % currentShortcut.items.length,
            });
            return true;
          }
          if (event.key === 'Enter' && currentShortcut.items.length > 0) {
            event.preventDefault();
            insertShortcut(currentShortcut.items[currentShortcut.selectedIndex], currentShortcut);
            return true;
          }
          if (event.key === 'Escape') {
            event.preventDefault();
            setShortcutState(null);
            return true;
          }
        }
        if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
          event.preventDefault();
          onSend();
          return true;
        }
        return false;
      },
    },
    onUpdate: ({ editor: updatedEditor }) => {
      onChange(getEditorMarkdown(updatedEditor));
      setShortcutState(detectShortcuts(updatedEditor, shortcutsRef.current));
    },
  });

  editorRef.current = editor;

  useEffect(() => {
    if (!editor) return;
    if (!value && !editor.isEmpty) {
      editor.commands.clearContent();
    }
  }, [editor, value]);

  if (!editor) return null;

  const manualShortcuts = manualShortcutQuery.trim()
    ? filterShortcuts(shortcuts, manualShortcutQuery)
    : shortcuts.slice(0, 8);
  const hasContent = editor.getText().trim().length > 0;
  const canUseAITools = hasContent && !rewriteMutation.isPending;
  const aiTools: Array<{ operation: SupportAIRewriteOperation; label: string; icon: typeof ArrowUpDownIcon }> = [
    { operation: 'expand', label: 'Expand', icon: ArrowUpDownIcon },
    { operation: 'rephrase', label: 'Rephrase', icon: ArrowReloadHorizontalIcon },
    { operation: 'fix_grammar', label: 'Fix grammar', icon: TickDouble01Icon },
    { operation: 'more_friendly', label: 'More friendly', icon: SmileIcon },
    { operation: 'more_formal', label: 'More formal', icon: Briefcase01Icon },
  ];

  const openLinkModal = () => {
    const { from, to } = editor.state.selection;
    const selectedText = editor.state.doc.textBetween(from, to, ' ');
    const attrs = editor.getAttributes('link') as { href?: string };
    setLinkInitial({ label: selectedText, url: attrs.href ?? '' });
    setLinkModalOpen(true);
  };

  const handleLinkInsert = (label: string, url: string) => {
    const text = label.trim() || url.trim();
    if (!text || !url.trim()) return;
    if (editor.state.selection.empty) {
      editor.chain().focus().insertContent(`<a href="${url.trim()}">${text}</a>`).run();
    } else {
      editor.chain().focus().extendMarkRange('link').setLink({ href: url.trim() }).run();
    }
    setLinkModalOpen(false);
  };

  return (
    <div className="relative rounded-xl border border-border/60 bg-card">
      {shortcutState ? (
        <div className="absolute bottom-full left-0 right-0 z-50 mb-2 px-1">
          <div className="rounded-xl border border-border/60 bg-popover shadow-lg">
            <ShortcutList
              items={shortcutState.items}
              selectedIndex={shortcutState.selectedIndex}
              onSelect={(shortcut) => insertShortcut(shortcut, shortcutState)}
            />
          </div>
        </div>
      ) : null}

      <div className="px-4 py-3">
        <EditorContent editor={editor} />
      </div>

      <LinkInsertModal
        open={linkModalOpen}
        onOpenChange={setLinkModalOpen}
        workspaceId={workspaceId}
        initialLabel={linkInitial.label}
        initialUrl={linkInitial.url}
        onInsert={handleLinkInsert}
        onRemove={editor.isActive('link') ? () => {
          editor.chain().focus().unsetLink().run();
          setLinkModalOpen(false);
        } : undefined}
      />

      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border/40 px-3 py-2">
        <div className="flex flex-wrap items-center gap-0.5">
          <EmojiPicker onEmojiSelect={(emoji) => editor.chain().focus().insertContent(emoji).run()} />
          <div className="mx-0.5 h-4 w-px bg-border/40" />
          <FormatButton title="Bold" active={editor.isActive('bold')} onClick={() => editor.chain().focus().toggleBold().run()}>
            <TextBoldIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton title="Italic" active={editor.isActive('italic')} onClick={() => editor.chain().focus().toggleItalic().run()}>
            <TextItalicIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton title="Underline" active={editor.isActive('underline')} onClick={() => editor.chain().focus().toggleUnderline().run()}>
            <TextUnderlineIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton title="Insert link" active={editor.isActive('link')} onClick={openLinkModal}>
            <Link01Icon className="h-3.5 w-3.5" />
          </FormatButton>
          <div className="mx-0.5 h-4 w-px bg-border/40" />
          <FormatButton title="Bullet list" active={editor.isActive('bulletList')} onClick={() => editor.chain().focus().toggleBulletList().run()}>
            <LeftToRightListBulletIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton title="Numbered list" active={editor.isActive('orderedList')} onClick={() => editor.chain().focus().toggleOrderedList().run()}>
            <LeftToRightListNumberIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton title="Quote" active={editor.isActive('blockquote')} onClick={() => editor.chain().focus().toggleBlockquote().run()}>
            <QuoteDownIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton title="Inline code" active={editor.isActive('code')} onClick={() => editor.chain().focus().toggleCode().run()}>
            <CodeIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton title="Strikethrough" active={editor.isActive('strike')} onClick={() => editor.chain().focus().toggleStrike().run()}>
            <TextStrikethroughIcon className="h-3.5 w-3.5" />
          </FormatButton>
        </div>

        <div className="flex items-center gap-1">
          <Popover open={manualShortcutsOpen} onOpenChange={(open) => {
            setManualShortcutsOpen(open);
            if (!open) setManualShortcutQuery('');
          }}>
            <PopoverTrigger asChild>
              <Button type="button" variant="ghost" size="sm" className="h-7 rounded-full px-3 text-xs">
                Shortcuts
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="w-[360px] p-0">
              <div className="border-b p-2">
                <QuietSearchInput
                  placeholder="Search shortcuts..."
                  value={manualShortcutQuery}
                  onChange={(event) => setManualShortcutQuery(event.target.value)}
                />
              </div>
              <ShortcutList items={manualShortcuts} onSelect={(shortcut) => insertShortcut(shortcut)} />
            </PopoverContent>
          </Popover>

          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button type="button" variant="ghost" size="sm" disabled={!canUseAITools} className="h-7 rounded-full px-3 text-xs">
                {rewriteMutation.isPending ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <SparklesIcon className="h-3 w-3" />}
                AI Tools
                <ArrowUp01Icon className="h-3 w-3 rotate-180" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48">
              {aiTools.slice(0, 3).map((tool) => {
                const Icon = tool.icon;
                return (
                  <DropdownMenuItem key={tool.operation} onSelect={() => { void handleRewrite(tool.operation); }}>
                    <Icon className="h-4 w-4" />
                    <span>{tool.label}</span>
                  </DropdownMenuItem>
                );
              })}
              <DropdownMenuSeparator />
              {aiTools.slice(3).map((tool) => {
                const Icon = tool.icon;
                return (
                  <DropdownMenuItem key={tool.operation} onSelect={() => { void handleRewrite(tool.operation); }}>
                    <Icon className="h-4 w-4" />
                    <span>{tool.label}</span>
                  </DropdownMenuItem>
                );
              })}
            </DropdownMenuContent>
          </DropdownMenu>

        </div>
      </div>
    </div>
  );
}

export function NewConversationDialog({ workspaceId, workspaceSlug, open, onOpenChange }: NewConversationDialogProps) {
  const navigate = useNavigate();
  const selectedMailboxId = useSupportInboxStore((s) => s.selectedMailboxId);
  const selectConversation = useSupportInboxStore((s) => s.selectConversation);
  const setActivePanel = useSupportInboxStore((s) => s.setActivePanel);
  const { data: inboxScopes } = useInboxScopes(workspaceId);
  const { data: tags = [] } = useSupportTags(workspaceId);
  const [recipientSearch, setRecipientSearch] = useState('');
  const [selectedContact, setSelectedContact] = useState<CRMContact | null>(null);
  const [subject, setSubject] = useState('');
  const [message, setMessage] = useState('');
  const [sendChat, setSendChat] = useState(false);
  const [sendEmail, setSendEmail] = useState(true);
  const [ccEmails, setCcEmails] = useState<string[]>([]);
  const [ccInput, setCcInput] = useState('');
  const [ccVisible, setCcVisible] = useState(false);
  const [bccEmails, setBccEmails] = useState<string[]>([]);
  const [bccInput, setBccInput] = useState('');
  const [bccVisible, setBccVisible] = useState(false);
  const [recipientOpen, setRecipientOpen] = useState(false);
  const recipientInputRef = useRef<HTMLInputElement | null>(null);
  const recipientFieldRef = useRef<HTMLDivElement | null>(null);
  const [mailboxId, setMailboxId] = useState(selectedMailboxId === 'all' ? 'shared' : selectedMailboxId);
  const [selectedTagIds, setSelectedTagIds] = useState<string[]>([]);
  const createConversation = useCreateConversationWithMessage(workspaceId);
  const contactsQuery = useContacts(workspaceId, {
    search: recipientSearch.trim(),
    per_page: 8,
  });

  const contacts = contactsQuery.data?.data ?? [];
  const mailboxOptions = [
    inboxScopes?.shared_inbox,
    ...(inboxScopes?.mailboxes ?? []),
  ].filter(isMailboxOption);
  const rawEmail = selectedContact?.email || recipientSearch.trim();
  const channels = [
    ...(sendChat ? ['chat' as const] : []),
    ...(sendEmail ? ['email' as const] : []),
  ];
  const canSendChat = !!selectedContact;
  const canSend = !!subject.trim() && !!message.trim() && channels.length > 0 && (!sendEmail || rawEmail.includes('@')) && (!sendChat || canSendChat);
  const selectedTags = tags.filter((tag) => selectedTagIds.includes(tag.id));

  useEffect(() => {
    if (!open) return;
    setRecipientSearch('');
    setSelectedContact(null);
    setSubject('');
    setMessage('');
    setSendChat(false);
    setSendEmail(true);
    setCcEmails([]);
    setCcInput('');
    setCcVisible(false);
    setBccEmails([]);
    setBccInput('');
    setBccVisible(false);
    setRecipientOpen(false);
    setMailboxId(selectedMailboxId === 'all' ? 'shared' : selectedMailboxId);
    setSelectedTagIds([]);
    window.requestAnimationFrame(() => {
      recipientInputRef.current?.focus();
    });
  }, [open, selectedMailboxId]);

  const selectedLabel = useMemo(() => {
    if (!selectedContact) return null;
    return `${contactName(selectedContact)}${selectedContact.email ? ` <${selectedContact.email}>` : ''}`;
  }, [selectedContact]);

  const handleSend = async () => {
    if (!canSend || createConversation.isPending) return;
    const customerEmail = sendEmail ? rawEmail : selectedContact?.email;
    try {
      const response = await createConversation.mutateAsync({
        subject: subject.trim(),
        content: message.trim(),
        customer_name: selectedContact ? contactName(selectedContact) : undefined,
        customer_email: customerEmail?.trim() || undefined,
        crm_contact_id: selectedContact?.id,
        mailbox_id: mailboxValue(mailboxId),
        channels,
        tag_ids: selectedTagIds,
        cc_emails: sendEmail ? ccEmails : [],
        bcc_emails: sendEmail ? bccEmails : [],
      });
      const conversationId = response.conversation.id;
      selectConversation(conversationId);
      setActivePanel('thread');
      onOpenChange(false);
      if (workspaceSlug) {
        void navigate({
          to: '/w/$slug/support/$conversationId',
          params: { slug: workspaceSlug, conversationId },
        });
      }
    } catch {
      // The mutation hook shows the user-facing error toast.
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[92vh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>New conversation</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label>To</Label>
            {selectedContact ? (
              <div className="flex items-center justify-between gap-2 rounded-md border bg-muted/30 px-3 py-2 text-sm">
                <span className="min-w-0 truncate">{selectedLabel}</span>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2"
                  onClick={() => {
                    setSelectedContact(null);
                    setSendChat(false);
                    setRecipientOpen(false);
                    window.requestAnimationFrame(() => {
                      recipientInputRef.current?.focus();
                    });
                  }}
                >
                  Change
                </Button>
              </div>
            ) : (
              <div
                ref={recipientFieldRef}
                className="relative"
                onBlur={(event) => {
                  if (!event.currentTarget.contains(event.relatedTarget)) {
                    setRecipientOpen(false);
                  }
                }}
              >
                  <div className="relative">
                    <Input
                      ref={recipientInputRef}
                      value={recipientSearch}
                      onChange={(event) => {
                        const nextValue = event.target.value;
                        setRecipientSearch(nextValue);
                        setRecipientOpen(nextValue.trim().length > 0);
                      }}
                      onFocus={() => setRecipientOpen(recipientSearch.trim().length > 0)}
                      placeholder="Search contacts or enter email"
                      className="h-9"
                    />
                  </div>
                  {recipientOpen && recipientSearch.trim().length > 0 ? (
                    <div className="absolute left-0 right-0 top-full z-50 mt-1 max-h-72 overflow-y-auto rounded-md border bg-popover p-1 text-popover-foreground shadow-md">
                      {contacts.length === 0 && !recipientSearch.includes('@') ? (
                        <div className="px-3 py-3 text-xs text-muted-foreground">No contacts found.</div>
                      ) : null}
                      {contacts.map((contact) => (
                        <button
                          key={contact.id}
                          type="button"
                          className="flex w-full items-center gap-2 rounded-sm px-2 py-2 text-left outline-none hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground"
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => {
                            setSelectedContact(contact);
                            setRecipientSearch(contact.email ?? contactName(contact));
                            setSendChat(true);
                            setSendEmail(!!contact.email);
                            setRecipientOpen(false);
                          }}
                        >
                            <div className="min-w-0 flex flex-1 items-baseline gap-2">
                              <span className="truncate text-sm font-medium">{contactName(contact)}</span>
                              {contact.email ? <span className="truncate text-xs text-muted-foreground">{contact.email}</span> : null}
                            </div>
                            <Badge variant="outline" className="shrink-0">{contact.lifecycle_stage}</Badge>
                        </button>
                      ))}
                      {recipientSearch.includes('@') ? (
                        <button
                          type="button"
                          className="flex w-full items-center gap-2 rounded-sm px-2 py-2 text-left text-blue-700 outline-none hover:bg-blue-50 focus:bg-blue-50 dark:text-blue-300 dark:hover:bg-blue-950/30 dark:focus:bg-blue-950/30"
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => {
                            setSelectedContact(null);
                            setSendEmail(true);
                            setSendChat(false);
                            setRecipientOpen(false);
                          }}
                        >
                            <div className="min-w-0 flex flex-1 items-baseline gap-2">
                              <span className="truncate text-sm font-medium">{recipientSearch.trim()}</span>
                              <span className="shrink-0 text-xs text-blue-600 dark:text-blue-300">New email recipient</span>
                            </div>
                            <Badge variant="outline" className="shrink-0 border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-300">New</Badge>
                        </button>
                      ) : null}
                    </div>
                  ) : null}
              </div>
            )}
            {sendEmail ? (
              <div className="space-y-2">
                {(!ccVisible || !bccVisible) ? (
                  <div className="flex flex-wrap gap-1.5">
                    {!ccVisible ? (
                      <Button type="button" variant="ghost" size="sm" className="h-7 gap-1 px-2 text-xs text-muted-foreground" onClick={() => setCcVisible(true)}>
                        <PlusSignIcon className="h-3 w-3" />
                        Cc
                      </Button>
                    ) : null}
                    {!bccVisible ? (
                      <Button type="button" variant="ghost" size="sm" className="h-7 gap-1 px-2 text-xs text-muted-foreground" onClick={() => setBccVisible(true)}>
                        <PlusSignIcon className="h-3 w-3" />
                        Bcc
                      </Button>
                    ) : null}
                  </div>
                ) : null}
                {(ccVisible || bccVisible) ? (
                  <div className="grid gap-2 sm:grid-cols-2">
                    {ccVisible ? (
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between gap-2">
                          <Label htmlFor="new-conversation-cc" className="text-xs">Cc</Label>
                          {ccEmails.length === 0 && !ccInput.trim() ? (
                            <button
                              type="button"
                              className="inline-flex h-5 w-5 items-center justify-center rounded-sm text-muted-foreground hover:bg-muted hover:text-foreground"
                              aria-label="Remove Cc"
                              onClick={() => setCcVisible(false)}
                            >
                              <Cancel01Icon className="h-3.5 w-3.5" />
                            </button>
                          ) : null}
                        </div>
                        <EmailChipInput
                          value={ccEmails}
                          onValueChange={setCcEmails}
                          inputValue={ccInput}
                          onInputValueChange={setCcInput}
                          placeholder="Add Cc recipients"
                          className="min-h-9 py-1.5"
                        />
                      </div>
                    ) : null}
                    {bccVisible ? (
                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between gap-2">
                          <Label htmlFor="new-conversation-bcc" className="text-xs">Bcc</Label>
                          {bccEmails.length === 0 && !bccInput.trim() ? (
                            <button
                              type="button"
                              className="inline-flex h-5 w-5 items-center justify-center rounded-sm text-muted-foreground hover:bg-muted hover:text-foreground"
                              aria-label="Remove Bcc"
                              onClick={() => setBccVisible(false)}
                            >
                              <Cancel01Icon className="h-3.5 w-3.5" />
                            </button>
                          ) : null}
                        </div>
                        <EmailChipInput
                          value={bccEmails}
                          onValueChange={setBccEmails}
                          inputValue={bccInput}
                          onInputValueChange={setBccInput}
                          placeholder="Add Bcc recipients"
                          className="min-h-9 py-1.5"
                        />
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>

          <div className="space-y-1">
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="mr-1 text-xs text-muted-foreground">Channel</span>
              <button
                type="button"
                className={cn(
                  'inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 text-xs transition-colors',
                  sendEmail
                    ? 'border-primary/30 bg-primary/10 text-primary'
                    : 'border-transparent bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground',
                )}
                onClick={() => setSendEmail((value) => !value)}
              >
                <Mail01Icon className="h-3.5 w-3.5" />
                Email
              </button>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="inline-flex">
                    <button
                      type="button"
                      disabled={!canSendChat}
                      className={cn(
                        'inline-flex h-7 items-center gap-1.5 rounded-full border px-2.5 text-xs transition-colors',
                        sendChat
                          ? 'border-primary/30 bg-primary/10 text-primary'
                          : 'border-transparent bg-muted/50 text-muted-foreground hover:bg-muted hover:text-foreground',
                        !canSendChat && 'cursor-not-allowed opacity-50',
                      )}
                      onClick={() => {
                        if (!canSendChat) return;
                        setSendChat((value) => !value);
                      }}
                    >
                      <Message01Icon className="h-3.5 w-3.5" />
                      Chat
                    </button>
                  </span>
                </TooltipTrigger>
                {!canSendChat ? (
                  <TooltipContent side="top" className="text-xs">Select an existing contact to send via chat.</TooltipContent>
                ) : null}
              </Tooltip>
            </div>
            {sendChat && !sendEmail ? (
              <p className="text-xs text-muted-foreground">This message will be seen when the customer is online or returns to the chat widget.</p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="new-conversation-subject">Subject</Label>
            <Input id="new-conversation-subject" value={subject} onChange={(event) => setSubject(event.target.value)} placeholder="Conversation title" />
          </div>

          <div className="space-y-1.5">
            <Label>Message</Label>
            <NewConversationMessageEditor
              workspaceId={workspaceId}
              value={message}
              onChange={setMessage}
              selectedContact={selectedContact}
              subject={subject}
              onSend={handleSend}
            />
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>Inbox</Label>
              <select
                className="h-9 w-full rounded-md border bg-background px-3 text-sm"
                value={mailboxId}
                onChange={(event) => setMailboxId(event.target.value)}
              >
                {mailboxOptions.map((mailbox) => (
                  <option key={mailbox.id} value={mailbox.id}>{mailbox.name}</option>
                ))}
              </select>
            </div>
            <div className="space-y-1.5">
              <Label>Tags <span className="font-normal text-muted-foreground">(optional)</span></Label>
              <div className="min-h-9 rounded-md border px-2 py-1.5">
                <SupportTagPicker
                  workspaceId={workspaceId}
                  selectedTags={selectedTags}
                  selectedTagIds={selectedTagIds}
                  onSelectedTagIdsChange={setSelectedTagIds}
                />
              </div>
            </div>
          </div>
        </div>
        <DialogFooter className="border-t pt-4">
          <Button disabled={!canSend || createConversation.isPending} onClick={handleSend}>
            {createConversation.isPending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : null}
            Send
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
