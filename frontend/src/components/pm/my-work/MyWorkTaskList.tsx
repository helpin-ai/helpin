import { useEffect, useMemo, useState } from 'react';
import { ArrowRight01Icon, InformationCircleIcon, Settings02Icon } from '@/lib/icons';
import type { Task } from '@/lib/pmTypes';
import type { MemberWithUser } from '@/lib/types';
import { QuietDropdown, QuietEmptyState, QuietSearchInput, QuietTextAction } from '@/components/design-system/quiet';
import { PMFilterPill, PMFilterTrigger, type PMFilterDefinition } from '@/components/pm/PMFilterControls';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { TooltipProvider } from '@/components/ui/tooltip';
import { PRIORITY_CONFIG } from '@/lib/pmConstants';
import { cn } from '@/lib/utils';
import { defaultMyWorkPreferences, filterMyWork, groupMyWork, matchesMyWorkFilter, myWorkFilters, type MyWorkPreferences } from './myWorkModel';
import { MyWorkTaskRow } from './MyWorkTaskRow';

type Props = { tasks: Task[]; workspaceId: string; memberId: string; assigned: boolean; members: MemberWithUser[]; teams: {id:string;name:string}[]; findTeamName: (id?:string)=>string|undefined; canEdit: boolean; onOpen: (task:Task)=>void; onChanged: ()=>void };
function readPreferences(key: string): MyWorkPreferences {
  try {
    const saved = JSON.parse(localStorage.getItem(key) || '{}');
    return {...defaultMyWorkPreferences,
      search: typeof saved.search === 'string' ? saved.search : '',
      team: typeof saved.team === 'string' ? saved.team : '',
      priority: typeof saved.priority === 'string' && (saved.priority === '' || saved.priority in PRIORITY_CONFIG) ? saved.priority : '',
      filter: myWorkFilters.includes(saved.filter) ? saved.filter : 'All',
      group: ['focus','stage','team'].includes(saved.group) ? saved.group : 'focus',
      compact: saved.compact === true, showCompleted: saved.showCompleted !== false,
    };
  } catch { return {...defaultMyWorkPreferences}; }
}
export function MyWorkTaskList(props: Props) {
  const key = `helpin:my-work:${props.workspaceId}:${props.memberId}:${props.assigned?'assigned':'requested'}`;
  const [prefs,setPrefs] = useState(()=>readPreferences(key));
  const [now,setNow] = useState(()=>new Date());
  useEffect(()=>{const timer=window.setInterval(()=>setNow(new Date()),60_000);return ()=>clearInterval(timer)},[]);
  useEffect(()=>{try{localStorage.setItem(key,JSON.stringify(prefs))}catch{/* Storage may be unavailable. */}},[key,prefs]);
  const update = (patch: Partial<MyWorkPreferences>) => setPrefs(value=>({...value,...patch}));
  const memberNames = useMemo(()=>new Map(props.members.map(member=>[member.id,member.full_name || member.email])),[props.members]);
  const ownerNames = (task:Task) => (task.owner_member_ids || []).map(id=>memberNames.get(id) || 'Workspace member').join(', ') || 'Unassigned';
  const matches = filterMyWork(props.tasks,prefs,props.assigned?()=>'':ownerNames,now);
  const groups = groupMyWork(matches,prefs,props.assigned,props.findTeamName,now,props.memberId);
  const countTasks = filterMyWork(props.tasks,{...prefs,filter:'All',showCompleted:false},props.assigned?()=>'':ownerNames,now);
  const definitions: PMFilterDefinition<'team'|'priority'>[] = [
    {key:'team',label:'Team',singleSelect:true,options:props.teams.map(team=>({value:team.id,label:team.name}))},
    {key:'priority',label:'Priority',singleSelect:true,options:Object.entries(PRIORITY_CONFIG).map(([value,config])=>({value,label:config.label}))},
  ];
  const active = new Set(definitions.filter(def=>prefs[def.key]).map(def=>def.key));
  const groupOptions = [{value:'focus',label:'Focus'}, {value:'stage',label:'Stage'}, {value:'team',label:'Team'}];
  return <TooltipProvider><div className="@container">
    <div className="flex flex-wrap items-center gap-x-3 gap-y-3 border-b border-quiet-divider-strong py-4">
      <div className="flex flex-wrap items-center gap-1" aria-label="Task filters">
        {myWorkFilters.map(filter=> <QuickTooltip key={filter} label={filter==='Due soon'?'Due today or within the next three days':filter==='All'?'All active tasks; recent completions appear separately':filter==='Blocked'?'Waiting on another dependency':filter==='Overdue'?'Past their due date':'Tasks currently underway'}>
          <button type="button" aria-pressed={prefs.filter===filter} onClick={()=>update({filter})} className={cn('rounded-md px-2.5 py-1.5 text-xs transition-colors hover:bg-quiet-hover focus-visible:outline-2 focus-visible:outline-ring',prefs.filter===filter?'bg-quiet-icon-well font-medium text-quiet-text-primary':filter==='Overdue'&&countTasks.some(task=>matchesMyWorkFilter(task,filter,now))?'text-quiet-accent':'text-quiet-text-secondary')}>
            {filter} <span className="ml-1 tabular-nums">({countTasks.filter(task=>matchesMyWorkFilter(task,filter,now)).length})</span>
          </button>
        </QuickTooltip>)}
      </div>
      <div className="ml-auto flex min-w-0 flex-wrap items-center gap-2">
        <QuietSearchInput aria-label="Search tasks" placeholder="Search tasks…" value={prefs.search} onChange={event=>update({search:event.target.value})} containerClassName="w-44 max-w-full" />
        <PMFilterTrigger definitions={definitions} values={{team:prefs.team?[prefs.team]:[],priority:prefs.priority?[prefs.priority]:[]}} visibleKeys={active} activeCount={active.size} onAdd={()=>{}} onToggle={(key,value)=>update({[key]:value})}/>
        <QuietDropdown label="Display" trigger={<QuietTextAction><Settings02Icon className="size-3.5"/>Display</QuietTextAction>} multiple searchMode="off"
          selected={[prefs.group,...(prefs.compact?['compact']:[]),...(prefs.showCompleted?['completed']:[])]}
          groups={[{id:'group',label:'Group by',options:groupOptions},{id:'rows',options:[{value:'compact',label:'Compact rows'},{value:'completed',label:'Show completed'}]}]}
          onSelect={value=>value==='compact'?update({compact:!prefs.compact}):value==='completed'?update({showCompleted:!prefs.showCompleted}):update({group:value as MyWorkPreferences['group']})}/>
      </div>
      {active.size>0 && <div className="flex basis-full flex-wrap gap-2">{definitions.filter(def=>active.has(def.key)).map(def=><PMFilterPill key={def.key} definition={def} selected={[prefs[def.key]]} onToggle={value=>update({[def.key]:value})} onRemove={()=>update({[def.key]:''})}/>)}</div>}
    </div>
    {!groups.length ? <QuietEmptyState title="No matching tasks" description={null} action={<QuietTextAction onClick={()=>setPrefs({...defaultMyWorkPreferences})}>Clear filters</QuietTextAction>}/> : groups.map(group=> <TaskGroup key={`${prefs.group}:${group.id}:${prefs.search}`} {...props} group={group} prefs={prefs} now={now} ownerNames={ownerNames} />)}
  </div></TooltipProvider>;
}
function TaskGroup({group,prefs,now,ownerNames,...props}:Props & {group:ReturnType<typeof groupMyWork>[number];prefs:MyWorkPreferences;now:Date;ownerNames:(task:Task)=>string}) {
  const [collapsed,setCollapsed] = useState(Boolean(group.completed) && !prefs.search.trim());
  const [showAll,setShowAll] = useState(false);
  const searching = Boolean(prefs.search.trim());
  const open = !collapsed;
  const visible = searching || showAll ? group.tasks : group.tasks.slice(0,10);
  return <section aria-label={group.label} className="border-b border-quiet-divider-strong">
    <div className="flex min-h-12 items-center gap-1">
      <button type="button" aria-expanded={open} onClick={()=>setCollapsed(value=>!value)} className="flex items-center gap-2 rounded-sm py-2 text-xs font-semibold text-quiet-text-primary focus-visible:outline-2 focus-visible:outline-ring">
        <ArrowRight01Icon className={cn('size-3 text-quiet-text-tertiary',open&&'rotate-90')}/>{group.label} <span className="font-normal tabular-nums text-quiet-text-tertiary">({group.tasks.length})</span>
      </button>
      {group.label==='Focus now' && <QuickTooltip label="Focus order is based on due dates, blockers, priority, active state, and recent updates."><button type="button" aria-label="How focus order works" className="rounded-sm p-1 text-quiet-muted focus-visible:outline-2 focus-visible:outline-ring"><InformationCircleIcon className="size-3.5"/></button></QuickTooltip>}
    </div>
    {open && <div className="pb-3">{visible.map(task=><MyWorkTaskRow key={task.id} task={task} workspaceId={props.workspaceId} teamName={props.findTeamName(task.team_id) || task.team_name} ownerNames={props.assigned?undefined:ownerNames(task)} compact={prefs.compact} canEdit={props.canEdit} needsInput={!task.completed && task.latest_run_status==='paused' && (props.assigned || Boolean(task.owner_member_ids?.includes(props.memberId)))} now={now} onOpen={()=>props.onOpen(task)} onChanged={props.onChanged}/>)}
      {!searching && group.tasks.length>10 && <QuietTextAction onClick={()=>setShowAll(value=>!value)} className="mt-3">{showAll?'Show less':`Show ${group.tasks.length-10} more`}</QuietTextAction>}
    </div>}
  </section>;
}
