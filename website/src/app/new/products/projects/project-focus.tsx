'use client';

import { useEffect, useState, type CSSProperties } from 'react';
import { ArrowDown, ArrowUp, ArrowUpRight, Bug, Check, ChevronDown, GitBranch, LayoutGrid, List, LockKeyhole, MessageSquare, MoreHorizontal, Pause, Play, Plus, X } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import { TaskFeatureIcon, TaskIcon, TaskPriorityIcon, TaskSprintIcon } from './task-demo-icons';

type Task = { key: string; title: string; state: 'Ready' | 'In Progress' | 'In Review'; owner: 'Sam' | 'Alex' | 'Jules'; blocked?: boolean; points: number };
const tasks: Task[] = [
  { key: 'ORB-491', title: 'Add Slack alerts for failed syncs', state: 'Ready', owner: 'Sam', blocked: true, points: 3 },
  { key: 'ORB-494', title: 'Fix duplicate webhook delivery', state: 'In Progress', owner: 'Alex', points: 3 },
  { key: 'ORB-492', title: 'Fix missing rows in CSV exports', state: 'In Review', owner: 'Sam', points: 2 },
  { key: 'ORB-493', title: 'Add recurring task templates', state: 'Ready', owner: 'Jules', points: 2 },
  { key: 'ORB-498', title: 'Add API retry limits', state: 'In Progress', owner: 'Alex', points: 2 },
];
const states = ['Ready', 'In Progress', 'In Review'] as const;
const stateColors = ['#818cf8', '#d99552', '#af73c3'];

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
  const { container, playing, cycle } = useBentoPlayback(19000);
  const [paused, setPaused] = useState(false);
  const [frame, setFrame] = useState(3);
  const active = playing && !paused;
  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timers = [3000, 6000, 9000].map((time, index) => setTimeout(() => setFrame(index + 1), time));
    return () => timers.forEach(clearTimeout);
  }, [active, cycle]);
  const phase = active ? frame : 3;
  const filtered = phase >= 2;
  const captions = ['SEE THE WORK ACROSS YOUR TEAM', 'COMPARE TASKS, STATES, AND OWNERS', 'FIND THE DEPENDENCY BEHIND A BLOCKER', 'ASK AGENT WHAT NEEDS ATTENTION'];
  return <div className="project-focus" ref={container} data-playing={active} data-phase={phase}>
    <div className="pf-playback"><span>{captions[phase]}</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} task priorities animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div className="pf-scene" key={cycle} role="img" aria-label="Illustrative OrbitDesk sprint review. Switch from board to list and filter blocked tasks. Sam’s task ORB-491, Slack alerts, is blocked by Alex’s in-progress task ORB-494, duplicate webhook delivery. Ask Agent connects the dependency to Maya’s request at Northstar Labs and recommends finishing Alex’s fix first, then reviewing Sam’s CSV export fix ORB-492. The agent recommends priorities without changing tasks.">
      <div aria-hidden="true">
        <div className="pf-header"><span className="pf-workspace"><span className="ps-workspace">O</span>OrbitDesk <span className="pf-divider">/</span> Engineering</span><span className="pf-add"><Plus size={13} />Add task</span></div>
        <div className="pf-views"><span><TaskIcon name="ViewIcon" />Views</span><span>Everything</span><span className="pf-current-view">Sprint 24</span><Plus size={13} /></div>
        <div className="pf-toolbar"><span className="pf-filter"><TaskIcon name="FilterHorizontalIcon" />Filters</span><span className="pf-filter-chip"><TaskSprintIcon />Sprint 24<ChevronDown size={11} /></span><span className="pf-filter-chip pf-blocked-filter" data-visible={filtered}><LockKeyhole size={11} />Blocked<X size={10} /></span><div className="pf-view-switch"><span data-selected={phase === 0}><LayoutGrid size={14} />Board</span><span data-selected={phase > 0}><List size={14} />List</span></div></div>
        <div className="pf-content">
          <div className="pf-board" data-visible={phase === 0}>
            {states.map((state, index) => <div className="pf-column" key={state} style={{ '--pf-state-color': stateColors[index] } as CSSProperties}><div className="pf-column-title"><State value={state} /><span>{tasks.filter(task => task.state === state).length}<Plus size={13} /></span></div>{tasks.filter(task => task.state === state).map(task => <div className="pf-card" key={task.key}><div className="pf-card-title"><TaskTitle task={task} />{task.title.startsWith('Fix') ? <Bug className="pf-bug" size={16} /> : <TaskFeatureIcon />}</div><span className="pf-card-sprint"><TaskSprintIcon />Sprint 24</span>{task.blocked && <span className="pf-blocker"><LockKeyhole size={11} />Blocked by ORB-494</span>}<div className="pf-card-footer"><TaskPriorityIcon /><span>{task.points} pts</span><Owner name={task.owner} /></div></div>)}</div>)}
          </div>
          <div className="pf-list-area" data-visible={phase > 0}>
            <div className="pf-list-heading"><span>Task</span><span>State</span><span>Owner</span></div>
            <div className="pf-list-rows">{tasks.filter(task => !filtered || task.blocked).map(task => <div className="pf-row" data-blocked={task.blocked || undefined} key={task.key}><TaskTitle task={task} /><State value={task.state} /><Owner name={task.owner} /></div>)}</div>
            <div className="pf-list-total">{filtered ? '1 blocked task' : '5 tasks'}<span>Sprint 24</span></div>
            <div className="pf-dependency" data-visible={filtered}><div className="pf-dependency-label"><GitBranch size={13} />Blocked by</div><div className="pf-dependency-task"><TaskTitle task={tasks[1]} /><div><State value="In Progress" /><Owner name="Alex" /></div></div><p>Resolve duplicate events before enabling Slack alerts.</p><div className="pf-customer"><MessageSquare size={14} /><span><strong>Maya Chen · Northstar Labs</strong>“We need Slack alerts we can trust.”</span></div></div>
          </div>
          <div className="pf-context-note" data-visible={phase === 1 || phase === 2}><span className="pf-note-icon"><GitBranch size={24} /></span><strong>{phase === 1 ? 'The same work. Another view.' : 'The blocker has an owner.'}</strong><p>{phase === 1 ? 'Task states and owners stay together when you change views.' : 'Sam’s Slack alerts depend on Alex’s webhook fix. The original customer request stays linked.'}</p><span><ArrowDown size={13} />{phase === 1 ? 'Narrow the list to blocked work' : 'Ask Agent where to focus'}</span></div>
          <div className="pf-ask" data-visible={phase === 3}>
            <div className="pf-ask-header"><img src="/brand/helpin-icon-white.svg" width={27} height={27} alt="" /><span><strong>Ask Agent</strong><small>Sprint priorities</small></span><ArrowUpRight size={15} /><MoreHorizontal size={16} /><X size={15} /></div>
            <div className="pf-ask-chat"><div className="pf-question"><img src="/new/avatars/sam.webp" width={24} height={24} alt="" /><p>What needs attention in Sprint 24?</p></div><div className="pf-answer"><div className="pf-answer-label"><img src="/brand/helpin-icon-white.svg" width={19} height={19} alt="" />Ask Agent</div><p><strong>Start with Alex’s webhook fix.</strong> ORB-494 blocks Sam’s Slack alerts task for Northstar Labs.</p><div className="pf-recommendation"><span>01</span><div><strong>Unblock the customer request</strong><p>Finish ORB-494, then pick up ORB-491.</p></div></div><div className="pf-recommendation"><span>02</span><div><strong>Review the export fix</strong><p>Sam’s ORB-492 is ready for review.</p></div></div><div className="pf-sources"><span><GitBranch size={11} />3 linked tasks</span><span><MessageSquare size={11} />Customer history</span></div></div></div>
            <div className="pf-composer"><span>Ask a follow-up…</span><div><Plus size={14} /><small>Workspace context</small><span><ArrowUp size={14} /></span></div></div>
          </div>
        </div>
        <div className="pf-bottom"><span><Check size={12} />Customer history stays attached</span><span>OrbitDesk · Engineering</span></div>
      </div>
    </div>
  </div>;
}
