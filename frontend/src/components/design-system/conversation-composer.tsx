import type { Editor } from '@tiptap/react';
import { Fragment, type HTMLAttributes, type ReactNode } from 'react';
import {
  ArrowReloadHorizontalIcon,
  ArrowUp01Icon,
  ArrowUpDownIcon,
  AttachmentIcon,
  Briefcase01Icon,
  CodeIcon,
  LeftToRightListBulletIcon,
  LeftToRightListNumberIcon,
  Link01Icon,
  Loading01Icon,
  QuoteDownIcon,
  SentIcon,
  SmileIcon,
  SparklesIcon,
  TextBoldIcon,
  TextItalicIcon,
  TextStrikethroughIcon,
  TextUnderlineIcon,
  TickDouble01Icon,
} from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

export type ConversationRewriteOperation =
  | 'expand'
  | 'rephrase'
  | 'fix_grammar'
  | 'more_friendly'
  | 'more_formal';

const rewriteTools: Array<{
  operation: ConversationRewriteOperation;
  label: string;
  icon: typeof ArrowUpDownIcon;
}> = [
  { operation: 'expand', label: 'Expand', icon: ArrowUpDownIcon },
  { operation: 'rephrase', label: 'Rephrase', icon: ArrowReloadHorizontalIcon },
  { operation: 'fix_grammar', label: 'Fix grammar', icon: TickDouble01Icon },
  { operation: 'more_friendly', label: 'More friendly', icon: SmileIcon },
  { operation: 'more_formal', label: 'More formal', icon: Briefcase01Icon },
];

export function QuietConversationComposer({
  children,
  focused = false,
  tone = 'default',
  className,
}: {
  children: ReactNode;
  focused?: boolean;
  tone?: 'default' | 'note';
  className?: string;
}) {
  return (
    <div
      data-quiet-conversation-composer=""
      data-tone={tone}
      className={cn(
        'relative rounded-xl border border-border/40 bg-card transition-colors',
        focused && (tone === 'note' ? 'border-amber-400 dark:border-amber-500' : 'border-blue-500 dark:border-blue-400'),
        tone === 'note' && 'bg-amber-50/50 dark:bg-amber-950/10',
        className,
      )}
    >
      {children}
    </div>
  );
}

export function QuietComposerEditorSurface({ children, className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('px-4 py-3', className)} {...props}>{children}</div>;
}

export function QuietComposerFormatButton({
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
          onClick={(event) => {
            event.preventDefault();
            onClick();
          }}
          onMouseDown={(event) => event.preventDefault()}
          className={cn(
            'inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md transition-colors',
            active ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
          )}
        >
          {children}
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="text-xs">{title}</TooltipContent>
    </Tooltip>
  );
}

export function QuietComposerAITools({
  disabled = false,
  pending = false,
  onSelect,
}: {
  disabled?: boolean;
  pending?: boolean;
  onSelect: (operation: ConversationRewriteOperation) => void | Promise<void>;
}) {
  const available = !disabled && !pending;
  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="inline-flex">
            <DropdownMenuTrigger asChild>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                disabled={!available}
                className={cn(
                  'h-auto rounded-full px-3 py-1 text-xs font-medium transition-colors',
                  available ? 'text-muted-foreground hover:bg-muted hover:text-foreground' : 'text-muted-foreground/50',
                )}
              >
                {pending ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <SparklesIcon className="h-3 w-3" />}
                AI Tools
                <ArrowUp01Icon className="h-3 w-3 rotate-180" />
              </Button>
            </DropdownMenuTrigger>
          </span>
        </TooltipTrigger>
        {!available && !pending ? (
          <TooltipContent side="top" className="text-xs">Write something first to use AI tools</TooltipContent>
        ) : null}
      </Tooltip>
      <DropdownMenuContent align="start" className="w-48">
        {rewriteTools.map((tool, index) => {
          const Icon = tool.icon;
          return (
            <Fragment key={tool.operation}>
              {index === 3 ? <DropdownMenuSeparator /> : null}
              <DropdownMenuItem onSelect={() => void onSelect(tool.operation)}>
                <Icon className="h-4 w-4" />
                <span>{tool.label}</span>
              </DropdownMenuItem>
            </Fragment>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function QuietComposerToolbar({
  editor,
  emoji,
  onAttach,
  attachLabel = 'Attach file',
  onLink,
  leading,
  trailing,
  onSubmit,
  submitLabel = 'Send',
  submitDisabled = false,
  submitting = false,
  showShortcut = true,
  tone = 'default',
  className,
}: {
  editor: Editor;
  emoji?: ReactNode;
  onAttach?: () => void;
  attachLabel?: string;
  onLink?: () => void;
  leading?: ReactNode;
  trailing?: ReactNode;
  onSubmit?: () => void;
  submitLabel?: string;
  submitDisabled?: boolean;
  submitting?: boolean;
  showShortcut?: boolean;
  tone?: 'default' | 'note';
  className?: string;
}) {
  const formats = [
    { title: 'Bold (Ctrl+B)', active: editor.isActive('bold'), action: () => editor.chain().focus().toggleBold().run(), icon: TextBoldIcon },
    { title: 'Italic (Ctrl+I)', active: editor.isActive('italic'), action: () => editor.chain().focus().toggleItalic().run(), icon: TextItalicIcon },
    { title: 'Underline (Ctrl+U)', active: editor.isActive('underline'), action: () => editor.chain().focus().toggleUnderline().run(), icon: TextUnderlineIcon },
    { title: 'Insert link', active: editor.isActive('link'), action: onLink, icon: Link01Icon },
    { title: 'Bullet list', active: editor.isActive('bulletList'), action: () => editor.chain().focus().toggleBulletList().run(), icon: LeftToRightListBulletIcon },
    { title: 'Numbered list', active: editor.isActive('orderedList'), action: () => editor.chain().focus().toggleOrderedList().run(), icon: LeftToRightListNumberIcon },
    { title: 'Quote', active: editor.isActive('blockquote'), action: () => editor.chain().focus().toggleBlockquote().run(), icon: QuoteDownIcon },
    { title: 'Inline code', active: editor.isActive('code'), action: () => editor.chain().focus().toggleCode().run(), icon: CodeIcon },
    { title: 'Strikethrough', active: editor.isActive('strike'), action: () => editor.chain().focus().toggleStrike().run(), icon: TextStrikethroughIcon },
  ];

  return (
    <div className={cn('flex min-w-0 items-center justify-between gap-2 px-4 pb-3', className)}>
      <div className="flex min-w-0 items-center gap-0.5 overflow-x-auto [scrollbar-width:none]">
        {emoji}
        {onAttach ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <Button type="button" variant="ghost" size="sm" className="h-7 w-7 shrink-0 p-0 text-muted-foreground/60 hover:text-foreground" onClick={onAttach} aria-label={attachLabel}>
                <AttachmentIcon className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent side="top">{attachLabel}</TooltipContent>
          </Tooltip>
        ) : null}
        {emoji || onAttach ? <div className="mx-0.5 h-4 w-px shrink-0 bg-border/40" /> : null}
        {formats.map(({ title, active, action, icon: Icon }, index) => action ? (
          <span key={title} className="contents">
            {(index === 4 || index === 7) ? <span className="mx-0.5 h-4 w-px shrink-0 bg-border/40" /> : null}
            <QuietComposerFormatButton title={title} active={active} onClick={action}>
              <Icon className="h-3.5 w-3.5" />
            </QuietComposerFormatButton>
          </span>
        ) : null)}
        {leading}
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {trailing}
        {showShortcut ? (
          <kbd className="hidden items-center gap-1 font-mono text-[15px] leading-none text-muted-foreground sm:inline-flex">
            <span>{typeof navigator !== 'undefined' && navigator.platform?.includes('Mac') ? '⌘' : 'Ctrl'}</span>
            <span>↵</span>
          </kbd>
        ) : null}
        {onSubmit ? (
          <Button
            type="button"
            size="sm"
            disabled={submitDisabled || submitting}
            onClick={onSubmit}
            className={cn('h-7 gap-1.5 rounded-full px-3 text-xs', tone === 'note' && 'bg-amber-500 text-white hover:bg-amber-600')}
          >
            {submitting ? <Loading01Icon className="h-3 w-3 animate-spin" /> : <SentIcon className="h-3 w-3" />}
            {submitLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
