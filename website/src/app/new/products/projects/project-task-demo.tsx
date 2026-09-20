'use client';

import { useId, useState } from 'react';
import { Bug, CalendarDays, Check, Circle, Columns3, List, ListFilter, SignalHigh } from 'lucide-react';

const TASKS = [
  { id: 'ORB-491', title: 'Add Slack alerts for failed syncs', type: 'Feature', state: 'Planned', label: 'Integrations', owner: 'Sam Rivera', initials: 'SR', date: 'Sep 25', priority: 'High' },
  { id: 'ORB-492', title: 'Fix CSV export timeout on large projects', type: 'Bug', state: 'Planned', label: 'Reporting', owner: 'Alex Liu', initials: 'AL', date: 'Sep 26', priority: 'High' },
  { id: 'ORB-494', title: 'Fix duplicate webhook deliveries', type: 'Bug', state: 'In progress', label: 'Integrations', owner: 'Sam Rivera', initials: 'SR', date: 'Sep 21', priority: 'High' },
  { id: 'ORB-498', title: 'Draft the Slack alert setup guide', type: 'Chore', state: 'In progress', label: 'Docs', owner: 'Jordan Shah', initials: 'JS', date: 'Sep 24', priority: 'Medium' },
  { id: 'ORB-495', title: 'Preserve filters after page refresh', type: 'Bug', state: 'In review', label: 'Projects', owner: 'Alex Liu', initials: 'AL', date: 'Sep 23', priority: 'Medium' },
  { id: 'ORB-496', title: 'Add workspace audit log export', type: 'Feature', state: 'In review', label: 'Security', owner: 'Sam Rivera', initials: 'SR', date: 'Sep 23', priority: 'Medium' },
];
function Owner({ task }: { task: typeof TASKS[number] }) { return task.initials === 'SR' ? <img className="ptd-avatar" src="/new/avatars/sam.webp" width={23} height={23} alt={task.owner} title={task.owner} /> : <span className={`ptd-avatar ptd-avatar-${task.initials}`} role="img" aria-label={task.owner} title={task.owner}>{task.initials}</span>; }
export function ProjectTaskDemo() {
  const [view, setView] = useState<'board'|'list'>('board');
  const [bugs, setBugs] = useState(false);
  const id = useId();
  const tasks = TASKS.filter(task => !bugs || task.type === 'Bug');
  return <div className="project-task-demo">
    <div className="ptd-header"><div><span className="ptd-workspace">O</span><strong>OrbitDesk</strong><span className="ptd-divider">/</span><span>Engineering</span></div><span className="ptd-count" aria-live="polite">{tasks.length} tasks</span></div>
    <div className="ptd-toolbar"><div className="ptd-heading"><h3>Tasks</h3><span>Everything</span></div><div className="ptd-controls"><button type="button" className="ptd-filter" aria-pressed={bugs} aria-controls={id} onClick={() => setBugs(!bugs)}><ListFilter size={14} />Bugs only{bugs && <Check size={12} />}</button><div className="ptd-view-switch" role="group" aria-label="Task view"><button type="button" aria-pressed={view === 'board'} aria-controls={id} onClick={() => setView('board')}><Columns3 size={14} />Board</button><button type="button" aria-pressed={view === 'list'} aria-controls={id} onClick={() => setView('list')}><List size={14} />List</button></div></div></div>
    <div id={id} className="ptd-content" role="region" aria-label={`${view === 'board' ? 'Board' : 'List'} of example project tasks`}>
      {view === 'board' ? <div className="ptd-board">{['Planned','In progress','In review'].map((state,index) => <div className={`ptd-column ptd-state-${index}`} key={state}><div className="ptd-column-title"><Circle size={12} /><strong>{state}</strong><span>{tasks.filter(task => task.state === state).length}</span></div>{tasks.filter(task => task.state === state).map(task => <article className="ptd-card" key={task.id}><span className="ptd-key">{task.id}</span><h4>{task.title}</h4><div className="ptd-tags"><span data-type={task.type}>{task.type === 'Bug' && <Bug size={10} />}{task.type}</span><span>{task.label}</span></div><div className="ptd-meta"><span><CalendarDays size={12} />{task.date}</span><span title={`${task.priority} priority`}><SignalHigh size={13} /><span className="ptd-priority-label">{task.priority}</span></span><Owner task={task} /></div></article>)}</div>)}</div> : <div className="ptd-list">{tasks.map(task => <article key={task.id}><span className="ptd-key">{task.id}</span><h4>{task.title}</h4><span className="ptd-list-state">{task.state}</span><span className="ptd-list-type" data-type={task.type}>{task.type}</span><Owner task={task} /></article>)}</div>}
    </div>
    <div className="ptd-footer"><span><Check size={12} />The same tasks, in the view you need.</span><span>Owners · Priorities · Labels · Dates</span></div>
  </div>;
}
