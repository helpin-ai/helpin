import { useId, useState, type ReactNode } from 'react';
import { QuietEmptyState, QuietIconAction, QuietTextAction, quietUnderlineControlClassName } from '@/components/design-system/quiet';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Textarea } from '@/components/ui/textarea';
import { InformationCircleIcon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { UpgradeRequiredDialog } from '@edition';
import { getUpgradeRequiredReason } from '@edition/errors';

export function PlaybookHelp({ label, children }: { label: string; children: string }) {
  const [open, setOpen] = useState(false);
  // A non-form trigger keeps help available inside read-only fieldsets.
  return <Tooltip open={open} onOpenChange={setOpen}>
    <TooltipTrigger asChild><QuietIconAction asChild><span tabIndex={0} role="button" aria-label={label} aria-expanded={open}
      onPointerDown={(event) => event.preventDefault()}
      onClick={(event) => { event.preventDefault(); setOpen((current) => !current); }}
      onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); setOpen((current) => !current); } }}
    ><InformationCircleIcon className="size-3.5" /></span></QuietIconAction></TooltipTrigger>
    <TooltipContent>{children}</TooltipContent>
  </Tooltip>;
}

export function PlaybookLabel({ label, help }: { label: string; help: string }) {
  return <span className="inline-flex items-center gap-1">{label}<PlaybookHelp label={`About ${label.toLowerCase()}`}>{help}</PlaybookHelp></span>;
}

export function PlaybookField({ label, help, children }: { label: string; help?: string; children: (id: string) => ReactNode }) {
  const id = useId();
  return <div className="min-w-0 space-y-1.5">
    <div className="flex items-center gap-1"><label htmlFor={id} className="text-sm font-medium text-quiet-text-secondary">{label}</label>{help && <PlaybookHelp label={`About ${label.toLowerCase()}`}>{help}</PlaybookHelp>}</div>
    {children(id)}
  </div>;
}

export function PlaybookTextarea(props: React.ComponentProps<typeof Textarea>) {
  return <Textarea {...props} className={cn(quietUnderlineControlClassName, 'min-h-16 resize-y rounded-none bg-transparent px-0.5 shadow-none focus-visible:ring-0', props.className)} />;
}

export function PlaybookError({ error, retry }: { error: unknown; retry?: () => void }) {
  const [showPlan, setShowPlan] = useState(false);
  const reason = getUpgradeRequiredReason(error);
  return <div role="alert" className="py-5 text-sm text-quiet-accent"><p>{error instanceof Error ? error.message : 'Could not load this view.'}</p>{reason ? <QuietTextAction onClick={() => setShowPlan(true)}>View plan options</QuietTextAction> : retry && <QuietTextAction onClick={retry}>Try again</QuietTextAction>}{showPlan && <UpgradeRequiredDialog open onOpenChange={setShowPlan} reason={reason} />}</div>;
}

export function PlaybookLoading() {
  return <div role="status" className="space-y-4 py-6"><span className="text-sm text-quiet-text-tertiary">Loading…</span>{[0, 1, 2].map((key) => <div key={key} aria-hidden="true" className="h-10 animate-pulse border-b border-quiet-divider-light" />)}</div>;
}

export function PlaybookPagination({ page, total, pageSize = 25, onChange, busy }: { page: number; total: number; pageSize?: number; onChange: (page: number) => void; busy?: boolean }) {
  if (!total && page === 1) return null;
  return <nav aria-label="Pagination" className="flex items-center justify-between gap-3 border-t border-quiet-divider-strong py-3 text-xs text-quiet-text-tertiary">
    <span>{Math.min((page - 1) * pageSize + 1, total)}–{Math.min(page * pageSize, total)} of {total}</span>
    <div className="flex gap-2"><QuietTextAction disabled={page <= 1 || busy} onClick={() => onChange(page - 1)}>Previous</QuietTextAction><QuietTextAction disabled={page * pageSize >= total || busy} onClick={() => onChange(page + 1)}>Next</QuietTextAction></div>
  </nav>;
}

export function PlaybookNoAccess() {
  return <QuietEmptyState title="CRM access required" description="Ask a workspace admin for access to Playbooks." />;
}
