import { CheckmarkCircle02Icon, Copy01Icon, Loading01Icon } from '@/lib/icons';
import { QuietIconAction, QuietPrimaryAction, QuietTextAction } from '@/components/design-system/quiet';
import { DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { useCopyToClipboard } from '@/hooks/useCopyToClipboard';
import { buildSenderInstructions, senderDNSRecords, type SenderDNSDetails } from './supportEmailSenderInstructions';
import { cn } from '@/lib/utils';

function CopyAction({ value, label, iconOnly = false }: { value: string; label: string; iconOnly?: boolean }) {
  const { copy, copied, error } = useCopyToClipboard();
  const Icon = copied ? CheckmarkCircle02Icon : Copy01Icon;
  return <span className="inline-flex min-w-0 flex-col items-start">
    {iconOnly ? <QuietIconAction type="button" aria-label={copied ? `${label} copied` : label} onClick={() => copy(value)}><Icon /></QuietIconAction>
      : <QuietTextAction type="button" onClick={() => copy(value)}><Icon className="size-3.5" />{copied ? 'Copied' : label}</QuietTextAction>}
    {error && <span role="alert" className="text-xs text-destructive">Could not copy. Select the text to copy it.</span>}
  </span>;
}

export function SupportEmailSenderDNSDialog({ email, details, busy, saving, complete, manageDNS, error, notice, retrySave, onCheck, onClose }: {
  email: string;
  details: SenderDNSDetails;
  busy: boolean;
  saving: boolean;
  complete: boolean;
  manageDNS: boolean;
  error: string;
  notice: string;
  retrySave: boolean;
  onCheck: () => void;
  onClose: () => void;
}) {
  const settingsUrl = new URL(window.location.href);
  settingsUrl.search = '?tab=senders';
  settingsUrl.hash = '';
  const records = senderDNSRecords(details);
  const missingRecords = records.some(record => !record.name || !record.value);
  const title = busy ? saving ? 'Saving inbox selection…' : 'Checking DNS records…' : complete ? email : manageDNS ? details.domain : `Verify ${details.domain}`;

  return <>
    <div className={cn('flex shrink-0 items-center gap-4 px-6 pb-5 pt-7 pr-14', (busy || complete) && 'py-8')}>
      {(busy || complete) && <span className={cn('flex size-10 shrink-0 items-center justify-center rounded-full', complete ? 'bg-quiet-positive/10 text-quiet-positive' : 'bg-quiet-icon-well text-quiet-text-tertiary')}>
        {busy ? <Loading01Icon className="size-5 motion-safe:animate-spin" /> : <CheckmarkCircle02Icon className="size-5" />}
      </span>}
      <div className="min-w-0 space-y-2">
        <DialogTitle tabIndex={-1} className="break-all leading-snug">{title}</DialogTitle>
        <DialogDescription className={complete ? 'text-quiet-positive' : undefined}>{busy ? details.domain : complete ? 'Verified' : manageDNS ? 'DNS records for this sending domain.' : 'Add these records at your DNS provider to enable sending.'}</DialogDescription>
      </div>
    </div>
    <span className="sr-only" role="status" aria-live="polite">{busy ? title : complete ? `${email} verified` : ''}</span>
    {!busy && !complete && <div className="min-h-0 overflow-y-auto px-6 pb-6">
      {error && <p className="mb-5 text-sm text-destructive" role="alert">{error}</p>}
      {notice && <p className="mb-5 text-sm text-quiet-text-secondary" role="status">{notice}</p>}
      <div className="divide-y divide-quiet-divider-strong">
        {records.map(record => <section key={record.type} className="py-5 first:pt-0 last:pb-0">
          <div className="flex items-center justify-between gap-3"><h3 className="text-sm font-semibold">{record.label}</h3><span className={cn('text-xs', record.verified ? 'text-quiet-positive' : 'text-quiet-text-tertiary')}>{record.verified ? 'Verified' : 'Awaiting record'}</span></div>
          <p className="mt-2 mb-4 text-xs leading-relaxed text-quiet-text-tertiary">{record.description}</p>
          <dl className="space-y-3">
            {([['Type', record.type], ['Name', record.name], ['Value', record.value]] as const).map(([label, value]) => <div key={label} className="grid grid-cols-[48px_minmax(0,1fr)_28px] items-start gap-3">
              <dt className="pt-1 text-xs text-quiet-text-tertiary">{label}</dt>
              <dd className="min-w-0 pt-1 font-mono text-xs leading-relaxed [overflow-wrap:anywhere]">{value || <span className="font-sans text-muted-foreground">Not available. Check verification to refresh.</span>}</dd>
              {label !== 'Type' && value && <CopyAction iconOnly label={`Copy ${record.label} ${label.toLowerCase()}`} value={value} />}
            </div>)}
          </dl>
        </section>)}
      </div>
      {!missingRecords && <div className="mt-6 flex flex-wrap gap-x-6 gap-y-3 border-t border-quiet-divider-strong pt-5">
        <CopyAction label="Copy for AI" value={buildSenderInstructions('ai', email, details, settingsUrl.href)} />
        <CopyAction label="Copy for developer" value={buildSenderInstructions('developer', email, details, settingsUrl.href)} />
      </div>}
    </div>}
    <div className="flex shrink-0 items-center justify-between gap-4 border-t border-quiet-divider-strong px-6 py-4">
      {complete ? <QuietPrimaryAction className="ml-auto" onClick={onClose}>Done</QuietPrimaryAction> : <>
        <QuietTextAction onClick={onClose}>{manageDNS ? 'Close' : 'Finish later'}</QuietTextAction>
        <QuietPrimaryAction disabled={busy} onClick={onCheck}>{busy ? saving ? 'Saving…' : 'Checking…' : retrySave ? 'Retry save' : error || notice ? 'Check again' : manageDNS ? 'Recheck DNS' : 'Check verification'}</QuietPrimaryAction>
      </>}
    </div>
  </>;
}
