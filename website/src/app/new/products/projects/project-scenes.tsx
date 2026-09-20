'use client';

import { useState, type CSSProperties, type ReactNode } from 'react';
import { ArrowDown, CalendarDays, Check, GitPullRequest, Link2, MessageSquare, Pause, Play, Target } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

type Variant = 'context'|'sprint'|'roadmap'|'agents';
const SCENES: Record<Variant,{label:string;description:string}> = {
  context: { label: 'customer request', description: 'Illustrative OrbitDesk workflow: Maya at Northstar Labs requests Slack alerts for failed syncs. Task ORB-491 keeps her original conversation, customer company, and requirements attached, with Sam assigned.' },
  sprint: { label: 'sprint planning', description: 'Illustrative sprint closeout: Sprint 24 finishes with six of eight tasks completed. Two unfinished tasks are carried into Sprint 25, with the previous sprint recorded.' },
  roadmap: { label: 'roadmap', description: 'Illustrative roadmap: the objective Improve integration reliability groups three scheduled epics: Sync alerts, Webhook reliability, and Export performance, with dates and health visible.' },
  agents: { label: 'project agents', description: 'Illustrative agent workflow: Atlas prepares a plan for sync alerts, Scribe refines task ORB-491, Forge prepares changes and tests, and Lens returns review findings. Sam reviews the proposed change, which is not merged.' },
};
function Step({children,at=0,className=''}:{children:ReactNode;at?:number;className?:string}) { return <div className={`ps-step ${className}`} style={{'--ps-delay':`${at}s`} as CSSProperties}>{children}</div>; }
function Context() { return <>
  <div className="ps-customer"><img src="/new/avatars/maya.webp" width={32} height={32} alt="" /><div><strong>Maya Chen</strong><span>Northstar Labs · Customer conversation</span></div></div>
  <div className="ps-question">Can you alert our team in Slack when a sync fails? We only notice when a customer reports it.</div>
  <div className="ps-link-line"><span /><Link2 size={15} /><span /></div>
  <Step at={.7} className="ps-request-task"><div><span className="ps-key">ORB-491</span><span className="ps-badge">Feature</span></div><h3>Add Slack alerts for failed syncs</h3><p>Notify the team with the failed sync, affected account, and a link to investigate.</p><div className="ps-task-properties"><span>Planned</span><span>High priority</span><span><img src="/new/avatars/sam.webp" width={18} height={18} alt="" />Sam Rivera</span></div></Step>
  <Step at={1.8} className="ps-context-link"><MessageSquare size={14} /><span>Maya’s conversation attached</span><Check size={13} /></Step>
  <Step at={2.6} className="ps-context-link"><Link2 size={14} /><span>Northstar Labs linked</span><Check size={13} /></Step>
</>; }
function Sprint() { return <>
  <div className="ps-sprint-head"><div><span className="ps-key">ENGINEERING</span><h3>Sprint 24</h3></div><span className="ps-badge">Completed</span></div>
  <div className="ps-sprint-date"><CalendarDays size={13} />Sep 7–18</div>
  <div className="ps-progress-label"><span>6 of 8 tasks completed</span><strong>75%</strong></div><div className="ps-progress-track"><span /></div>
  <div className="ps-sprint-stats"><div><strong>6</strong><span>Completed</span></div><div><strong>2</strong><span>Unfinished</span></div></div>
  <Step at={1.3} className="ps-rollover"><ArrowDown size={17} /><span>Carry unfinished work forward</span></Step>
  <Step at={2.1} className="ps-next-sprint"><div><h3>Sprint 25</h3><span>Sep 21–Oct 2</span></div><span className="ps-badge">2 carried over</span></Step>
  <Step at={2.8} className="ps-closeout"><Check size={13} />Sprint 24 closeout recorded</Step>
</>; }
function Roadmap() { return <>
  <div className="ps-objective"><Target size={18} /><div><span>Objective</span><strong>Improve integration reliability</strong></div></div>
  <div className="ps-roadmap-months"><span>SEP</span><span>OCT</span><span>NOV</span></div>
  <div className="ps-roadmap-rows">
    <div><span>Sync alerts<small>Sep 21 – Oct 9</small></span><Step at={.35} className="ps-roadmap-bar ps-bar-one"><span>On track</span></Step></div>
    <div><span>Webhook reliability<small>Oct 1 – Oct 23</small></span><Step at={1} className="ps-roadmap-bar ps-bar-two"><span>At risk</span></Step></div>
    <div><span>Export performance<small>Oct 19 – Nov 13</small></span><Step at={1.65} className="ps-roadmap-bar ps-bar-three"><span>Planned</span></Step></div>
  </div>
  <Step at={2.5} className="ps-roadmap-note"><Target size={14} /><span>3 epics connected to one objective</span></Step>
</>; }
function Agents() { return <>
  <div className="ps-agent-task"><span className="ps-key">ORB-491</span><h3>Add Slack alerts for failed syncs</h3><span><MessageSquare size={12} />Maya’s request stays attached</span></div>
  <div className="ps-agent-steps">{[
    {name:'Atlas',file:'atlas',action:'Shape the plan',detail:'Requirements and scope'},
    {name:'Scribe',file:'scribe',action:'Refine the task',detail:'Implementation details'},
    {name:'Forge',file:'forge',action:'Prepare the change',detail:'Code and regression tests'},
    {name:'Lens',file:'lens',action:'Review the work',detail:'Findings and verification'},
  ].map((agent,index)=><Step at={.4+index*.8} key={agent.name}><img src={`/new/agents/${agent.file}.svg`} width={34} height={34} alt="" /><div><span>{agent.name}</span><strong>{agent.action}</strong><small>{agent.detail}</small></div><Check size={14} /></Step>)}</div>
  <Step at={4} className="ps-human-review"><GitPullRequest size={18} /><div><strong>Ready for Sam’s review</strong><span>Proposed changes · Not merged</span></div><img src="/new/avatars/sam.webp" width={27} height={27} alt="" /></Step>
</>; }
export function ProjectScene({variant}:{variant:Variant}) {
  const {container,playing,cycle}=useBentoPlayback(10000);
  const [paused,setPaused]=useState(false);
  return <div className={`project-scene ps-${variant}`} ref={container} data-playing={playing&&!paused}><div className="ps-toolbar"><span><span className="ps-workspace">O</span>OrbitDesk<span className="ps-toolbar-separator">/</span>{variant==='roadmap'?'Roadmap':variant==='sprint'?'Sprints':'Projects'}</span><button type="button" aria-label={`${paused?'Play':'Pause'} ${SCENES[variant].label} animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={12} aria-hidden="true"/>:<Pause size={12} aria-hidden="true"/>}</button></div><div className="ps-scene-content" key={cycle} role="img" aria-label={SCENES[variant].description}><div aria-hidden="true">{variant==='context'?<Context/>:variant==='sprint'?<Sprint/>:variant==='roadmap'?<Roadmap/>:<Agents/>}</div></div></div>;
}
