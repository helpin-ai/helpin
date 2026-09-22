'use client';

import { useId, useState } from 'react';
import { ArrowRight, CalendarDays, Check, FileText, GitBranch, Target } from 'lucide-react';

const EPICS = [
  { name: 'Identity mapping', health: 'On track', tone: 'green', done: 3, total: 3, owner: 'Alex Morgan', spec: 'Validate group-to-role mappings before pilot enrollment.', tasks: [{ key: 'PRJ-212', name: 'Validate group-to-role mappings', state: 'Done' }, { key: 'PRJ-213', name: 'Verify mapping regression tests', state: 'Done' }] },
  { name: 'SSO pilot', health: 'On track', tone: 'green', done: 2, total: 3, owner: 'Sam Rivera', spec: 'Admin-only enrollment and a reviewed setup process.', tasks: [{ key: 'PRJ-214', name: 'Add an admin-only SSO pilot', state: 'Done' }, { key: 'PRJ-217', name: 'Prepare the SSO pilot guide', state: 'Done' }] },
  { name: 'Rollout readiness', health: 'At risk', tone: 'amber', done: 1, total: 3, owner: 'Jules Park', spec: 'Help pilot accounts complete setup and review the results.', tasks: [{ key: 'PRJ-222', name: 'Verify pilot account setup', state: 'Done' }, { key: 'PRJ-223', name: 'Support the remaining pilot accounts', state: 'Ready' }] },
];

export function ProjectHealth() {
  const [selected, setSelected] = useState(1);
  const id = useId();
  const epic = EPICS[selected];
  const done = EPICS.reduce((sum, item) => sum + item.done, 0);
  const total = EPICS.reduce((sum, item) => sum + item.total, 0);
  return <div className="projects-outcome-bento">
    <article className="pob-card pob-goal">
      <div className="pob-copy"><span className="pob-step">01 / THE GOAL</span><h3>Agree on the result you’re working toward.</h3><p>Make success specific enough that the team can tell whether it happened.</p></div>
      <div className="pob-demo pob-goal-demo">
        <div className="pob-demo-label"><Target size={14} aria-hidden="true" /><span>Objective</span><span className="pob-badge">Active</span></div>
        <h4>Make enterprise SSO easier to roll out.</h4>
        <div className="pob-properties"><span><img src="/new/avatars/sam.webp" width={22} height={22} alt="" />Sam Rivera</span><span><CalendarDays size={13} aria-hidden="true" />Nov 13</span></div>
        <div className="pob-key-result"><span>KEY RESULT</span><strong>Eight pilot accounts complete SSO setup.</strong></div>
      </div>
    </article>
    <article className="pob-card pob-plan">
      <div className="pob-copy"><span className="pob-step">02 / THE PLAN</span><h3>Show which work supports the goal.</h3><p>Make the relationship between the objective and the planned work easy to follow.</p></div>
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
        <div className="pob-plan-footer"><Target size={13} aria-hidden="true" />3 epics, one objective.</div>
      </div>
    </article>
    <article className="pob-card pob-measure">
      <div className="pob-copy"><span className="pob-step">03 / THE RESULT</span><h3>Look beyond the completed tasks.</h3><p>Use the progress review to ask whether the work is producing the intended result.</p></div>
      <div className="pob-demo pob-measure-demo">
        <p className="pob-recorded-note">Example: later progress review</p><div className="pob-metrics"><div><span>Delivery progress</span><strong>{Math.round(done / total * 100)}<small>%</small></strong><progress value={done} max={total} aria-label={done + ' of ' + total + ' linked tasks completed'} /><p>{done} of {total} linked tasks completed</p></div><div><span>Outcome progress</span><strong>25<small>%</small></strong><progress value={2} max={8} aria-label="2 of 8 pilot accounts completed setup" /><p>2 of 8 pilot accounts completed setup</p></div></div>
        <div className="pob-health-update"><span>Recorded health</span><span className="pob-health pob-amber"><i />At risk</span></div>
        <p className="pob-recorded-note">The feature is available. Most pilot accounts still need to complete setup.</p>
      </div>
    </article>
  </div>;
}
