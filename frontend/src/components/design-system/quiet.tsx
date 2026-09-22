import type { ElementType, ReactNode } from 'react';

import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { DialogContent } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { ArrowLeft02Icon, ArrowRight01Icon, Search01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

export const workspaceSidebarSafeInsetClassName =
  'group-data-[sidebar-toggle-visible=true]/workspace-main:pl-14';

export const quietRelatedItemTitleClassName =
  'text-[12.5px] font-medium text-foreground/90';

export const quietRelationshipResultRowClassName = 'min-w-0 overflow-hidden';

export function QuietRelationshipDialogContent({ className, ...props }: React.ComponentProps<typeof DialogContent>) {
  return (
    <DialogContent
      className={cn('min-w-0 overflow-hidden sm:max-w-xl lg:max-w-2xl xl:max-w-3xl [&>*]:min-w-0', className)}
      {...props}
    />
  );
}

export function QuietRelationshipResults({ className, ...props }: React.ComponentProps<'div'>) {
  return <div className={cn('min-w-0 overflow-x-hidden', className)} {...props} />;
}

export function QuietPageViewport({ children, className, contentClassName }: { children: ReactNode; className?: string; contentClassName?: string }) {
  return (
    <div className={cn('h-full overflow-auto p-4 pb-20 [scrollbar-gutter:stable] md:p-6 md:pb-24', className)}>
      <div className={cn('mx-auto w-full max-w-7xl', contentClassName)}>{children}</div>
    </div>
  );
}

export function QuietPageHeader({
  title,
  context,
  navigation,
  description,
  actions,
  variant = 'content',
  className,
}: {
  title: ReactNode;
  context?: ReactNode;
  navigation?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  variant?: 'content' | 'shell';
  className?: string;
}) {
  return (
    <header className={cn(workspaceSidebarSafeInsetClassName, variant === 'shell' ? 'border-b border-quiet-divider-strong px-4 py-4 sm:px-6 lg:px-8' : '', className)}>
      {navigation ? <div className="mb-2 min-w-0">{navigation}</div> : null}
      <div className={cn('flex justify-between gap-3 sm:gap-6', variant === 'shell' ? 'items-center' : 'items-start')}>
        <div className="min-w-0">
          <h1 className="text-[20px] font-semibold leading-[1.18] tracking-[-0.018em] text-quiet-text-primary">
            {title}
            {context ? <span className="font-normal text-quiet-text-tertiary"> ({context})</span> : null}
          </h1>
          {description ? <p className="mt-1.5 max-w-[760px] text-sm leading-[1.6] text-quiet-text-tertiary [text-wrap:pretty]">{description}</p> : null}
        </div>
        {actions ? <div className="flex shrink-0 flex-wrap items-center justify-end gap-3">{actions}</div> : null}
      </div>
    </header>
  );
}

export function QuietIdentityHeader({
  leadingAction,
  avatar,
  title,
  meta,
  status,
  actions,
  className,
}: {
  leadingAction?: ReactNode;
  avatar?: ReactNode;
  title: ReactNode;
  meta?: ReactNode;
  status?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <section className={cn('border-b border-quiet-divider-strong px-4 pb-4 pt-6 sm:px-6 lg:px-8', workspaceSidebarSafeInsetClassName, className)}>
      <div className="flex items-start gap-3.5">
        {leadingAction ? <div className="mt-1.5 shrink-0">{leadingAction}</div> : null}
        {avatar ? <div className="shrink-0">{avatar}</div> : null}
        <div className="min-w-0 flex-1">
          <div className="min-w-0 text-[24px] font-semibold leading-[1.18] tracking-[-0.018em] text-quiet-text-primary">{title}</div>
          {meta || status ? (
            <div className="mt-1.5 flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-1 text-[12.5px] text-quiet-text-tertiary">
              {meta}
              {status}
            </div>
          ) : null}
        </div>
        {actions ? <div className="flex shrink-0 items-center gap-3">{actions}</div> : null}
      </div>
    </section>
  );
}

export interface QuietBreadcrumbItem {
  id: string;
  label: ReactNode;
  icon?: ReactNode;
  onClick?: () => void;
}

export function QuietBreadcrumbs({
  items,
  onBack,
  backLabel = 'Go back',
  className,
}: {
  items: QuietBreadcrumbItem[];
  onBack?: () => void;
  backLabel?: string;
  className?: string;
}) {
  return (
    <nav aria-label="Breadcrumb" className={cn('flex min-w-0 items-center gap-1 overflow-hidden text-[12.5px] text-quiet-text-tertiary', className)}>
      {onBack ? (
        <QuietIconAction className="mr-1 shrink-0" onClick={onBack} aria-label={backLabel} title={backLabel}>
          <ArrowLeft02Icon className="h-[15px] w-[15px]" />
        </QuietIconAction>
      ) : null}
      {items.map((item, index) => (
        <span key={item.id} className="flex min-w-0 items-center gap-1">
          {index > 0 ? <ArrowRight01Icon aria-hidden="true" className="h-3 w-3 shrink-0 text-quiet-muted" /> : null}
          {item.onClick ? (
            <button
              type="button"
              onClick={item.onClick}
              className="inline-flex min-w-0 items-center gap-1 border-b border-transparent font-medium transition-colors hover:border-quiet-field hover:text-quiet-text-primary focus-visible:border-b-2 focus-visible:border-quiet-text-primary focus-visible:outline-none"
            >
              {item.icon ? <span className="shrink-0">{item.icon}</span> : null}
              <span className="truncate">{item.label}</span>
            </button>
          ) : (
            <span className="inline-flex min-w-0 items-center gap-1 font-medium text-quiet-text-secondary">
              {item.icon ? <span className="shrink-0">{item.icon}</span> : null}
              <span className="truncate">{item.label}</span>
            </span>
          )}
        </span>
      ))}
    </nav>
  );
}

export function QuietDetailHeader({
  breadcrumbs,
  avatar,
  title,
  meta,
  status,
  state,
  actions,
  allowTitleWrap = false,
  className,
}: {
  breadcrumbs?: ReactNode;
  avatar?: ReactNode;
  title: ReactNode;
  meta?: ReactNode;
  status?: ReactNode;
  state?: ReactNode;
  actions?: ReactNode;
  allowTitleWrap?: boolean;
  className?: string;
}) {
  return (
    <header className={cn('shrink-0 border-b border-quiet-divider-strong px-4 pb-2 pt-2 sm:px-6 lg:px-8', workspaceSidebarSafeInsetClassName, breadcrumbs && 'md:grid md:grid-cols-[minmax(0,1fr)_auto] md:gap-x-5', className)}>
      {breadcrumbs ? <div className="mb-0.5 min-w-0 md:col-start-1 md:row-start-1">{breadcrumbs}</div> : null}
      <div className={cn("grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-start gap-x-2 sm:gap-x-5", breadcrumbs && "md:contents")}>
        <div className={cn("flex min-w-0 items-start gap-2.5 sm:gap-3", breadcrumbs && "md:col-start-1 md:row-start-2")}>
          {avatar ? (
            <div className="h-8 w-8 shrink-0 sm:h-9 sm:w-9 [&>*]:h-full [&>*]:w-full">
              {avatar}
            </div>
          ) : null}
          <div className="min-w-0 flex-1 overflow-hidden">
            <div className={cn(
              'min-w-0 text-[20px] font-semibold leading-[1.18] tracking-[-0.018em] text-quiet-text-primary [&>*]:max-w-full',
              allowTitleWrap ? 'break-words' : 'truncate',
            )}>
              {title}
            </div>
            {meta || status ? (
              <div className="mt-0.5 flex min-w-0 items-center gap-x-3 overflow-hidden whitespace-nowrap">
                {meta ? <div className="min-w-0 overflow-hidden [&>*]:flex-nowrap">{meta}</div> : null}
                {status ? <div className="flex shrink-0 flex-nowrap items-center gap-x-3 whitespace-nowrap">{status}</div> : null}
              </div>
            ) : null}
          </div>
        </div>
        {state || actions ? (
          <div className={cn("flex min-w-0 shrink-0 flex-col items-end gap-0.5 overflow-hidden", breadcrumbs && "md:col-start-2 md:row-start-1 md:row-span-2 md:self-center")}>
            {actions ? <div className="flex min-w-0 flex-nowrap items-center justify-end gap-1.5 sm:gap-3">{actions}</div> : null}
            {state ? <div className="flex min-w-0 max-w-28 flex-nowrap items-center justify-end gap-x-3 overflow-hidden whitespace-nowrap sm:max-w-none">{state}</div> : null}
          </div>
        ) : null}
      </div>
    </header>
  );
}

export function QuietDetailLayout({ main, rail, className, railClassName }: { main: ReactNode; rail?: ReactNode; className?: string; railClassName?: string }) {
  return (
    <div className={cn('relative grid min-h-0 min-w-0 flex-1 grid-cols-1 overflow-hidden lg:grid-cols-[minmax(0,1fr)_300px]', className)}>
      <div className="min-h-0 min-w-0 overflow-hidden">{main}</div>
      {rail ? <QuietDetailRail className={railClassName}>{rail}</QuietDetailRail> : null}
    </div>
  );
}

export function QuietDetailRail({ children, className }: { children: ReactNode; className?: string }) {
  return <aside className={cn('min-h-0 border-t border-quiet-divider-strong lg:overflow-y-auto lg:border-l lg:border-t-0', className)}>{children}</aside>;
}

export function QuietSectionHeader({ title, icon: Icon, count, action, className }: { title: ReactNode; icon?: ElementType; count?: ReactNode; action?: ReactNode; className?: string }) {
  return (
    <div className={cn('flex min-h-8 items-center gap-2', className)}>
      {Icon ? <Icon className="h-[15px] w-[15px] shrink-0 text-quiet-muted" /> : null}
      <div className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">{title}</div>
      {count !== undefined ? <span className="text-[11.5px] tabular-nums text-quiet-muted">{count}</span> : null}
      {action ? <div className="ml-auto">{action}</div> : null}
    </div>
  );
}

export function QuietSection({
  title,
  icon,
  count,
  action,
  children,
  className,
  bodyClassName,
}: {
  title?: ReactNode;
  icon?: ElementType;
  count?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <section className={cn('border-b border-quiet-divider-strong px-4 py-5 sm:px-6 lg:px-8', className)}>
      {title ? <QuietSectionHeader title={title} icon={icon} count={count} action={action} className="mb-2" /> : null}
      <div className={bodyClassName}>{children}</div>
    </section>
  );
}

export type QuietMetricTone = 'neutral' | 'positive' | 'warning' | 'danger';

const quietMetricToneClassName: Record<QuietMetricTone, string> = {
  neutral: 'text-quiet-text-primary',
  positive: 'text-quiet-positive',
  warning: 'text-quiet-accent',
  danger: 'text-quiet-accent',
};

export function QuietMetricGrid({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'grid border-y border-quiet-divider-strong divide-y divide-quiet-divider-light md:grid-cols-2 md:divide-x md:divide-y-0 xl:grid-cols-4',
        className,
      )}
    >
      {children}
    </div>
  );
}

export function QuietMetricBlock({
  label,
  value,
  description,
  tone = 'neutral',
  trailing,
  onClick,
  className,
}: {
  label: ReactNode;
  value: ReactNode;
  description: ReactNode;
  tone?: QuietMetricTone;
  trailing?: ReactNode;
  onClick?: () => void;
  className?: string;
}) {
  const Comp = onClick ? 'button' : 'div';
  return (
    <Comp
      type={onClick ? 'button' : undefined}
      onClick={onClick}
      className={cn(
        'flex min-w-0 items-end justify-between gap-4 p-4 text-left transition-colors',
        onClick && 'hover:bg-quiet-row-hover focus-visible:bg-quiet-row-hover focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-quiet-text-primary',
        className,
      )}
    >
      <span className="min-w-0 space-y-1">
        <span className="block text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">{label}</span>
        <span className={cn('block text-[20px] font-semibold tracking-[-0.018em]', quietMetricToneClassName[tone])}>{value}</span>
        <span className="block text-[12px] text-quiet-text-tertiary">{description}</span>
      </span>
      {trailing ? <span className="shrink-0">{trailing}</span> : null}
    </Comp>
  );
}

const quietFocus = 'focus-visible:outline-none focus-visible:ring-0';

export function QuietTextAction({ className, ...props }: React.ComponentProps<typeof Button>) {
  return (
    <Button
      variant="ghost"
      size="sm"
      className={cn(
        'h-auto rounded-none border-0 bg-transparent p-0 text-[12.5px] font-normal text-quiet-text-secondary hover:bg-transparent hover:text-quiet-text-primary focus-visible:bg-transparent focus-visible:text-quiet-text-primary',
        quietFocus,
        className,
      )}
      {...props}
    />
  );
}

export function QuietIconAction({ className, ...props }: React.ComponentProps<typeof Button>) {
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      className={cn(
        'size-7 rounded-[6px] border-0 bg-transparent p-1.5 text-quiet-text-tertiary hover:bg-quiet-hover hover:text-quiet-text-primary focus-visible:bg-quiet-hover focus-visible:text-quiet-text-primary',
        quietFocus,
        className,
      )}
      {...props}
    />
  );
}

export function QuietPrimaryAction({ className, ...props }: React.ComponentProps<typeof Button>) {
  return (
    <Button
      size="sm"
      className={cn('border-0 bg-quiet-action text-quiet-action-ink hover:bg-quiet-action-hover focus-visible:bg-quiet-action-hover focus-visible:ring-0', quietFocus, className)}
      {...props}
    />
  );
}

type QuietDetailActionTone = 'primary' | 'secondary' | 'danger';

export function QuietDetailAction({
  icon,
  label,
  tone = 'secondary',
  iconOnly = false,
  href,
  target,
  rel,
  className,
  ...props
}: Omit<React.ComponentProps<typeof Button>, 'asChild' | 'children' | 'size' | 'variant'> & {
  icon: ReactNode;
  label: string;
  tone?: QuietDetailActionTone;
  iconOnly?: boolean;
  href?: string;
  target?: string;
  rel?: string;
}) {
  const actionClassName = cn(
    'size-8 rounded-full border-0 p-0 focus-visible:border-quiet-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-quiet-text-primary/20',
    tone === 'primary'
      ? cn('bg-quiet-action text-quiet-action-ink hover:bg-quiet-action-hover', !iconOnly && 'sm:w-auto sm:rounded-4xl sm:px-3')
      : cn(
          'bg-transparent text-quiet-text-secondary hover:bg-quiet-hover hover:text-quiet-text-primary',
          !iconOnly && 'sm:h-auto sm:w-auto sm:rounded-none sm:p-0 sm:hover:bg-transparent',
        ),
    tone === 'danger' && 'text-quiet-text-tertiary hover:text-quiet-accent sm:hover:text-quiet-accent',
    className,
  );
  const content = (
    <>
      {icon}
      <span className={iconOnly ? 'sr-only' : 'max-sm:sr-only'}>{label}</span>
    </>
  );
  const action = href ? (
    <Button
      asChild
      size="sm"
      variant={tone === 'primary' ? 'default' : 'ghost'}
      className={actionClassName}
      aria-label={label}
      {...props}
    >
      <a href={href} target={target} rel={rel}>{content}</a>
    </Button>
  ) : (
    <Button
      type="button"
      size="sm"
      variant={tone === 'primary' ? 'default' : 'ghost'}
      className={actionClassName}
      aria-label={label}
      {...props}
    >
      {content}
    </Button>
  );

  return <QuickTooltip label={label}>{action}</QuickTooltip>;
}

export const quietUnderlineControlClassName =
  'h-auto rounded-none border-0 border-b border-quiet-field bg-transparent px-0.5 py-1.5 text-sm text-quiet-text-primary shadow-none outline-none transition-colors hover:border-quiet-text-primary focus-visible:border-b-2 focus-visible:border-quiet-text-primary focus-visible:ring-0';

export function QuietUnderlineInput({ className, ...props }: React.ComponentProps<typeof Input>) {
  return <Input variant="plain" className={cn(quietUnderlineControlClassName, className)} {...props} />;
}

export function QuietUnderlineTextarea({ className, ...props }: React.ComponentProps<typeof Textarea>) {
  return <Textarea className={cn(quietUnderlineControlClassName, className)} {...props} />;
}

export function QuietSearchInput({
  className,
  containerClassName,
  trailing,
  type = 'search',
  ...props
}: React.ComponentProps<typeof Input> & { containerClassName?: string; trailing?: ReactNode }) {
  return (
    <div className={cn('relative', containerClassName)}>
      <Search01Icon className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground/70" />
      <Input
        type={type}
        className={cn(
          'h-9 w-full border-border/70 bg-muted/30 pl-9 text-sm placeholder:text-muted-foreground/70 focus-visible:bg-background',
          trailing && 'pr-10',
          className,
        )}
        {...props}
      />
      {trailing ? <div className="absolute right-1.5 top-1/2 -translate-y-1/2">{trailing}</div> : null}
    </div>
  );
}

export function QuietTitleInput({
  className,
  presentation = 'entity',
  ...props
}: React.ComponentProps<typeof Input> & { presentation?: 'entity' | 'header' }) {
  return (
    <Input
      variant="plain"
      className={cn(
        quietUnderlineControlClassName,
        'w-full truncate font-semibold',
        presentation === 'header'
          ? 'pb-0.5 text-[20px] leading-[1.18] tracking-[-0.018em] md:text-[20px]'
          : 'pb-2 text-[26px] leading-[1.15] tracking-[-0.02em] md:text-[26px]',
        className,
      )}
      {...props}
    />
  );
}

export function QuietTitleTextarea({
  className,
  rows = 1,
  presentation = 'entity',
  ...props
}: React.ComponentProps<'textarea'> & { presentation?: 'entity' | 'header' }) {
  return (
    <textarea
      rows={rows}
      className={cn(
        'w-full resize-none overflow-hidden whitespace-pre-wrap border-0 border-b border-transparent bg-transparent p-0 text-left font-semibold text-quiet-text-primary outline-none [field-sizing:content] placeholder:text-quiet-muted hover:border-quiet-field focus-visible:border-b-2 focus-visible:border-quiet-text-primary',
        presentation === 'header'
          ? 'min-h-6 pb-0.5 text-[20px] leading-[1.18] tracking-[-0.018em] md:text-[20px]'
          : 'min-h-[30px] pb-1 text-[26px] leading-[1.15] tracking-[-0.02em]',
        className,
      )}
      {...props}
    />
  );
}

export function QuietMetaLine({ items, className }: { items: ReactNode[]; className?: string }) {
  const visible = items.filter((item) => item !== null && item !== undefined && item !== false);
  return (
    <span className={cn('inline-flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11.5px] text-quiet-muted', className)}>
      {visible.map((item, index) => (
        <span key={index} className="inline-flex min-w-0 items-center gap-2">
          {index > 0 ? <span aria-hidden="true" className="h-[10px] w-px shrink-0 bg-quiet-meta-separator" /> : null}
          <span className="min-w-0">{item}</span>
        </span>
      ))}
    </span>
  );
}

const statusToneClassName = {
  neutral: 'bg-quiet-muted',
  current: 'bg-quiet-text-primary',
  lifecycle: 'bg-quiet-lifecycle',
  positive: 'bg-quiet-positive',
  blocker: 'bg-quiet-accent',
} as const;

const statusBadgeToneClassName = {
  neutral: 'border-quiet-field bg-quiet-hover text-quiet-text-secondary',
  current: 'border-quiet-text-primary/20 bg-quiet-text-primary/10 text-quiet-text-primary',
  lifecycle: 'border-quiet-lifecycle/25 bg-quiet-lifecycle/10 text-quiet-lifecycle',
  positive: 'border-quiet-positive/25 bg-quiet-positive/10 text-quiet-positive',
  blocker: 'border-quiet-accent/25 bg-quiet-accent/10 text-quiet-accent',
} as const;

export function QuietStatusText({ children, tone = 'neutral', pulse = false, className }: { children: ReactNode; tone?: keyof typeof statusToneClassName; pulse?: boolean; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-1.5 text-[11.5px] font-semibold uppercase tracking-[0.03em] text-quiet-text-tertiary', className)}>
      <span className={cn('size-1.5 shrink-0 rounded-full', statusToneClassName[tone], pulse && 'motion-safe:animate-pulse')} />
      {children}
    </span>
  );
}

export function QuietStatusBadge({ children, tone = 'neutral', color, className }: { children: ReactNode; tone?: keyof typeof statusBadgeToneClassName; color?: string; className?: string }) {
  return (
    <Badge
      variant="outline"
      style={color ? {
        color,
        borderColor: `color-mix(in srgb, ${color} 25%, transparent)`,
        backgroundColor: `color-mix(in srgb, ${color} 10%, transparent)`,
      } : undefined}
      className={cn(
        'h-5 rounded-full px-2 py-0 text-[10.5px] font-semibold uppercase tracking-[0.03em]',
        statusBadgeToneClassName[tone],
        className,
      )}
    >
      {children}
    </Badge>
  );
}

export function QuietPropertyRow({ icon: Icon, label, value, emptyPrompt, className }: { icon?: ElementType; label: ReactNode; value?: ReactNode; emptyPrompt?: ReactNode; className?: string }) {
  const empty = value === null || value === undefined || value === '';
  return (
    <div className={cn('group flex min-h-8 items-center gap-2.5 px-4 py-[7px] transition-colors hover:bg-quiet-row-hover', className)}>
      {Icon ? <Icon className="h-[15px] w-[15px] shrink-0 text-quiet-muted" /> : <span className="w-[15px] shrink-0" />}
      <span className="w-[92px] shrink-0 text-sm text-quiet-text-secondary">{label}</span>
      <span className={cn('min-w-0 flex-1 truncate text-sm', empty ? 'font-normal text-quiet-muted' : 'font-medium text-quiet-text-primary')}>{empty ? emptyPrompt : value}</span>
    </div>
  );
}

const stateMarkerClassName = {
  none: 'bg-transparent',
  current: 'bg-quiet-text-primary',
  blocker: 'bg-quiet-accent',
  positive: 'bg-quiet-positive',
} as const;

export function QuietListRow({
  actor,
  meta,
  title,
  detail,
  provenance,
  trailing,
  state = 'none',
  className,
  onClick,
}: {
  actor?: ReactNode;
  meta?: ReactNode;
  title: ReactNode;
  detail?: ReactNode;
  provenance?: ReactNode;
  trailing?: ReactNode;
  state?: keyof typeof stateMarkerClassName;
  className?: string;
  onClick?: () => void;
}) {
  const Comp = onClick ? 'button' : 'div';
  return (
    <Comp
      type={onClick ? 'button' : undefined}
      onClick={onClick}
      className={cn(
        'group relative grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-x-3 border-b border-quiet-divider-light px-5 py-3 text-left transition-colors hover:bg-quiet-row-hover focus-visible:bg-quiet-row-hover focus-visible:outline-none',
        className,
      )}
    >
      <span aria-hidden="true" className={cn('absolute inset-y-0 left-0 w-[3px]', stateMarkerClassName[state])} />
      <span className="min-w-0">
        {actor || meta ? (
          <span className="mb-0.5 flex min-w-0 items-center gap-3 text-[12.5px] text-quiet-text-secondary">
            <span className="min-w-0 flex-1 truncate">{actor}</span>
            <span className="shrink-0 text-[11.5px] text-quiet-muted">{meta}</span>
          </span>
        ) : null}
        <span className="block text-[13.5px] font-semibold leading-5 tracking-[-0.008em] text-quiet-text-primary">{title}</span>
        {detail ? <span className="mt-0.5 block text-[12.5px] leading-5 text-quiet-text-tertiary">{detail}</span> : null}
        {provenance ? <span className="mt-1 block text-[11.5px] text-quiet-muted">{provenance}</span> : null}
      </span>
      {trailing ? <span className="self-center text-quiet-text-tertiary">{trailing}</span> : null}
    </Comp>
  );
}

export function QuietEmptyState({ title, description, action, children, className }: { title: ReactNode; description: ReactNode; action?: ReactNode; children?: ReactNode; className?: string }) {
  return (
    <div className={cn('border-y border-quiet-divider-strong px-4 py-8 text-left sm:px-6', className)}>
      <h3 className="text-[14px] font-medium text-quiet-text-primary">{title}</h3>
      <p className="mt-1 max-w-[620px] text-sm leading-[1.6] text-quiet-text-tertiary [text-wrap:pretty]">{description}</p>
      {children ? <div className="mt-4">{children}</div> : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}

export {
  QuietComposerAITools,
  QuietComposerEditorSurface,
  QuietComposerFormatButton,
  QuietComposerToolbar,
  QuietConversationComposer,
} from './conversation-composer';
export type { ConversationRewriteOperation } from './conversation-composer';
export { QuietFilterDropdown, QuietSelect } from './quiet-select';
export type { QuietFilterDropdownProps, QuietSelectOption, QuietSelectProps } from './quiet-select';

export { QuietDropdown } from './quiet-dropdown';
export type { QuietDropdownOption, QuietDropdownOptionGroup, QuietDropdownProps, QuietDropdownSearchMode } from './quiet-dropdown';
