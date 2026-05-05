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
  Tick01Icon,
  UserIcon,
} from '@/lib/icons';
import type { CommandBarPageContext } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import type { PageContextScopeOption } from '@/components/command-bar/pageContext';

const TYPE_LABEL: Record<CommandBarPageContext['entity_type'], string> = {
  task: 'Task',
  epic: 'Epic',
  document: 'Doc',
  crm_contact: 'Contact',
  crm_deal: 'Deal',
  workspace: 'Workspace',
};

function chipIcon(type: CommandBarPageContext['entity_type']) {
  switch (type) {
    case 'task':
      return RecordIcon;
    case 'epic':
      return BookOpen01Icon;
    case 'document':
      return File01Icon;
    case 'crm_contact':
      return UserIcon;
    case 'crm_deal':
      return Briefcase01Icon;
    default:
      return FolderKanbanIcon;
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
  busy?: boolean;
  disabled?: boolean;
  autoFocus?: boolean;
  textareaRef?: React.RefObject<HTMLTextAreaElement | null>;
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
  busy,
  disabled,
  autoFocus,
  textareaRef,
}: DockInputProps) {
  const localRef = useRef<HTMLTextAreaElement | null>(null);
  const ref = textareaRef ?? localRef;

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
  }, [value, ref]);

  useEffect(() => {
    if (autoFocus) ref.current?.focus();
  }, [autoFocus, ref]);

  const placeholder =
    mode === 'list'
      ? 'Search runs or ask something new…'
      : 'Ask, or type / to run an agent or pipeline';

  // Hide the workspace-level chip — it just restates the current workspace
  // (already visible in the sidebar) and provides no scoping signal. Keep it
  // for entity-scoped contexts (task / epic / doc / contact / deal) where the
  // chip tells the user "your input runs against this thing."
  const showChip = !!pageContext && (pageContext.entity_type !== 'workspace' || !!pageContext.metadata?.context_scope);
  const sendDisabled = !value.trim() || busy || disabled;

  const showContextRow = mode === 'conversation' && (showChip || !!onAddContext);

  return (
    <div className="flex flex-col gap-2 px-3.5 pb-2.5 pt-2">
      {showContextRow ? (
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-1.5">
            {showChip ? (
              <ContextChip
                context={pageContext!}
                options={contextOptions}
                activeKey={activeContextKey}
                onChange={onContextKeyChange}
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
            <kbd className="rounded border bg-muted px-1 py-0 font-mono text-[10px]">/</kbd> for agents
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
      </div>
    </div>
  );
}

function ContextChip({
  context,
  options,
  activeKey,
  onChange,
}: {
  context: CommandBarPageContext;
  options: PageContextScopeOption[];
  activeKey?: string | null;
  onChange?: (key: string) => void;
}) {
  const Icon = chipIcon(context.entity_type);
  const blockScoped = isBlockScopedDocument(context);
  const allTasks = isAllTasksContext(context);
  const title = contextTitle(context);
  const label = contextScopeLabel(context);
  const hasOptions = options.length > 1 && !!onChange;
  const chipClassName = cn(
    'inline-flex max-w-[300px] items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] text-foreground transition',
    blockScoped || allTasks ? 'border-orange-500/30 bg-orange-500/10' : 'border-border/70 bg-muted/30',
    hasOptions ? 'cursor-pointer hover:border-foreground/30 hover:bg-muted/50' : '',
  );
  const body = (
    <>
      <Icon className="h-3 w-3 shrink-0" />
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

  if (!hasOptions) {
    return (
      <span
        title={title}
        className={chipClassName}
      >
        {body}
      </span>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        type="button"
        title={title}
        className={chipClassName}
      >
        {body}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        {options.map((option) => {
          const OptionIcon = chipIcon(option.context.entity_type);
          const checked = option.key === activeKey;
          return (
            <DropdownMenuItem
              key={option.key}
              onClick={() => onChange?.(option.key)}
              className="cursor-pointer items-start gap-2"
            >
              <OptionIcon className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
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
}
