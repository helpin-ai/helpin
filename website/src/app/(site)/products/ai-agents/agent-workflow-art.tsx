'use client';

import { useState } from 'react';
import { BookOpen, Check, Code2, GitPullRequest, ListChecks, MessageSquare, Pause, Play, Plug, ShieldCheck, Terminal } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowAgent, WorkflowClick, WorkflowSources, type ContextSource } from '../../_components/WorkflowParts';
import { StreamingText } from '../../_components/StreamingText';
import './agent-workflow-story.css';

type Variant = 'context' | 'coordination' | 'tools' | 'coding' | 'approval';
const STORIES: Record<Variant, { request: string; sources: ContextSource[] }> = {
  context: { request: 'What’s blocking Maya’s export, and what have we tried?', sources: [{ icon: MessageSquare, label: 'Maya’s conversation', detail: '10,000 of 18,400 rows exported' }, { icon: BookOpen, label: 'Export guide', detail: 'The smaller-export workaround was tried' }, { icon: ListChecks, label: 'EXP-142', detail: 'Full export fix is in progress' }] },
  coordination: { request: 'Investigate the export issue and update the guide.', sources: [{ icon: MessageSquare, label: 'Customer report', detail: 'Maya needs the full list of contacts' }, { icon: ListChecks, label: 'EXP-142', detail: 'Existing task and acceptance criteria' }, { icon: BookOpen, label: 'Export guide', detail: 'Current troubleshooting instructions' }] },
  tools: { request: 'Check the linked issue before I update Maya.', sources: [{ icon: MessageSquare, label: 'Maya’s conversation', detail: 'Customer is waiting for the export fix' }, { icon: Plug, label: 'External MCP · Issue lookup', detail: 'Reading EXP-142 with the allowed tool' }, { icon: ListChecks, label: 'Issue tracker response', detail: 'In progress · Assigned to Sam' }] },
  coding: { request: 'Fix EXP-142 and prepare a pull request.', sources: [{ icon: ListChecks, label: 'Task and customer report', detail: 'Export all 18,400 contacts' }, { icon: Code2, label: 'Connected repository', detail: 'Reading export-contacts.ts' }, { icon: Terminal, label: 'Regression coverage', detail: 'Checking multi-page export behavior' }] },
  approval: { request: 'Add the investigation to EXP-142.', sources: [{ icon: MessageSquare, label: 'Support findings', detail: 'A smaller export does not meet the request' }, { icon: Code2, label: 'Code investigation', detail: 'Remaining pages are not retrieved' }, { icon: ListChecks, label: 'Task EXP-142', detail: 'Proposed update prepared' }] },
};

export function AgentWorkflowArt({ variant }: { variant: Variant }) {
  const [paused, setPaused] = useState(false);
  const { container, playing, phase, cycle, reducedMotion } = useWorkflowPlayback({ paused, resetKey: variant, beats: [0, 1100, 2700, 4300, 6000, 8300, 10300], duration: 15500 });
  const story = STORIES[variant];
  const name = variant === 'coding' ? 'Forge' : 'Helpin AI';
  return <div ref={container} className={`awa-art awa-${variant} awa-story`} data-playing={playing} data-phase={phase}>
    <div className="awa-toolbar"><span><span className="awa-workspace-mark">O</span>OrbitDesk<span className="awa-toolbar-slash">/</span>Helpin AI</span>{!reducedMotion && <button type="button" aria-label={`${paused ? 'Play' : 'Pause'} ${variant} animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button>}</div>
    <div className="awa-story-body" key={cycle}>
      <div className="awa-story-request">{story.request}</div>
      <WorkflowAgent name={name} role={variant === 'coding' ? 'Coding agent' : 'Workspace assistant'} status={phase === 0 ? 'Thinking…' : phase < 4 ? 'Gathering context' : phase < 6 ? 'Working on your request' : variant === 'approval' ? 'Update approved' : 'Ready for review'} working={phase > 0 && phase < 6} />
      <div className="awa-story-stage">
        {phase > 0 && phase < 4 && <WorkflowSources sources={story.sources} phase={phase} />}
        {phase >= 4 && <div className="wf-outcome">
          {variant === 'context' && <div className="awa-story-answer"><StreamingText active={playing} duration={2100} text="Maya still needs all 18,400 contacts. The smaller export didn’t solve it. EXP-142 is in progress, so the next step is to finish and test pagination." /></div>}
          {variant === 'tools' && <div className="awa-story-answer"><span className="awa-story-tool"><Plug size={14} />MCP · Issue lookup completed</span><StreamingText active={playing} duration={1700} text="EXP-142 is in progress with Sam. I’ve drafted a reply that confirms the issue is being worked on without promising a release date." />{phase >= 5 && <span className="awa-story-badge"><FileDraftIcon />Reply drafted · Not sent</span>}</div>}
          {variant === 'coordination' && <><div className="awa-story-parallel">{[{ name: 'Forge', task: 'Checking pagination', result: 'Pagination fix needed', icon: Code2 }, { name: 'Quill', task: 'Reading the guide', result: 'Troubleshooting update drafted', icon: BookOpen }].map(({ name: agent, task, result, icon: Icon }) => <div key={agent}><img src={`/new/agents/${agent.toLowerCase()}.svg`} width={30} height={30} alt="" /><strong>{agent}</strong><small><Icon size={12} />{phase < 5 ? task : result}</small>{phase >= 5 && <Check size={15} />}</div>)}</div>{phase >= 6 && <div className="awa-story-final"><ListChecks size={18} /><div><strong>Findings combined</strong><span>Fix pagination. Test the full export. Review the guide update.</span></div></div>}</>}
          {variant === 'coding' && <><div className="awa-story-diff"><div><Code2 size={14} />export-contacts.ts</div><pre><span>− return firstPage.rows;</span><span>+ return await collectAllPages();</span></pre></div>{phase >= 5 && <div className="awa-story-test"><Check size={15} />Regression test passed · 18,400 rows</div>}{phase >= 6 && <div className="awa-story-final"><GitPullRequest size={18} /><div><strong>PR #728 ready for Sam</strong><span>Lens reviewed the change · Awaiting approval</span></div></div>}</>}
          {variant === 'approval' && <div className="awa-story-approval"><ShieldCheck size={20} /><strong>Update EXP-142?</strong><p>Attach the report, pagination findings and proposed regression test.</p><div className="awa-story-approve">{phase < 6 ? 'Approve update' : <><Check size={14} />Approved by Sam</>}{phase === 5 && playing && <WorkflowClick />}</div></div>}
          <WorkflowSources sources={story.sources} phase={4} compact />
        </div>}
      </div>
    </div>
  </div>;
}
function FileDraftIcon() { return <MessageSquare size={13} />; }
