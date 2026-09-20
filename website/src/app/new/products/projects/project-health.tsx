'use client';

import { useId, useState } from 'react';
import { ArrowUpRight, CalendarDays, Check, FileText, Target } from 'lucide-react';

const EPICS = [
  { name: 'Sync alerts', health: 'On track', tone: 'green', done: 4, total: 6, owner: 'Sam Rivera', spec: 'Alert payloads and delivery rules', tasks: [{ key: 'ORB-491', name: 'Add Slack alerts for failed syncs', state: 'Ready' }, { key: 'ORB-497', name: 'Write the Slack alerts setup guide', state: 'In progress' }] },
  { name: 'Webhook reliability', health: 'At risk', tone: 'amber', done: 3, total: 4, owner: 'Alex Morgan', spec: 'Event deduplication and retry handling', tasks: [{ key: 'ORB-494', name: 'Fix duplicate webhook delivery', state: 'In progress' }, { key: 'ORB-496', name: 'Add duplicate-delivery regression tests', state: 'Done' }] },
  { name: 'Rollout readiness', health: 'On track', tone: 'green', done: 1, total: 2, owner: 'Jules Park', spec: 'Pilot rollout and verification checklist', tasks: [{ key: 'ORB-503', name: 'Verify pilot workspace setup', state: 'Done' }, { key: 'ORB-504', name: 'Schedule the remaining pilot rollouts', state: 'Ready' }] },
];

export function ProjectHealth() {
  const [selected, setSelected] = useState(0);
  const id = useId();
  const epic = EPICS[selected];
  const done = EPICS.reduce((sum, item) => sum + item.done, 0);
  const total = EPICS.reduce((sum, item) => sum + item.total, 0);
  return <div className="project-health po-preview">
    <div className="ph-toolbar"><span><span className="ps-workspace">O</span>OrbitDesk / Objectives</span><span>Customer outcomes</span></div>
    <div className="ph-objective"><div><span className="ph-label"><Target size={13} aria-hidden="true" />Objective</span><h3>Make integration failures easier to act on.</h3></div><span className="ph-status">Active</span></div>
    <div className="po-properties"><span><img src="/new/avatars/sam.webp" width={20} height={20} alt="" />Sam Rivera</span><span><CalendarDays size={12} />Nov 13</span><span className="po-health"><i />At risk</span></div>
    <div className="po-result"><span className="ph-label">Key result</span><strong>Enable sync alerts for 5 pilot accounts</strong><small>Outcome progress recorded by the team</small></div>
    <div className="po-metrics"><div><span>Delivery progress</span><strong>{Math.round(done / total * 100)}<small>%</small></strong><progress value={done} max={total} aria-label={`${done} of ${total} linked tasks completed`} /><p>{done} of {total} linked tasks completed</p></div><div><span>Outcome progress</span><strong>40<small>%</small></strong><progress value={2} max={5} aria-label="2 of 5 pilot accounts enabled" /><p>2 of 5 pilot accounts enabled</p></div></div>
    <div className="po-linked-label"><span>Linked epics</span><small>3 epics · {total} tasks</small></div>
    <div className="po-epics" role="group" aria-label="Explore linked epic requirements and tasks">{EPICS.map((item, index) => <button type="button" key={item.name} aria-pressed={selected === index} aria-controls={id} onClick={() => setSelected(index)}><span>{item.name}</span><span className={`ph-health ph-${item.tone}`}><i />{item.health}</span><small>{item.done}/{item.total}</small><ArrowUpRight size={12} aria-hidden="true" /></button>)}</div>
    <div className="po-epic-detail" id={id} role="region" aria-label={`${epic.name} requirements and tasks`}><div className="po-detail-heading"><strong>{epic.name}</strong><span>{epic.owner}</span></div><div className="po-spec"><FileText size={13} /><span>{epic.spec}</span></div><div className="po-task-label">Selected tasks</div><div className="po-task-list">{epic.tasks.map(task => <div key={task.key}><span className="po-task-dot" data-done={task.state === 'Done'}>{task.state === 'Done' && <Check size={8} />}</span><span><small>{task.key}</small><strong>{task.name}</strong></span><span>{task.state}</span></div>)}</div></div>
  </div>;
}
