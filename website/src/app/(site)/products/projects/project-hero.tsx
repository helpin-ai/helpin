'use client';

import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { Check, CheckCheck, GitMerge, GitPullRequest, Mail, MessageSquare, Pause, Play, Rocket } from 'lucide-react';
import { ProjectAgentBadge, ProjectHeroBoard } from './project-hero-board';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './project-hero.css';
import './project-hero-light.css';
import { TaskCalendarIcon, TaskFeatureIcon, TaskIcon, TaskPriorityIcon, TaskSprintIcon, TaskTickIcon, type TaskIconName } from './task-demo-icons';

function Property({ icon, label, children }: { icon: TaskIconName | ReactNode; label: string; children: ReactNode }) {
  return <div className="pth-property"><span className="pth-property-icon">{typeof icon === 'string' ? <TaskIcon name={icon as TaskIconName} /> : icon}</span><span className="pth-property-label">{label}</span><span className="pth-property-value">{children}</span></div>;
}

// Presentational snapshot of TaskDetailPanel / QuietDetailHeader, using the same
// Inter font, field order and icon geometry, with marketing surface colors. No app mutations.
export function ProjectHero({ theme = 'dark' }: { theme?: 'light' | 'dark' }) {
  const { container, playing, cycle, reducedMotion } = useBentoPlayback(30000);
  const [paused, setPaused] = useState(false);
  const active = playing && !paused;
  const [frame, setFrame] = useState(7);
  const elapsed = useRef(0);
  const lastCycle = useRef(cycle);
  useEffect(() => {
    if (lastCycle.current !== cycle) {
      lastCycle.current = cycle;
      elapsed.current = 0;
      setFrame(0);
    }
    if (!active) return;
    if (elapsed.current === 0) setFrame(0);
    const started = performance.now();
    const timings = [2500, 6500, 10500, 14000, 17500, 21000, 23000];
    const timers = timings.flatMap((time, index) => time > elapsed.current
      ? [setTimeout(() => setFrame(index + 1), time - elapsed.current)] : []);
    return () => {
      elapsed.current += performance.now() - started;
      timers.forEach(clearTimeout);
    };
  }, [active, cycle]);
  // Visibility pauses preserve the task's column. Explicit pause and reduced
  // motion show the completed story; pressing play starts a fresh run.
  const phase = paused || reducedMotion ? 7 : frame;
  function togglePlayback() {
    if (paused) { elapsed.current = 0; setFrame(0); }
    setPaused(value => !value);
  }
  const state = ['Ready', 'In Progress', 'In Review', 'In Review', 'Done'][Math.min(phase, 4)];
  const colors = ['#818cf8', '#d99552', '#af73c3', '#af73c3', '#83b397'];
  const captions = ['TASK CREATED · CUSTOMER HISTORY ATTACHED', 'CODING AGENT BUILDS · TASK MOVES TO IN PROGRESS', 'CODE REVIEWER CHECKS · TASK MOVES TO IN REVIEW', 'SAM REVIEWS · MERGE AWAITS APPROVAL', 'CHANGE MERGED · TASK COMPLETE', 'RELEASE PUBLISHED · FOLLOW-UP PREPARED', 'SAM APPROVES · CUSTOMER UPDATE READY', 'CUSTOMER NOTIFIED · LOOP CLOSED'];
  const delay = (seconds: number) => ({ '--task-delay': `${seconds}s` }) as CSSProperties;
  return <div className="project-hero-art" data-theme={theme} ref={container} data-playing={active} data-phase={phase}>
    <div className="project-hero-art-label"><span>{captions[phase]}</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} project delivery animation`} aria-pressed={paused} onClick={togglePlayback}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div className="project-hero-stage" key={cycle} role="img" aria-label="Illustrative OrbitDesk delivery workflow. PRJ-214 adds an admin-only SSO pilot for Northstar Labs. Sam reviews the plan and linked rollout discussion. Coding agent prepares pilot controls and tests; Code reviewer checks them. Sam reviews the proposed changes before merging. The team separately confirms the release. Only after release does Sam approve the customer update, which is sent to Maya with the setup guide and checklist.">
      <ProjectHeroBoard phase={phase} />
      <div className="project-hero-task" aria-hidden="true"><div>
        <div className="pth-header">
          <div className="pth-breadcrumb"><span><TaskIcon name="CheckListIcon" />Tasks</span><TaskIcon name="ArrowRight01Icon" /><span><TaskIcon name="Layers01Icon" />SSO pilot</span><TaskIcon name="ArrowRight01Icon" /><span><TaskSprintIcon />Sprint 24</span></div>
          <h2>Add an admin-only SSO pilot</h2>
          <div className="pth-meta"><span>PRJ-214</span><i />Engineering</div>
          <div className="pth-actions"><div><TaskIcon name="Link01Icon" /><TaskIcon name="MoreVerticalIcon" /><TaskIcon name="ArrowUpRight01Icon" /><TaskIcon name="Cancel01Icon" /></div><span className="pth-saved"><TaskTickIcon />All changes saved</span></div>
        </div>
        <div className="pth-layout">
          <div className="pth-main">
            <div className="pth-tabs"><span className={phase === 0 ? "pth-tab-active" : ""}>Overview</span><span className={phase > 0 ? "pth-tab-active" : ""}>Delivery</span></div>
            <div className="pth-description" hidden={phase > 0}>
              <div className="pth-intro"><p>Let Northstar test SSO with selected administrators before opening it to the rest of the company.</p><span className="pth-edit"><TaskIcon name="PencilEdit01Icon" /></span></div>
              <p>In the rollout review, Maya agreed to an admin-only pilot. Group-to-role mapping still needs validation before the pilot begins.</p>

              <h3>Acceptance criteria</h3>
              <ul className="pth-criteria"><li style={delay(2.8)}>Only selected administrators join the pilot.</li><li style={delay(3.55)}>Group-to-role mappings are validated before enrollment.</li><li style={delay(4.3)}>Access stays unchanged for everyone outside the pilot.</li></ul>
            </div>
            <div className="pth-delivery" hidden={phase === 0}>
              <div className="pth-delivery-repo"><TaskIcon name="GitBranchIcon" /><span>orbitdesk / app<small>prj-214-sso-pilot → main</small></span></div>
              <p className="pth-delivery-context">Northstar’s admin-only SSO pilot. The rollout discussion stays attached through every handoff.</p>
              <div className="pth-delivery-timeline">
                <div data-complete="true"><span className="pth-delivery-dot"><Check size={10} /></span><div><strong>Plan reviewed</strong><p>Sam confirmed the scope and linked the rollout discussion.</p></div></div>
                <div data-complete={phase > 1} data-current={phase === 1}><span className="pth-delivery-dot">{phase > 1 && <Check size={10} />}</span><div><strong><ProjectAgentBadge agent="forge" working={phase === 1} />Coding agent <small>{phase === 1 ? 'running' : 'completed'}</small></strong><p>{phase === 1 ? 'Preparing pilot controls and tests.' : 'Change prepared · Coding agent added the pilot controls and tests.'}</p></div></div>
                <div data-complete={phase > 2} data-current={phase === 2} data-pending={phase < 2}><span className="pth-delivery-dot">{phase > 2 && <Check size={10} />}</span><div><strong><ProjectAgentBadge agent="lens" working={phase === 2} />Code reviewer <small>{phase < 2 ? 'queued' : phase === 2 ? 'running' : 'completed'}</small></strong><p>{phase > 2 ? 'Code and test coverage reviewed. Ready for Sam.' : 'Review the change against the customer requirements.'}</p></div></div>
                <div data-complete={phase >= 4} data-current={phase === 3} data-pending={phase < 3}><span className="pth-delivery-dot">{phase >= 4 && <Check size={10} />}</span><div><strong><img src="/new/avatars/sam.webp" width={22} height={22} alt="" />Sam Rivera <small>{phase >= 4 ? 'approved' : phase === 3 ? 'reviewing' : 'pending'}</small></strong><p>{phase >= 4 ? 'Review completed · Agent findings and proposed changes reviewed by Sam.' : 'Review Change #728 before merging into main.'}</p></div></div>
                <div data-complete={phase >= 4} data-pending={phase < 4}><span className="pth-delivery-dot">{phase >= 4 && <Check size={10} />}</span><div><strong>{phase >= 4 ? <GitMerge size={18} /> : <GitPullRequest size={18} />}Change #728 <small>{phase >= 4 ? 'merged' : 'awaiting approval'}</small></strong><p>{phase >= 4 ? 'Merged into main. PRJ-214 moved to Done.' : 'Add an admin-only SSO pilot'}</p></div></div>
                <div data-complete={phase >= 5} data-pending={phase < 5}><span className="pth-delivery-dot">{phase >= 5 && <Check size={10} />}</span><div><strong><Rocket size={18} />{phase >= 5 ? 'Release confirmed' : 'Release pending'}</strong><p>{phase >= 5 ? 'The team released the approved change.' : 'A merged change still needs release confirmation.'}</p></div></div>
              </div>
            </div>
          </div>
          <div className="pth-sidebar">
            <div className="pth-id"><span>Task ID:</span><strong>PRJ-214</strong><TaskIcon name="Copy01Icon" /><TaskIcon name="GitBranchIcon" /></div>
            <div className="pth-property-group"><Property icon="UserGroupIcon" label="Team">Engineering</Property><Property icon="HashtagIcon" label="State"><i className="pth-state" style={{ background: colors[Math.min(phase, 4)] }} />{state}</Property></div>
            <div className="pth-property-group"><Property icon="UserIcon" label="Owners"><img src="/new/avatars/sam.webp" width={18} height={18} alt="" />Sam Rivera</Property><Property icon="UserIcon" label="Requester"><img src="/new/avatars/sam.webp" width={18} height={18} alt="" />Sam Rivera</Property></div>
            <div className="pth-property-group"><Property icon="DashboardSpeed01Icon" label="Priority"><TaskPriorityIcon />High</Property><Property icon="HashtagIcon" label="Type"><TaskFeatureIcon />Feature</Property><Property icon="Tag01Icon" label="Labels"><span className="pth-label">+ Add label</span></Property></div>
            <div className="pth-property-group"><Property icon="Layers01Icon" label="Epic"><span className="pth-epic">SSO pilot</span></Property><Property icon={<TaskSprintIcon />} label="Sprint">Sprint 24</Property></div>
            <div className="pth-property-group"><Property icon="LayoutGridIcon" label="Estimate">3 points</Property><Property icon={<TaskCalendarIcon />} label="Due date">Not set</Property></div>
            <div className="pth-property-group"><Property icon={<TaskSprintIcon />} label="Recurrence">None</Property></div>
            <div className="pth-rail-section"><span><TaskIcon name="ArrowRight01Icon" />Delivery</span><span>orbitdesk/app</span></div>
            <div className="pth-rail-section"><span><TaskIcon name="ArrowRight01Icon" />Related</span><span>Northstar Labs</span></div>
          </div>
        </div>
      </div>
    </div>
      <div className="pth-followup" data-visible={phase >= 5} data-approved={phase >= 6} data-sent={phase === 7} aria-hidden="true">
        <div className="pth-followup-header"><img src={theme === 'light' ? '/brand/helpin-icon-ink.svg' : '/brand/helpin-icon-white.svg'} width={26} height={26} alt="" /><span><strong>Ask Agent</strong><small>Back to the customer</small></span><span className="pth-followup-status">{phase === 7 ? <><CheckCheck size={12} />Sent</> : phase === 6 ? <><Check size={12} />Approved</> : 'For review'}</span></div>
        <div className="pth-followup-body"><div className="pth-release"><Rocket size={13} /><span>Release confirmed</span><small>PRJ-214</small></div><p className="pth-followup-explainer">The team released the approved change. The customer follow-up is prepared for review.</p><div className="pth-followup-customer"><img src="/new/avatars/maya.webp" width={25} height={25} alt="" /><span><strong>Maya Chen</strong><small>Northstar Labs · Original conversation</small></span></div><div className="pth-followup-message"><div><Mail size={13} /><strong>{phase === 7 ? 'Customer update sent' : 'Customer update prepared'}</strong></div><p>Hi Maya, the admin-only SSO pilot is ready. Here’s the setup guide and checklist for your team.</p></div><div className="pth-followup-approval"><img src="/new/avatars/sam.webp" width={21} height={21} alt="" /><span>{phase >= 6 ? 'After release · Approved by Sam' : 'After release · Waiting for Sam’s approval'}</span><Check size={13} /></div></div>
        <div className="pth-followup-footer">{phase === 7 ? <CheckCheck size={14} /> : <MessageSquare size={14} />}<span>{phase === 7 ? 'Sent to Maya · Customer loop closed' : 'Reply to Maya’s original conversation'}</span></div>
      </div>
    </div>
  </div>;
}
