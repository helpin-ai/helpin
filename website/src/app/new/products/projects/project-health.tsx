'use client';

import { useId, useState } from 'react';
import { ArrowRight, CalendarDays, Check, FileText, GitBranch, Target } from 'lucide-react';

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
  return <div className="projects-outcome-bento">
    <article className="pob-card pob-goal">
      <div className="pob-copy"><span className="pob-step">01 / THE GOAL</span><h3>Give the work a goal.</h3><p>Define a key result, name the owners, and set a target date.</p></div>
      <div className="pob-demo pob-goal-demo">
        <div className="pob-demo-label"><Target size={14} aria-hidden="true" /><span>Objective</span><span className="pob-badge">Active</span></div>
        <h4>Make integration failures easier to act on.</h4>
        <div className="pob-properties"><span><img src="/new/avatars/sam.webp" width={22} height={22} alt="" />Sam Rivera</span><span><CalendarDays size={13} aria-hidden="true" />Nov 13</span></div>
        <div className="pob-key-result"><span>KEY RESULT</span><strong>Enable sync alerts for 5 pilot accounts</strong></div>
      </div>
    </article>
    <article className="pob-card pob-plan">
      <div className="pob-copy"><span className="pob-step">02 / THE PLAN</span><h3>Connect the plan.</h3><p>Link epics with the requirements and tasks that move the objective forward.</p></div>
      <div className="pob-demo pob-plan-demo">
        <div className="pob-demo-label"><GitBranch size={14} aria-hidden="true" /><span>Linked epics</span><small>3 epics · {total} tasks</small></div>
        <p className="pob-hint">Select an epic to see its requirements and tasks.</p>
        <div className="pob-epics" role="group" aria-label="Explore linked epic requirements and tasks">{EPICS.map((item, index) => <button type="button" key={item.name} aria-pressed={selected === index} aria-controls={id} onClick={() => setSelected(index)}><span>{item.name}</span><span className={'pob-health pob-' + item.tone}><i />{item.health}</span><small>{item.done}/{item.total}</small><ArrowRight size={14} aria-hidden="true" /></button>)}</div>
        <div className="pob-epic-detail" id={id} role="region" aria-label={epic.name + ' requirements and tasks'}>
          <div className="pob-detail-heading"><div><span className="pob-micro">SELECTED EPIC</span><h4>{epic.name}</h4></div><span>{epic.owner}</span></div>
          <div className="pob-spec"><FileText size={16} aria-hidden="true" /><div><span className="pob-micro">REQUIREMENTS</span><p>{epic.spec}</p></div></div>
          <span className="pob-micro pob-task-label">SELECTED TASKS</span>
          <div className="pob-task-list">{epic.tasks.map(task => <div key={task.key}><span className="pob-task-dot" data-done={task.state === 'Done'}>{task.state === 'Done' && <Check size={9} aria-hidden="true" />}</span><span><small>{task.key}</small><strong>{task.name}</strong></span><span>{task.state}</span></div>)}</div>
        </div>
        <div className="pob-plan-footer"><Target size={13} aria-hidden="true" />Connected to the same objective.</div>
      </div>
    </article>
    <article className="pob-card pob-measure">
      <div className="pob-copy"><span className="pob-step">03 / THE RESULT</span><h3>Measure what changed.</h3><p>Compare completed work with outcome progress, and record health as the plan develops.</p></div>
      <div className="pob-demo pob-measure-demo">
        <div className="pob-metrics"><div><span>Delivery progress</span><strong>{Math.round(done / total * 100)}<small>%</small></strong><progress value={done} max={total} aria-label={done + ' of ' + total + ' linked tasks completed'} /><p>{done} of {total} linked tasks completed</p></div><div><span>Outcome progress</span><strong>40<small>%</small></strong><progress value={2} max={5} aria-label="2 of 5 pilot accounts enabled" /><p>2 of 5 pilot accounts enabled</p></div></div>
        <div className="pob-health-update"><span>Recorded health</span><span className="pob-health pob-amber"><i />At risk</span></div>
        <p className="pob-recorded-note">Outcome progress recorded by the team.</p>
      </div>
    </article>
  </div>;
}
