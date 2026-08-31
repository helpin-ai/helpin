import type { ElementType, ReactNode } from 'react';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';

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
  description,
  actions,
  variant = 'content',
  className,
}: {
  title: ReactNode;
  context?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  variant?: 'content' | 'shell';
  className?: string;
}) {
  return (
    <header className={cn('flex justify-between gap-6 group-data-[sidebar-toggle-visible=true]/workspace-main:pl-14', variant === 'shell' ? 'items-center border-b border-quiet-divider-strong px-4 py-4 sm:px-6 lg:px-8' : 'items-start', className)}>
      <div className="min-w-0">
        <h1 className="text-[20px] font-semibold leading-[1.18] tracking-[-0.018em] text-quiet-text-primary">
          {title}
          {context ? <span className="font-normal text-quiet-text-tertiary"> ({context})</span> : null}
        </h1>
        {description ? <p className="mt-1.5 max-w-[760px] text-sm leading-[1.6] text-quiet-text-tertiary [text-wrap:pretty]">{description}</p> : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-3">{actions}</div> : null}
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
    <section className={cn('border-b border-quiet-divider-strong px-4 pb-4 pt-6 group-data-[sidebar-toggle-visible=true]/workspace-main:pl-14 sm:px-6 lg:px-8', className)}>
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

export const quietUnderlineControlClassName =
  'h-auto rounded-none border-0 border-b border-quiet-field bg-transparent px-0.5 py-1.5 text-sm text-quiet-text-primary shadow-none outline-none transition-colors hover:border-quiet-text-primary focus-visible:border-b-2 focus-visible:border-quiet-text-primary focus-visible:ring-0';

export function QuietUnderlineInput({ className, ...props }: React.ComponentProps<typeof Input>) {
  return <Input variant="plain" className={cn(quietUnderlineControlClassName, className)} {...props} />;
}

export function QuietTitleInput({ className, ...props }: React.ComponentProps<typeof Input>) {
  return <Input variant="plain" className={cn(quietUnderlineControlClassName, 'w-full pb-2 text-[26px] font-semibold leading-[1.15] tracking-[-0.02em] md:text-[26px]', className)} {...props} />;
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

export function QuietStatusText({ children, tone = 'neutral', pulse = false, className }: { children: ReactNode; tone?: keyof typeof statusToneClassName; pulse?: boolean; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-1.5 text-[11.5px] font-semibold uppercase tracking-[0.03em] text-quiet-text-tertiary', className)}>
      <span className={cn('size-1.5 shrink-0 rounded-full', statusToneClassName[tone], pulse && 'motion-safe:animate-pulse')} />
      {children}
    </span>
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
