'use client';

import { useState } from 'react';
import { ArrowDown, BookOpen, Check, Code2, FileText, GitPullRequest, ImageIcon, ListChecks, Mail, MessageSquare, Mic, Pause, Play, ShieldCheck, Terminal, Users, Video } from 'lucide-react';
import { useWorkflowPlayback } from './useWorkflowPlayback';
import { WorkflowAgent, WorkflowClick, WorkflowSources, type ContextSource } from './WorkflowParts';
import { StreamingText } from './StreamingText';
import './work-scene.css';
import './work-scene-story.css';

type Scene = 'docs' | 'coverage' | 'review' | 'sales' | 'team';
const BEATS = [0, 1200, 2800, 4400, 6000, 7800, 9700];
const DOCS = [
  { label: 'Release', request: 'Update the guide for selected exports.', source: 'Selected exports released', title: 'Export selected contacts', action: 'Steps updated', Icon: FileText },
  { label: 'Screen change', request: 'Refresh the screenshot in our export guide.', source: 'Export moved to Actions', title: 'Export your contacts', action: 'Screenshot added', Icon: ImageIcon },
  { label: 'Walkthrough', request: 'Add a video showing how to export contacts.', source: 'Record the current export flow', title: 'Export your contacts', action: 'Recording added', Icon: Video },
];
const SOURCES: Record<Exclude<Scene, 'docs'>, ContextSource[]> = {
  coverage: [{ icon: MessageSquare, label: 'Unanswered conversations', detail: '“Where is the export button?” · 6 conversations' }, { icon: BookOpen, label: 'Current export guide', detail: 'Still shows the old toolbar' }, { icon: ListChecks, label: 'Recent release', detail: 'Export moved to Actions' }],
  review: [{ icon: MessageSquare, label: 'Maya’s report', detail: 'Export is missing 8,400 contacts' }, { icon: Terminal, label: 'Connected logs', detail: 'Only the first page was returned' }, { icon: Code2, label: 'Pull request #728', detail: 'Pagination fix and regression test' }],
  sales: [{ icon: MessageSquare, label: 'Customer conversation', detail: 'Second team wants to join this month' }, { icon: Users, label: 'Northstar account', detail: 'Sam owns the renewal' }, { icon: Mic, label: 'Last sales call', detail: 'Security approval is the remaining blocker' }],
  team: [{ icon: MessageSquare, label: 'Maya’s conversation', detail: 'Export stops at 10,000 of 18,400 rows' }, { icon: ListChecks, label: 'Task EXP-142', detail: 'Customer report and logs attached' }, { icon: BookOpen, label: 'Export guide', detail: 'Troubleshooting steps need an update' }],
};
const REQUESTS = { coverage: 'Find unanswered questions we can fix in our docs.', review: 'Check the export fix before I approve it.', sales: '“Can we add another team to our plan?”', team: 'Fix the export and update the guide.' };

export function WorkScene({ variant }: { variant: Scene }) {
  const [paused, setPaused] = useState(false);
  const [selected, setSelected] = useState(1);
  const [replay, setReplay] = useState(0);
  const { container, playing, reducedMotion, phase, cycle } = useWorkflowPlayback({ beats: BEATS, duration: 15700, paused, resetKey: `${variant}-${selected}-${replay}` });
  const doc = DOCS[selected];
  const name = variant === 'sales' ? 'Beacon' : variant === 'review' ? 'Lens' : variant === 'team' ? 'Scribe' : 'Quill';
  const sources = variant === 'docs' ? [{ icon: ListChecks, label: doc.source, detail: 'Product change linked to this run' }, { icon: BookOpen, label: doc.title, detail: 'Reading the existing article and instructions' }] : SOURCES[variant];
  const showContext = variant === 'docs' ? phase < 3 : phase < 4;
  const status = phase === 0 ? 'Thinking…' : showContext ? 'Gathering context' : variant === 'docs' ? (phase >= 6 ? 'Update ready for review' : selected === 0 ? 'Updating the article' : phase < 5 ? 'Using the product in a browser' : 'Adding the capture to the draft') : variant === 'coverage' ? 'Preparing an article update' : variant === 'review' ? (phase >= 6 ? 'Waiting for your review' : 'Checking the change against the report') : variant === 'sales' ? 'Drafting a follow-up for Sam' : 'Coordinating the work';
  return <div ref={container} className={`work-scene work-scene-${variant} ws-story`} data-playing={playing} data-phase={phase}>
    <div className="ws-top"><span><i aria-hidden="true">O</i>OrbitDesk</span>{!reducedMotion && <button type="button" aria-label={`${paused ? 'Play' : 'Pause'} workflow animation`} aria-pressed={paused} onClick={() => setPaused(!paused)}>{paused ? <Play size={13} /> : <Pause size={13} />}</button>}</div>
    {variant === 'docs' && <div className="ws-tabs" role="group" aria-label="Documentation example">{DOCS.map((item, index) => <button key={item.label} aria-pressed={selected === index} onClick={() => { setSelected(index); setReplay(value => value + 1); }}>{item.label}</button>)}</div>}
    <div className="ws-story-body" key={cycle}>
      <div className="ws-story-request">{variant === 'docs' ? doc.request : REQUESTS[variant]}</div>
      <WorkflowAgent name={name} role={variant === 'sales' ? 'Sales agent' : variant === 'review' ? 'Review agent' : variant === 'team' ? 'Planning agent' : 'Docs agent'} status={status} working={phase > 0 && phase < 6} />
      <div className="ws-story-stage">
        {showContext && phase > 0 && <WorkflowSources sources={sources} phase={phase} />}
        {!showContext && variant === 'docs' && <div className="wf-outcome ws-demo-document">
          <div className="ws-doc-title"><FileText size={15} /><strong>{doc.title}</strong><span className="ws-tag ws-amber">{phase < 6 ? 'Editing' : 'Draft ready'}</span></div>
          {selected === 0 ? <div className="ws-article-steps"><span className="ws-old-step">Open Contacts and export the full list.</span>{phase >= 4 && <div className="ws-new-step"><StreamingText active={playing} text="Select the contacts you need. Open Actions, then choose Export selected contacts." duration={1600} /></div>}</div> : <div className={`ws-capture ws-browser-stage${phase >= 5 ? ' ws-captured' : ''}`}>
            <div className="ws-capture-bar"><span /><span /><span /><small>app.orbitdesk.example / contacts</small>{selected === 2 && phase < 5 && <b className="ws-record-indicator">REC</b>}</div>
            <div className="ws-product"><span className="ws-micro">CONTACTS</span><div className="ws-contact"><span className="ws-initial">M</span>Maya Chen<Check size={14} /></div><div className="ws-action">Actions <ArrowDown size={12} />{phase === 3 && playing && <WorkflowClick />}</div>{phase >= 4 && <div className="ws-menu"><Check size={13} />Export contacts{phase === 4 && selected === 2 && playing && <WorkflowClick />}</div>}{phase >= 5 && <div className="ws-capture-success"><doc.Icon size={19} /><span>{doc.action}</span>{selected === 2 && <small>00:12</small>}</div>}</div>
          </div>}
          {phase >= 6 && <div className="ws-draft-ready"><ShieldCheck size={16} /><span>Article update ready for your review</span></div>}
        </div>}
        {!showContext && variant === 'coverage' && <div className="wf-outcome"><div className="ws-identified-gap"><MessageSquare size={18} /><div><strong>Customers can’t find Export.</strong><span>6 conversations · Outdated guide</span></div></div>{phase >= 5 && <div className="ws-proposal"><FileText size={18} /><div><strong>Export your contacts</strong><span>New Actions menu steps and screenshot</span></div><span className="ws-tag ws-amber">Draft</span></div>}{phase >= 6 && <div className="ws-draft-ready"><ShieldCheck size={15} />Ready for review</div>}</div>}
        {!showContext && variant === 'review' && <div className="wf-outcome ws-review-flow"><div className="ws-run-result"><img src="/new/agents/forge.svg" alt="" width={28} height={28} /><div><strong>Forge</strong><span>Pagination fixed · 18,400 rows exported</span></div><Check size={16} /></div>{phase >= 5 && <div className="ws-run-result"><img src="/new/agents/lens.svg" alt="" width={28} height={28} /><div><strong>Lens</strong><span>Report, code and regression test checked</span></div><Check size={16} /></div>}{phase >= 6 && <div className="ws-human-review"><GitPullRequest size={18} /><div><strong>PR #728 · Sam’s review</strong><span>Awaiting approval</span></div></div>}</div>}
        {!showContext && variant === 'sales' && <div className="wf-outcome ws-proposed-email"><div><Mail size={15} /><strong>To Maya</strong><span className="ws-tag ws-amber">Draft</span></div><p><StreamingText active={playing} duration={2000} text="Hi Maya, happy to help your second team get started. Can we confirm their timing and review the security approval together?" /></p>{phase >= 5 && <div className="ws-next-action"><CalendarDaysIcon />Follow up tomorrow · Sam</div>}</div>}
        {!showContext && variant === 'team' && <div className="wf-outcome ws-coordination">{[['scribe','Scribe','Scoped EXP-142 from the report'],['forge','Forge','Code and regression test ready'],['lens','Lens','Review findings attached'],['quill','Quill','Updated guide drafted']].map(([icon,agent,action],i) => phase >= i + 3 && <div className="ws-run-result" key={icon}><img src={`/new/agents/${icon}.svg`} width={28} height={28} alt="" /><div><strong>{agent}</strong><span>{action}</span></div><Check size={15} /></div>)}{phase >= 6 && <div className="ws-draft-ready"><ShieldCheck size={15} />Changes ready for your team</div>}</div>}
        {!showContext && <WorkflowSources sources={sources} phase={6} compact />}
      </div>
    </div>
  </div>;
}
function CalendarDaysIcon() { return <ListChecks size={14} />; }
