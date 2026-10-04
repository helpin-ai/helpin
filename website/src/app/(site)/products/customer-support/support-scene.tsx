'use client';

import { useState } from 'react';
import { BookOpen, Check, GitPullRequest, ListChecks, MessageSquare, Pause, Play, ShieldCheck, Terminal } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowAgent, WorkflowClick, WorkflowSources, type ContextSource } from '../../_components/WorkflowParts';
import { StreamingText } from '../../_components/StreamingText';
import './support-scene.css';
import './support-story.css';

type Variant = 'answer' | 'handoff' | 'followup';
const STORIES: Record<Variant, { request: string; sources: ContextSource[]; reply: string }> = {
  answer: { request: 'How do I export only the contacts I selected?', sources: [{ icon: MessageSquare, label: 'This conversation', detail: 'Maya is exporting a selected group' }, { icon: BookOpen, label: 'Export guide', detail: 'Actions → Export selected contacts' }, { icon: ListChecks, label: 'Latest release', detail: 'Selected exports are available' }], reply: 'Select the contacts you need, open Actions, then choose Export selected contacts.' },
  handoff: { request: 'The CSV has 10,000 rows. I need all 18,400.', sources: [{ icon: MessageSquare, label: 'Earlier replies', detail: 'Maya already tried the export again' }, { icon: Terminal, label: 'Connected logs', detail: 'Pagination stops after the first page' }, { icon: ListChecks, label: 'EXP-142', detail: 'Existing task · Sam is investigating' }], reply: 'I found the incomplete export in the logs. I’m bringing Sam in with your report and the linked task.' },
  followup: { request: 'Let me know when the full export is ready.', sources: [{ icon: MessageSquare, label: 'Maya’s original request', detail: 'Waiting for a complete export' }, { icon: ListChecks, label: 'EXP-142', detail: 'Fix reviewed and tested' }, { icon: GitPullRequest, label: 'Release confirmed', detail: 'Pagination fix is live' }], reply: 'Hi Maya, the export fix is live. You can now export the full list. Could you try it again and let us know if you need help?' },
};
export function SupportScene({ variant }: { variant: Variant }) {
  const [paused, setPaused] = useState(false);
  const { container, phase, cycle, playing, reducedMotion } = useWorkflowPlayback({ paused, resetKey: variant, beats: [0, 1100, 2600, 4100, 5700, 8400, 10100], duration: 15300 });
  const story = STORIES[variant];
  return <div className={`support-scene support-story support-scene-${variant}`} ref={container} data-playing={playing} data-phase={phase}>
    <div className="support-scene-toolbar"><span><span className="sw-workspace">O</span>OrbitDesk</span>{!reducedMotion && <button type="button" aria-label={`${paused ? 'Play' : 'Pause'} ${variant} animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button>}</div>
    <div className="ss-body" key={cycle}>
      <div className="ss-customer"><img src="/new/avatars/maya.webp" width={29} height={29} alt="" /><div><strong>Maya Chen</strong><small>Northstar Labs</small></div><span>{variant === 'handoff' && phase >= 5 ? 'Sam assigned' : variant === 'followup' ? 'Open' : 'AI handling'}</span></div>
      <div className="ss-customer-message">{story.request}</div>
      <WorkflowAgent name="Echo" role="Support agent" working={phase > 0 && phase < 4} status={phase === 0 ? 'Thinking…' : phase < 4 ? 'Checking the context' : variant === 'handoff' ? (phase >= 5 ? 'Handed over to Sam' : 'Preparing the handoff') : variant === 'followup' && phase < 6 ? 'Update ready for approval' : 'Replying to Maya'} />
      <div className="ss-stage">
        {phase > 0 && phase < 4 && <WorkflowSources sources={story.sources} phase={phase} />}
        {phase >= 4 && <div className="wf-outcome">
          <div className="ss-echo-message"><strong>{variant === 'followup' && phase < 6 ? 'Draft · Not sent' : 'Echo'}</strong><p><StreamingText active={playing} duration={2000} text={story.reply} /></p>{variant === 'answer' && <span className="ss-guide"><BookOpen size={12} />Export your contacts</span>}{variant === 'followup' && phase >= 6 && <span className="ss-guide"><Check size={12} />Sent after Sam’s approval</span>}</div>
          {variant === 'handoff' && phase >= 5 && <div className="ss-handoff"><span><img src="/new/avatars/sam.webp" width={23} height={23} alt="" />Assigned to Sam</span><small>EXP-142 · Report and logs attached</small></div>}
          {variant === 'followup' && phase === 5 && <div className="ss-approval"><ShieldCheck size={13} />Approve &amp; send{playing && <WorkflowClick />}</div>}
          <WorkflowSources sources={story.sources} phase={4} compact />
        </div>}
      </div>
      <div className="ss-composer" aria-hidden="true">Reply to Maya…<span>↵</span></div>
    </div>
  </div>;
}
