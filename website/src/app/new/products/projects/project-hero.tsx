'use client';

import { useState, type CSSProperties, type ReactNode } from 'react';
import { Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import { TaskCalendarIcon, TaskFeatureIcon, TaskIcon, TaskPriorityIcon, TaskSprintIcon, TaskTickIcon, type TaskIconName } from './task-demo-icons';

function Property({ icon, label, children }: { icon: TaskIconName | ReactNode; label: string; children: ReactNode }) {
  return <div className="pth-property"><span className="pth-property-icon">{typeof icon === 'string' ? <TaskIcon name={icon as TaskIconName} /> : icon}</span><span className="pth-property-label">{label}</span><span className="pth-property-value">{children}</span></div>;
}

// Presentational snapshot of TaskDetailPanel / QuietDetailHeader, using the same
// Inter font, dark popover tokens, field order and icon geometry. No app mutations.
export function ProjectHero() {
  const { container, playing, cycle } = useBentoPlayback(9500);
  const [paused, setPaused] = useState(false);
  const delay = (seconds: number) => ({ '--task-delay': `${seconds}s` }) as CSSProperties;
  return <div className="project-hero-art" ref={container} data-playing={playing && !paused}>
    <div className="project-hero-art-label"><span>FROM CUSTOMER REQUEST TO CLEAR TASK</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} project planning animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div className="project-hero-task" key={cycle} role="img" aria-label="Illustrative Helpin task detail in OrbitDesk. ORB-491, Add Slack alerts for failed syncs. The Overview describes Maya’s request from Northstar Labs, a three-step workflow and three acceptance criteria. The task is ready, owned and requested by Sam Rivera, high priority, and planned for Sprint 24. These are requirements to review, not completed work.">
      <div aria-hidden="true">
        <div className="pth-header">
          <div className="pth-breadcrumb"><span><TaskIcon name="CheckListIcon" />Tasks</span><TaskIcon name="ArrowRight01Icon" /><span><TaskIcon name="Layers01Icon" />Sync reliability</span><TaskIcon name="ArrowRight01Icon" /><span><TaskSprintIcon />Sprint 24</span></div>
          <h2>Add Slack alerts for failed syncs</h2>
          <div className="pth-meta"><span>ORB-491</span><i />Engineering</div>
          <div className="pth-actions"><div><TaskIcon name="Link01Icon" /><TaskIcon name="MoreVerticalIcon" /><TaskIcon name="ArrowUpRight01Icon" /><TaskIcon name="Cancel01Icon" /></div><span className="pth-saved"><TaskTickIcon />All changes saved</span></div>
        </div>
        <div className="pth-layout">
          <div className="pth-main">
            <div className="pth-tabs"><span className="pth-tab-active">Overview</span><span>Delivery</span></div>
            <div className="pth-description">
              <div className="pth-intro"><p>Notify the team in Slack when a sync fails, so they can investigate before a customer reports it.</p><span className="pth-edit"><TaskIcon name="PencilEdit01Icon" /></span></div>
              <p>Maya at Northstar Labs needs the affected account, the error, and a link to investigate in each alert.</p>
              <h3>Workflow</h3>
              <ol><li>Detect a failed sync.</li><li>Send an alert to the connected Slack channel.</li><li>Open the sync details from the alert.</li></ol>
              <h3>Acceptance criteria</h3>
              <ul className="pth-criteria"><li style={delay(.5)}>Include the account and failure reason.</li><li style={delay(1.25)}>Link directly to the affected sync.</li><li style={delay(2)}>Send one alert per failed sync.</li></ul>
            </div>
          </div>
          <div className="pth-sidebar">
            <div className="pth-id"><span>Task ID:</span><strong>ORB-491</strong><TaskIcon name="Copy01Icon" /><TaskIcon name="GitBranchIcon" /></div>
            <div className="pth-property-group"><Property icon="UserGroupIcon" label="Team">Engineering</Property><Property icon="HashtagIcon" label="State"><i className="pth-state" />Ready</Property></div>
            <div className="pth-property-group"><Property icon="UserIcon" label="Owners"><img src="/new/avatars/sam.webp" width={18} height={18} alt="" />Sam Rivera</Property><Property icon="UserIcon" label="Requester"><img src="/new/avatars/sam.webp" width={18} height={18} alt="" />Sam Rivera</Property></div>
            <div className="pth-property-group"><Property icon="DashboardSpeed01Icon" label="Priority"><TaskPriorityIcon />High</Property><Property icon="HashtagIcon" label="Type"><TaskFeatureIcon />Feature</Property><Property icon="Tag01Icon" label="Labels"><span className="pth-label">+ Add label</span></Property></div>
            <div className="pth-property-group"><Property icon="Layers01Icon" label="Epic"><span className="pth-epic">Sync reliability</span></Property><Property icon={<TaskSprintIcon />} label="Sprint">Sprint 24</Property></div>
            <div className="pth-property-group"><Property icon="LayoutGridIcon" label="Estimate">3 points</Property><Property icon={<TaskCalendarIcon />} label="Due date">Sep 25</Property></div>
            <div className="pth-property-group"><Property icon={<TaskSprintIcon />} label="Recurrence">None</Property></div>
            <div className="pth-rail-section"><span><TaskIcon name="ArrowRight01Icon" />Delivery</span><span>Not configured</span></div>
            <div className="pth-rail-section"><span><TaskIcon name="ArrowRight01Icon" />Related</span></div>
          </div>
        </div>
      </div>
    </div>
  </div>;
}
