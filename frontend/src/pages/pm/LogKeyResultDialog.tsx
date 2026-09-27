import { useState, type FormEvent } from 'react';
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import type { KeyResult } from '@/lib/pmTypes';

export function LogKeyResultDialog({ result, onClose, onSave }: { result: KeyResult; onClose: () => void; onSave: (value: number) => Promise<void> }) {
  const [value, setValue] = useState(String(result.current_value));
  const [completed, setCompleted] = useState(result.progress >= 100);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const boolean = result.result_type === 'boolean';
  const suffix = result.result_type === 'percent' ? '%' : '';
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (saving) return;
    const current = boolean ? (completed ? result.target_value : 0) : Number(value);
    if (!boolean && (!value.trim() || !Number.isFinite(current))) { setError('Enter a valid current value.'); return; }
    setSaving(true); setError(null);
    try { await onSave(current); onClose(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not log result. Try again.'); }
    finally { setSaving(false); }
  };
  return <Dialog open onOpenChange={open => { if (!open && !saving) onClose(); }}>
    <DialogContent className="sm:max-w-lg">
      <DialogHeader><DialogTitle>Log result</DialogTitle><DialogDescription>{result.name}</DialogDescription></DialogHeader>
      <form onSubmit={event => void submit(event)} className="space-y-6">
        {boolean ? <label className="flex items-center gap-2 text-sm"><Checkbox checked={completed} disabled={saving} onCheckedChange={checked => setCompleted(checked === true)} />Completed</label> : <>
          <div className="flex flex-wrap gap-6 text-xs text-quiet-text-tertiary"><span>Current <span className="ml-1 font-medium text-quiet-text-primary">{result.current_value.toLocaleString()}{suffix}</span></span><span>Target <span className="ml-1 font-medium text-quiet-text-primary">{result.target_value.toLocaleString()}{suffix}</span></span></div>
          <div><label htmlFor="key-result-value" className="text-sm font-medium text-quiet-text-secondary">New current value</label>
            <div className="relative mt-1"><QuietUnderlineInput id="key-result-value" type="number" step="any" required autoFocus disabled={saving} value={value} onChange={event => setValue(event.target.value)} className={suffix ? 'pr-6' : undefined} />
              {suffix && <span className="absolute inset-y-0 right-1 flex items-center text-sm text-quiet-text-tertiary">{suffix}</span>}
            </div>
          </div>
        </>}
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <DialogFooter><QuietTextAction type="button" disabled={saving} onClick={onClose}>Cancel</QuietTextAction><QuietPrimaryAction type="submit" disabled={saving}>{saving ? 'Saving…' : 'Log result'}</QuietPrimaryAction></DialogFooter>
      </form>
    </DialogContent>
  </Dialog>;
}
