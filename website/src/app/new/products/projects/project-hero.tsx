'use client';

import { useEffect, useState, type CSSProperties, type ReactNode } from 'react';
import { Check, CheckCheck, GitMerge, GitPullRequest, Mail, MessageSquare, Pause, Play, Rocket } from 'lucide-react';
import { ProjectAgentBadge, ProjectHeroBoard } from './project-hero-board';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './project-hero.css';
import { TaskCalendarIcon, TaskFeatureIcon, TaskIcon, TaskPriorityIcon, TaskSprintIcon, TaskTickIcon, type TaskIconName } from './task-demo-icons';

function Property({ icon, label, children }: { icon: TaskIconName | ReactNode; label: string; children: ReactNode }) {
  return <div className="pth-property"><span className="pth-property-icon">{typeof icon === 'string' ? <TaskIcon name={icon as TaskIconName} /> : icon}</span><span className="pth-property-label">{label}</span><span className="pth-property-value">{children}</span></div>;
}

// Presentational snapshot of TaskDetailPanel / QuietDetailHeader, using the same
// Inter font, field order and icon geometry, with marketing surface colors. No app mutations.
export function ProjectHero() {
  const { container, playing, cycle } = useBentoPlayback(30000);
  const [paused, setPaused] = useState(false);
  const active = playing && !paused;
  const [frame, setFrame] = useState(7);
  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timers = [2500, 6500, 10500, 14000, 17500, 21000, 23000].map((time, index) => setTimeout(() => setFrame(index + 1), time));
    return () => timers.forEach(clearTimeout);
  }, [active, cycle]);
  // Server, reduced-motion and paused views show the complete delivery history.
  const phase = active ? frame : 7;
  const state = ['Ready', 'In Progress', 'In Review', 'In Review', 'Done'][Math.min(phase, 4)];
  const colors = ['#818cf8', '#d99552', '#af73c3', '#af73c3', '#83b397'];
  const captions = ['TASK CREATED · CUSTOMER CONTEXT ATTACHED', 'FORGE BUILDS · TASK MOVES TO IN PROGRESS', 'LENS REVIEWS · TASK MOVES TO IN REVIEW', 'SAM REVIEWS · MERGE AWAITS APPROVAL', 'PR MERGED · TASK COMPLETE', 'RELEASE PUBLISHED · FOLLOW-UP PREPARED', 'SAM APPROVES · CUSTOMER UPDATE READY', 'CUSTOMER NOTIFIED · LOOP CLOSED'];
  const delay = (seconds: number) => ({ '--task-delay': `${seconds}s` }) as CSSProperties;
  return <div className="project-hero-art" ref={container} data-playing={active} data-phase={phase}>
    <div className="project-hero-art-label"><span>{captions[phase]}</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} project delivery animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div className="project-hero-stage" key={cycle} role="img" aria-label="Illustrative OrbitDesk delivery workflow. A customer request becomes task ORB-491, Add Slack alerts for failed syncs. Forge builds and tests the change as the task moves to In Progress. Lens reviews it in In Review. Sam approves the pull request before PR #728 merges and the task moves to Done. Another task shows Forge already running. The opened task retains the customer requirements and delivery history. After the release is published, a configured follow-up workflow asks Ask Agent to prepare Maya’s update. Sam approves the message, then it is sent to Maya in the original conversation.">
      <ProjectHeroBoard phase={phase} />
      <div className="project-hero-task" aria-hidden="true"><div>
        <div className="pth-header">
          <div className="pth-breadcrumb"><span><TaskIcon name="CheckListIcon" />Tasks</span><TaskIcon name="ArrowRight01Icon" /><span><TaskIcon name="Layers01Icon" />Sync reliability</span><TaskIcon name="ArrowRight01Icon" /><span><TaskSprintIcon />Sprint 24</span></div>
          <h2>Add Slack alerts for failed syncs</h2>
          <div className="pth-meta"><span>ORB-491</span><i />Engineering</div>
          <div className="pth-actions"><div><TaskIcon name="Link01Icon" /><TaskIcon name="MoreVerticalIcon" /><TaskIcon name="ArrowUpRight01Icon" /><TaskIcon name="Cancel01Icon" /></div><span className="pth-saved"><TaskTickIcon />All changes saved</span></div>
        </div>
        <div className="pth-layout">
          <div className="pth-main">
            <div className="pth-tabs"><span className={phase === 0 ? "pth-tab-active" : ""}>Overview</span><span className={phase > 0 ? "pth-tab-active" : ""}>Delivery</span></div>
            <div className="pth-description" hidden={phase > 0}>
              <div className="pth-intro"><p>Notify the team in Slack when a sync fails, so they can investigate before a customer reports it.</p><span className="pth-edit"><TaskIcon name="PencilEdit01Icon" /></span></div>
              <p>Maya at Northstar Labs needs the affected account, the error, and a link to investigate in each alert.</p>
              <h3>Workflow</h3>
              <ol><li>Detect a failed sync.</li><li>Send an alert to the connected Slack channel.</li><li>Open the sync details from the alert.</li></ol>
              <h3>Acceptance criteria</h3>
              <ul className="pth-criteria"><li style={delay(2.8)}>Include the account and failure reason.</li><li style={delay(3.55)}>Link directly to the affected sync.</li><li style={delay(4.3)}>Send one alert per failed sync.</li></ul>
            </div>
            <div className="pth-delivery" hidden={phase === 0}>
              <div className="pth-delivery-repo"><TaskIcon name="GitBranchIcon" /><span>orbitdesk / app<small>orb-491-slack-alerts → main</small></span></div>
              <p className="pth-delivery-context">Slack alerts for Northstar Labs. The customer request stays attached through every handoff.</p>
              <div className="pth-delivery-timeline">
                <div data-complete="true"><span className="pth-delivery-dot"><Check size={10} /></span><div><strong>Task created</strong><p>Sam linked Maya’s request and acceptance criteria.</p></div></div>
                <div data-complete={phase > 1} data-current={phase === 1}><span className="pth-delivery-dot">{phase > 1 && <Check size={10} />}</span><div><strong><ProjectAgentBadge agent="forge" working={phase === 1} />Forge <small>{phase === 1 ? 'running' : 'completed'}</small></strong><p>{phase === 1 ? 'Building Slack alerts and adding regression tests.' : 'Slack alerts implemented. Regression tests added.'}</p></div></div>
                <div data-complete={phase > 2} data-current={phase === 2} data-pending={phase < 2}><span className="pth-delivery-dot">{phase > 2 && <Check size={10} />}</span><div><strong><ProjectAgentBadge agent="lens" working={phase === 2} />Lens <small>{phase < 2 ? 'queued' : phase === 2 ? 'running' : 'completed'}</small></strong><p>{phase > 2 ? 'Code and test coverage reviewed. Ready for Sam.' : 'Review the change against the customer requirements.'}</p></div></div>
                <div data-complete={phase >= 4} data-current={phase === 3} data-pending={phase < 3}><span className="pth-delivery-dot">{phase >= 4 && <Check size={10} />}</span><div><strong><img src="/new/avatars/sam.webp" width={22} height={22} alt="" />Sam Rivera <small>{phase >= 4 ? 'approved' : phase === 3 ? 'reviewing' : 'pending'}</small></strong><p>{phase >= 4 ? 'Reviewed the changes and approved the merge.' : 'Review PR #728 before merging into main.'}</p></div></div>
                <div data-complete={phase >= 4} data-pending={phase < 4}><span className="pth-delivery-dot">{phase >= 4 && <Check size={10} />}</span><div><strong>{phase >= 4 ? <GitMerge size={18} /> : <GitPullRequest size={18} />}PR #728 <small>{phase >= 4 ? 'merged' : 'awaiting approval'}</small></strong><p>{phase >= 4 ? 'Merged into main. ORB-491 moved to Done.' : 'Add Slack alerts for failed syncs'}</p></div></div>
              </div>
            </div>
          </div>
          <div className="pth-sidebar">
            <div className="pth-id"><span>Task ID:</span><strong>ORB-491</strong><TaskIcon name="Copy01Icon" /><TaskIcon name="GitBranchIcon" /></div>
            <div className="pth-property-group"><Property icon="UserGroupIcon" label="Team">Engineering</Property><Property icon="HashtagIcon" label="State"><i className="pth-state" style={{ background: colors[Math.min(phase, 4)] }} />{state}</Property></div>
            <div className="pth-property-group"><Property icon="UserIcon" label="Owners"><img src="/new/avatars/sam.webp" width={18} height={18} alt="" />Sam Rivera</Property><Property icon="UserIcon" label="Requester"><img src="/new/avatars/sam.webp" width={18} height={18} alt="" />Sam Rivera</Property></div>
            <div className="pth-property-group"><Property icon="DashboardSpeed01Icon" label="Priority"><TaskPriorityIcon />High</Property><Property icon="HashtagIcon" label="Type"><TaskFeatureIcon />Feature</Property><Property icon="Tag01Icon" label="Labels"><span className="pth-label">+ Add label</span></Property></div>
            <div className="pth-property-group"><Property icon="Layers01Icon" label="Epic"><span className="pth-epic">Sync reliability</span></Property><Property icon={<TaskSprintIcon />} label="Sprint">Sprint 24</Property></div>
            <div className="pth-property-group"><Property icon="LayoutGridIcon" label="Estimate">3 points</Property><Property icon={<TaskCalendarIcon />} label="Due date">Sep 25</Property></div>
            <div className="pth-property-group"><Property icon={<TaskSprintIcon />} label="Recurrence">None</Property></div>
            <div className="pth-rail-section"><span><TaskIcon name="ArrowRight01Icon" />Delivery</span><span>orbitdesk/app</span></div>
            <div className="pth-rail-section"><span><TaskIcon name="ArrowRight01Icon" />Related</span></div>
          </div>
        </div>
      </div>
    </div>
      <div className="pth-followup" data-visible={phase >= 5} data-approved={phase >= 6} data-sent={phase === 7} aria-hidden="true">
        <div className="pth-followup-header"><img src="/brand/helpin-icon-white.svg" width={26} height={26} alt="" /><span><strong>Ask Agent</strong><small>Release follow-up</small></span><span className="pth-followup-status">{phase === 7 ? <><CheckCheck size={12} />Sent</> : phase === 6 ? <><Check size={12} />Approved</> : 'For review'}</span></div>
        <div className="pth-followup-body"><div className="pth-release"><Rocket size={13} /><span>Release published</span><small>ORB-491</small></div><p className="pth-followup-explainer">Your follow-up workflow found the customer waiting for this change.</p><div className="pth-followup-customer"><img src="/new/avatars/maya.webp" width={25} height={25} alt="" /><span><strong>Maya Chen</strong><small>Northstar Labs · Original conversation</small></span></div><div className="pth-followup-message"><div><Mail size={13} /><strong>{phase === 7 ? 'Customer update sent' : 'Customer update prepared'}</strong></div><p>Hi Maya — Slack alerts for failed syncs are ready. Each alert includes the affected account, the failure reason, and a link to investigate.</p></div><div className="pth-followup-approval"><img src="/new/avatars/sam.webp" width={21} height={21} alt="" /><span>{phase >= 6 ? 'Approved by Sam' : 'Waiting for Sam’s approval'}</span><Check size={13} /></div></div>
        <div className="pth-followup-footer">{phase === 7 ? <CheckCheck size={14} /> : <MessageSquare size={14} />}<span>{phase === 7 ? 'Sent to Maya · Customer loop closed' : 'Reply to Maya’s original conversation'}</span></div>
      </div>
    </div>
  </div>;
}
