import type { CSSProperties } from 'react';
import { WorkflowClick } from '../../_components/WorkflowParts';
import { ArrowDown, CalendarDays, Check, CheckCheck, Flag, GitBranch, Target } from 'lucide-react';

export function SprintPlanning({ phase }: { phase: number }) {
  const planning = phase < 2;
  const moved = phase === 1 || phase >= 3;
  const closed = phase >= 3;
  return <div className="pp-sprint" data-stage={phase}>
    <div className="pp-heading"><div><span className="pp-eyebrow">ENGINEERING</span><h3>Sprint 24</h3></div><span className="pp-status" data-tone={closed ? 'sage' : 'neutral'}>{closed ? <Check size={12} /> : <i />}{closed ? 'Completed' : planning ? 'Planned' : 'Active'}</span></div>
    <div className="pp-date"><CalendarDays size={12} />Sep 21–Oct 2</div>
    <div className="pp-progress-label"><span>{planning ? <><strong>{phase === 0 ? '6' : '8'} tasks</strong> {phase === 0 ? 'already planned' : 'committed to the sprint'}</> : <><strong>6 of 8</strong> tasks completed</>}</span><strong>{planning ? (phase === 0 ? '2 selected' : 'Ready to start') : '75%'}</strong></div>
    <div className="pp-progress">{Array.from({ length: 8 }, (_, index) => <span key={index} data-done={index < 6 || phase === 1} style={{ '--pp-delay': `${index * .14}s` } as CSSProperties}>{!planning && index < 6 && <Check size={11} />}</span>)}</div>
    <div className="pp-progress-legend"><span><i />{planning ? 'Sprint commitment' : '6 completed'}</span><span><i />{planning ? 'Owners stay attached' : '2 unfinished'}</span></div>
    <div className="pp-transfer" data-carried={moved}>
      <div className="pp-source"><div className="pp-source-summary"><CheckCheck size={20} /><span><strong>{planning ? (phase === 0 ? '2 backlog tasks selected' : '8 tasks committed to Sprint 24') : 'Completed work stays recorded in Sprint 24.'}</strong><small>{planning ? (phase === 0 ? 'Add them to the 6 planned tasks' : 'Selected work is ready for the team') : (moved ? 'Two unfinished tasks carried forward' : '2 unfinished tasks to carry forward')}</small></span></div></div>
      <div className="pp-transfer-path"><span /><ArrowDown size={14} /><span className="pp-transfer-caption">{planning ? 'Backlog → Sprint 24' : 'Sprint 24 → Sprint 25'}</span><span /></div>
      <div className="pp-destination"><div><h4>{planning ? 'Sprint 24' : 'Sprint 25'}</h4><span className="pp-status" data-tone={moved ? 'sage' : 'neutral'}>{moved ? (planning ? '2 tasks added' : '2 carried over') : 'Planned'}</span></div><span className="pp-date"><CalendarDays size={11} />{planning ? 'Sep 21–Oct 2' : 'Oct 5–16'}</span></div>
      <div className="pp-carry-tasks">{[{ key: 'PRJ-214', title: 'Add an admin-only SSO pilot', owner: 'Sam' }, { key: 'PRJ-217', title: 'Prepare the SSO pilot guide', owner: 'Jules' }].map(task => <div key={task.key}><span className="pp-task-state" /><div><small>{task.key}</small><strong>{task.title}</strong></div>{task.owner === 'Sam' ? <img src="/new/avatars/sam.webp" width={22} height={22} alt="" /> : <span className="pp-owner pp-owner-jules">JP</span>}</div>)}</div>
    </div>
    {(phase === 0 || phase === 2) && <div className="pp-action-demo">{phase === 0 ? 'Add to Sprint 24' : 'Complete sprint'}<WorkflowClick delay={600} /></div>}
    <div className="pp-closeout" data-recorded={phase === 4}><CheckCheck size={14} /><span>{['Choose the next tasks from the backlog', 'Sprint commitment recorded', 'At closeout · Review the unfinished work', 'Carryover keeps the original sprint linked', 'Sprint 24 closeout recorded'][phase]}</span></div>
  </div>;
}

const epics = [
  { name: 'Identity mapping', date: 'Sep 21 – Oct 2', health: 'On track', tone: 'sage', start: 20 / 91 * 100, width: 12 / 91 * 100, owner: 'Alex', next: 'Validate the mapping before pilot enrollment.' },
  { name: 'SSO pilot', date: 'Oct 5 – Oct 16', health: 'On track', tone: 'sage', start: 34 / 91 * 100, width: 12 / 91 * 100, owner: 'Sam', next: 'Prepare the pilot controls and reviewed setup guide.' },
  { name: 'Rollout readiness', date: 'Oct 19 – Nov 13', health: 'On track', tone: 'sage', start: 48 / 91 * 100, width: 26 / 91 * 100, owner: 'Jules', next: 'Schedule and verify the remaining pilot rollouts.' },
];
export function RoadmapPlanning({ phase }: { phase: number }) {
  const focus = epics[Math.min(phase, 2)];
  return <div className="pp-roadmap">
    <div className="pp-objective"><span><Target size={19} /></span><div><small>OBJECTIVE</small><strong>Increase ARR by $500k this quarter.</strong></div><span className="pp-epic-count">3 epics</span></div>
    <div className="pp-months"><span>SEP</span><span>OCT</span><span>NOV</span></div>
    <div className="pp-roadmap-rows">{epics.map((epic, index) => <div className="pp-epic" data-focused={phase === index} data-drawn={phase >= index} key={epic.name} style={{ '--pp-start': `${epic.start}%`, '--pp-width': `${epic.width}%` } as CSSProperties}>
      <div className="pp-epic-heading"><strong>{epic.name}</strong><span className="pp-status" data-tone={epic.tone}><i />{epic.health}</span></div>
      <div className="pp-epic-meta"><span><CalendarDays size={10} />{epic.date}</span><span>{epic.owner}</span></div>
      <div className="pp-epic-track"><div className="pp-epic-bar" data-tone={epic.tone}><span /><i /><i /></div></div>
    </div>)}</div>
    <div className="pp-roadmap-focus"><span className="pp-focus-icon">{phase === 3 ? <Target size={18} /> : <Flag size={18} />}</span><div><strong>{phase === 3 ? 'Three epics. One shared objective.' : focus.name}</strong><p>{phase === 3 ? 'See how the work connects across this quarter.' : focus.next}</p></div></div>
    <div className="pp-roadmap-footer"><GitBranch size={12} /><span>Grouped by objective</span><span>Sep–Nov</span></div>
  </div>;
}
