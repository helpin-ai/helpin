'use client';

import { useState } from 'react';
import { BookOpen, Check, Code2, FileText, GitPullRequest, ListChecks, MessageSquare, Pause, Play, Plug, ShieldCheck, Terminal } from 'lucide-react';
import { useWorkflowPlayback } from '../useWorkflowPlayback';
import { WorkflowAgent, WorkflowSources, type ContextSource } from '../WorkflowParts';
import { StreamingText } from '../StreamingText';
import './platform-agent-story.css';

const CONTENT: Record<'inbound'|'outbound'|'event', { title: string; request: string; sources: ContextSource[]; answer: string }> = {
  inbound: { title: 'Helpin MCP', request: 'Find our contact export guide.', sources: [{icon:Plug,label:'Helpin MCP',detail:'Connected to OrbitDesk · Docs access'},{icon:BookOpen,label:'search_documents',detail:'Query: contact export guide'},{icon:FileText,label:'read_document',detail:'Export your contacts · Published article'}], answer: 'Found the guide. Select the contacts, open Actions, then choose Export selected contacts.' },
  outbound: { title: 'External MCP', request: 'Check the issue before I update Maya.', sources: [{icon:MessageSquare,label:'Maya’s conversation',detail:'Customer is waiting for a complete export'},{icon:Plug,label:'External MCP · Read issue',detail:'Selected read tool · EXP-142'},{icon:ListChecks,label:'Issue tracker response',detail:'In progress · Assigned to Sam'}], answer: 'EXP-142 is still in progress. I’ve prepared a reply confirming the investigation, with no unconfirmed release date.' },
  event: { title: 'Pull request automation', request: 'PR #728 opened · Fix incomplete exports', sources: [{icon:ListChecks,label:'get_task_context',detail:'EXP-142 · Export all 18,400 contacts'},{icon:Code2,label:'Pull request diff',detail:'Pagination change in export-contacts.ts'},{icon:Terminal,label:'Test results',detail:'Multi-page export passes · Failed-page case missing'}], answer: 'The full export passes. Add a regression test for a failed later page before merging.' },
};
export function PlatformAgentStory({ type }: { type: keyof typeof CONTENT }) {
  const [paused,setPaused] = useState(false);
  const { container, playing, phase, cycle, reducedMotion } = useWorkflowPlayback({ paused, duration: 14500, resetKey:type });
  const story = CONTENT[type];
  return <div className={`platform-scene pas-story pas-${type}`} ref={container} data-playing={playing} data-phase={phase}>
    <div className="platform-scene-toolbar"><span><b className="platform-orbit-mark">O</b>OrbitDesk<span className="platform-divider">/</span>{story.title}</span>{!reducedMotion && <button type="button" aria-label={`${paused?'Play':'Pause'} ${story.title.toLowerCase()} animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={12}/>:<Pause size={12}/>}</button>}</div>
    <div className="pas-body" key={cycle}>
      <div className={`pas-request${type === 'event' ? ' pas-event' : ''}`}>{type === 'event' && <GitPullRequest size={16}/>}<span>{story.request}</span>{type === 'event' && <small>GitHub · Matching rule</small>}</div>
      {type === 'inbound' ? <div className="pas-client"><Terminal size={22}/><div><strong>Claude Code</strong><span>{phase === 0 ? 'Thinking…' : phase < 4 ? 'Reading Helpin through MCP' : 'Guide found'}</span></div></div> : <WorkflowAgent name={type === 'event' ? 'Lens' : 'Helpin AI'} role={type === 'event' ? 'Review agent' : 'Workspace assistant'} status={phase === 0 ? (type === 'event' ? 'Run started by automation' : 'Thinking…') : phase < 4 ? 'Gathering context' : 'Result ready'} working={phase < 4} />}
      <div className="pas-stage">{phase > 0 && phase < 4 && <WorkflowSources sources={story.sources} phase={phase}/>}
      {phase >= 4 && <div className="wf-outcome"><div className="pas-answer"><StreamingText text={story.answer} active={playing} duration={1900}/>{phase >= 5 && <div className="pas-artifact">{type === 'inbound' ? <><BookOpen size={14}/>Export your contacts ↗</> : type === 'event' ? <><ShieldCheck size={14}/>Review complete · Sam’s decision pending</> : <><MessageSquare size={14}/>Reply draft ready for review</>}</div>}</div><WorkflowSources sources={story.sources} phase={4} compact/></div>}
      </div>
    </div>
  </div>;
}
