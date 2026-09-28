import { useState, type FormEvent } from 'react';
import { QuietPrimaryAction, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/design-system/quiet-dropdown-select';
import { useConfirm } from '@/components/ui/confirm-dialog';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import type { CreateKeyResultRequest, KeyResult, KeyResultType } from '@/lib/pmTypes';

export function KeyResultEditorDialog({ result, onClose, onSave, onDelete }: {
  result?: KeyResult;
  onClose: () => void;
  onSave: (values: CreateKeyResultRequest) => Promise<void>;
  onDelete?: () => Promise<void>;
}) {
  const confirm = useConfirm();
  const [name, setName] = useState(result?.name ?? '');
  const [type, setType] = useState<KeyResultType>(result?.result_type ?? 'percent');
  const [start, setStart] = useState(String(result?.result_type === 'boolean' ? 0 : result?.initial_value ?? 0));
  const [target, setTarget] = useState(String(result?.result_type === 'boolean' ? 100 : result?.target_value ?? 100));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const isCompletion = type === 'boolean';

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (saving || !name.trim()) return;
    const initialValue = isCompletion ? 0 : Number(start);
    const currentValue = isCompletion ? 0 : initialValue;
    const targetValue = isCompletion ? 1 : Number(target);
    if (![initialValue, targetValue].every(Number.isFinite) || (!isCompletion && (!start.trim() || !target.trim()))) {
      setError('Enter a valid number in each value field.');
      return;
    }
    setSaving(true); setError(null);
    try {
      await onSave({ name: name.trim(), result_type: type, initial_value: initialValue, ...(!result ? { current_value: currentValue } : {}), target_value: targetValue });
      onClose();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not save key result. Try again.');
    } finally { setSaving(false); }
  };

  const remove = async () => {
    if (!onDelete || saving) return;
    const accepted = await confirm({ title: 'Delete key result?', description: `“${result?.name}” will be removed from this objective.`, confirmText: 'Delete', variant: 'destructive' });
    if (!accepted) return;
    setSaving(true); setError(null);
    try { await onDelete(); onClose(); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Could not delete key result. Try again.'); }
    finally { setSaving(false); }
  };

  const numberField = (id: string, label: string, value: string, onChange: (value: string) => void) => (
    <div className="min-w-0">
      <label htmlFor={id} className="text-sm font-medium text-quiet-text-secondary">{label}</label>
      <div className="relative mt-1">
        <QuietUnderlineInput id={id} type="number" step="any" required disabled={saving} value={value} onChange={event => onChange(event.target.value)} className={type === 'percent' ? 'pr-6' : undefined} />
        {type === 'percent' && <span className="pointer-events-none absolute inset-y-0 right-1 flex items-center text-sm text-quiet-text-tertiary">%</span>}
      </div>
    </div>
  );

  return (
    <Dialog open onOpenChange={open => { if (!open && !saving) onClose(); }}>
      <DialogContent className="sm:max-w-lg" aria-describedby={undefined}>
        <DialogHeader><DialogTitle>{result ? 'Edit key result' : 'Add key result'}</DialogTitle></DialogHeader>
        <form onSubmit={event => void submit(event)} className="space-y-6">
          <div>
            <label htmlFor="key-result-name" className="text-sm font-medium text-quiet-text-secondary">Name</label>
            <QuietUnderlineInput id="key-result-name" required value={name} disabled={saving} onChange={event => setName(event.target.value)} placeholder="e.g., Increase activation rate" className="mt-1" autoFocus />
          </div>
          <div>
            <label htmlFor="key-result-type" className="text-sm font-medium text-quiet-text-secondary">Measure as</label>
            <Select value={type} disabled={saving} onValueChange={value => setType(value as KeyResultType)}>
              <SelectTrigger id="key-result-type" variant="underline" className="mt-1 w-full px-0.5"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value="percent">Percentage</SelectItem><SelectItem value="numeric">Number</SelectItem><SelectItem value="boolean">Complete / incomplete</SelectItem></SelectContent>
            </Select>
          </div>
          {!isCompletion && (
            <div className="grid gap-4 sm:grid-cols-2">
              {numberField('key-result-start', 'Starting value', start, setStart)}
              {numberField('key-result-target', 'Target value', target, setTarget)}
            </div>
          )}
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <DialogFooter className="gap-2 sm:justify-between">
            <div>{onDelete && <QuietTextAction type="button" className="text-destructive hover:text-destructive" disabled={saving} onClick={() => void remove()}>Delete key result</QuietTextAction>}</div>
            <div className="flex items-center justify-end gap-2">
              <QuietTextAction type="button" disabled={saving} onClick={onClose}>Cancel</QuietTextAction>
              <QuietPrimaryAction type="submit" disabled={saving || !name.trim()}>{saving ? 'Saving…' : result ? 'Save changes' : 'Add key result'}</QuietPrimaryAction>
            </div>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
