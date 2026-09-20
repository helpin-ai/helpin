'use client';

import { useId, useState } from 'react';
import { ArrowUpRight, Check, Flag, GitBranch, Target } from 'lucide-react';

const EPICS = [
  { name: 'Sync alerts', health: 'On track', tone: 'green', done: 6, total: 8, date: 'Oct 9', update: 'Two pilot accounts are testing Slack alerts. Finish retry handling and the setup guide before the next rollout.', next: 'Review the retry-handling change.', owner: 'Sam Rivera', relationship: 'Setup guide waits on ORB-491', context: 'Maya’s original request · Northstar Labs' },
  { name: 'Webhook reliability', health: 'At risk', tone: 'amber', done: 3, total: 7, date: 'Oct 23', update: 'Duplicate deliveries are reproduced. The retry work is waiting for the event-deduplication task to finish.', next: 'Resolve the deduplication dependency.', owner: 'Sam Rivera', relationship: 'Retry handling is blocked by ORB-494', context: 'Incident notes · Linked engineering tasks' },
  { name: 'Export performance', health: 'Not set', tone: 'neutral', done: 0, total: 5, date: 'Nov 13', update: 'The next step is to reproduce the timeout against a large project and agree the scope before sprint planning.', next: 'Reproduce the customer’s export timeout.', owner: 'Alex Liu', relationship: 'ORB-492 linked to the export investigation', context: 'Customer conversation · Export requirements' },
];

export function ProjectHealth() {
  const [selected, setSelected] = useState(0);
  const id = useId();
  const epic = EPICS[selected];
  return <div className="project-health">
    <div className="ph-toolbar"><span><span className="ps-workspace">O</span>OrbitDesk / Objectives</span><span>Illustrative workspace</span></div>
    <div className="ph-objective"><div><span className="ph-label"><Target size={13} aria-hidden="true" />Customer outcome</span><h3>Make integration failures easier to act on.</h3></div><span className="ph-status">Active</span></div>
    <div className="ph-key-result"><div><span className="ph-label">Key result</span><strong>Roll out sync alerts to 5 pilot accounts</strong><span>Progress recorded by the team</span></div><div><b>2 <span>/ 5</span></b><progress max={5} value={2} aria-label="Pilot accounts using sync alerts: 2 of 5" /></div></div>
    <div className="ph-epics" role="group" aria-label="Explore example epics">{EPICS.map((item, index) => <button type="button" key={item.name} aria-pressed={selected === index} aria-controls={id} onClick={() => setSelected(index)}><span className="ph-epic-name">{item.name}<ArrowUpRight size={14} aria-hidden="true" /></span><span className={`ph-health ph-${item.tone}`}><i />{item.health}</span><span className="ph-task-progress">{item.done}/{item.total} tasks <span>Target {item.date}</span></span></button>)}</div>
    <div className="ph-update" id={id} role="region" aria-label={`${epic.name} progress and next step`}>
      <div className="ph-update-heading"><span className="ph-label">{epic.name} / Latest update</span><span>{epic.owner}</span></div><p>{epic.update}</p><div className="ph-dependency"><GitBranch size={14} aria-hidden="true" />{epic.relationship}</div><div className="ph-next"><Flag size={14} aria-hidden="true" /><span><strong>Next step</strong>{epic.next}</span></div><div className="ph-context"><Check size={13} aria-hidden="true" />{epic.context}</div>
    </div>
  </div>;
}
