import { useEffect, useRef } from 'react';
import {
  ArrowDown01Icon,
  ArrowUp01Icon,
  BookOpen01Icon,
  Briefcase01Icon,
  File01Icon,
  FolderKanbanIcon,
  Loading01Icon,
  PlusSignIcon,
  RecordIcon,
  Search01Icon,
  StopIcon,
  Tick01Icon,
  Cancel01Icon,
  UserIcon,
} from '@/lib/icons';
import type { CommandBarPageContext } from '@/lib/pmTypes';
import type { DockEntityReference } from '@/lib/dockTypes';
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
  workspace: 'Workspace',
  repository: 'Repository',
};

function ContextIcon({ type, className = 'h-3 w-3 shrink-0' }: { type: CommandBarPageContext['entity_type']; className?: string }) {
  switch (type) {
    case 'task':
      return <RecordIcon className={className} />;
    case 'epic':
      return <BookOpen01Icon className={className} />;
    case 'document':
      return <File01Icon className={className} />;
    case 'crm_contact':
      return <UserIcon className={className} />;
    case 'crm_deal':
      return <Briefcase01Icon className={className} />;
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

function contextScopeLabel(context: CommandBarPageContext) {
  if (isBlockScopedDocument(context)) return 'Block';
  if (isAllTasksContext(context)) return 'All tasks';
  return TYPE_LABEL[context.entity_type] ?? context.entity_type;
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
  busy?: boolean;
  disabled?: boolean;
  autoFocus?: boolean;
  textareaRef?: React.RefObject<HTMLTextAreaElement | null>;
  /** When set, the send button becomes a stop button for the active run. */
  onStop?: () => void;
  stopping?: boolean;
  placeholder?: string;
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
  busy,
  disabled,
  autoFocus,
  textareaRef,
  onStop,
  stopping,
  placeholder: placeholderOverride,
}: DockInputProps) {
  const localRef = useRef<HTMLTextAreaElement | null>(null);
  const ref = textareaRef ?? localRef;
  const referencePickerRef = useRef<DockReferencePickerHandle | null>(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
  }, [value, ref]);

  useEffect(() => {
    if (autoFocus && !disabled) ref.current?.focus();
  }, [autoFocus, disabled, ref]);

  const placeholder = placeholderOverride ?? (
    mode === 'list'
      ? 'Search runs or ask something new…'
      : 'Tell Atlas, Forge, Lens, or any agent what to do'
  );

  // Hide the workspace-level chip — it just restates the current workspace
  // (already visible in the sidebar) and provides no scoping signal. Keep it
  // for entity-scoped contexts (task / epic / doc / contact / deal) where the
  // chip tells the user "your input runs against this thing."
  const showChip = !!pageContext && (pageContext.entity_type !== 'workspace' || !!pageContext.metadata?.context_scope);
  const sendDisabled = !value.trim() || busy || disabled;

  const canAddReferences = !!workspaceId && !!onAddReference;
  const showContextRow = mode === 'conversation' && (showChip || !!onAddContext || canAddReferences || references.length > 0);

  return (
    <div className="flex flex-col gap-2 px-3.5 pb-2.5 pt-2">
      {showContextRow ? (
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            {showChip ? (
              <ContextChip
                context={pageContext!}
                options={contextOptions}
                activeKey={activeContextKey}
                onChange={onContextKeyChange}
                onClear={onClearContext}
              />
            ) : null}
            {references.map((reference) => (
              <span
                key={`${reference.entity_type}:${reference.entity_id}`}
                className="inline-flex max-w-[260px] items-center gap-1 rounded-full border border-border/70 bg-muted/30 px-2 py-0.5 text-[11px]"
                title={reference.display_title}
              >
                <span className="text-[10px] font-medium uppercase text-muted-foreground">
                  {TYPE_LABEL[reference.entity_type]}
                </span>
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
          <span className="shrink-0 text-[11px] text-muted-foreground">
            Press <kbd className="rounded border bg-muted px-1 py-0 font-mono text-[10px]">/</kbd> to open
          </span>
        </div>
      ) : null}
      <div className="flex items-end gap-2 rounded-xl border border-border/70 bg-background/80 px-2.5 py-1.5 transition focus-within:border-foreground/30">
        {mode === 'list' ? (
          <Search01Icon className="mb-1.5 h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        ) : null}
        <textarea
          ref={ref}
          value={value}
          onChange={(e) => onChange(e.target.value)}
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
          className="block w-full flex-1 resize-none bg-transparent py-1 text-sm leading-5 placeholder:text-muted-foreground focus:outline-none disabled:opacity-60"
        />
        {onStop ? (
          <button
            type="button"
            onClick={onStop}
            disabled={stopping}
            title="Stop agent"
            aria-label="Stop agent"
            className={cn(
              'mb-0.5 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition',
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
              'mb-0.5 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full transition',
              sendDisabled
                ? 'cursor-not-allowed bg-muted text-muted-foreground'
                : 'bg-orange-500 text-white hover:bg-orange-500/90',
            )}
          >
            {busy ? (
              <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <ArrowUp01Icon className="h-3.5 w-3.5" />
            )}
          </button>
        )}
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
}: {
  context: CommandBarPageContext;
  options: PageContextScopeOption[];
  activeKey?: string | null;
  onChange?: (key: string) => void;
  onClear?: () => void;
}) {
  const blockScoped = isBlockScopedDocument(context);
  const allTasks = isAllTasksContext(context);
  const title = contextTitle(context);
  const label = contextScopeLabel(context);
  const hasOptions = options.length > 1 && !!onChange;
  const canClear = context.entity_type === 'document' && !!onClear;
  const chipClassName = cn(
    'inline-flex max-w-[300px] items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] text-foreground transition',
    blockScoped || allTasks ? 'border-orange-500/30 bg-orange-500/10' : 'border-border/70 bg-muted/30',
    hasOptions ? 'cursor-pointer hover:border-foreground/30 hover:bg-muted/50' : '',
  );
  const body = (
    <>
      <ContextIcon type={context.entity_type} />
      <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </span>
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
