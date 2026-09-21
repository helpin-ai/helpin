import '../../_components/product-previews/preview-navigation.css';
import type { CSSProperties } from 'react';
import { TaskFeatureIcon, TaskIcon, TaskPriorityIcon, TaskSprintIcon } from './task-demo-icons';

type BoardTask = { key: string; title: string; points: number; owner: 'Sam' | 'Alex' | 'Jules'; type: 'feature' | 'bug' };
const selectedTask: BoardTask = { key: 'ORB-491', title: 'Add Slack alerts for failed syncs', points: 3, owner: 'Sam', type: 'feature' };
const columns: { name: string; color: string; state: 'unstarted' | 'started' | 'completed'; tasks: BoardTask[] }[] = [
  { name: 'Ready', color: '#818cf8', state: 'unstarted', tasks: [
    { key: 'ORB-493', title: 'Add recurring task templates', points: 2, owner: 'Jules', type: 'feature' },
    { key: 'ORB-501', title: 'Add a workspace activity digest', points: 2, owner: 'Sam', type: 'feature' },
  ] },
  { name: 'In Progress', color: '#d99552', state: 'started', tasks: [
    { key: 'ORB-494', title: 'Fix duplicate webhook delivery', points: 3, owner: 'Alex', type: 'bug' },
    { key: 'ORB-498', title: 'Add API retry limits', points: 2, owner: 'Alex', type: 'feature' },
  ] },
  { name: 'In Review', color: '#af73c3', state: 'started', tasks: [
    { key: 'ORB-492', title: 'Fix missing rows in CSV exports', points: 2, owner: 'Sam', type: 'bug' },
  ] },
  { name: 'Done', color: '#83b397', state: 'completed', tasks: [
    { key: 'ORB-495', title: 'Keep saved filters after refresh', points: 1, owner: 'Jules', type: 'bug' },
  ] },
];

export function ProjectAgentBadge({ agent, working = false }: { agent: 'forge' | 'lens'; working?: boolean }) {
  return <span className="phb-agent-avatar" data-working={working}>{working && <svg viewBox="0 0 100 100"><polygon points="30,2 70,2 98,30 98,70 70,98 30,98 2,70 2,30" fill="none" stroke="currentColor" strokeWidth="4" strokeLinejoin="round" /></svg>}<img src={`/new/agents/${agent}.svg`} width={24} height={24} alt="" /></span>;
}

// Geometry mirrors StateTypeIcon / TaskTypeIcon from frontend/src/lib/pmIcons.tsx.
function StateIcon({ state }: { state: 'unstarted' | 'started' | 'completed' }) {
  return <svg width={16} height={16} viewBox="0 0 24 24" fill="none" aria-hidden="true">{state === 'completed' ? <><circle cx="12" cy="12" r="8" fill="currentColor" /><path d="m8 12 3 3 5-6" stroke="var(--project-surface, #171b18)" strokeWidth="2" /></> : state === 'unstarted' ? <path d="M7 12H17" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" /> : <><circle cx="12" cy="12" r="7" stroke="currentColor" strokeWidth="2" /><path d="M12 5A7 7 0 0 1 12 19Z" fill="currentColor" /></>}</svg>;
}
function BugIcon() {
  return <svg width={18} height={18} viewBox="0 0 24 24" fill="none" className="phb-bug" aria-hidden="true"><g transform="translate(12 12.5) scale(1.18) translate(-12 -12.5)"><ellipse cx="12" cy="14.5" rx="4.25" ry="4.5" fill="currentColor" /><path d="M10.75 10.5Q9.25 8.5 8 6.5M13.25 10.5Q14.75 8.5 16 6.5M8 12.25L5.5 11.25M7.75 14.5H5M8 16.75L5.5 17.75M16 12.25L18.5 11.25M16.25 14.5H19M16 16.75L18.5 17.75" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" /><circle cx="8" cy="6.5" r=".9" fill="currentColor" /><circle cx="16" cy="6.5" r=".9" fill="currentColor" /></g></svg>;
}
function BoardPriority({ taskKey }: { taskKey: string }) {
  if (['ORB-491', 'ORB-494', 'ORB-492'].includes(taskKey)) return <TaskPriorityIcon />;
  const low = taskKey === 'ORB-502';
  return <svg width={14} height={14} viewBox="0 0 24 24" fill="currentColor" style={{ color: low ? '#7ab4d4' : '#d9ae63' }} aria-hidden="true">{low ? <rect x="10" y="9" width="4" height="10" rx="1" /> : <><rect x="6.5" y="11" width="4" height="8" rx="1" /><rect x="13.5" y="8" width="4" height="11" rx="1" /></>}</svg>;
}
function Owner({ owner }: { owner: BoardTask['owner'] }) {
  return owner === 'Sam' ? <img src="/new/avatars/sam.webp" width={24} height={24} alt="" /> : <span className={`phb-owner phb-owner-${owner.toLowerCase()}`}>{owner === 'Alex' ? 'AM' : 'JP'}</span>;
}

function BoardCard({ task, phase = 0 }: { task: BoardTask; phase?: number }) {
  const selected = task.key === 'ORB-491';
  const agent = selected && phase === 2 ? 'lens' : 'forge';
  const running = selected ? phase === 1 || phase === 2 : task.key === 'ORB-494';
  return <div className={`phb-card${selected ? ' phb-card-selected' : ''}`} data-task-key={task.key}>
    <div className="phb-card-title"><h4><span>{task.key}:</span> {task.title}</h4>{task.type === 'feature' ? <TaskFeatureIcon /> : <BugIcon />}</div>
    <div className="phb-sprint"><TaskSprintIcon />Sprint 24</div>
    {(selected || running) && <div className="phb-agent-status">{running ? <><ProjectAgentBadge agent={agent} working />{agent === 'forge' ? 'Coding agent is coding' : 'Code reviewer is reviewing'}<span className="phb-running-dot" /></> : <><TaskIcon name={'GitBranchIcon'} />{phase >= 4 ? 'Change #728 merged' : phase === 3 ? 'Awaiting Sam’s approval' : 'Ready to run Coding agent'}</>}</div>}
    <div className="phb-card-footer"><span className="phb-priority"><BoardPriority taskKey={task.key} /></span><span className="phb-estimate">{task.points} pts</span><Owner owner={task.owner} /></div>
  </div>;
}

// Focused crop of KanbanBoard / TaskCard, including its octagonal running-agent badge.
export function ProjectHeroBoard({ phase }: { phase: number }) {
  const activeColumn = phase >= 4 ? 3 : Math.min(phase, 2);
  const storyAgentRunning = phase === 1 || phase === 2;
  return <div className="project-hero-board" aria-hidden="true">
    <div className="phb-navigation preview-sidebar"><div className="phb-workspace"><span className="ps-workspace">O</span><strong>OrbitDesk</strong><TaskIcon name="ArrowRight01Icon" /></div><div className="phb-nav-body"><span><TaskIcon name="CheckListIcon" />My Work</span><span><TaskIcon name="LayoutGridIcon" />Objectives</span><span><TaskIcon name="Layers01Icon" />Roadmap</span><span><TaskIcon name="ChartColumnIcon" />Reports</span><small>YOUR TEAMS</small><span className="phb-team"><TaskIcon name="ArrowRight01Icon" />Engineering</span><div className="phb-team-items"><span className="phb-nav-active"><TaskIcon name="CheckListIcon" />Tasks</span><span><TaskIcon name="Layers01Icon" />Epics</span><span><TaskSprintIcon />Sprints</span></div><span><TaskIcon name="ArrowRight01Icon" />Support</span></div><div className="phb-search"><TaskIcon name="Search01Icon" />Search OrbitDesk</div></div>
    <div className="phb-main"><div className="phb-heading"><h3>Tasks <span>(Engineering)</span></h3><span className="phb-add"><TaskIcon name="PlusSignIcon" />Add task</span></div><div className="phb-views"><span><TaskIcon name="ViewIcon" />Views</span><span className="phb-view-active">Everything</span><span>Owned by me</span><span>Requested by me</span></div><div className="phb-toolbar"><span><TaskIcon name="FilterHorizontalIcon" />Filters</span><div className="phb-members"><Owner owner="Sam" /><Owner owner="Alex" /><Owner owner="Jules" /></div><div className="phb-agent-count"><span className="phb-agent-count-avatars"><ProjectAgentBadge agent="forge" />{storyAgentRunning && <ProjectAgentBadge agent={phase === 2 ? "lens" : "forge"} />}</span><span>{storyAgentRunning ? 2 : 1} {storyAgentRunning ? "agents" : "agent"} running</span><span className="phb-running-dot" /></div><div className="phb-group"><span>By States</span><span>By Members</span></div><TaskIcon name="LayoutGridIcon" /></div>
    <div className="phb-columns" style={{ '--active-column': activeColumn } as CSSProperties}>
      {columns.map((column, index) => <div className="phb-column" key={column.name} style={{ '--column-color': column.color } as CSSProperties}>
        <div className="phb-column-header"><div><strong><StateIcon state={column.state} />{column.name}</strong><div className="phb-totals"><span><TaskIcon name="File01Icon" />{column.tasks.length + (activeColumn === index ? 1 : 0)}</span><span><TaskIcon name="ChartColumnIcon" />{column.tasks.reduce((sum, task) => sum + task.points, 0) + (activeColumn === index ? 3 : 0)}</span></div></div><TaskIcon name="PlusSignIcon" /></div>
        <div className="phb-cards" data-active={activeColumn === index}>{column.tasks.map(task => <BoardCard key={task.key} task={task} />)}<span className="phb-add-card"><TaskIcon name="PlusSignIcon" />Add task</span></div>
      </div>)}
      <div className="phb-moving-task" data-column={activeColumn}><BoardCard task={selectedTask} phase={phase} /></div>
    </div></div>
  </div>;
}
