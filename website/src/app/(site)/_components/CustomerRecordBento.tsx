'use client';

import { BookOpen, CalendarDays, Check, Code2, FileText, GitPullRequest, ListChecks, Mail, MessageSquare, Mic, ShieldCheck, Terminal, Users } from 'lucide-react';
import { useWorkflowPlayback } from './useWorkflowPlayback';
import { WorkflowAgent, WorkflowSources, type ContextSource } from './WorkflowParts';
import { StreamingText } from './StreamingText';
import './customer-record-stories.css';

export type RecordVariant = 'conversations' | 'meetings' | 'projects' | 'deals' | 'docs' | 'email-calendar' | 'coding';
type Story = { name: string; role: string; request: string; sources: ContextSource[]; working: string; result: string };
const STORIES: Record<RecordVariant, Story> = {
  conversations: {
    name: 'Echo', role: 'Support agent', request: 'Can we test SSO with just our pilot team?', working: 'Checking the setup and earlier replies', result: 'Reply ready',
    sources: [{ icon: MessageSquare, label: 'Maya’s conversation', detail: 'Okta selected · SAML app already created' }, { icon: BookOpen, label: 'Okta setup guide', detail: 'Pilot groups are supported' }, { icon: ListChecks, label: 'Rollout task', detail: 'Sam owns the pilot setup' }],
  },
  meetings: {
    name: 'Helpin AI', role: 'Meeting assistant', request: 'Turn the rollout meeting into next steps.', working: 'Reading the decisions and existing work', result: '2 linked tasks created',
    sources: [{ icon: Mic, label: 'Security review · 24:18', detail: '“Sam will validate the Okta setup.”' }, { icon: MessageSquare, label: 'Maya’s conversation', detail: 'Pilot needs approval before Friday' }, { icon: ListChecks, label: 'Rollout project', detail: 'No validation task exists yet' }],
  },
  projects: {
    name: 'Helpin AI', role: 'Task planning', request: 'Plan the fix for Maya’s incomplete export.', working: 'Checking the report against existing work', result: 'Task created · EXP-142',
    sources: [{ icon: MessageSquare, label: 'Maya’s report', detail: '10,000 rows exported · 18,400 expected' }, { icon: ListChecks, label: 'Related tasks', detail: 'No existing fix for the export limit' }, { icon: Terminal, label: 'Connected logs', detail: 'Export stops after the first page' }],
  },
  coding: {
    name: 'Forge', role: 'Coding agent', request: 'Fix EXP-142 and open a pull request.', working: 'Reading the task, report and code', result: 'PR #728 ready for review',
    sources: [{ icon: ListChecks, label: 'EXP-142', detail: 'Export all 18,400 contacts' }, { icon: MessageSquare, label: 'Customer report', detail: 'A smaller export won’t solve Maya’s issue' }, { icon: Code2, label: 'Connected repository', detail: 'export-contacts.ts · pagination missing' }],
  },
  deals: {
    name: 'Beacon', role: 'Sales agent', request: 'What should I follow up on with Northstar?', working: 'Checking what is holding up the renewal', result: 'Follow-up drafted for Sam',
    sources: [{ icon: Users, label: 'Northstar renewal', detail: '$42,000 · decision this Friday' }, { icon: Mail, label: 'Maya’s last email', detail: 'Waiting for security approval' }, { icon: Mic, label: 'Security review', detail: 'Okta pilot results needed' }],
  },
  docs: {
    name: 'Echo', role: 'Support agent', request: 'Help Maya get her SSO pilot approved.', working: 'Reading product guidance and team instructions', result: 'Next step prepared',
    sources: [{ icon: BookOpen, label: 'Public Okta guide', detail: 'Run the pilot connection test' }, { icon: ShieldCheck, label: 'Internal rollout SOP', detail: 'Security review required before launch' }, { icon: Users, label: 'Northstar account', detail: 'Sam is the rollout owner' }],
  },
  'email-calendar': {
    name: 'Helpin AI', role: 'Workspace assistant', request: 'Prepare me for today’s Northstar call.', working: 'Gathering the latest customer context', result: 'Meeting brief ready',
    sources: [{ icon: CalendarDays, label: 'Today · 10:00', detail: 'Northstar security review · Maya and Sam' }, { icon: Mail, label: 'Latest email', detail: 'Maya asked for pilot approval' }, { icon: ListChecks, label: 'Rollout task', detail: 'Okta validation is still open' }],
  },
};

function Outcome({ variant, streaming, phase }: { variant: RecordVariant; streaming: boolean; phase: number }) {
  if (variant === 'conversations') return <div className="cr2-reply"><span className="cr2-reply-author">Echo</span><p><StreamingText active={streaming} duration={1900} text="Yes. Assign only your pilot group to the Okta app. Sam can help validate the setup before the wider rollout." /></p><span className="cr2-source-link"><BookOpen size={13} />Test your Okta pilot</span></div>;
  if (variant === 'meetings') return <div className="cr2-task-list">{['Validate the Okta pilot', 'Share the approved setup guide'].map((task, i) => <div key={task}><span className="cr2-task-id">SSO-{143 + i}</span><strong>{task}</strong><span className="cr2-assignee">{i ? 'Maya' : 'Sam'}</span></div>)}<span className="cr2-source-link"><Mic size={13} />Linked to the security review</span></div>;
  if (variant === 'projects') return <div className="cr2-task"><span className="cr2-task-id">EXP-142 <span>To do</span></span><strong>Export every contact, not just the first page.</strong><div className="cr2-acceptance"><ListChecks size={15} /><span>All 18,400 rows included<br />Regression test for multiple pages</span></div><span className="cr2-source-link"><MessageSquare size={13} />Maya’s report attached</span></div>;
  if (variant === 'coding') return <div className="cr2-code"><div><Code2 size={14} />export-contacts.ts</div><pre><span>− return firstPage.rows;</span><span>+ return await collectAllPages();</span></pre><div className="cr2-code-result">{phase < 5 ? <><i className="wf-working-dot" />Running the export test…</> : <><Check size={14} />18,400 rows exported <span><GitPullRequest size={13} />PR #728 · Review</span></>}</div></div>;
  if (variant === 'deals') return <div className="cr2-followup"><span className="cr2-task-id">Northstar Labs <span>Draft · Not sent</span></span><p><StreamingText active={streaming} duration={1600} text="Hi Maya, can we review the Okta pilot results before Friday’s renewal decision? Sam can join to help with approval." /></p><span className="cr2-source-link"><CalendarDays size={13} />Suggested follow-up · Tomorrow</span></div>;
  if (variant === 'docs') return <div className="cr2-checklist"><strong>Northstar pilot approval</strong><div><Check size={15} /><span>Setup instructions from the Okta guide</span></div><div><Check size={15} /><span>Security checklist from your internal SOP</span></div><div className="cr2-review"><ShieldCheck size={16} /><span>Assigned to Sam for approval</span></div></div>;
  return <div className="cr2-brief"><span className="cr2-task-id">10:00 · Northstar Labs</span><strong>Security &amp; rollout review</strong><div><span>Discuss</span><p>Approve the pilot after Sam validates Okta.</p></div><div><span>Still open</span><p>Confirm the date for the wider rollout.</p></div></div>;
}

export function CustomerRecordBento({ variant }: { variant: RecordVariant }) {
  const { container, phase, cycle, playing } = useWorkflowPlayback({ duration: 14500, resetKey: variant });
  const story = STORIES[variant];
  const result = phase >= 4;
  return <div ref={container} className={`record-bento-art cr2-story cr2-${variant}`} data-playing={playing} data-phase={phase}>
    <div className="cr2-window" key={cycle}>
      <div className="cr2-window-bar"><span className="cr2-orbit">O</span>OrbitDesk<span className="cr2-window-area">{variant === 'conversations' ? 'Support' : 'Helpin AI'}</span></div>
      <div className="cr2-request"><img src={`/new/avatars/${variant === 'conversations' ? 'maya' : 'sam'}.webp`} width={25} height={25} alt="" loading="lazy" /><span>{story.request}</span></div>
      <WorkflowAgent name={story.name} role={story.role} working={phase > 0 && phase < 4} status={phase === 0 ? 'Thinking…' : result ? (variant === 'coding' && phase === 4 ? 'Implementing and testing' : story.result) : story.working} />
      <div className="cr2-stage">
        {phase > 0 && !result && <WorkflowSources sources={story.sources} phase={phase} />}
        {result && <div className="wf-outcome"><Outcome variant={variant} streaming={playing} phase={phase} /><WorkflowSources sources={story.sources} phase={4} compact /></div>}
      </div>
    </div>
  </div>;
}
