import { useState } from 'react';
import { format, parseISO } from 'date-fns';
import { toast } from 'sonner';
import { CancelCircleIcon, CheckmarkCircle02Icon, Clock01Icon, Key01Icon, Loading01Icon, MessagePreview01Icon, SecurityCheckIcon } from '@/lib/icons';
import { PRIORITY_CONFIG, PriorityIcon } from '@/lib/pmConstants';
import { StateTypeIcon } from '@/lib/pmIcons';
import type { Task, WorkflowState, UpdateTaskRequest } from '@/lib/pmTypes';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { QuietDropdown, QuietStatusText, QuietTextAction } from '@/components/design-system/quiet';
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { DatePicker } from '@/components/ui/date-picker';
import { cn } from '@/lib/utils';
import { deadlineDays } from './myWorkModel';

const runLabels: Record<string,string> = { queued:'Agent queued', running:'Agent running', completed:'Agent completed', failed:'Agent failed', cancelled:'Agent cancelled' };
function agentLabel(task: Task) {
  if (!task.latest_run_status) return null;
  if (task.latest_run_status !== 'paused') return runLabels[task.latest_run_status] || `Agent ${task.latest_run_status}`;
  return ({human_approval:'Agent needs approval', authentication:'Agent needs auth', awaiting_user_message:'Agent awaiting reply'} as Record<string,string>)[task.latest_run_pause_reason || ''] || 'Agent needs input';
}
export function MyWorkTaskRow({ task, workspaceId, teamName, ownerNames, compact, canEdit, needsInput, now, onOpen, onChanged }: {
  task: Task; workspaceId: string; teamName?: string; ownerNames?: string; compact: boolean; canEdit: boolean; needsInput: boolean; now: Date; onOpen: () => void; onChanged: () => void;
}) {
  const [saving,setSaving] = useState(false);
  const [states,setStates] = useState<WorkflowState[]>([]);
  const [stateLoading,setStateLoading] = useState(false);
  const [stateError,setStateError] = useState('');
  const days = deadlineDays(task.deadline,now);
  const runStatus = task.latest_run_status;
  const label = agentLabel(task);
  const RunIcon = runStatus === 'running' ? Loading01Icon : runStatus === 'completed' ? CheckmarkCircle02Icon : runStatus === 'failed' || runStatus === 'cancelled' ? CancelCircleIcon : runStatus === 'paused' ? task.latest_run_pause_reason === 'human_approval' ? SecurityCheckIcon : task.latest_run_pause_reason === 'authentication' ? Key01Icon : MessagePreview01Icon : Clock01Icon;
  const tone = runStatus === 'completed' ? 'positive' : runStatus === 'failed' || runStatus === 'paused' ? 'blocker' : runStatus === 'running' ? 'current' : 'neutral';
  async function save(payload: UpdateTaskRequest, stateId?: string) {
    if (saving || !canEdit) return;
    setSaving(true);
    try {
      const result = stateId ? await pmTaskService.move(workspaceId,task.id,{state_id:stateId}) : await pmTaskService.update(workspaceId,task.id,payload);
      if (result.error) throw new Error(result.error);
      onChanged();
    } catch (error) { toast.error(error instanceof Error ? error.message : 'Could not update task. Try again.'); }
    finally { setSaving(false); }
  }
  async function loadStates(open: boolean) {
    if (!open || states.length || stateLoading) return;
    setStateLoading(true); setStateError('');
    try {
      const result = await pmWorkflowService.get(workspaceId,task.workflow_id);
      if (result.error || !result.data) throw new Error(result.error || 'Unable to load stages.');
      setStates(result.data.states);
    } catch { setStateError('Couldn’t load stages. Close and reopen to retry.'); }
    finally {setStateLoading(false);}
  }
  const stage = <><StateTypeIcon stateType={task.state_type || 'backlog'} className="size-3.5 shrink-0" style={task.state_color ? {color:task.state_color} : undefined} /><span className="truncate">{task.state_name || 'No stage'}</span></>;
  const propertyClass = 'inline-flex min-w-0 items-center gap-1.5 rounded-sm text-xs text-quiet-text-secondary focus-visible:outline-2 focus-visible:outline-ring';
  const date = days === null ? null : parseISO(task.deadline!.slice(0,10));
  const deadlineText = days === null ? '' : task.completed ? format(date!,'MMM d') : days < 0 ? `${Math.abs(days)} ${days === -1 ? 'day' : 'days'} overdue` : days === 0 ? 'Today' : days === 1 ? 'Tomorrow' : format(date!,'MMM d');
  return <div className={cn('group grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-2 border-b border-quiet-divider-light px-2 py-3 hover:bg-quiet-row-hover @min-[820px]:grid-cols-[minmax(220px,1fr)_110px_130px_90px_115px_64px]', compact && 'py-2')}>
    <div className="col-span-2 grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-x-3 @min-[820px]:col-span-1">
      <button type="button" onClick={onOpen} className="col-span-2 grid grid-cols-subgrid items-baseline rounded-sm text-left focus-visible:outline-2 focus-visible:outline-ring">
        <span className="shrink-0 whitespace-nowrap font-mono text-[10.5px] tabular-nums text-quiet-muted">{task.task_key}</span>
        <span className={cn('min-w-0 text-[13.5px] font-medium leading-5',task.completed && 'text-quiet-text-tertiary line-through')}>{task.name}</span>
      </button>
      {(teamName || ownerNames || task.blocked || label) && <div className={cn('col-start-2 mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11.5px] text-quiet-text-tertiary', !ownerNames && !task.blocked && !label && '@min-[820px]:hidden')}>
        {teamName && <span className="@min-[820px]:hidden">{teamName}</span>}{ownerNames && <span>{ownerNames}</span>}
        {task.blocked && <QuickTooltip label={task.blocker || 'Waiting on another dependency'}><span tabIndex={0} className="rounded-sm font-medium text-quiet-accent">Blocked</span></QuickTooltip>}
        {label && <QuietStatusText tone={tone} pulse={runStatus==='running'}><RunIcon className={cn('size-3',runStatus==='running' && 'motion-safe:animate-spin')} />{label}</QuietStatusText>}
      </div>}
    </div>
    <span className="hidden min-w-0 truncate text-xs text-quiet-text-tertiary @min-[820px]:block" title={teamName}>{teamName || '—'}</span>
    {canEdit ? <QuietDropdown label={`Stage for ${task.task_key}`} trigger={<button type="button" aria-label={`Change stage for ${task.task_key}`} className={propertyClass} disabled={saving}>{stage}</button>} selected={[task.workflow_state_id]} loading={stateLoading} error={stateError || undefined} onOpenChange={open=>void loadStates(open)} options={states.map(state=>({value:state.id,label:state.name,leading:<StateTypeIcon stateType={state.state_type} className="size-3.5" style={state.color?{color:state.color}:undefined}/>}))} onSelect={id=>void save({},id)} /> : <span className={propertyClass}>{stage}</span>}
    {canEdit ? <QuietDropdown label={`Priority for ${task.task_key}`} trigger={<button type="button" aria-label={`Change priority for ${task.task_key}`} disabled={saving} className={propertyClass}><PriorityIcon priority={task.priority} className="size-3.5" />{PRIORITY_CONFIG[task.priority].label}</button>} selected={[task.priority]} options={Object.entries(PRIORITY_CONFIG).map(([value,config])=>({value,label:config.label,leading:<PriorityIcon priority={value as Task['priority']} className="size-3.5"/>}))} onSelect={priority=>void save({priority:priority as Task['priority']})} /> : <span className={propertyClass}><PriorityIcon priority={task.priority} className="size-3.5"/>{PRIORITY_CONFIG[task.priority].label}</span>}
    <Tooltip>
      {canEdit ? <DatePicker value={task.deadline?.slice(0,10)} onChange={deadline=>void save({deadline})} kind="due"
        trigger={<TooltipTrigger asChild><button type="button" disabled={saving} aria-label={`Due date for ${task.task_key}${date ? `: ${format(date,'MMMM d, yyyy')}` : ''}`} className={cn('justify-self-start rounded-sm text-xs focus-visible:outline-2 focus-visible:outline-ring', !task.completed && days !== null && days < 0 ? 'text-quiet-accent' : 'text-quiet-text-tertiary')}>{deadlineText || 'Set due date'}</button></TooltipTrigger>} />
        : <TooltipTrigger asChild><span tabIndex={0} className={cn('rounded-sm text-xs',!task.completed && days !== null && days < 0 ? 'text-quiet-accent':'text-quiet-text-tertiary')}>{deadlineText || 'No due date'}</span></TooltipTrigger>}
      <TooltipContent>{date ? `Due ${format(date,'MMMM d, yyyy')}` : 'Set a due date'}</TooltipContent>
    </Tooltip>
    <QuietTextAction onClick={onOpen} className={cn('justify-self-end text-xs', !needsInput && 'opacity-100 @min-[820px]:opacity-0 @min-[820px]:group-hover:opacity-100 @min-[820px]:group-focus-within:opacity-100')}>{needsInput?'Review':'Open'}</QuietTextAction>
  </div>;
}
