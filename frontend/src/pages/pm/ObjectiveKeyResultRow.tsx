import { useEffect, useState } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import { QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Checkbox } from '@/components/ui/checkbox';
import { Progress } from '@/components/ui/progress';
import { PencilEdit02Icon } from '@/lib/icons';
import { pmObjectiveService } from '@/lib/services/pmObjectiveService';
import type { KeyResult, UpdateKeyResultRequest } from '@/lib/pmTypes';
import { useObjectiveAutosave } from './useObjectiveAutosave';

const numberFormat = new Intl.NumberFormat(undefined, { maximumFractionDigits: 20 });

export function ObjectiveKeyResultRow({ kr, workspaceId, memberMap, onUpdate, onEdit, readOnly, registerSave, onDirtyChange }: {
  kr: KeyResult;
  workspaceId: string;
  memberMap: Map<string, string>;
  onUpdate: (updated: KeyResult) => void;
  onEdit: () => void;
  readOnly?: boolean;
  registerSave: (id: string, save: (() => Promise<void>) | null) => void;
  onDirtyChange: (id: string, dirty: boolean) => void;
}) {
  const [valueDraft, setValueDraft] = useState<string | null>(null);
  const [source, setSource] = useState(kr);
  const { queuePatch, flush, saving, dirty, error } = useObjectiveAutosave<UpdateKeyResultRequest>({
    pendingUploads: 0,
    debounceMs: null,
    save: async patch => {
      const { data, error: saveError } = await pmObjectiveService.updateKeyResult(workspaceId, kr.id, patch);
      if (saveError || !data) throw new Error(saveError ?? 'Could not save key result');
      onUpdate(data);
    },
  });
  if (source !== kr && !dirty) { setSource(kr); setValueDraft(null); }
  useEffect(() => {
    registerSave(kr.id, flush);
    return () => registerSave(kr.id, null);
  }, [registerSave, kr.id, flush]);
  useEffect(() => {
    onDirtyChange(kr.id, dirty);
    return () => onDirtyChange(kr.id, false);
  }, [onDirtyChange, kr.id, dirty]);

  const saveValue = () => { void flush().catch(() => undefined); };
  const openEditor = async () => {
    try { await flush(); onEdit(); } catch { /* Keep the draft and Retry visible. */ }
  };
  const isCompletion = kr.result_type === 'boolean';
  const suffix = kr.result_type === 'percent' ? '%' : '';
  const progress = Math.round(kr.progress);
  const updatedByName = kr.updated_by ? memberMap.get(kr.updated_by) : undefined;
  const lastUpdated = formatDistanceToNow(parseISO(kr.updated_at), { addSuffix: true });
  const valueId = `key-result-${kr.id}-current`;

  return (
    <li className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-x-4 gap-y-3 border-b border-quiet-divider-light py-4 sm:grid-cols-[minmax(0,1fr)_minmax(10rem,14rem)_auto]">
      <div className="min-w-0">
        <p className="text-sm font-semibold leading-6 break-words text-quiet-text-primary">{kr.name}</p>
        <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-quiet-text-tertiary">
          {!isCompletion && <span className="tabular-nums">Started at {numberFormat.format(kr.initial_value)}{suffix}</span>}
          {!isCompletion && <span aria-hidden="true" className="text-quiet-muted">·</span>}
          <time dateTime={kr.updated_at} title={updatedByName ? `Updated by ${updatedByName}` : undefined}>Updated {lastUpdated}</time>
        </div>
      </div>
      <div className="col-span-2 row-start-2 min-w-0 sm:col-span-1 sm:col-start-2 sm:row-start-1">
        {isCompletion ? (
          readOnly ? <span className={`text-sm ${progress >= 100 ? 'text-quiet-positive' : 'text-quiet-text-secondary'}`}>{progress >= 100 ? 'Completed' : 'Not completed'}</span> : (
            <label className="inline-flex min-h-9 cursor-pointer items-center gap-2 text-sm text-quiet-text-secondary">
              <Checkbox
                checked={progress >= 100}
                aria-label={`Mark ${kr.name} ${progress >= 100 ? 'not done' : 'done'}`}
                disabled={saving}
                onCheckedChange={checked => { queuePatch({ current_value: checked ? kr.target_value : kr.initial_value }); saveValue(); }}
              />
              {progress >= 100 ? 'Completed' : 'Mark complete'}
            </label>
          )
        ) : (
          <div className="grid grid-cols-2 items-end gap-4">
            <div className="min-w-0">
              <label htmlFor={readOnly ? undefined : valueId} className="text-xs text-quiet-text-tertiary">Current</label>
              {readOnly ? <p className="py-1 text-sm font-medium tabular-nums text-quiet-text-primary">{numberFormat.format(kr.current_value)}{suffix}</p> : (
                <div className="relative">
                  <QuietUnderlineInput
                    id={valueId}
                    type="number"
                    step="any"
                    value={valueDraft ?? String(kr.current_value)}
                    aria-label={`Current value for ${kr.name}`}
                    onChange={event => {
                      setValueDraft(event.target.value);
                      const value = Number(event.target.value);
                      queuePatch({ current_value: event.target.value.trim() && Number.isFinite(value) ? value : kr.current_value });
                    }}
                    onBlur={saveValue}
                    onKeyDown={event => { if (event.key === 'Enter') { event.preventDefault(); saveValue(); } }}
                    className={`py-1 font-medium tabular-nums ${suffix ? 'pr-5' : ''}`}
                  />
                  {suffix && <span className="pointer-events-none absolute inset-y-0 right-0 flex items-center text-sm text-quiet-text-secondary">{suffix}</span>}
                </div>
              )}
            </div>
            <div className="min-w-0 text-right">
              <span className="text-xs text-quiet-text-tertiary">Target</span>
              <p className="break-words py-1 text-sm font-medium tabular-nums text-quiet-text-primary">{numberFormat.format(kr.target_value)}{suffix}</p>
            </div>
          </div>
        )}
        <div className="mt-2 flex items-center gap-3">
          <Progress value={kr.progress} aria-label={`Progress for ${kr.name}`} aria-valuenow={kr.progress} aria-valuetext={`${progress}% complete`} className="h-1 bg-quiet-divider-light" indicatorClassName="bg-quiet-positive" />
          <span className="w-9 shrink-0 text-right text-xs tabular-nums text-quiet-text-secondary">{progress}%</span>
        </div>
      </div>
      {!readOnly && <QuietTextAction className="col-start-2 row-start-1 self-start sm:col-start-3" aria-label={`Edit key result ${kr.name}`} onClick={() => void openEditor()}><PencilEdit02Icon className="size-3.5" />Edit</QuietTextAction>}
      {saving && <p role="status" className="col-span-full text-xs text-quiet-text-tertiary">Saving…</p>}
      {error && <div className="col-span-full flex flex-wrap items-center gap-2 text-xs text-destructive" role="alert">{error}<QuietTextAction onClick={saveValue}>Retry</QuietTextAction></div>}
    </li>
  );
}
