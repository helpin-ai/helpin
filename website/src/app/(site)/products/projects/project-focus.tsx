'use client';

import { useEffect, useState, type CSSProperties } from 'react';
import { ArrowDown, ArrowUp, ArrowUpRight, Bug, Check, ChevronDown, GitBranch, LayoutGrid, List, LockKeyhole, MessageSquare, MoreHorizontal, Pause, Play, Plus, X } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowClick, WorkflowSources } from '../../_components/WorkflowParts';
import { StreamingText } from '../../_components/StreamingText';
import { TaskFeatureIcon, TaskIcon, TaskPriorityIcon, TaskSprintIcon } from './task-demo-icons';

type Task = { key: string; title: string; state: 'Ready' | 'In Progress' | 'In Review' | 'Done'; owner: 'Sam' | 'Alex' | 'Jules'; blocked?: boolean; points: number };
const tasks: Task[] = [
  { key: 'PRJ-214', title: 'Add an admin-only SSO pilot', state: 'Ready', owner: 'Sam', blocked: true, points: 3 },
  { key: 'PRJ-212', title: 'Validate group-to-role mappings', state: 'In Progress', owner: 'Alex', points: 3 },
  { key: 'PRJ-217', title: 'Prepare the SSO pilot guide', state: 'In Progress', owner: 'Jules', points: 2 },
  { key: 'PRJ-219', title: 'Improve password-reset errors', state: 'In Review', owner: 'Sam', points: 2 },
  { key: 'PRJ-208', title: 'Update the backup verification checklist', state: 'Done', owner: 'Jules', points: 1 },
];
const states = ['Ready', 'In Progress', 'In Review', 'Done'] as const;
const stateColors = ['#818cf8', '#d99552', '#af73c3', '#83b397'];

function Owner({ name }: { name: Task['owner'] }) {
  return <span className="pf-owner">{name === 'Sam' ? <img src="/new/avatars/sam.webp" width={22} height={22} alt="" /> : <i data-owner={name}>{name === 'Alex' ? 'AM' : 'JP'}</i>}{name}</span>;
}
function State({ value }: { value: Task['state'] }) {
  return <span className="pf-state"><i style={{ background: stateColors[states.indexOf(value)] }} />{value}</span>;
}
function TaskTitle({ task }: { task: Task }) {
  return <span className="pf-task-title"><small>{task.key}</small><strong>{task.title}</strong></span>;
}

// TaskCard / TaskListView field conventions and dependency labels, presented as
// a scripted, read-only workspace. Ask Agent suggests a next step; no task changes.
export function ProjectFocus() {
  const [paused, setPaused] = useState(false);
  const { container, playing: active, cycle, phase } = useWorkflowPlayback({ paused, beats: [0,3000,6000,9000,11200,13600], duration: 20500 });
  const filtered = phase >= 2;
  const captions = ['SEE THE WORK ACROSS YOUR TEAM', 'COMPARE TASKS, STATES, AND OWNERS', 'FIND THE DEPENDENCY BEHIND A BLOCKER', 'ASK AGENT WHAT NEEDS ATTENTION'];
  return <div className="project-focus" ref={container} data-playing={active} data-phase={phase}>

    <div className="pf-scene" key={cycle} role="region" aria-label="Illustrative OrbitDesk Sprint 24 board and list. PRJ-214, Northstar’s admin-only SSO pilot, waits on PRJ-212, role mapping, assigned to Alex and marked In progress. Jules can prepare the guide independently. Sam has password-reset changes in review. A completed backup verification checklist keeps internal maintenance visible. Ask Agent suggests next steps without changing priorities.">
      <div>
        <div className="pf-header"><span className="pf-workspace"><span className="ps-workspace">O</span>OrbitDesk <span className="pf-divider">/</span> Engineering</span><span className="pf-add"><Plus size={13} />Add task</span><button type="button" className="pf-inline-playback" aria-label={`${paused ? 'Play' : 'Pause'} task priorities animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12}/> : <Pause size={12}/>}</button></div>
        <div className="pf-views"><span><TaskIcon name="ViewIcon" />Views</span><span>Everything</span><span className="pf-current-view">Sprint 24</span><Plus size={13} /></div>
        <div className="pf-toolbar"><span className="pf-filter"><TaskIcon name="FilterHorizontalIcon" />Filters{phase === 1 && active && <WorkflowClick delay={1400}/>}</span><span className="pf-filter-chip"><TaskSprintIcon />Sprint 24<ChevronDown size={11} /></span><span className="pf-filter-chip pf-blocked-filter" data-visible={filtered}><LockKeyhole size={11} />Blocked<X size={10} /></span><div className="pf-view-switch"><span data-selected={phase === 0}><LayoutGrid size={14} />Board</span><span data-selected={phase > 0}><List size={14} />List{phase === 0 && active && <WorkflowClick delay={1400}/>}</span></div></div>
        <div className="pf-content">
          <div className="pf-board" data-visible={phase === 0}>
            {states.map((state, index) => <div className="pf-column" key={state} style={{ '--pf-state-color': stateColors[index] } as CSSProperties}><div className="pf-column-title"><State value={state} /><span>{tasks.filter(task => task.state === state).length}<Plus size={13} /></span></div>{tasks.filter(task => task.state === state).map(task => <div className="pf-card" key={task.key}><div className="pf-card-title"><TaskTitle task={task} />{task.key === 'PRJ-219' ? <Bug className="pf-bug" size={16} /> : <TaskFeatureIcon />}</div><span className="pf-card-sprint"><TaskSprintIcon />Sprint 24</span>{task.blocked && <span className="pf-blocker"><LockKeyhole size={11} />Waiting on PRJ-212</span>}<div className="pf-card-footer"><TaskPriorityIcon /><span>{task.points} pts</span><Owner name={task.owner} /></div></div>)}</div>)}
          </div>
          <div className="pf-list-area" data-visible={phase > 0}>
            <div className="pf-list-heading"><span>Task</span><span>State</span><span>Owner</span></div>
            <div className="pf-list-rows">{tasks.filter(task => !filtered || task.blocked).map(task => <div className="pf-row" data-blocked={task.blocked || undefined} key={task.key}><TaskTitle task={task} /><State value={task.state} /><Owner name={task.owner} /></div>)}</div>
            <div className="pf-list-total">{filtered ? '1 blocked task' : '5 tasks'}<span>Sprint 24</span></div>
            <div className="pf-dependency" data-visible={filtered}><div className="pf-dependency-label"><GitBranch size={13} />Blocked by</div><div className="pf-dependency-task"><TaskTitle task={tasks[1]} /><div><State value="In Progress" /><Owner name="Alex" /></div></div><p>Confirm the role mappings before enabling the pilot.</p><div className="pf-customer"><MessageSquare size={14} /><span><strong>Maya Chen · Northstar Labs</strong>Northstar wants to test with administrators without changing access for the wider team.</span></div></div>
          </div>
          {phase === 2 && <div className="pf-context-note" data-visible="true"><img src="/new/avatars/maya.webp" width={32} height={32} alt=""/><strong>Maya Chen</strong><p>“We’d like to test SSO with our admins first. We’re not ready to switch everyone over.”</p><span className="pf-ask-trigger">Ask Helpin AI<WorkflowClick delay={1600}/></span></div>}

          <div className="pf-ask" data-visible={phase >= 3}>
            <div className="pf-ask-header"><img src="/brand/helpin-icon-white.svg" width={27} height={27} alt="" /><span><strong>Helpin AI</strong><small>Ask what needs attention.</small></span><ArrowUpRight size={15} /><MoreHorizontal size={16} /><X size={15} /></div>
            <div className="pf-ask-chat"><div className="pf-question"><img src="/new/avatars/sam.webp" width={24} height={24} alt="" /><p>What’s blocking Northstar’s pilot, and what can we move forward now?</p></div><div className="pf-answer"><div className="pf-answer-label"><img src="/brand/helpin-icon-white.svg" width={19} height={19} alt=""/>Helpin AI</div>{phase < 5 ? <WorkflowSources sources={[{icon:GitBranch,label:'Linked task PRJ-212',detail:'Role mapping · Alex · In progress'},{icon:MessageSquare,label:'Maya’s conversation',detail:'Admin-only pilot required'}]} phase={Math.max(0,phase - 2)}/> : <div className="wf-outcome"><p><StreamingText text="The pilot is waiting on Alex’s role-mapping task, PRJ-212. Jules can prepare the setup guide now, while Sam reviews the password-reset change." active={active} duration={2100}/></p><div className="pf-sources"><span><GitBranch size={11}/>3 linked tasks</span><span><MessageSquare size={11}/>Customer history</span></div></div>}</div></div>
            <div className="pf-composer"><span>Ask about the next step…</span><div><Plus size={14} /><small>Workspace context</small><span><ArrowUp size={14} /></span></div></div>
          </div>
        </div>
        <div className="pf-bottom"><span><Check size={12} />Suggested next steps · No priorities changed</span></div>
      </div>
    </div>
  </div>;
}
