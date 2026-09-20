'use client';

import { useEffect, useId, useRef, useState } from 'react';
import { Bug, CalendarDays, Check, Circle, Columns3, List, ListFilter, SignalHigh, MessageSquare, Link2, X } from 'lucide-react';

const TASKS = [
  { id: 'ORB-491', title: 'Add Slack alerts for failed syncs', type: 'Feature', state: 'Planned', label: 'Integrations', owner: 'Sam Rivera', initials: 'SR', date: 'Sep 25', priority: 'High' },
  { id: 'ORB-492', title: 'Fix CSV export timeout on large projects', type: 'Bug', state: 'Planned', label: 'Reporting', owner: 'Alex Liu', initials: 'AL', date: 'Sep 26', priority: 'High' },
  { id: 'ORB-494', title: 'Fix duplicate webhook deliveries', type: 'Bug', state: 'In progress', label: 'Integrations', owner: 'Sam Rivera', initials: 'SR', date: 'Sep 21', priority: 'High' },
  { id: 'ORB-498', title: 'Draft the Slack alert setup guide', type: 'Chore', state: 'In progress', label: 'Docs', owner: 'Jordan Shah', initials: 'JS', date: 'Sep 24', priority: 'Medium' },
  { id: 'ORB-495', title: 'Preserve filters after page refresh', type: 'Bug', state: 'In review', label: 'Projects', owner: 'Alex Liu', initials: 'AL', date: 'Sep 23', priority: 'Medium' },
  { id: 'ORB-496', title: 'Add workspace audit log export', type: 'Feature', state: 'In review', label: 'Security', owner: 'Sam Rivera', initials: 'SR', date: 'Sep 23', priority: 'Medium' },
];
const TASK_CONTEXT: Record<string, { source: string; request: string; checklist: string[]; related: string }> = {
  'ORB-491': { source: 'Maya Chen · Northstar Labs', request: 'Can you alert our team in Slack when a sync fails? We only notice when a customer reports it.', checklist: ['Include the affected account and failed sync.', 'Link the alert to the investigation.', 'Document how to connect the Slack channel.'], related: 'Blocks ORB-498 · Slack alert setup guide' },
  'ORB-492': { source: 'Customer conversation · Reporting', request: 'Large project exports time out before our team can download the CSV.', checklist: ['Reproduce the timeout on a large project.', 'Keep the exported columns and filters intact.', 'Add a regression test for large exports.'], related: 'Epic · Export performance' },
  'ORB-494': { source: 'Engineering investigation · Integrations', request: 'Retries can deliver the same webhook event more than once. Make repeated deliveries safe to handle.', checklist: ['Reproduce a duplicate delivery.', 'Deduplicate matching event identifiers.', 'Test retry and failure cases.'], related: 'Blocks retry handling · Webhook reliability' },
  'ORB-498': { source: 'Linked request · Northstar Labs', request: 'Give the pilot team a clear guide for connecting Slack and understanding a failed-sync alert.', checklist: ['Describe the channel setup.', 'Show a sample alert and investigation link.', 'Review the guide against the released behavior.'], related: 'Blocked by ORB-491 · Slack alerts' },
  'ORB-495': { source: 'Product feedback · Projects', request: 'The team loses its selected filters after refreshing the project view.', checklist: ['Retain the selected filters on refresh.', 'Restore the correct board or list view.', 'Check behavior when a filter no longer exists.'], related: 'Linked document · Project view behavior' },
  'ORB-496': { source: 'Security review · Customer requirements', request: 'Prepare a workspace audit log export that administrators can review outside the app.', checklist: ['Confirm who can export workspace events.', 'Include timestamps and actor details.', 'Verify the output against the audit log.'], related: 'Linked document · Audit export requirements' },
};
function Owner({ task }: { task: typeof TASKS[number] }) { return task.initials === 'SR' ? <img className="ptd-avatar" src="/new/avatars/sam.webp" width={23} height={23} alt={task.owner} title={task.owner} /> : <span className={`ptd-avatar ptd-avatar-${task.initials}`} role="img" aria-label={task.owner} title={task.owner}>{task.initials}</span>; }
export function ProjectTaskDemo() {
  const [view, setView] = useState<'board'|'list'>('board');
  const [bugs, setBugs] = useState(false);
  const [selected, setSelected] = useState<typeof TASKS[number] | null>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const opener = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    if (!selected || !dialog.current) return;
    dialog.current.showModal();
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => { document.body.style.overflow = previous; };
  }, [selected]);
  const openTask = (task: typeof TASKS[number], button: HTMLButtonElement) => { opener.current = button; setSelected(task); };
  const closeTask = () => { setSelected(null); opener.current?.focus(); };
  const context = selected ? TASK_CONTEXT[selected.id] : null;
  const id = useId();
  const tasks = TASKS.filter(task => !bugs || task.type === 'Bug');
  return <div className="project-task-demo">
    <div className="ptd-header"><div><span className="ptd-workspace">O</span><strong>OrbitDesk</strong><span className="ptd-divider">/</span><span>Engineering</span></div><span className="ptd-count" aria-live="polite">{tasks.length} tasks</span></div>
    <div className="ptd-toolbar"><div className="ptd-heading"><h3>Tasks</h3><span>Everything</span></div><div className="ptd-controls"><button type="button" className="ptd-filter" aria-pressed={bugs} aria-controls={id} onClick={() => setBugs(!bugs)}><ListFilter size={14} />Bugs only{bugs && <Check size={12} />}</button><div className="ptd-view-switch" role="group" aria-label="Task view"><button type="button" aria-pressed={view === 'board'} aria-controls={id} onClick={() => setView('board')}><Columns3 size={14} />Board</button><button type="button" aria-pressed={view === 'list'} aria-controls={id} onClick={() => setView('list')}><List size={14} />List</button></div></div></div>
    <div id={id} className="ptd-content" role="region" aria-label={`${view === 'board' ? 'Board' : 'List'} of example project tasks`}>
      {view === 'board' ? <div className="ptd-board">{['Planned','In progress','In review'].map((state,index) => <div className={`ptd-column ptd-state-${index}`} key={state}><div className="ptd-column-title"><Circle size={12} /><strong>{state}</strong><span>{tasks.filter(task => task.state === state).length}</span></div>{tasks.filter(task => task.state === state).map(task => <article className="ptd-card" key={task.id}><span className="ptd-key">{task.id}</span><h4><button type="button" onClick={event => openTask(task, event.currentTarget)} aria-haspopup="dialog" aria-label={`Open ${task.id}: ${task.title}`}>{task.title}</button></h4><div className="ptd-tags"><span data-type={task.type}>{task.type === 'Bug' && <Bug size={10} />}{task.type}</span><span>{task.label}</span></div><div className="ptd-meta"><span><CalendarDays size={12} />{task.date}</span><span title={`${task.priority} priority`}><SignalHigh size={13} /><span className="ptd-priority-label">{task.priority}</span></span><Owner task={task} /></div></article>)}</div>)}</div> : <div className="ptd-list">{tasks.map(task => <article key={task.id}><span className="ptd-key">{task.id}</span><h4><button type="button" onClick={event => openTask(task, event.currentTarget)} aria-haspopup="dialog" aria-label={`Open ${task.id}: ${task.title}`}>{task.title}</button></h4><span className="ptd-list-state">{task.state}</span><span className="ptd-list-type" data-type={task.type}>{task.type}</span><Owner task={task} /></article>)}</div>}
    </div>
    <dialog ref={dialog} className="project-task-dialog" aria-labelledby={`${id}-detail-title`} onClose={closeTask} onClick={event => { if (event.target === event.currentTarget) dialog.current?.close(); }}>
      {selected && context && <div className="ptd-detail-body"><div className="ptd-detail-top"><span>{selected.id} <span>/ Illustrative task</span></span><button type="button" autoFocus aria-label="Close task details" onClick={() => dialog.current?.close()}><X size={18} aria-hidden="true" /></button></div><h2 id={`${id}-detail-title`}>{selected.title}</h2><div className="ptd-detail-properties"><span>{selected.state}</span><span>{selected.priority} priority</span><span>Due {selected.date}</span><span>{selected.owner}</span></div><div className="ptd-detail-request"><span><MessageSquare size={14} aria-hidden="true" />{context.source}</span><p>{context.request}</p></div><h3>What needs to be true</h3><ul>{context.checklist.map(item => <li key={item}><span className="ptd-checklist-box" aria-hidden="true" />{item}</li>)}</ul><div className="ptd-detail-related"><Link2 size={15} aria-hidden="true" />{context.related}</div><p className="ptd-detail-note">The request, requirements, and related work stay with the task.</p></div>}
    </dialog>
    <div className="ptd-footer"><span><Check size={12} />The same tasks, in the view you need.</span><span>Illustrative workspace · Open a task to explore</span></div>
  </div>;
}
