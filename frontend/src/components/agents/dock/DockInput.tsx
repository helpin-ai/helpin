import { useEffect, useRef } from 'react';
import {
  ArrowUp01Icon,
  BookOpen01Icon,
  Briefcase01Icon,
  File01Icon,
  FolderKanbanIcon,
  Loading01Icon,
  PlusSignIcon,
  RecordIcon,
  Search01Icon,
  UserIcon,
} from '@/lib/icons';
import type { CommandBarPageContext } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

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

export interface DockInputProps {
  mode: 'conversation' | 'list';
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  onFocusChange?: (focused: boolean) => void;
  onAddContext?: () => void;
  pageContext: CommandBarPageContext | null;
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

  const showChip = !!pageContext;
  const sendDisabled = !value.trim() || busy || disabled;

  return (
    <div className="flex flex-col gap-2 px-3.5 pb-2.5 pt-2">
      {mode === 'conversation' ? (
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-1.5">
            {showChip ? <ContextChip context={pageContext!} /> : null}
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

function ContextChip({ context }: { context: CommandBarPageContext }) {
  const Icon = chipIcon(context.entity_type);
  return (
    <span
      title={context.display_title || context.entity_id}
      className="inline-flex max-w-[260px] items-center gap-1.5 rounded-full border border-border/70 bg-muted/30 px-2 py-0.5 text-[11px] text-foreground"
    >
      <Icon className="h-3 w-3 shrink-0" />
      <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {TYPE_LABEL[context.entity_type] ?? context.entity_type}
      </span>
      <span className="truncate font-medium">{context.display_title || context.entity_id}</span>
    </span>
  );
}
