import { BookOpen, Building2, Check, ListChecks, MessageSquare } from 'lucide-react';
import { WorkflowAgent, WorkflowClick, WorkflowSources } from '../../_components/WorkflowParts';

export function ProjectIntake({ phase }: { phase: number }) {
  const created = phase >= 5;
  return <div className="project-intake pi-story" data-phase={phase}>
    <div className="pi-source"><div className="pi-person"><img src="/new/avatars/maya.webp" width={30} height={30} alt="" /><span><strong>Maya Chen</strong><small>Northstar Labs · Customer conversation</small></span><MessageSquare size={15} /></div><p>“We’d like to test SSO with our admins first.”</p></div>
    <WorkflowAgent name="atlas" role="Planning agent" status={phase === 0 ? 'Thinking…' : phase < 3 ? 'Checking the request and existing work' : created ? 'PRJ-214 created' : 'Task ready for Sam’s review'} working={phase > 0 && phase < 3} />
    <div className="pi-stage">
      {phase > 0 && phase < 3 && <WorkflowSources sources={[{ icon: ListChecks, label: 'Related project work', detail: 'No admin-only pilot task exists yet' }, { icon: BookOpen, label: 'SSO rollout requirements', detail: 'Validate role mapping before wider rollout' }]} phase={phase} />}
      {phase >= 3 && <div className="pi-task wf-outcome" data-prepared="true" data-created={created}>
        <div className="pi-task-top"><span>{created ? 'PRJ-214' : 'PROPOSED TASK'}</span><span className="pi-status">{created ? <><Check size={11} />Created</> : 'For review'}</span></div>
        <h3>Add an admin-only SSO pilot</h3><p>Selected administrators can test SSO before the wider rollout.</p>
        <div className="pi-story-properties"><span>Engineering</span><span>High priority</span><span><img src="/new/avatars/sam.webp" width={17} height={17} alt="" />Sam</span><span>Sprint 24</span></div>
        <div className="pi-requirements"><ul><li>Limit access to selected administrators.</li><li>Validate group-to-role mappings.</li></ul></div>
        <div className="pi-context-links" data-linked={created}><span><MessageSquare size={12} />Maya’s request</span><span><Building2 size={12} />Northstar Labs</span></div>
        <div className="pi-story-approve">{created ? <><Check size={13} />Created after Sam’s approval</> : 'Approve & create task'}{phase === 4 && <WorkflowClick />}</div>
      </div>}
    </div>
  </div>;
}
