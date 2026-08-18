import { useEffect, useRef, useState } from 'react';
import {
  ArrowDown01Icon,
  ArrowUp01Icon,
  AttachmentIcon,
  BookOpen01Icon,
  Briefcase01Icon,
  CheckListIcon,
  File01Icon,
  FolderKanbanIcon,
  GitBranchIcon,
  Loading01Icon,
  Image01Icon,
  Message01Icon,
  PlusSignIcon,
  Search01Icon,
  StopIcon,
  Tick01Icon,
  Cancel01Icon,
  UserIcon,
} from '@/lib/icons';
import type { CommandBarPageContext } from '@/lib/pmTypes';
import type { DockEntityReference } from '@/lib/dockTypes';
import type { DockChatMediaAttachment } from '@/lib/dockTypes';
import { getClipboardImageFiles } from '@/lib/clipboardAttachments';
import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import type { PageContextScopeOption } from '@/components/command-bar/pageContext';
import { DockReferencePicker, type DockReferencePickerHandle } from './DockReferencePicker';

const TYPE_LABEL: Record<CommandBarPageContext['entity_type'], string> = {
  task: 'Task',
  epic: 'Epic',
  document: 'Doc',
  crm_contact: 'Contact',
  crm_deal: 'Deal',
  // Keep the dock chip compact; the conversation itself is already obvious
  // from the support surface and its title.
  support_conversation: 'Support',
  workspace: 'Workspace',
  repository: 'Repository',
};

function ContextIcon({ type, className = 'h-3 w-3 shrink-0' }: { type: CommandBarPageContext['entity_type']; className?: string }) {
  switch (type) {
    case 'task':
      return <CheckListIcon className={className} />;
    case 'epic':
      return <BookOpen01Icon className={className} />;
    case 'document':
      return <File01Icon className={className} />;
    case 'crm_contact':
      return <UserIcon className={className} />;
    case 'crm_deal':
      return <Briefcase01Icon className={className} />;
    case 'support_conversation':
      return <Message01Icon className={className} />;
    case 'repository':
      return <GitBranchIcon className={className} />;
    default:
      return <FolderKanbanIcon className={className} />;
  }
}

function isBlockScopedDocument(context: CommandBarPageContext) {
  return context.entity_type === 'document' && context.metadata?.context_scope === 'block';
}

function isAllTasksContext(context: CommandBarPageContext) {
  return context.entity_type === 'workspace' && context.metadata?.context_scope === 'all_tasks';
}

function blockScopeTitle(context: CommandBarPageContext) {
  const excerpt = typeof context.metadata?.block_excerpt === 'string' ? context.metadata.block_excerpt.trim() : '';
  const base = `Agent context is the selected block in ${context.display_title || 'this document'}. The full document is reference.`;
  return excerpt ? `${base}\n\nBlock excerpt: ${excerpt}` : base;
}

function contextTitle(context: CommandBarPageContext) {
  if (isBlockScopedDocument(context)) return blockScopeTitle(context);
  if (isAllTasksContext(context)) return 'Agent context is all tasks in this workspace.';
  return context.display_title || context.entity_id;
}

export interface DockInputProps {
  mode: 'conversation' | 'list';
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  onFocusChange?: (focused: boolean) => void;
  onAddContext?: () => void;
  pageContext: CommandBarPageContext | null;
  contextOptions?: PageContextScopeOption[];
  activeContextKey?: string | null;
  onContextKeyChange?: (key: string) => void;
  onClearContext?: () => void;
  workspaceId?: string;
  references?: DockEntityReference[];
  onAddReference?: (reference: DockEntityReference) => void;
  onRemoveReference?: (reference: DockEntityReference) => void;
  mediaAttachments?: DockChatMediaAttachment[];
  onAddMedia?: (files: File[]) => void;
  onRemoveMedia?: (attachment: DockChatMediaAttachment) => void;
  busy?: boolean;
  disabled?: boolean;
  autoFocus?: boolean;
  textareaRef?: React.RefObject<HTMLTextAreaElement | null>;
  /** When set, the send button becomes a stop button for the active run. */
  onStop?: () => void;
  stopping?: boolean;
  placeholder?: string;
  showShortcutHint?: boolean;
}

export function shouldUseExpandedComposerLayout({
  value,
  scrollHeight,
  singleLineHeight,
  currentlyExpanded,
}: {
  value: string;
  scrollHeight: number;
  singleLineHeight: number;
  currentlyExpanded: boolean;
}) {
  if (!value.trim()) return false;
  // Once text wraps, keep the text row above the actions until the user
  // clears the composer. This avoids a flicker at widths where freeing the
  // action controls makes that same text fit back onto a single line.
  return currentlyExpanded || scrollHeight > singleLineHeight + 1;
}

export function composerTextareaHeight({
  value,
  scrollHeight,
  singleLineHeight,
}: {
  value: string;
  scrollHeight: number;
  singleLineHeight: number;
}) {
  if (!value.trim()) return singleLineHeight;
  return Math.min(scrollHeight, 160);
}

export function composerPlaceholderForContext(contextType?: CommandBarPageContext['entity_type']) {
  return contextType === 'support_conversation'
    ? 'Ask about this conversation…'
    : 'Ask a question or delegate work to agents…';
}

export function canClearDockContext(
  _contextType: CommandBarPageContext['entity_type'],
  hasClearAction: boolean,
) {
  return hasClearAction;
}

export function contextChipMaxWidth(canAddContext: boolean) {
  return canAddContext ? 'calc(100% - 116px)' : undefined;
}

export function usesSeparateComposerActionRow(mode: DockInputProps['mode'], expanded: boolean) {
  return mode === 'conversation' && expanded;
}

export function sendControlClassName(disabled: boolean) {
  return disabled
    ? 'cursor-not-allowed bg-muted text-muted-foreground'
    : 'bg-foreground text-background hover:bg-foreground/85';
}

export function DockInput({
  mode,
  value,
  onChange,
  onSubmit,
  onFocusChange,
  onAddContext,
  pageContext,
  contextOptions = [],
  activeContextKey,
  onContextKeyChange,
  onClearContext,
  workspaceId,
  references = [],
  onAddReference,
  onRemoveReference,
  mediaAttachments = [],
  onAddMedia,
  onRemoveMedia,
  busy,
  disabled,
  autoFocus,
  textareaRef,
  onStop,
  stopping,
  placeholder: placeholderOverride,
  showShortcutHint = true,
}: DockInputProps) {
  const localRef = useRef<HTMLTextAreaElement | null>(null);
  const ref = textareaRef ?? localRef;
  const [expandedComposer, setExpandedComposer] = useState(false);
  const referencePickerRef = useRef<DockReferencePickerHandle | null>(null);
  const mediaInputRef = useRef<HTMLInputElement | null>(null);
  const documentInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = 'auto';
    const style = window.getComputedStyle(el);
    const lineHeight = Number.parseFloat(style.lineHeight) || 20;
    const singleLineHeight = lineHeight
      + (Number.parseFloat(style.paddingTop) || 0)
      + (Number.parseFloat(style.paddingBottom) || 0);
    setExpandedComposer((current) => shouldUseExpandedComposerLayout({
      value,
      scrollHeight: el.scrollHeight,
      singleLineHeight,
      currentlyExpanded: current,
    }));
    el.style.height = `${composerTextareaHeight({
      value,
      scrollHeight: el.scrollHeight,
      singleLineHeight,
    })}px`;
  }, [value, ref]);

  useEffect(() => {
    if (autoFocus && !disabled) ref.current?.focus();
  }, [autoFocus, disabled, ref]);

  const placeholder = placeholderOverride ?? (
    mode === 'list'
      ? 'Search runs or ask something new…'
      : composerPlaceholderForContext(pageContext?.entity_type)
  );

  // Hide the workspace-level chip — it just restates the current workspace
  // (already visible in the sidebar) and provides no scoping signal. Keep it
  // for entity-scoped contexts (task / epic / doc / contact / deal) where the
  // chip tells the user "your input runs against this thing."
  const showChip = !!pageContext && (pageContext.entity_type !== 'workspace' || !!pageContext.metadata?.context_scope);
  const hasReadyAttachment = mediaAttachments.some((attachment) => attachment.status === 'ready');
  const sendDisabled = (!value.trim() && !hasReadyAttachment) || !!busy || !!disabled;

  const canAddReferences = !!workspaceId && !!onAddReference;
  const canAddContext = !!onAddContext || canAddReferences;
  const showContextRow = mode === 'conversation' && (showChip || !!onAddContext || canAddReferences || references.length > 0);
  const separateActionRow = usesSeparateComposerActionRow(mode, expandedComposer);
  const attachmentButton = mode === 'conversation' && onAddMedia ? (
    <>
      <input
        ref={mediaInputRef}
        type="file"
        accept="image/jpeg,image/png,image/gif,image/webp,video/mp4,video/quicktime,video/webm,video/mpeg"
        multiple
        className="hidden"
        onChange={(event) => {
          const files = Array.from(event.currentTarget.files ?? []);
          event.currentTarget.value = '';
          if (files.length > 0) onAddMedia(files);
        }}
      />
      <input
        ref={documentInputRef}
        type="file"
        accept="application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,text/plain,text/markdown,text/csv,application/json,.pdf,.docx,.txt,.md,.markdown,.csv,.json"
        multiple
        className="hidden"
        onChange={(event) => {
          const files = Array.from(event.currentTarget.files ?? []);
          event.currentTarget.value = '';
          if (files.length > 0) onAddMedia(files);
        }}
      />
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button type="button" title="Attach files" aria-label="Attach files" disabled={disabled || busy} className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-muted-foreground transition hover:bg-muted hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50">
            <AttachmentIcon className="h-3.5 w-3.5" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" side="top" className="w-52">
          <DropdownMenuItem onSelect={() => mediaInputRef.current?.click()} className="gap-2">
            <Image01Icon className="h-4 w-4" />
            <span className="flex flex-col">
              <span>Images &amp; videos</span>
              <span className="text-[10px] text-muted-foreground">PNG, JPG, GIF, WebP, MP4, MOV</span>
            </span>
          </DropdownMenuItem>
          <DropdownMenuItem onSelect={() => documentInputRef.current?.click()} className="gap-2">
            <File01Icon className="h-4 w-4" />
            <span className="flex flex-col">
              <span>Documents</span>
              <span className="text-[10px] text-muted-foreground">PDF, DOCX, TXT, Markdown, CSV, JSON</span>
            </span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  ) : null;
  const submitControl = onStop ? (
    <button
      type="button"
      onClick={onStop}
      disabled={stopping}
      title={stopping ? 'Stopping agent' : 'Stop agent'}
      aria-label={stopping ? 'Stopping agent' : 'Stop agent'}
      className={cn(
        'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition',
        stopping
          ? 'cursor-not-allowed bg-muted text-muted-foreground'
          : 'bg-foreground text-background hover:bg-foreground/85',
      )}
    >
      {stopping ? (
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
      ) : (
        <StopIcon className="h-3.5 w-3.5" />
      )}
    </button>
  ) : (
    <button
      type="button"
      onClick={onSubmit}
      disabled={sendDisabled}
      title="Send"
      className={cn(
        'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition',
        sendControlClassName(sendDisabled),
      )}
    >
      {busy ? (
        <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
      ) : (
        <ArrowUp01Icon className="h-3.5 w-3.5" />
      )}
    </button>
  );

  return (
    <div className="flex flex-col gap-2 px-3.5 pb-2.5 pt-2">
      {showContextRow ? (
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 flex-1 flex-wrap items-center gap-1.5">
            {showChip ? (
              <ContextChip
                context={pageContext!}
                options={contextOptions}
                activeKey={activeContextKey}
                onChange={onContextKeyChange}
                onClear={onClearContext}
                reserveSpaceForAddContext={canAddContext}
              />
            ) : null}
            {references.map((reference) => (
              <span
                key={`${reference.entity_type}:${reference.entity_id}`}
                className="inline-flex max-w-[260px] items-center gap-1 rounded-full border border-border/70 bg-muted/30 px-2 py-0.5 text-[11px]"
                title={reference.display_title}
                aria-label={`${TYPE_LABEL[reference.entity_type]}: ${reference.display_title}`}
              >
                <ContextIcon type={reference.entity_type} />
                <span className="truncate font-medium">{reference.display_title}</span>
                <button
                  type="button"
                  aria-label={`Remove ${reference.display_title} reference`}
                  onClick={() => onRemoveReference?.(reference)}
                  className="-mr-1 inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-background hover:text-foreground"
                >
                  <Cancel01Icon className="h-3 w-3" />
                </button>
              </span>
            ))}
            {canAddReferences ? (
              <DockReferencePicker
                ref={referencePickerRef}
                workspaceId={workspaceId!}
                selected={references}
                onSelect={onAddReference!}
              />
            ) : null}
            {onAddContext ? (
              <button
                type="button"
                onClick={onAddContext}
                className="inline-flex items-center gap-1 rounded-full border border-dashed border-border/70 px-2 py-0.5 text-[11px] text-muted-foreground transition hover:border-foreground/40 hover:text-foreground"
              >
                <PlusSignIcon className="h-3 w-3" />
                Add context
              </button>
            ) : null}
          </div>
          {showShortcutHint ? (
            <span className="shrink-0 text-[11px] text-muted-foreground">
              Press <kbd className="rounded border bg-muted px-1 py-0 font-mono text-[10px]">/</kbd> to open
            </span>
          ) : null}
        </div>
      ) : null}
      {mediaAttachments.length > 0 ? (
        <div className="flex flex-wrap gap-1.5">
          {mediaAttachments.map((attachment) => (
            <span key={attachment.local_id} className="inline-flex max-w-[220px] items-center gap-1 rounded-md border border-border/70 bg-muted/30 px-2 py-1 text-[11px]">
              {attachment.preview_url && attachment.file_type.startsWith('image/') ? (
                <img src={attachment.preview_url} alt="" className="h-5 w-5 rounded object-cover" />
              ) : (
                <AttachmentIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              )}
              <span className="truncate font-medium">{attachment.file_name}</span>
              {attachment.status === 'uploading' ? <Loading01Icon className="h-3 w-3 shrink-0 animate-spin text-muted-foreground" /> : null}
              {attachment.status === 'failed' ? <span className="text-destructive">Failed</span> : null}
              <button type="button" aria-label={`Remove ${attachment.file_name}`} onClick={() => onRemoveMedia?.(attachment)} className="-mr-1 inline-flex h-4 w-4 shrink-0 items-center justify-center rounded text-muted-foreground hover:bg-background hover:text-foreground">
                <Cancel01Icon className="h-3 w-3" />
              </button>
            </span>
          ))}
        </div>
      ) : null}
      <div className={cn(
        'flex rounded-xl border border-border/70 bg-background/80 px-2.5 py-1.5 transition focus-within:border-foreground/30',
        separateActionRow ? 'flex-col gap-0' : 'flex-wrap items-center gap-2',
      )}>
        {mode === 'list' ? (
          <Search01Icon className="mb-1.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        ) : null}
        {!separateActionRow ? attachmentButton : null}
        <textarea
          ref={ref}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onPaste={(event) => {
            if (!onAddMedia || busy || disabled) return;
            const imageFiles = getClipboardImageFiles(event.clipboardData);
            if (imageFiles.length > 0) {
              event.preventDefault();
              onAddMedia(imageFiles);
              return;
            }
            const pastedText = event.clipboardData.getData('text/plain');
            if (pastedText.length > 10_000) {
              event.preventDefault();
              onAddMedia([new File([pastedText], 'Pasted text.txt', { type: 'text/plain' })]);
            }
          }}
          onFocus={() => onFocusChange?.(true)}
          onBlur={() => onFocusChange?.(false)}
          onKeyDown={(e) => {
            if (e.key === '@' && canAddReferences) {
              e.preventDefault();
              referencePickerRef.current?.open();
              return;
            }
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              onSubmit();
            }
          }}
          rows={1}
          placeholder={placeholder}
          disabled={disabled}
          className={cn(
            'block min-w-0 flex-1 resize-none bg-transparent py-1 text-sm leading-5 placeholder:text-muted-foreground focus:outline-none disabled:opacity-60',
            separateActionRow && 'w-full flex-none',
          )}
        />
        {separateActionRow ? (
          <div className="flex w-full items-center justify-between pt-0.5">
            {attachmentButton}
            {submitControl}
          </div>
        ) : submitControl}
      </div>
    </div>
  );
}

function ContextChip({
  context,
  options,
  activeKey,
  onChange,
  onClear,
  reserveSpaceForAddContext = false,
}: {
  context: CommandBarPageContext;
  options: PageContextScopeOption[];
  activeKey?: string | null;
  onChange?: (key: string) => void;
  onClear?: () => void;
  reserveSpaceForAddContext?: boolean;
}) {
  const blockScoped = isBlockScopedDocument(context);
  const allTasks = isAllTasksContext(context);
  const title = contextTitle(context);
  const hasOptions = options.length > 1 && !!onChange;
  const canClear = canClearDockContext(context.entity_type, !!onClear);
  const chipClassName = cn(
    'inline-flex max-w-[300px] items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] text-foreground transition',
    blockScoped || allTasks ? 'border-orange-500/30 bg-orange-500/10' : 'border-border/70 bg-muted/30',
    hasOptions ? 'cursor-pointer hover:border-foreground/30 hover:bg-muted/50' : '',
  );
  const body = (
    <>
      <ContextIcon type={context.entity_type} />
      <span className="truncate font-medium">{context.display_title || context.entity_id}</span>
      {blockScoped ? (
        <span className="ml-0.5 inline-flex shrink-0 items-center gap-1 rounded-full bg-background/80 px-1.5 py-0.5 text-[10px] font-medium text-orange-700 dark:text-orange-300">
          <span className="h-1.5 w-1.5 rounded-full bg-orange-500" />
          Block selected
        </span>
      ) : null}
      {hasOptions ? <ArrowDown01Icon className="h-3 w-3 shrink-0 text-muted-foreground" /> : null}
    </>
  );
  const clearButton = canClear ? (
    <button
      type="button"
      aria-label="Remove document context"
      title="Remove document context"
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onClear?.();
      }}
      className="-mr-1 inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-muted-foreground transition hover:bg-background hover:text-foreground"
    >
      <Cancel01Icon className="h-3 w-3" />
    </button>
  ) : null;

  if (!hasOptions) {
    return (
      <span
        title={title}
        className={chipClassName}
        style={{ maxWidth: contextChipMaxWidth(reserveSpaceForAddContext) }}
      >
        {body}
        {clearButton}
      </span>
    );
  }

  const dropdown = (
    <DropdownMenu>
      <DropdownMenuTrigger
        type="button"
        title={title}
        className={chipClassName}
        style={{ maxWidth: contextChipMaxWidth(reserveSpaceForAddContext) }}
      >
        {body}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="z-[80] w-64">
        {options.map((option) => {
          const checked = option.key === activeKey;
          return (
            <DropdownMenuItem
              key={option.key}
              onClick={() => onChange?.(option.key)}
              className="cursor-pointer items-start gap-2"
            >
              <ContextIcon type={option.context.entity_type} className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium">{option.label}</span>
                {option.description ? (
                  <span className="block truncate text-xs text-muted-foreground">{option.description}</span>
                ) : null}
              </span>
              {checked ? <Tick01Icon className="mt-0.5 h-4 w-4 shrink-0 text-orange-500" /> : null}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );

  if (!clearButton) return dropdown;

  return (
    <span className="inline-flex min-w-0 items-center gap-1">
      {dropdown}
      {clearButton}
    </span>
  );
}
