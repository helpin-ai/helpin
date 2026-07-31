import { useRef, useEffect, useState, useCallback, useMemo, type KeyboardEvent, type ReactNode } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import { Markdown } from 'tiptap-markdown';
import {
  SentIcon, AttachmentIcon, Cancel01Icon, Loading01Icon,
  Mail01Icon, SparklesIcon, ArrowUp01Icon, ArrowUpDownIcon, ArrowReloadHorizontalIcon,
  TickDouble01Icon, SmileIcon, Briefcase01Icon, PlusSignIcon,
  TextBoldIcon, TextItalicIcon, TextUnderlineIcon, TextStrikethroughIcon,
  CodeIcon, QuoteDownIcon, LeftToRightListBulletIcon, LeftToRightListNumberIcon, Link01Icon,
  StickyNote01Icon, PencilEdit01Icon, Delete01Icon, ArrowLeft02Icon, Search01Icon,
} from '@/lib/icons';
import { useQuery } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command';
import { Input } from '@/components/ui/input';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Textarea } from '@/components/ui/textarea';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { MentionHighlight } from '@/components/pm/mention-highlight';
import { MentionSuggestionsList } from '@/components/pm/MentionSuggestionsList';
import { getMemberMentionHandle, getMentionSuggestions, type MentionSuggestionItem } from '@/components/pm/mentionSuggestions';
import { useCannedResponses, useConversation, useCreateCannedResponse, useDeleteCannedResponse, useRewriteSupportDraft, useSendMessage, useUpdateCannedResponse, useUpdateConversationEmailRecipients, useUploadSupportAttachment } from '@/hooks/queries/useSupport';
import { queryKeys } from '@/lib/queryKeys';
import { workspacesService } from '@/lib/services/workspacesService';
import { unwrap } from '@/lib/queryUtils';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { cn } from '@/lib/utils';
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired';
import { toast } from 'sonner';
import type { AssignableMember } from '@/lib/types';
import type { SupportAIRewriteOperation, SupportAttachmentPayload, SupportCannedResponse } from '@/lib/pmTypes';
import { EmojiPicker } from './EmojiPicker';
import { LinkInsertModal } from './LinkInsertModal';
import { useShortcutComposerStore } from './shortcutDialogStore';
import { SHORTCUT_VARIABLES, resolveShortcutVariables } from './shortcutVariables';
import { DEFAULT_SHORTCUT_CATEGORY, normalizeShortcutCategory, shortcutCategoryOptions } from './shortcutCategories';
import { filterShortcuts, stripShortcutContent } from './shortcutFiltering';
import { getClipboardImageFiles } from './clipboardAttachments';
import { restoreAttachmentsFromMessage, type PendingSupportAttachment } from './draftAttachments';

const OFFLINE_EMAIL_CONFIRM_STORAGE_PREFIX = 'support_offline_email_confirm';
const RESTORE_SUPPORT_DRAFT_EVENT = 'support:restore-draft';

interface ReplyComposerProps {
  workspaceId: string;
  conversationId: string;
  emailFallbackHint?: {
    email: string;
  } | null;
  onUpgradeRequired?: (reason: UpgradeRequiredReason) => void;
}

function loadSkipOfflineEmailConfirm(storageKey: string): boolean {
  try {
    return localStorage.getItem(storageKey) === '1';
  } catch {
    return false;
  }
}

function saveSkipOfflineEmailConfirm(storageKey: string, skip: boolean) {
  try {
    if (skip) {
      localStorage.setItem(storageKey, '1');
    } else {
      localStorage.removeItem(storageKey);
    }
  } catch {}
}

function getEditorMarkdown(editorInstance: ReturnType<typeof useEditor> | null | undefined): string {
  if (!editorInstance) return '';
  const storage = (editorInstance.storage as { markdown?: { getMarkdown(): string } }).markdown;
  if (storage?.getMarkdown) return storage.getMarkdown();
  return editorInstance.getText();
}

function normalizeRecipientEmails(values: string[], excluded: string[] = []): string[] {
  const excludedSet = new Set(excluded.map((value) => value.trim().toLowerCase()).filter(Boolean));
  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const value of values) {
    const email = value.trim().toLowerCase();
    if (!email || excludedSet.has(email) || seen.has(email)) continue;
    seen.add(email);
    normalized.push(email);
  }
  return normalized;
}

const URL_TOKEN_REGEX =
  /^((?:https?|ftp):\/\/\S+|www\.\S+\.[a-z]{2,}\S*|[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+\.[a-z]{2,}(?:[/?#]\S*)?)$/i;
const TRAILING_PUNCT_REGEX = /[.,;:!?)\]}>'"]+$/;

function normalizeUrl(raw: string): string {
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(raw)) return raw;
  return `https://${raw}`;
}

function findBareUrlBeforeCaret(
  editorInstance: ReturnType<typeof useEditor> | null | undefined,
): { from: number; to: number; href: string } | null {
  if (!editorInstance) return null;
  const { state } = editorInstance;
  const { selection } = state;
  if (!selection.empty) return null;

  const $from = selection.$from;
  const paragraphStart = $from.start();
  const caret = $from.pos;
  if (caret <= paragraphStart) return null;

  const textBefore = state.doc.textBetween(paragraphStart, caret, '\n', '\ufffc');
  const match = textBefore.match(/(\S+)$/);
  if (!match) return null;

  const rawToken = match[1];
  const trail = rawToken.match(TRAILING_PUNCT_REGEX);
  const trimmed = trail ? rawToken.slice(0, -trail[0].length) : rawToken;
  if (!trimmed || !URL_TOKEN_REGEX.test(trimmed)) return null;

  const tokenStart = caret - rawToken.length;
  const tokenEnd = tokenStart + trimmed.length;

  const linkMark = state.schema.marks.link;
  if (linkMark && state.doc.rangeHasMark(tokenStart, tokenEnd, linkMark)) return null;

  return { from: tokenStart, to: tokenEnd, href: normalizeUrl(trimmed) };
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
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          aria-label={title}
          aria-pressed={active}
          onClick={(e) => { e.preventDefault(); onClick(); }}
          onMouseDown={(e) => e.preventDefault()}
          className={cn(
            'inline-flex h-6 w-6 items-center justify-center rounded-md transition-colors',
            active
              ? 'bg-accent text-foreground'
              : 'text-muted-foreground hover:bg-muted hover:text-foreground',
          )}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="text-xs">{title}</TooltipContent>
    </Tooltip>
  );
}

function detectMentions(
  editorInstance: ReturnType<typeof useEditor>,
  members: AssignableMember[],
): { from: number; to: number; items: MentionSuggestionItem[]; selectedIndex: number } | null {
  if (!editorInstance) return null;
  if (members.length === 0) return null;

  const { selection } = editorInstance.state;
  if (!selection.empty) return null;

  const textBefore = selection.$from.parent.textBetween(
    0,
    selection.$from.parentOffset,
    undefined,
    '\ufffc',
  );
  const match = textBefore.match(/(?:^|\s)@([a-z0-9._-]*)$/i);
  if (!match) return null;

  const query = match[1].toLowerCase();
  const items = getMentionSuggestions(query, members, [], 8);
  if (items.length === 0) return null;

  return {
    from: selection.from - (query.length + 1),
    to: selection.from,
    items: items.slice(0, 8),
    selectedIndex: 0,
  };
}

function validateShortcutCode(shortCode: string) {
  const value = shortCode.trim();
  if (!value) return 'Shortcut is required';
  if (!value.startsWith('!')) return 'Shortcut must start with !';
  if (/\s/.test(value)) return 'Shortcut cannot contain spaces';
  if (value.length < 2) return 'Shortcut must have at least one character after !';
  return null;
}

function seedShortcutCode(seed?: string) {
  const trimmed = seed?.trim() ?? '';
  if (!trimmed) return '';
  return trimmed.startsWith('!') ? trimmed : `!${trimmed}`;
}

function emptyShortcutForm(seed?: { seedShortCode?: string; seedContent?: string }): ShortcutFormState {
  return {
    shortCode: seedShortcutCode(seed?.seedShortCode),
    category: DEFAULT_SHORTCUT_CATEGORY,
    content: seed?.seedContent ?? '',
  };
}

function shortcutFormFromShortcut(shortcut: SupportCannedResponse): ShortcutFormState {
  return {
    shortCode: shortcut.short_code,
    category: normalizeShortcutCategory(shortcut.tag),
    content: shortcut.content,
  };
}

function normalizeShortcutFormCategory(form: ShortcutFormState) {
  return normalizeShortcutCategory(form.category);
}

function isShortcutPanelPortalTarget(target: EventTarget | Node | null) {
  if (!(target instanceof Element)) return false;
  return !!target.closest(
    [
      '[data-slot="select-content"]',
      '[data-slot="select-item"]',
      '[data-slot="dropdown-menu-content"]',
      '[data-slot="dropdown-menu-item"]',
      '[data-slot="popover-content"]',
      '[data-slot="command"]',
      '[data-slot="command-item"]',
      '[data-radix-popper-content-wrapper]',
    ].join(','),
  );
}

function groupShortcuts(shortcuts: SupportCannedResponse[]) {
  const groups = new Map<string, SupportCannedResponse[]>();
  for (const shortcut of shortcuts) {
    const tag = normalizeShortcutCategory(shortcut.tag);
    groups.set(tag, [...(groups.get(tag) ?? []), shortcut]);
  }
  return Array.from(groups.entries()).map(([tag, items]) => ({ tag, items }));
}

function detectShortcuts(
  editorInstance: ReturnType<typeof useEditor>,
  shortcuts: SupportCannedResponse[],
): { from: number; to: number; query: string; items: SupportCannedResponse[]; selectedIndex: number } | null {
  if (!editorInstance) return null;
  const { selection } = editorInstance.state;
  if (!selection.empty) return null;

  const textBefore = selection.$from.parent.textBetween(
    0,
    selection.$from.parentOffset,
    undefined,
    '\ufffc',
  );
  // Trigger only when `!` is at the start of the line/block or directly
  // follows whitespace. This treats `!` as a command sigil only when it's
  // initiating a new token, so sentence punctuation like "That's great!"
  // and intra-word `!` ("foo!bar") don't open the panel.
  const match = textBefore.match(/(^|\s)!([^\s!]*)$/);
  if (!match) return null;

  const query = match[2].toLowerCase();
  // Anchor the replacement range at the `!` itself (not the leading
  // whitespace) so inserting a shortcut preserves any preceding space.
  return {
    from: selection.from - (query.length + 1),
    to: selection.from,
    query,
    items: filterShortcuts(shortcuts, query),
    selectedIndex: 0,
  };
}

function highlightMatch(text: string, query: string): ReactNode {
  if (!text) return text;
  const needle = query.replace(/^!/, '').trim();
  if (!needle) return text;
  const lower = text.toLowerCase();
  const idx = lower.indexOf(needle.toLowerCase());
  if (idx === -1) return text;
  const end = idx + needle.length;
  return (
    <>
      {text.slice(0, idx)}
      <mark className="bg-transparent font-semibold text-foreground">{text.slice(idx, end)}</mark>
      {text.slice(end)}
    </>
  );
}

function ShortcutsList({
  shortcuts,
  selectedIndex,
  onSelect,
  onEdit,
  onDelete,
  compact = false,
  query = '',
}: {
  shortcuts: SupportCannedResponse[];
  selectedIndex?: number;
  onSelect: (shortcut: SupportCannedResponse) => void;
  onEdit: (shortcut: SupportCannedResponse) => void;
  onDelete: (shortcut: SupportCannedResponse) => void;
  compact?: boolean;
  query?: string;
}) {
  const selectedRef = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    selectedRef.current?.scrollIntoView({ block: 'nearest' });
  }, [selectedIndex]);

  let flatIndex = 0;
  return (
    <div role="listbox" aria-label="Message shortcuts" className="space-y-2">
      {groupShortcuts(shortcuts).map((group) => (
        <div key={group.tag}>
          <div className="px-2 pb-1">
            <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground/60">
              {group.tag} ({group.items.length})
            </span>
          </div>
          <div className="space-y-0.5">
            {group.items.map((shortcut) => {
              const currentIndex = flatIndex++;
              const active = selectedIndex === currentIndex;
              return (
                <div
                  key={shortcut.id}
                  role="option"
                  aria-selected={active}
                  className={cn(
                    'group/shortcut flex w-full items-start gap-2 rounded-md pr-1 transition-colors',
                    active ? 'bg-accent text-accent-foreground' : 'hover:bg-muted',
                  )}
                >
                  <button
                    ref={active ? selectedRef : undefined}
                    type="button"
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => onSelect(shortcut)}
                    className="flex min-w-0 flex-1 items-start gap-3 px-2 py-2 text-left"
                  >
                    <span className="mt-0.5 shrink-0 font-mono text-xs font-semibold text-primary">
                      {highlightMatch(shortcut.short_code, query)}
                    </span>
                    <span className="min-w-0 flex-1">
                      <span
                        className={cn(
                          'block truncate text-sm text-foreground/90',
                          compact && 'max-w-[640px]',
                        )}
                      >
                        {stripShortcutContent(shortcut.content) || shortcut.short_code}
                      </span>
                    </span>
                  </button>
                  <div className="flex shrink-0 items-center gap-0.5 py-1.5 opacity-0 transition-opacity group-hover/shortcut:opacity-100 group-focus-within/shortcut:opacity-100">
                    <button
                      type="button"
                      aria-label={`Edit ${shortcut.short_code}`}
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => onEdit(shortcut)}
                      className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-background hover:text-foreground"
                    >
                      <PencilEdit01Icon className="h-3.5 w-3.5" />
                    </button>
                    <button
                      type="button"
                      aria-label={`Delete ${shortcut.short_code}`}
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => onDelete(shortcut)}
                      className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                    >
                      <Delete01Icon className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

type ShortcutPanelMode = 'list' | 'create' | 'edit';

type ShortcutFormState = {
  shortCode: string;
  category: string;
  content: string;
};

function ShortcutFormPanel({
  mode,
  form,
  shortcuts,
  editingId,
  pending,
  deleteConfirm,
  onBack,
  onClose,
  onChange,
  onSubmit,
  onDelete,
  onDeleteConfirmChange,
}: {
  mode: Exclude<ShortcutPanelMode, 'list'>;
  form: ShortcutFormState;
  shortcuts: SupportCannedResponse[];
  editingId?: string | null;
  pending: boolean;
  deleteConfirm: boolean;
  onBack: () => void;
  onClose: () => void;
  onChange: (form: ShortcutFormState) => void;
  onSubmit: () => void;
  onDelete: () => void;
  onDeleteConfirmChange: (confirm: boolean) => void;
}) {
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const [categoryOpen, setCategoryOpen] = useState(false);
  const [categorySearch, setCategorySearch] = useState('');
  const [touched, setTouched] = useState({ shortCode: false, content: false });
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const normalizedCode = form.shortCode.trim();
  const shortcutError = validateShortcutCode(form.shortCode);
  const duplicate = shortcuts.some((item) => item.id !== editingId && item.short_code === normalizedCode);
  const contentMissing = stripShortcutContent(form.content).length === 0;
  const canSubmit = !shortcutError && !duplicate && !contentMissing && !pending;
  const showShortcutError = (touched.shortCode || submitAttempted) && !!shortcutError;
  const showDuplicateError = (touched.shortCode || submitAttempted) && !shortcutError && duplicate;
  const showContentError = (touched.content || submitAttempted) && contentMissing;
  const categories = shortcutCategoryOptions(shortcuts);
  const selectedCategory = normalizeShortcutCategory(form.category);
  const normalizedCategorySearch = categorySearch.trim();
  const filteredCategories = categories.filter((category) =>
    category.toLowerCase().includes(normalizedCategorySearch.toLowerCase()),
  );
  const canCreateCategory =
    normalizedCategorySearch.length > 0 &&
    !categories.some((category) => category.toLowerCase() === normalizedCategorySearch.toLowerCase());
  const exactCategoryMatch = categories.find((category) =>
    category.toLowerCase() === normalizedCategorySearch.toLowerCase(),
  );

  const selectCategory = (category: string) => {
    onChange({ ...form, category: normalizeShortcutCategory(category) });
    setCategorySearch('');
    setCategoryOpen(false);
  };

  const focusMessageField = () => {
    requestAnimationFrame(() => textareaRef.current?.focus());
  };

  const moveFromCategoryToMessage = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Tab' || event.shiftKey) return;
    event.preventDefault();
    if (normalizedCategorySearch) {
      selectCategory(exactCategoryMatch ?? filteredCategories[0] ?? normalizedCategorySearch);
    } else {
      setCategoryOpen(false);
    }
    focusMessageField();
  };

  const insertAtSelection = (value: string) => {
    const textarea = textareaRef.current;
    if (!textarea) {
      onChange({ ...form, content: `${form.content}${value}` });
      return;
    }
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const next = `${form.content.slice(0, start)}${value}${form.content.slice(end)}`;
    onChange({ ...form, content: next });
    requestAnimationFrame(() => {
      textarea.focus();
      textarea.setSelectionRange(start + value.length, start + value.length);
    });
  };

  const wrapSelection = (before: string, after = before) => {
    const textarea = textareaRef.current;
    if (!textarea) {
      insertAtSelection(`${before}${after}`);
      return;
    }
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const selected = form.content.slice(start, end);
    const next = `${form.content.slice(0, start)}${before}${selected}${after}${form.content.slice(end)}`;
    onChange({ ...form, content: next });
    requestAnimationFrame(() => {
      textarea.focus();
      textarea.setSelectionRange(start + before.length, start + before.length + selected.length);
    });
  };

  const prefixLines = (prefix: string) => {
    const textarea = textareaRef.current;
    if (!textarea) {
      insertAtSelection(prefix);
      return;
    }
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const selected = form.content.slice(start, end) || '';
    const replacement = selected
      .split('\n')
      .map((line) => `${prefix}${line}`)
      .join('\n');
    const next = `${form.content.slice(0, start)}${replacement}${form.content.slice(end)}`;
    onChange({ ...form, content: next });
    requestAnimationFrame(() => {
      textarea.focus();
      textarea.setSelectionRange(start, start + replacement.length);
    });
  };

  return (
    <div className="flex max-h-[420px] flex-col">
      <div className="flex items-center gap-2 border-b border-border/40 px-3 py-2">
        <button
          type="button"
          onClick={onBack}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          aria-label="Back to shortcuts"
        >
          <ArrowLeft02Icon className="h-4 w-4" />
        </button>
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium">{mode === 'create' ? 'New shortcut' : `Edit ${normalizedCode || 'shortcut'}`}</div>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
          aria-label="Close shortcuts"
        >
          <Cancel01Icon className="h-4 w-4" />
        </button>
      </div>

      <div className="space-y-3 overflow-y-auto p-3">
        <div className="grid gap-3 sm:grid-cols-[minmax(140px,0.65fr)_minmax(160px,0.8fr)]">
          <div className="space-y-1.5">
            <Label htmlFor="shortcut-code" className="text-xs">Shortcut <span className="text-destructive">*</span></Label>
            <Input
              id="shortcut-code"
              value={form.shortCode}
              onChange={(event) => {
                setTouched((current) => ({ ...current, shortCode: true }));
                onChange({ ...form, shortCode: event.target.value });
              }}
              onBlur={() => setTouched((current) => ({ ...current, shortCode: true }))}
              placeholder="!hello"
              className="h-8 font-mono text-sm"
            />
            {showShortcutError ? (
              <p className="text-xs text-destructive">{shortcutError}</p>
            ) : showDuplicateError ? (
              <p className="text-xs text-destructive">That shortcut already exists.</p>
            ) : null}
          </div>
          <div className="space-y-1.5">
            <Label className="text-xs">Category</Label>
            <Popover open={categoryOpen} onOpenChange={setCategoryOpen}>
              <PopoverTrigger asChild>
                <button
                  type="button"
                  onKeyDown={moveFromCategoryToMessage}
                  className="flex h-8 w-full items-center justify-between rounded-md border border-transparent bg-input/50 px-3 text-left text-sm outline-none transition-colors hover:bg-input focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30"
                >
                  <span className="truncate">{selectedCategory}</span>
                  <ArrowUpDownIcon className="ml-2 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                </button>
              </PopoverTrigger>
              <PopoverContent align="start" className="w-[var(--radix-popover-trigger-width)] p-1">
                <Command shouldFilter={false}>
                  <CommandInput
                    value={categorySearch}
                    onValueChange={setCategorySearch}
                    placeholder="Choose or type a category"
                    onKeyDown={(event) => {
                      if (event.key === 'Tab' && !event.shiftKey) {
                        moveFromCategoryToMessage(event);
                        return;
                      }
                      if (event.key !== 'Enter' || !normalizedCategorySearch) return;
                      event.preventDefault();
                      selectCategory(exactCategoryMatch ?? filteredCategories[0] ?? normalizedCategorySearch);
                    }}
                  />
                  <CommandList>
                    {filteredCategories.length === 0 && !canCreateCategory ? (
                      <CommandEmpty>No categories found.</CommandEmpty>
                    ) : null}
                    <CommandGroup>
                      {filteredCategories.map((category) => (
                        <CommandItem
                          key={category}
                          value={category}
                          data-checked={selectedCategory === category}
                          onSelect={() => selectCategory(category)}
                        >
                          {category}
                        </CommandItem>
                      ))}
                      {canCreateCategory ? (
                        <>
                          <CommandSeparator />
                          <CommandItem
                            value={`create-${normalizedCategorySearch}`}
                            onSelect={() => selectCategory(normalizedCategorySearch)}
                            className="font-medium text-primary data-selected:text-primary"
                          >
                            <PlusSignIcon className="h-4 w-4" />
                            <span className="min-w-0 flex-1 truncate">Create new category</span>
                            <span className="max-w-28 truncate rounded bg-primary/10 px-1.5 py-0.5 font-mono text-[11px] text-primary">
                              {normalizedCategorySearch}
                            </span>
                          </CommandItem>
                        </>
                      ) : !normalizedCategorySearch ? (
                        <>
                          <CommandSeparator />
                          <div className="px-2 py-1.5 text-xs text-muted-foreground">
                            Type a new category name to create it.
                          </div>
                        </>
                      ) : null}
                    </CommandGroup>
                  </CommandList>
                </Command>
              </PopoverContent>
            </Popover>
          </div>
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between gap-2">
            <Label htmlFor="shortcut-content" className="text-xs">Message <span className="text-destructive">*</span></Label>
            <div className="flex items-center gap-0.5">
              <button type="button" onClick={() => wrapSelection('**')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Bold">
                <TextBoldIcon className="h-3.5 w-3.5" />
              </button>
              <button type="button" onClick={() => wrapSelection('_')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Italic">
                <TextItalicIcon className="h-3.5 w-3.5" />
              </button>
              <button type="button" onClick={() => wrapSelection('<u>', '</u>')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Underline">
                <TextUnderlineIcon className="h-3.5 w-3.5" />
              </button>
              <button type="button" onClick={() => wrapSelection('[', '](https://)')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Link">
                <Link01Icon className="h-3.5 w-3.5" />
              </button>
              <button type="button" onClick={() => prefixLines('- ')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Bullet list">
                <LeftToRightListBulletIcon className="h-3.5 w-3.5" />
              </button>
              <button type="button" onClick={() => prefixLines('1. ')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Numbered list">
                <LeftToRightListNumberIcon className="h-3.5 w-3.5" />
              </button>
              <button type="button" onClick={() => prefixLines('> ')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Quote">
                <QuoteDownIcon className="h-3.5 w-3.5" />
              </button>
              <button type="button" onClick={() => wrapSelection('`')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Inline code">
                <CodeIcon className="h-3.5 w-3.5" />
              </button>
              <button type="button" onClick={() => wrapSelection('~~')} className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground" aria-label="Strikethrough">
                <TextStrikethroughIcon className="h-3.5 w-3.5" />
              </button>
              <EmojiPicker
                align="end"
                side="top"
                onEmojiSelect={(emoji) => insertAtSelection(emoji)}
              />
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <button type="button" className="inline-flex h-7 items-center rounded-md px-2 text-xs font-medium text-muted-foreground hover:bg-muted hover:text-foreground">
                    Variables
                  </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-56">
                  {SHORTCUT_VARIABLES.map((variable) => (
                    <DropdownMenuItem key={variable.label} onSelect={() => insertAtSelection(variable.token)}>
                      <span className="font-mono text-xs">{variable.label}</span>
                    </DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
          <Textarea
            id="shortcut-content"
            ref={textareaRef}
            value={form.content}
            onChange={(event) => {
              setTouched((current) => ({ ...current, content: true }));
              onChange({ ...form, content: event.target.value });
            }}
            onBlur={() => setTouched((current) => ({ ...current, content: true }))}
            placeholder="Write the saved reply..."
            className="min-h-32 resize-y text-sm"
          />
          {showContentError ? <p className="text-xs text-destructive">Message is required.</p> : null}
        </div>
      </div>

      <div className="flex items-center justify-between gap-2 border-t border-border/40 bg-muted/30 px-3 py-2">
        {mode === 'edit' ? (
          <Button
            type="button"
            variant={deleteConfirm ? 'destructive' : 'ghost'}
            size="sm"
            onClick={() => {
              if (deleteConfirm) {
                onDelete();
              } else {
                onDeleteConfirmChange(true);
              }
            }}
            disabled={pending}
          >
            <Delete01Icon className="h-3.5 w-3.5" />
            {deleteConfirm ? 'Confirm delete' : 'Delete'}
          </Button>
        ) : <span />}
        <div className="flex items-center gap-2">
          {mode === 'edit' ? (
            <Button type="button" variant="ghost" size="sm" onClick={onBack}>Cancel</Button>
          ) : null}
          <Button
            type="button"
            size="sm"
            disabled={pending}
            onClick={() => {
              setSubmitAttempted(true);
              if (canSubmit) onSubmit();
            }}
          >
            {pending ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : null}
            {mode === 'create' ? 'Create' : 'Save'}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function ReplyComposer({ workspaceId, conversationId, emailFallbackHint, onUpgradeRequired }: ReplyComposerProps) {
  const { replyMode, setReplyMode, setDraft, clearDraft } = useSupportInboxStore();
  const sendMutation = useSendMessage(workspaceId, conversationId);
  const rewriteMutation = useRewriteSupportDraft(workspaceId, conversationId);
  const updateEmailRecipients = useUpdateConversationEmailRecipients(workspaceId);
  const user = useAuthStore((s) => s.user);
  const userId = user?.id ?? null;
  const workspaceName = useWorkspaceStore((s) => s.currentWorkspace?.name ?? null);
  const { data: conversation } = useConversation(workspaceId, conversationId);

  const { data: members = [] } = useQuery({
    queryKey: [...queryKeys.workspaces.members(workspaceId), 'assignable'],
    queryFn: async () => unwrap(await workspacesService.listAssignableMembers(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
  const { data: shortcuts = [] } = useCannedResponses(workspaceId);
  const createShortcutMutation = useCreateCannedResponse(workspaceId);
  const updateShortcutMutation = useUpdateCannedResponse(workspaceId);
  const deleteShortcutMutation = useDeleteCannedResponse(workspaceId);

  const draftTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const aiAssistedRef = useRef(false);
  const isTypingRef = useRef(false);
  const isNote = replyMode === 'note';
  const isNoteRef = useRef(isNote);
  isNoteRef.current = isNote;

  const lastTypingSentRef = useRef(0);
  const wsSend = useSupportPresenceStore((s) => s.wsSend);
  const wsConnected = useSupportPresenceStore((s) => s.wsConnected);
  const offlineEmailConfirmStorageKey = useMemo(
    () => `${OFFLINE_EMAIL_CONFIRM_STORAGE_PREFIX}:${workspaceId}:${userId ?? 'anonymous'}`,
    [workspaceId, userId],
  );
  const [skipOfflineEmailConfirm, setSkipOfflineEmailConfirm] = useState(() =>
    loadSkipOfflineEmailConfirm(offlineEmailConfirmStorageKey),
  );
  const [offlineEmailConfirmOpen, setOfflineEmailConfirmOpen] = useState(false);
  const [doNotAskAgain, setDoNotAskAgain] = useState(false);

  // Link insertion modal
  const [linkModalOpen, setLinkModalOpen] = useState(false);
  const [linkInitial, setLinkInitial] = useState<{ label: string; url: string }>({ label: '', url: '' });

  // Toolbar visibility — show when the editor is focused, or while interacting
  // with the toolbar itself, or when the link modal is open.
  const [editorFocused, setEditorFocused] = useState(false);
  const toolbarHasPointerRef = useRef(false);
  // toolbarHasPointerRef still used by the merged bottom bar to keep editor focus state

  useEffect(() => {
    setSkipOfflineEmailConfirm(loadSkipOfflineEmailConfirm(offlineEmailConfirmStorageKey));
  }, [offlineEmailConfirmStorageKey]);

  // File attachments
  const fileInputRef = useRef<HTMLInputElement>(null);
  const uploadMutation = useUploadSupportAttachment(workspaceId, conversationId);
  const [pendingAttachments, setPendingAttachments] = useState<PendingSupportAttachment[]>([]);

  const uploadFiles = useCallback(async (files: File[]) => {
    if (files.length === 0) return;
    for (const file of files) {
      if (file.size > 10 * 1024 * 1024) {
        toast.error(`${file.name} exceeds 10 MB limit`);
        continue;
      }
      const localId = `att-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
      const previewUrl = file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined;
      setPendingAttachments((prev) => [...prev, { localId, fileName: file.name, fileType: file.type, status: 'uploading', previewUrl, previewObjectUrl: !!previewUrl }]);
      try {
        const result = await uploadMutation.mutateAsync({ file });
        setPendingAttachments((prev) => prev.map((a) => a.localId === localId ? { ...a, status: 'done', attachmentId: result.id } : a));
      } catch {
        setPendingAttachments((prev) => prev.map((a) => a.localId === localId ? { ...a, status: 'error' } : a));
      }
    }
  }, [uploadMutation]);
  const uploadFilesRef = useRef(uploadFiles);
  uploadFilesRef.current = uploadFiles;

  const handleFileSelect = useCallback(async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    await uploadFiles(Array.from(files));
    if (fileInputRef.current) fileInputRef.current.value = '';
  }, [uploadFiles]);

  const removeAttachment = useCallback((localId: string) => {
    setPendingAttachments((prev) => {
      const att = prev.find((a) => a.localId === localId);
      if (att?.previewUrl && att.previewObjectUrl) URL.revokeObjectURL(att.previewUrl);
      return prev.filter((a) => a.localId !== localId);
    });
  }, []);

  // Mention state
  const [mentionState, setMentionState] = useState<{
    from: number;
    to: number;
    items: MentionSuggestionItem[];
    selectedIndex: number;
  } | null>(null);
  const mentionStateRef = useRef(mentionState);
  mentionStateRef.current = mentionState;
  const membersRef = useRef(members);
  membersRef.current = members;
  const shortcutsPanelOpenRef = useRef(false);
  const shortcutsPanelRef = useRef<HTMLDivElement | null>(null);
  const shortcutSearchInputRef = useRef<HTMLInputElement | null>(null);
  const panelIndexRef = useRef(0);
  const shortcutOpenRequest = useShortcutComposerStore((s) => s.openRequest);
  const shortcutSeedShortCode = useShortcutComposerStore((s) => s.seedShortCode);
  const shortcutSeedContent = useShortcutComposerStore((s) => s.seedContent);
  const clearShortcutComposerRequest = useShortcutComposerStore((s) => s.clear);
  const handledShortcutOpenRequestRef = useRef(0);
  const [shortcutsPanelOpen, setShortcutsPanelOpen] = useState(false);
  shortcutsPanelOpenRef.current = shortcutsPanelOpen;
  const [manualShortcutQuery, setManualShortcutQuery] = useState('');
  const [shortcutPanelMode, setShortcutPanelMode] = useState<ShortcutPanelMode>('list');
  const shortcutPanelModeRef = useRef(shortcutPanelMode);
  shortcutPanelModeRef.current = shortcutPanelMode;
  const [shortcutForm, setShortcutForm] = useState<ShortcutFormState>(() => emptyShortcutForm());
  const [editingShortcutId, setEditingShortcutId] = useState<string | null>(null);
  const [shortcutDeleteConfirm, setShortcutDeleteConfirm] = useState(false);
  const [shortcutState, setShortcutState] = useState<{
    from: number;
    to: number;
    query: string;
    items: SupportCannedResponse[];
    selectedIndex: number;
  } | null>(null);
  const shortcutStateRef = useRef(shortcutState);
  shortcutStateRef.current = shortcutState;
  const shortcutsRef = useRef(shortcuts);
  shortcutsRef.current = shortcuts;
  const manualShortcuts = useMemo(
    () => manualShortcutQuery.trim() ? filterShortcuts(shortcuts, manualShortcutQuery) : shortcuts,
    [manualShortcutQuery, shortcuts],
  );
  const manualShortcutsRef = useRef(manualShortcuts);
  manualShortcutsRef.current = manualShortcuts;
  const [panelIndex, setPanelIndex] = useState(0);
  panelIndexRef.current = panelIndex;
  const shortcutFormPending = createShortcutMutation.isPending || updateShortcutMutation.isPending || deleteShortcutMutation.isPending;

  const shortcutVariableContext = useMemo(() => ({
    customer: {
      fullName: conversation?.customer_name,
      email: conversation?.customer_email ?? emailFallbackHint?.email,
    },
    agent: {
      fullName: user?.full_name,
      email: user?.email,
    },
    workspaceName,
    conversationSubject: conversation?.subject,
  }), [conversation?.customer_email, conversation?.customer_name, conversation?.subject, emailFallbackHint?.email, user?.email, user?.full_name, workspaceName]);

  const resolveShortcutContent = useCallback(
    (content: string) => resolveShortcutVariables(content, shortcutVariableContext),
    [shortcutVariableContext],
  );
  const resolveShortcutContentRef = useRef(resolveShortcutContent);
  resolveShortcutContentRef.current = resolveShortcutContent;

  const openShortcutCreate = useCallback((seed?: { seedShortCode?: string; seedContent?: string }) => {
    setShortcutForm(emptyShortcutForm(seed));
    setEditingShortcutId(null);
    setShortcutDeleteConfirm(false);
    setManualShortcutQuery('');
    setShortcutPanelMode('create');
    setShortcutState(null);
    setShortcutsPanelOpen(true);
  }, []);

  const openShortcutEdit = useCallback((shortcut: SupportCannedResponse) => {
    setShortcutForm(shortcutFormFromShortcut(shortcut));
    setEditingShortcutId(shortcut.id);
    setShortcutDeleteConfirm(false);
    setManualShortcutQuery('');
    setShortcutPanelMode('edit');
    setShortcutState(null);
    setShortcutsPanelOpen(true);
  }, []);

  const returnToShortcutList = useCallback(() => {
    setShortcutPanelMode('list');
    setShortcutDeleteConfirm(false);
    setEditingShortcutId(null);
    setShortcutForm(emptyShortcutForm());
    setShortcutsPanelOpen(true);
  }, []);

  const closeShortcutsPanel = useCallback(() => {
    setShortcutsPanelOpen(false);
    setShortcutState(null);
    setShortcutPanelMode('list');
    setManualShortcutQuery('');
    setShortcutDeleteConfirm(false);
  }, []);

  useEffect(() => {
    setPanelIndex(0);
  }, [manualShortcutQuery]);

  useEffect(() => {
    if (!shortcutsPanelOpen || shortcutState || shortcutPanelMode !== 'list') return;
    requestAnimationFrame(() => shortcutSearchInputRef.current?.focus());
  }, [shortcutPanelMode, shortcutState, shortcutsPanelOpen]);

  useEffect(() => {
    if (shortcutOpenRequest === 0) return;
    if (shortcutOpenRequest <= handledShortcutOpenRequestRef.current) return;
    handledShortcutOpenRequestRef.current = shortcutOpenRequest;
    openShortcutCreate({
      seedShortCode: shortcutSeedShortCode,
      seedContent: shortcutSeedContent,
    });
    clearShortcutComposerRequest();
  }, [clearShortcutComposerRequest, openShortcutCreate, shortcutOpenRequest, shortcutSeedContent, shortcutSeedShortCode]);

  const submitShortcutForm = useCallback(async () => {
    const payload = {
      short_code: shortcutForm.shortCode.trim(),
      content: shortcutForm.content.trim(),
      tag: normalizeShortcutFormCategory(shortcutForm),
      // Deprecated compatibility for older API processes during rollout.
      // Current backend ignores this field; old backend required it.
      title: shortcutForm.shortCode.trim(),
    };
    if (shortcutPanelMode === 'edit' && editingShortcutId) {
      await updateShortcutMutation.mutateAsync({ responseId: editingShortcutId, payload });
      toast.success('Shortcut updated');
    } else {
      await createShortcutMutation.mutateAsync(payload);
      toast.success('Shortcut created');
    }
    returnToShortcutList();
  }, [createShortcutMutation, editingShortcutId, returnToShortcutList, shortcutForm, shortcutPanelMode, updateShortcutMutation]);

  const deleteEditingShortcut = useCallback(async () => {
    if (!editingShortcutId) return;
    await deleteShortcutMutation.mutateAsync(editingShortcutId);
    toast.success('Shortcut deleted');
    returnToShortcutList();
  }, [deleteShortcutMutation, editingShortcutId, returnToShortcutList]);

  // Typing indicator
  const sendTyping = useCallback((typing: boolean, typingContent?: string) => {
    if (isNoteRef.current || !wsSend || !wsConnected) return;
    if (!typing && typingTimerRef.current) {
      clearTimeout(typingTimerRef.current);
      typingTimerRef.current = null;
    }
    if (!typing) {
      isTypingRef.current = false;
      wsSend('support:typing:stop', { conversation_id: conversationId });
      return;
    }
    const now = Date.now();
    if (isTypingRef.current && now - lastTypingSentRef.current < 300) return;
    const eventType = isTypingRef.current ? 'support:typing:update' : 'support:typing:start';
    isTypingRef.current = true;
    lastTypingSentRef.current = now;
    wsSend(eventType, {
      conversation_id: conversationId,
      content: typingContent ?? '',
    });
  }, [conversationId, wsSend, wsConnected]);

  const handleTyping = useCallback((typingContent: string) => {
    sendTyping(true, typingContent);
    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    typingTimerRef.current = setTimeout(() => sendTyping(false), 5000);
  }, [sendTyping]);

  // Clean up typing indicator on unmount or conversation change
  useEffect(() => {
    return () => {
      if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
      if (isTypingRef.current && wsSend) {
        isTypingRef.current = false;
        wsSend('support:typing:stop', { conversation_id: conversationId });
      }
    };
  }, [conversationId, wsSend]);

  const handleSendRef = useRef<() => void>(() => {});

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
    Markdown.configure({
      html: false,
      linkify: true,
      transformPastedText: true,
      transformCopiedText: true,
    }),
    Placeholder.configure({
      placeholder: () => isNoteRef.current ? 'Add an internal note... (@ to mention)' : 'Write a reply... (@ to mention)',
    }),
    MentionHighlight.configure({
      validHandles: () => {
        const handles = new Set<string>();
        for (const m of membersRef.current) {
          const handle = getMemberMentionHandle(m);
          if (handle) handles.add(handle.toLowerCase());
        }
        return handles;
      },
    }),
  ], []);

  const editor = useEditor({
    extensions,
    editorProps: {
      attributes: {
        class: 'prose prose-sm dark:prose-invert max-w-none focus:outline-none min-h-[40px] max-h-[160px] overflow-y-auto text-sm leading-relaxed',
      },
      handleKeyDown: (_view, event) => {
        const currentShortcut = shortcutStateRef.current;
        if (currentShortcut && shortcutPanelModeRef.current === 'list') {
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
              selectedIndex:
                (currentShortcut.selectedIndex - 1 + currentShortcut.items.length) %
                currentShortcut.items.length,
            });
            return true;
          }
          if ((event.key === 'Enter' || event.key === 'Tab') && currentShortcut.items.length > 0) {
            const selected = currentShortcut.items[currentShortcut.selectedIndex];
            if (!selected || !editorRef.current) return false;
            event.preventDefault();
            editorRef.current
              .chain()
              .focus()
              .insertContentAt(
                { from: currentShortcut.from, to: currentShortcut.to },
                resolveShortcutContentRef.current(selected.content),
              )
              .run();
            setShortcutState(null);
            setShortcutsPanelOpen(false);
            return true;
          }
          if (event.key === 'Escape') {
            event.preventDefault();
            closeShortcutsPanel();
            return true;
          }
        }

        // Browse-mode keyboard nav (panel opened via toolbar, no `!` query)
        if (!currentShortcut && shortcutPanelModeRef.current === 'list' && shortcutsPanelOpenRef.current && manualShortcutsRef.current.length > 0) {
          const items = manualShortcutsRef.current;
          if (event.key === 'ArrowDown') {
            event.preventDefault();
            setPanelIndex((idx) => (idx + 1) % items.length);
            return true;
          }
          if (event.key === 'ArrowUp') {
            event.preventDefault();
            setPanelIndex((idx) => (idx - 1 + items.length) % items.length);
            return true;
          }
          if (event.key === 'Enter') {
            const selected = items[panelIndexRef.current];
            if (!selected || !editorRef.current) return false;
            event.preventDefault();
            editorRef.current.chain().focus().insertContent(resolveShortcutContentRef.current(selected.content)).run();
            setShortcutsPanelOpen(false);
            return true;
          }
          if (event.key === 'Escape') {
            event.preventDefault();
            closeShortcutsPanel();
            return true;
          }
        }

        const currentMention = mentionStateRef.current;
        if (currentMention && currentMention.items.length > 0) {
          if (event.key === 'ArrowDown') {
            event.preventDefault();
            setMentionState({
              ...currentMention,
              selectedIndex: (currentMention.selectedIndex + 1) % currentMention.items.length,
            });
            return true;
          }
          if (event.key === 'ArrowUp') {
            event.preventDefault();
            setMentionState({
              ...currentMention,
              selectedIndex:
                (currentMention.selectedIndex - 1 + currentMention.items.length) %
                currentMention.items.length,
            });
            return true;
          }
          if (event.key === 'Enter' || event.key === 'Tab') {
            const selected = currentMention.items[currentMention.selectedIndex];
            if (!selected || !editorRef.current) return false;
            event.preventDefault();
            editorRef.current
              .chain()
              .focus()
              .insertContentAt(
                { from: currentMention.from, to: currentMention.to },
                `@${selected.handle} `,
              )
              .run();
            setMentionState(null);
            return true;
          }
          if (event.key === 'Escape') {
            event.preventDefault();
            setMentionState(null);
            return true;
          }
        }

        // Ctrl/Cmd+Enter to submit
        if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
          event.preventDefault();
          handleSendRef.current();
          return true;
        }

        // Shift+Enter — auto-link the trailing URL token, then insert a hard break
        if (event.key === 'Enter' && event.shiftKey && !event.ctrlKey && !event.metaKey) {
          const ed = editorRef.current;
          const found = findBareUrlBeforeCaret(ed);
          if (ed && found) {
            event.preventDefault();
            ed.chain()
              .focus()
              .setTextSelection({ from: found.from, to: found.to })
              .setLink({ href: found.href })
              .setTextSelection(found.to)
              .unsetMark('link')
              .setHardBreak()
              .run();
            return true;
          }
        }

        return false;
      },
      handlePaste: (_view, event) => {
        const imageFiles = getClipboardImageFiles(event.clipboardData);
        if (imageFiles.length === 0) return false;
        event.preventDefault();
        void uploadFilesRef.current(imageFiles);
        return true;
      },
    },
    onUpdate: ({ editor: ed }) => {
      const text = ed.getText();
      const markdown = getEditorMarkdown(ed);

      // Debounce draft save (markdown so formatting persists across reloads)
      if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
      draftTimerRef.current = setTimeout(() => setDraft(conversationId, markdown), 500);

      // Typing indicator (plain text preview is enough)
      text.trim() ? handleTyping(text) : sendTyping(false);

      // Mention detection
      const mention = detectMentions(ed, membersRef.current);
      setMentionState(mention);
      const shortcut = detectShortcuts(ed, shortcutsRef.current);
      setShortcutState(shortcut);
      setShortcutsPanelOpen(!!shortcut);

      // Auto-switch to note mode when mention detected in reply mode
      if (mention && useSupportInboxStore.getState().replyMode === 'reply') {
        setReplyMode('note');
      }
    },
    onBlur: () => {
      window.setTimeout(() => {
        const active = document.activeElement;
        if (
          active &&
          (shortcutsPanelRef.current?.contains(active) || isShortcutPanelPortalTarget(active))
        ) return;
        setMentionState(null);
        closeShortcutsPanel();
      }, 0);
    },
  });

  const editorRef = useRef(editor);
  editorRef.current = editor;

  const insertShortcut = useCallback((shortcut: SupportCannedResponse, range?: { from: number; to: number }) => {
    const ed = editorRef.current;
    if (!ed) return;
    const content = resolveShortcutContent(shortcut.content);
    const chain = ed.chain().focus();
    if (range) {
      chain.insertContentAt(range, content).run();
    } else {
      chain.insertContent(content).run();
    }
    closeShortcutsPanel();
  }, [closeShortcutsPanel, resolveShortcutContent]);

  // Backup event handlers for TipTap v3 compatibility
  // Close the shortcuts panel when the user clicks anywhere outside both
  // the panel and the editor. Editor blur alone isn't reliable across all
  // focus-stealing surfaces, so we listen at the document level while open.
  useEffect(() => {
    if (!shortcutsPanelOpen && !shortcutState) return;
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node | null;
      if (!target) return;
      if (shortcutsPanelRef.current?.contains(target)) return;
      if (isShortcutPanelPortalTarget(target)) return;
      // When the panel is `!`-triggered (shortcutState active), keep it open
      // while the user is still typing in the editor. When it's manually
      // opened from the Shortcuts button (no shortcutState), clicking back
      // into the editor should dismiss it like any other outside click.
      if (shortcutState) {
        const editorEl = editor?.view?.dom;
        if (editorEl && editorEl.contains(target)) return;
      }
      closeShortcutsPanel();
    };
    document.addEventListener('pointerdown', onPointerDown, true);
    return () => document.removeEventListener('pointerdown', onPointerDown, true);
  }, [closeShortcutsPanel, shortcutsPanelOpen, shortcutState, editor]);

  useEffect(() => {
    if (!editor) return;

    const handleUpdate = () => {
      const text = editor.getText();
      const markdown = getEditorMarkdown(editor);
      if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
      draftTimerRef.current = setTimeout(() => setDraft(conversationId, markdown), 500);
      text.trim() ? handleTyping(text) : sendTyping(false);
      const mention = detectMentions(editor, membersRef.current);
      setMentionState(mention);
      const shortcut = detectShortcuts(editor, shortcutsRef.current);
      setShortcutState(shortcut);
      setShortcutsPanelOpen(!!shortcut);
      if (mention && useSupportInboxStore.getState().replyMode === 'reply') {
        setReplyMode('note');
      }
    };
    const handleBlur = () => {
      // Defer so that clicking a toolbar button (which steals focus briefly)
      // doesn't immediately collapse the toolbar.
      setTimeout(() => {
        const active = document.activeElement;
        if (
          !active ||
          (
            !shortcutsPanelRef.current?.contains(active) &&
            !isShortcutPanelPortalTarget(active)
          )
        ) {
          setMentionState(null);
          closeShortcutsPanel();
        }
        if (!toolbarHasPointerRef.current) {
          setEditorFocused(false);
        }
      }, 0);
    };
    const handleFocus = () => setEditorFocused(true);

    editor.on('update', handleUpdate);
    editor.on('blur', handleBlur);
    editor.on('focus', handleFocus);

    return () => {
      editor.off('update', handleUpdate);
      editor.off('blur', handleBlur);
      editor.off('focus', handleFocus);
    };
  }, [closeShortcutsPanel, editor, conversationId, handleTyping, sendTyping, setDraft, setReplyMode]);

  // Load draft when switching conversations
  useEffect(() => {
    if (!editor) return;
    const saved = useSupportInboxStore.getState().drafts[conversationId] ?? '';
    if (saved) {
      // Markdown extension parses markdown when content is a string
      editor.commands.setContent(saved);
    } else {
      editor.commands.clearContent();
    }
    return () => {
      if (draftTimerRef.current) {
        clearTimeout(draftTimerRef.current);
        draftTimerRef.current = null;
      }
    };
  }, [conversationId, editor]);

  useEffect(() => {
    if (!editor) return;
    const handleRestoreDraft = (event: Event) => {
      const detail = (event as CustomEvent<{
        conversationId?: string;
        markdown?: string;
        attachments?: SupportAttachmentPayload[];
      }>).detail;
      if (detail?.conversationId !== conversationId) return;
      const markdown = detail.markdown ?? '';
      setReplyMode('reply');
      setDraft(conversationId, markdown);
      setPendingAttachments((current) => {
        current.forEach((attachment) => {
          if (attachment.previewUrl && attachment.previewObjectUrl) URL.revokeObjectURL(attachment.previewUrl);
        });
        return restoreAttachmentsFromMessage(detail.attachments);
      });
      editor.commands.setContent(markdown);
      editor.commands.focus('end');
    };
    window.addEventListener(RESTORE_SUPPORT_DRAFT_EVENT, handleRestoreDraft);
    return () => window.removeEventListener(RESTORE_SUPPORT_DRAFT_EVENT, handleRestoreDraft);
  }, [conversationId, editor, setDraft, setReplyMode]);

  // Force placeholder redecoration when mode changes
  useEffect(() => {
    if (editor && editor.isEmpty) {
      editor.view.dispatch(editor.state.tr);
    }
  }, [editor, isNote]);

  const sendReply = useCallback(async () => {
    if (!editor) return;
    const markdown = getEditorMarkdown(editor).trim();
    const doneAttachments = pendingAttachments.filter((a) => a.status === 'done' && a.attachmentId);
    if ((!markdown && doneAttachments.length === 0) || sendMutation.isPending) return;

    if (typingTimerRef.current) clearTimeout(typingTimerRef.current);
    if (draftTimerRef.current) clearTimeout(draftTimerRef.current);
    sendTyping(false);

    const attachmentIds = doneAttachments.map((a) => a.attachmentId!);
    const isInternal = useSupportInboxStore.getState().replyMode === 'note';
    const primaryEmail = conversation?.customer_email?.trim() || emailFallbackHint?.email?.trim() || '';
    const normalizedCC = normalizeRecipientEmails(conversation?.email_cc ?? [], [primaryEmail]);

    await sendMutation.mutateAsync({
      content: markdown || ' ',
      is_internal: isInternal,
      ...(!isInternal && aiAssistedRef.current ? { ai_assisted: true } : {}),
      ...(!isInternal && primaryEmail ? { channels: ['email' as const] } : {}),
      ...(!isInternal && normalizedCC.length > 0 ? { cc_emails: normalizedCC } : {}),
      ...(attachmentIds.length > 0 ? { attachment_ids: attachmentIds } : {}),
    });

    // Clean up preview URLs
    pendingAttachments.forEach((a) => { if (a.previewUrl && a.previewObjectUrl) URL.revokeObjectURL(a.previewUrl); });
    setPendingAttachments([]);
		aiAssistedRef.current = false;
    editor.commands.clearContent();
    clearDraft(conversationId);
    editor.commands.focus();
  }, [clearDraft, conversation?.customer_email, conversation?.email_cc, conversationId, editor, emailFallbackHint?.email, pendingAttachments, sendMutation, sendTyping]);

  const handleSend = useCallback(async () => {
    if (!editor) return;
    const markdown = getEditorMarkdown(editor).trim();
    const hasUploadedAttachments = pendingAttachments.some((a) => a.status === 'done' && a.attachmentId);
    if ((!markdown && !hasUploadedAttachments) || sendMutation.isPending) return;

    if (!isNote && conversation?.primary_recipient_state === 'unconfirmed') {
      toast.error('Confirm the primary recipient before sending');
      return;
    }

    if (!isNote && emailFallbackHint && !skipOfflineEmailConfirm) {
      setDoNotAskAgain(false);
      setOfflineEmailConfirmOpen(true);
      return;
    }

    await sendReply();
  }, [conversation?.primary_recipient_state, editor, emailFallbackHint, isNote, pendingAttachments, sendMutation.isPending, sendReply, skipOfflineEmailConfirm]);

  const handleRewrite = useCallback(async (operation: SupportAIRewriteOperation) => {
    if (!editor) return;
    const text = editor.getText().trim();
    if (!text || rewriteMutation.isPending) return;

    try {
      const rewritten = await rewriteMutation.mutateAsync({
        content: text,
        operation,
      });

      editor.commands.setContent(rewritten.content);
      editor.commands.focus('end');
			aiAssistedRef.current = true;
    } catch (error) {
      const reason = getUpgradeRequiredReason(error);
      if (reason) onUpgradeRequired?.(reason);
    }
  }, [editor, onUpgradeRequired, rewriteMutation]);

  const handleConfirmOfflineEmailSend = useCallback(async () => {
    if (doNotAskAgain) {
      saveSkipOfflineEmailConfirm(offlineEmailConfirmStorageKey, true);
      setSkipOfflineEmailConfirm(true);
    }
    setOfflineEmailConfirmOpen(false);
    await sendReply();
  }, [doNotAskAgain, offlineEmailConfirmStorageKey, sendReply]);

  const confirmCurrentPrimary = useCallback(async () => {
    if (!conversation) return;
    await updateEmailRecipients.mutateAsync({
      conversationId: conversation.id,
      payload: {
        confirm_primary: true,
        cc_emails: normalizeRecipientEmails(conversation.email_cc ?? [], [conversation.customer_email ?? '']),
      },
    });
    toast.success('Primary recipient confirmed');
  }, [conversation, updateEmailRecipients]);

  const makeSuggestedPrimary = useCallback(async () => {
    if (!conversation?.suggested_primary_recipient_email) return;
    await updateEmailRecipients.mutateAsync({
      conversationId: conversation.id,
      payload: {
        primary_recipient_email: conversation.suggested_primary_recipient_email,
        primary_recipient_name: conversation.suggested_primary_recipient_name ?? undefined,
        confirm_primary: true,
      },
    });
    toast.success('Primary recipient updated');
  }, [conversation, updateEmailRecipients]);

  handleSendRef.current = handleSend;

  const openLinkModal = useCallback(() => {
    if (!editor) return;
    const attrs = editor.getAttributes('link') as { href?: string };
    const { from, to, empty } = editor.state.selection;
    let initialLabel = '';
    if (editor.isActive('link')) {
      // Expand selection to entire link mark range
      editor.chain().focus().extendMarkRange('link').run();
      const expanded = editor.state.selection;
      initialLabel = editor.state.doc.textBetween(expanded.from, expanded.to, ' ');
    } else if (!empty) {
      initialLabel = editor.state.doc.textBetween(from, to, ' ');
    }
    setLinkInitial({ label: initialLabel, url: attrs.href ?? '' });
    setLinkModalOpen(true);
  }, [editor]);

  const handleLinkInsert = useCallback((label: string, url: string) => {
    if (!editor) return;
    const chain = editor.chain().focus();
    if (editor.isActive('link')) {
      chain.extendMarkRange('link').unsetLink().run();
    }
    const { from, to, empty } = editor.state.selection;
    if (empty) {
      editor.chain().focus()
        .insertContent({ type: 'text', text: label, marks: [{ type: 'link', attrs: { href: url } }] })
        .run();
    } else {
      editor.chain().focus()
        .insertContentAt({ from, to }, { type: 'text', text: label, marks: [{ type: 'link', attrs: { href: url } }] })
        .run();
    }
  }, [editor]);

  const handleLinkRemove = useCallback(() => {
    if (!editor) return;
    editor.chain().focus().extendMarkRange('link').unsetLink().run();
  }, [editor]);

  if (!editor) return null;

  const content = editor.getText();
  const hasContent = content.trim().length > 0;
  const primaryRecipientEmail = conversation?.customer_email?.trim() || emailFallbackHint?.email?.trim() || '';
  const suggestedPrimaryEmail = conversation?.suggested_primary_recipient_email?.trim() || '';
  const primaryRecipientUnconfirmed = conversation?.primary_recipient_state === 'unconfirmed';
  const canUseAITools = hasContent && !rewriteMutation.isPending;
  const aiTools: Array<{ operation: SupportAIRewriteOperation; label: string; icon: typeof ArrowUpDownIcon }> = [
    { operation: 'expand', label: 'Expand', icon: ArrowUpDownIcon },
    { operation: 'rephrase', label: 'Rephrase', icon: ArrowReloadHorizontalIcon },
    { operation: 'fix_grammar', label: 'Fix grammar', icon: TickDouble01Icon },
    { operation: 'more_friendly', label: 'More friendly', icon: SmileIcon },
    { operation: 'more_formal', label: 'More formal', icon: Briefcase01Icon },
  ];

  return (
    <div
      className={cn(
        'relative mx-3 mb-4 rounded-xl border border-border/40 bg-card transition-colors',
        editorFocused && (
          isNote
            ? 'border-amber-400 dark:border-amber-500'
            : 'border-blue-500 dark:border-blue-400'
        ),
        isNote && 'bg-amber-50/50 dark:bg-amber-950/10'
      )}
    >
      {primaryRecipientUnconfirmed && !isNote && (
        <div className="flex flex-col gap-2 rounded-t-xl border-b border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-100">
          <div className="flex items-start gap-2">
            <Mail01Icon className="mt-0.5 h-3.5 w-3.5 shrink-0" />
            <p className="min-w-0">Support was copied on this email. Confirm who replies should go to before sending.</p>
          </div>
          <div className="flex flex-wrap gap-2 pl-5">
            {suggestedPrimaryEmail ? (
              <Button
                type="button"
                size="sm"
                className="h-7 rounded-full px-3 text-xs"
                disabled={updateEmailRecipients.isPending}
                onClick={() => { void makeSuggestedPrimary(); }}
              >
                Make {suggestedPrimaryEmail} primary
              </Button>
            ) : null}
            {primaryRecipientEmail ? (
              <Button
                type="button"
                size="sm"
                variant="outline"
                className="h-7 rounded-full bg-background/80 px-3 text-xs"
                disabled={updateEmailRecipients.isPending}
                onClick={() => { void confirmCurrentPrimary(); }}
              >
                Keep {primaryRecipientEmail} primary
              </Button>
            ) : null}
          </div>
        </div>
      )}

      {emailFallbackHint && !isNote && !primaryRecipientUnconfirmed && editor && !editor.isEmpty && (
        <div className="flex items-start gap-2 border-b border-border/20 bg-muted/20 px-4 py-2.5 text-xs text-muted-foreground rounded-t-xl">
          <Mail01Icon className="mt-0.5 h-3.5 w-3.5 shrink-0 text-blue-500" />
          <p>
            User is offline. Replies sent here will also be queued as an email to{' '}
            <span className="font-medium text-foreground">{emailFallbackHint.email}</span>.
          </p>
        </div>
      )}

      {/* Mention suggestions popover — floats above the composer */}
      {mentionState && mentionState.items.length > 0 && (
        <div className="absolute bottom-full left-0 right-0 z-50 mb-2 px-1">
          <div className="max-h-[260px] overflow-y-auto rounded-xl border border-border/60 bg-popover p-1.5 shadow-lg">
            <p className="px-2 pb-1 pt-0.5 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/50">
              Suggestions
            </p>
            <MentionSuggestionsList
              items={mentionState.items}
              selectedIndex={mentionState.selectedIndex}
              compact
              onSelect={(item) => {
                if (!editor) return;
                editor
                  .chain()
                  .focus()
                  .insertContentAt(
                    { from: mentionState.from, to: mentionState.to },
                    `@${item.handle} `,
                  )
                  .run();
                setMentionState(null);
              }}
            />
          </div>
        </div>
      )}

      {(shortcutState || shortcutsPanelOpen) && (() => {
        const items = shortcutState ? shortcutState.items : manualShortcuts;
        const isFiltering = !!shortcutState;
        const queryToken = shortcutState ? `!${shortcutState.query}` : '';
        const manualSearchActive = !isFiltering && manualShortcutQuery.trim().length > 0;
        const manualSearchToken = manualShortcutQuery.trim();
        const addShortcutSeed = isFiltering ? queryToken : manualSearchToken ? seedShortcutCode(manualSearchToken) : '';
        const isManaging = shortcutPanelMode !== 'list';
        return (
          <div ref={shortcutsPanelRef} className="absolute bottom-full left-0 right-0 z-50 mb-2 px-1">
            <div className={cn(
              'flex flex-col overflow-hidden rounded-xl border border-border/60 bg-popover shadow-lg',
              isManaging ? 'max-h-[520px]' : 'max-h-[340px]',
            )}>
              {isManaging ? (
                <ShortcutFormPanel
                  mode={shortcutPanelMode}
                  form={shortcutForm}
                  shortcuts={shortcuts}
                  editingId={editingShortcutId}
                  pending={shortcutFormPending}
                  deleteConfirm={shortcutDeleteConfirm}
                  onBack={returnToShortcutList}
                  onChange={(next) => {
                    setShortcutForm(next);
                    setShortcutDeleteConfirm(false);
                  }}
                  onSubmit={() => { void submitShortcutForm(); }}
                  onDelete={() => { void deleteEditingShortcut(); }}
                  onDeleteConfirmChange={setShortcutDeleteConfirm}
                  onClose={closeShortcutsPanel}
                />
              ) : (
                <>
                  <div className="space-y-2 border-b border-border/40 px-3 py-2">
                    {(isFiltering || manualSearchActive) ? (
                      <div className="flex items-center justify-between gap-2">
                        <div className="min-w-0">
                          {isFiltering && shortcutState ? (
                            <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[11px] text-foreground">{queryToken}</span>
                          ) : null}
                        </div>
                        <div className="flex shrink-0 items-center gap-1.5">
                          <span className="text-[11px] tabular-nums text-muted-foreground/70">
                            {items.length} {items.length === 1 ? 'match' : 'matches'}
                          </span>
                          <button
                            type="button"
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={closeShortcutsPanel}
                            className="inline-flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                            aria-label="Close shortcuts"
                          >
                            <Cancel01Icon className="h-4 w-4" />
                          </button>
                        </div>
                      </div>
                    ) : null}
                    {!isFiltering ? (
                      <div className="flex items-center gap-2">
                        <div className="relative min-w-0 flex-1">
                          <Search01Icon className="pointer-events-none absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                          <Input
                            ref={shortcutSearchInputRef}
                            value={manualShortcutQuery}
                            onChange={(event) => setManualShortcutQuery(event.target.value)}
                            onKeyDown={(event) => {
                              if (event.key === 'ArrowDown' && items.length > 0) {
                                event.preventDefault();
                                setPanelIndex((idx) => (idx + 1) % items.length);
                                return;
                              }
                              if (event.key === 'ArrowUp' && items.length > 0) {
                                event.preventDefault();
                                setPanelIndex((idx) => (idx - 1 + items.length) % items.length);
                                return;
                              }
                              if (event.key === 'Enter' && items.length > 0) {
                                event.preventDefault();
                                const selected = items[panelIndexRef.current];
                                if (selected) insertShortcut(selected);
                                return;
                              }
                              if (event.key === 'Escape') {
                                event.preventDefault();
                                closeShortcutsPanel();
                              }
                            }}
                            placeholder="Search shortcuts"
                            className="h-8 pl-8 text-sm"
                          />
                        </div>
                        {!(isFiltering || manualSearchActive) ? (
                          <button
                            type="button"
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={closeShortcutsPanel}
                            className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground"
                            aria-label="Close shortcuts"
                          >
                            <Cancel01Icon className="h-4 w-4" />
                          </button>
                        ) : null}
                      </div>
                    ) : null}
                  </div>

                  <div className="flex-1 overflow-y-auto p-1.5">
                    {items.length > 0 ? (
                      <ShortcutsList
                        shortcuts={items}
                        selectedIndex={shortcutState?.selectedIndex ?? (isFiltering ? undefined : panelIndex)}
                        query={shortcutState?.query ?? manualShortcutQuery}
                        compact
                        onSelect={(shortcut) => insertShortcut(shortcut, shortcutState ? { from: shortcutState.from, to: shortcutState.to } : undefined)}
                        onEdit={openShortcutEdit}
                        onDelete={(shortcut) => {
                          openShortcutEdit(shortcut);
                          setShortcutDeleteConfirm(true);
                        }}
                      />
                    ) : (
                      <div className="flex flex-col items-center gap-2 px-3 py-6 text-center">
                        <div className="rounded-full bg-muted/60 p-2">
                          <StickyNote01Icon className="h-4 w-4 text-muted-foreground" />
                        </div>
                        <p className="text-sm font-medium">
                          {isFiltering ? (
                            <>No shortcut matches <span className="font-mono">{queryToken}</span></>
                          ) : manualSearchActive ? (
                            <>No shortcut matches <span className="font-mono">{manualSearchToken}</span></>
                          ) : 'No shortcuts yet'}
                        </p>
                        <p className="max-w-xs text-xs text-muted-foreground">
                          Save common replies and insert them anytime with `!`.
                        </p>
                        <button
                          type="button"
                          onMouseDown={(event) => event.preventDefault()}
                          onClick={() => openShortcutCreate({ seedShortCode: addShortcutSeed })}
                          className="mt-1 inline-flex items-center gap-1.5 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90"
                        >
                          <PlusSignIcon className="h-3.5 w-3.5" />
                          {addShortcutSeed ? <>Add <span className="font-mono">{addShortcutSeed}</span></> : 'Add shortcut'}
                        </button>
                      </div>
                    )}
                  </div>

                  <div className="flex items-center justify-between gap-3 border-t border-border bg-muted/60 px-3 py-2 text-[11px] font-medium text-muted-foreground">
                    <div className="flex items-center gap-4">
                      <span className="flex items-center gap-1"><kbd className="font-mono text-[15px] leading-none text-foreground">↑↓</kbd> to navigate</span>
                      <span className="flex items-center gap-1"><kbd className="font-mono text-[15px] leading-none text-foreground">↵</kbd> to insert</span>
                      <span className="flex items-center gap-1"><kbd className="font-mono text-[9px] uppercase text-foreground">esc</kbd> to close</span>
                    </div>
                    <button
                      type="button"
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => openShortcutCreate({ seedShortCode: addShortcutSeed })}
                      className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-foreground hover:bg-background"
                    >
                      <PlusSignIcon className="h-3.5 w-3.5" />
                      Add
                    </button>
                  </div>
                </>
              )}
            </div>
          </div>
        );
      })()}

      {/* Mode toggle */}
      <div className="flex items-center gap-1 px-4 pt-3">
        <button
          type="button"
          onClick={() => {
            setReplyMode('reply');
            setShortcutsPanelOpen(false);
            setShortcutPanelMode('list');
          }}
          className={cn(
            'rounded-full px-3 py-1 text-xs font-medium transition-colors',
            !isNote
              ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/25 dark:text-blue-400'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted'
          )}
        >
          Reply
        </button>
        <button
          type="button"
          onClick={() => {
            setReplyMode('note');
            setShortcutsPanelOpen(false);
            setShortcutPanelMode('list');
          }}
          className={cn(
            'rounded-full px-3 py-1 text-xs font-medium transition-colors',
            isNote
              ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-400'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted'
          )}
        >
          Note
        </button>
        <button
          type="button"
          onClick={() => {
            const opening = !shortcutsPanelOpen;
            setShortcutsPanelOpen(opening);
            setShortcutState(null);
            setManualShortcutQuery('');
            setShortcutPanelMode('list');
            setShortcutDeleteConfirm(false);
            setPanelIndex(0);
          }}
          className={cn(
            'rounded-full px-3 py-1 text-xs font-medium transition-colors',
            shortcutsPanelOpen
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:bg-muted hover:text-foreground',
          )}
        >
          Shortcuts
        </button>
        <DropdownMenu>
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="inline-flex">
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={!canUseAITools}
                    className={cn(
                      'h-auto rounded-full px-3 py-1 text-xs font-medium transition-colors',
                      canUseAITools
                        ? 'text-muted-foreground hover:bg-muted hover:text-foreground'
                        : 'text-muted-foreground/50'
                    )}
                  >
                    {rewriteMutation.isPending ? (
                      <Loading01Icon className="h-3 w-3 animate-spin" />
                    ) : (
                      <SparklesIcon className="h-3 w-3" />
                    )}
                    AI Tools
                    <ArrowUp01Icon className="h-3 w-3 rotate-180" />
                  </Button>
                </DropdownMenuTrigger>
              </span>
            </TooltipTrigger>
            {!canUseAITools && (
              <TooltipContent side="top" className="text-xs">Write something first to use AI tools</TooltipContent>
            )}
          </Tooltip>
          <DropdownMenuContent align="start" className="w-48">
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

      {/* TipTap Editor */}
      <div
        className="px-4 py-3"
        onClickCapture={(e) => {
          const target = e.target as HTMLElement | null;
          const anchor = target?.closest('a');
          if (anchor) {
            e.preventDefault();
            e.stopPropagation();
            openLinkModal();
          }
        }}
      >
        <EditorContent editor={editor} />
      </div>

      <LinkInsertModal
        open={linkModalOpen}
        onOpenChange={setLinkModalOpen}
        workspaceId={workspaceId}
        initialLabel={linkInitial.label}
        initialUrl={linkInitial.url}
        onInsert={handleLinkInsert}
        onRemove={editor.isActive('link') ? handleLinkRemove : undefined}
      />

      {/* Attachment preview strip */}
      {pendingAttachments.length > 0 && (
        <div className="flex gap-2 overflow-x-auto px-4 pb-2 pt-1">
          {pendingAttachments.map((att) => (
            <div key={att.localId} className="relative flex-shrink-0">
              {att.previewUrl ? (
                <img src={att.previewUrl} alt={att.fileName} className="h-14 w-14 rounded-lg object-cover border border-border" />
              ) : (
                <div className="flex h-14 w-14 flex-col items-center justify-center rounded-lg border border-border bg-muted px-1">
                  <AttachmentIcon className="h-3.5 w-3.5 text-muted-foreground" />
                  <span className="mt-0.5 max-w-[48px] truncate text-[8px] text-muted-foreground">{att.fileName}</span>
                </div>
              )}
              {att.status === 'uploading' && (
                <div className="absolute inset-0 flex items-center justify-center rounded-lg bg-black/40">
                  <Loading01Icon className="h-4 w-4 animate-spin text-white" />
                </div>
              )}
              {att.status === 'error' && (
                <div className="absolute inset-0 flex items-center justify-center rounded-lg bg-red-500/20 border border-red-400">
                  <span className="text-[9px] font-medium text-red-600">Failed</span>
                </div>
              )}
              <button
                type="button"
                onClick={() => removeAttachment(att.localId)}
                className="absolute -right-1 -top-1 flex h-4 w-4 items-center justify-center rounded-full bg-foreground/80 text-background hover:bg-foreground"
              >
                <Cancel01Icon className="h-2.5 w-2.5" />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Bottom toolbar — formatting + actions in one row */}
      <div
        className="flex items-center justify-between px-4 pb-3"
        onMouseEnter={() => { toolbarHasPointerRef.current = true; }}
        onMouseLeave={() => {
          toolbarHasPointerRef.current = false;
          if (!editor.isFocused) setEditorFocused(false);
        }}
      >
        <div className="flex items-center gap-0.5">
          <EmojiPicker
            onEmojiSelect={(emoji) => {
              if (editorRef.current) {
                editorRef.current.chain().focus().insertContent(emoji).run();
              }
            }}
          />
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground/60 hover:text-foreground transition-colors" onClick={() => fileInputRef.current?.click()}>
                <AttachmentIcon className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">Attach file</TooltipContent>
          </Tooltip>
          <input
            ref={fileInputRef}
            type="file"
            className="hidden"
            multiple
            accept="image/*,.pdf,.doc,.docx,.txt,.csv,.xls,.xlsx,.zip,.gz,.tar,.md"
            onChange={(e) => handleFileSelect(e.target.files)}
          />
          <div className="mx-0.5 h-4 w-px bg-border/40" />
          <FormatButton
            title="Bold (Ctrl+B)"
            active={editor.isActive('bold')}
            onClick={() => editor.chain().focus().toggleBold().run()}
          >
            <TextBoldIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Italic (Ctrl+I)"
            active={editor.isActive('italic')}
            onClick={() => editor.chain().focus().toggleItalic().run()}
          >
            <TextItalicIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Underline (Ctrl+U)"
            active={editor.isActive('underline')}
            onClick={() => editor.chain().focus().toggleUnderline().run()}
          >
            <TextUnderlineIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Insert link"
            active={editor.isActive('link')}
            onClick={openLinkModal}
          >
            <Link01Icon className="h-3.5 w-3.5" />
          </FormatButton>
          <div className="mx-0.5 h-4 w-px bg-border/40" />
          <FormatButton
            title="Bullet list"
            active={editor.isActive('bulletList')}
            onClick={() => editor.chain().focus().toggleBulletList().run()}
          >
            <LeftToRightListBulletIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Numbered list"
            active={editor.isActive('orderedList')}
            onClick={() => editor.chain().focus().toggleOrderedList().run()}
          >
            <LeftToRightListNumberIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Quote"
            active={editor.isActive('blockquote')}
            onClick={() => editor.chain().focus().toggleBlockquote().run()}
          >
            <QuoteDownIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <div className="mx-0.5 h-4 w-px bg-border/40" />
          <FormatButton
            title="Inline code"
            active={editor.isActive('code')}
            onClick={() => editor.chain().focus().toggleCode().run()}
          >
            <CodeIcon className="h-3.5 w-3.5" />
          </FormatButton>
          <FormatButton
            title="Strikethrough"
            active={editor.isActive('strike')}
            onClick={() => editor.chain().focus().toggleStrike().run()}
          >
            <TextStrikethroughIcon className="h-3.5 w-3.5" />
          </FormatButton>
        </div>

        <div className="flex items-center gap-2">
          <kbd className="hidden items-center gap-1 font-mono text-[15px] leading-none text-muted-foreground sm:inline-flex">
            <span>{navigator.platform?.includes('Mac') ? '\u2318' : 'Ctrl'}</span>
            <span>{'\u21B5'}</span>
          </kbd>
          <Button
            size="sm"
            disabled={sendMutation.isPending || (!isNote && primaryRecipientUnconfirmed) || (!content.trim() && !pendingAttachments.some((a) => a.status === 'done'))}
            onClick={handleSend}
            className={cn(
              'h-7 gap-1.5 rounded-full px-3 text-xs',
              isNote && 'bg-amber-500 hover:bg-amber-600 text-white'
            )}
          >
            <SentIcon className="h-3 w-3" />
            {isNote ? 'Add Note' : primaryRecipientUnconfirmed ? 'Confirm recipient' : 'Send'}
          </Button>
        </div>
      </div>

      <AlertDialog open={offlineEmailConfirmOpen} onOpenChange={setOfflineEmailConfirmOpen}>
        <AlertDialogContent
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            window.requestAnimationFrame(() => {
              document.getElementById('offline-email-confirm-send')?.focus();
            });
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>Send this reply by email too?</AlertDialogTitle>
            <AlertDialogDescription>
              This visitor is currently offline. If you send this reply, Helpin will queue an email to{' '}
              <span className="font-medium text-foreground">{emailFallbackHint?.email}</span> and skip the email if the visitor comes back online before it sends.
            </AlertDialogDescription>
          </AlertDialogHeader>

          <div className="flex items-center gap-2">
            <Checkbox
              id="offline-email-dont-ask"
              checked={doNotAskAgain}
              onCheckedChange={(checked) => setDoNotAskAgain(!!checked)}
            />
            <Label htmlFor="offline-email-dont-ask" className="text-sm font-normal">
              Do not ask me again
            </Label>
          </div>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={sendMutation.isPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              id="offline-email-confirm-send"
              variant="default"
              disabled={sendMutation.isPending}
              onClick={(event) => {
                event.preventDefault();
                void handleConfirmOfflineEmailSend();
              }}
            >
              {sendMutation.isPending ? 'Sending...' : 'Send Reply'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
