import { useState } from 'react';
import { QuietIconAction, QuietPrimaryAction, QuietSection, QuietTextAction } from '@/components/design-system/quiet';
import { SheetDescription, SheetTitle } from '@/components/ui/sheet';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ArrowRight01Icon, BookOpen01Icon, Cancel01Icon, CheckmarkCircle02Icon, MoreHorizontalIcon } from '@/lib/icons';
import type { Signal, WorkState } from './data';

const me = 'Muhammad Azhar';
import { signalCopy } from './presentation';

interface Props {
  signal: Signal;
  readOnly: boolean;
  onClose: () => void;
  onUpdate: (values: Partial<Signal>, event: string) => void;
  onNotice: (message: string) => void;
  onResource: (resource: { title: string; body: string }) => void;
  onCompany: () => void;
}

export function SignalDetail({ signal, readOnly, onClose, onUpdate, onNotice, onResource, onCompany }: Props) {
  const [decision, setDecision] = useState<WorkState | null>(null);
  const [note, setNote] = useState('');
  const [reason, setReason] = useState('Not relevant');
  const presentation = signalCopy[signal.id] || { title: signal.title, next: signal.next };
  const sourceName = signal.source === 'Support conversation' ? 'Support' : signal.source;
  const sourceAction = signal.source === 'Support conversation' ? 'Open conversation' : signal.source === 'Email' ? 'Open email' : 'Open meeting';
  const sourceDate = `${signal.age === 'Yesterday' ? 'Sep 4' : 'Sep 5'}, 2026`;
  const openSource = () => onResource({
    title: `${signal.source} · Sample source`,
    body: `${signal.author} · ${sourceDate}\n\n“${signal.quote}”\n\nIllustrative content only. In the product, this opens the permitted original source.`,
  });
  const confirm = () => {
    if (!decision || readOnly) return;
    onUpdate({ state: decision, reviewed: true }, `${decision} by ${me} · Just now${decision === 'Dismissed' ? ` · ${reason}` : ''}${note.trim() ? ` · ${note.trim()}` : ''}`);
    onNotice(`${signal.company} signal ${decision.toLowerCase()}. Saved in this preview only.`);
    setDecision(null);
    setNote('');
  };

  return <>
    <div className="compact-detail-header">
      <div className="compact-detail-topline">
        <QuietTextAction className="detail-company-link" onClick={onCompany}>{signal.company}<ArrowRight01Icon className="size-3" /></QuietTextAction>
        <div className="compact-detail-tools">
          <DropdownMenu>
            <DropdownMenuTrigger asChild><QuietIconAction aria-label="More signal actions"><MoreHorizontalIcon className="size-4" /></QuietIconAction></DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {!signal.reviewed && <DropdownMenuItem disabled={readOnly} onSelect={() => {
                onUpdate({ reviewed: true }, `Reviewed by ${me} · Just now`);
                onNotice('Reviewed. The signal remains open.');
              }}>Mark reviewed</DropdownMenuItem>}
              {signal.state === 'Open' && <DropdownMenuItem disabled={readOnly} onSelect={() => setDecision('Dismissed')}>Dismiss</DropdownMenuItem>}
              {(!signal.reviewed || signal.state === 'Open') && <DropdownMenuSeparator />}
              {signal.history.length > 0 && <DropdownMenuItem onSelect={() => onResource({ title: 'Signal activity', body: [...signal.history].reverse().join('\n\n') })}>View activity</DropdownMenuItem>}
              <DropdownMenuItem onSelect={() => onResource({
                title: 'Signal information',
                body: `Assessment\n${signal.consequence}\n\nAssignment\n${signal.basis}\n\nSource\n${signal.author} · ${sourceDate}\nDetected 2 minutes after source\n\nPriority\n${signal.priority >= 15 ? 'High' : 'Normal'} · Sample score ${signal.priority}\nA ranking score, not a deadline or probability of sale.\n\nEvidence revision: 1`,
              })}>Signal information</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <QuietIconAction aria-label="Close signal" onClick={onClose}><Cancel01Icon className="size-4" /></QuietIconAction>
        </div>
      </div>
      <SheetTitle className="detail-title">{presentation.title}</SheetTitle>
      <SheetDescription className="sr-only">Customer evidence and actions for this signal.</SheetDescription>
      <div className="compact-detail-meta">
        <span className={signal.category === 'Retention' ? 'risk-text' : ''}>{signal.category}</span>
        {signal.state !== 'Open' ? <span className="compact-state">{signal.state}</span> : signal.reviewed ? <span className="compact-state">Reviewed</span> : null}
        <div className="compact-assignee" title={signal.basis}>
          <span className="muted">Owner</span>
          <Select value={signal.owner || 'Unassigned'} disabled={readOnly} size="ui" onValueChange={owner => {
            onUpdate({ owner: owner === 'Unassigned' ? '' : owner, basis: 'Explicit · assigned in this preview' }, `Assigned to ${owner} by ${me} · Just now`);
            onNotice('Assignment changed for this signal only.');
          }}>
            <SelectTrigger aria-label="Assign signal" className="preview-picker"><SelectValue /></SelectTrigger>
            <SelectContent>{[me, 'Sara Ahmed', 'Unassigned'].map(owner => <SelectItem key={owner} value={owner}>{owner}</SelectItem>)}</SelectContent>
          </Select>
        </div>
      </div>
    </div>

    <div className="detail-scroll compact-detail-body">
      <QuietSection className="compact-evidence">
        <div className="compact-source-meta">
          <span title={signal.author}>{signal.author.split(' · ')[0]}</span><span>·</span>
          <QuietTextAction onClick={openSource}>{sourceName}</QuietTextAction><span>·</span>
          <span title={sourceDate}>{signal.age}</span>
        </div>
        <blockquote>{signal.quote}</blockquote>
        {signal.deadline && <p className="compact-deadline">{signal.deadline}</p>}
        {signal.context && <div className="compact-context">
          <p>Also asked about 20 more seats; the sync issue is still unresolved.</p>
          <QuietTextAction onClick={() => onResource({ title: 'Related email · Sample context', body: 'Anna Lee · Northstar · Sep 4, 2026\n\n“We may need to add 20 seats for our new operations team. Could you share pricing?”\n\nContext only. This separate expansion inquiry is not included in this signal’s decision.' })}>Related email <ArrowRight01Icon className="size-3" /></QuietTextAction>
        </div>}
      </QuietSection>

      {signal.state === 'Open' && presentation.next && <QuietSection title="Suggested next step" className="compact-next">
        <p className="detail-body">{presentation.next}</p>
        {signal.task && <button className="resource-link" onClick={() => onResource({ title: signal.task!, body: 'In progress · Engineering\n\nExisting linked task. Its lifecycle is independent of this signal.' })}>
          <CheckmarkCircle02Icon className="size-4" /><span>{signal.task}<small>In progress</small></span><ArrowRight01Icon className="size-3" />
        </button>}
        {signal.doc && <button className="resource-link" onClick={() => onResource({ title: signal.doc!, body: 'Example of a relevant existing pricing reference. The real product opens the permitted document; no document is created.' })}>
          <BookOpen01Icon className="size-4" /><span>{signal.doc}</span><ArrowRight01Icon className="size-3" />
        </button>}
      </QuietSection>}
    </div>

    <div className="decision-footer compact-decision-footer">
      {decision && !readOnly ? <>
        <div className="decision-heading"><strong>{decision === 'Handled' ? 'Mark handled' : 'Dismiss signal'}</strong><QuietIconAction aria-label="Cancel decision" onClick={() => setDecision(null)}><Cancel01Icon className="size-4" /></QuietIconAction></div>
        {decision === 'Dismissed' && <Select value={reason} onValueChange={setReason} size="ui">
          <SelectTrigger aria-label="Dismissal reason" className="preview-picker"><SelectValue /></SelectTrigger>
          <SelectContent>{['Not relevant', 'Incorrect evidence', 'Wrong customer', 'Duplicate'].map(value => <SelectItem key={value} value={value}>{value}</SelectItem>)}</SelectContent>
        </Select>}
        <label className="note-label" htmlFor="outcome-note">Outcome note <span>(optional)</span></label>
        <textarea id="outcome-note" value={note} onChange={event => setNote(event.target.value)} placeholder="What was decided or done?" rows={2} />
        <div className="decision-actions"><QuietTextAction onClick={() => setDecision(null)}>Cancel</QuietTextAction><QuietPrimaryAction onClick={confirm}>Confirm {decision === 'Handled' ? 'handled' : 'dismissal'}</QuietPrimaryAction></div>
      </> : <div className="decision-actions">
        {readOnly ? <span>Read-only</span> : signal.state === 'Open' ? <QuietTextAction onClick={() => setDecision('Handled')}><CheckmarkCircle02Icon className="size-4" />Mark handled</QuietTextAction> : <QuietTextAction onClick={() => {
          onUpdate({ state: 'Open' }, `Reopened by ${me} · Just now`);
          onNotice('Signal reopened. Previous decisions remain in history.');
        }}>Reopen signal</QuietTextAction>}
        <QuietPrimaryAction onClick={openSource}>{sourceAction}<ArrowRight01Icon className="size-3.5" /></QuietPrimaryAction>
      </div>}
    </div>
  </>;
}
