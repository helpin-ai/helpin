'use client';

import { StreamingText } from '../../_components/StreamingText';

import { useState, type ReactNode } from 'react';
import { ChevronDown, CircleHelp, Building2, Check, CircleCheck, Copy, Link2, ListChecks, Mail, Pause, Play, Quote, Video } from 'lucide-react';
import { useMeetingPlayback } from './use-meeting-playback';
import { FOLLOWUP_DRAFT, MEETING_EVIDENCE, MEETING_TITLE } from './meeting-demo-data';
import './meeting-evidence.css';
import './meeting-work.css';

type Variant = 'task' | 'followup' | 'context';
function Step({children,at=0,className=''}:{children:ReactNode;at?:number;className?:string}){return <div className={`mt-step ${className}`} data-stage={Math.min(3,Math.ceil(at))}>{children}</div>;}
function Avatar({person}:{person:'maya'|'sam'}){return <img className="mt-avatar" src={`/new/avatars/${person}.webp`} width={28} height={28} alt=""/>;}
function Task({phase}:{phase:number}) {
 return <div className="mt-action-preview">
  <div className="mt-action-header"><div className="mt-action-top"><span><ListChecks size={15}/>Action item</span><span className="mt-action-status" data-complete={phase===3}>{phase===3?<><Check size={12}/>Accepted</>:'Pending'}</span></div>
  <h4>Send the SSO setup guide and pilot checklist</h4>
  <div className="mt-action-owner"><Avatar person="sam"/><span>Owner: Sam Rivera</span><span>Due date: Not set</span></div>
  <div className="mt-action-evidence"><span><Quote size={12}/>Sam Rivera · Transcript · 28:14</span><blockquote>“I’ll take care of sending the setup guide and the checklist for the pilot.”</blockquote></div></div>
  <div className="mt-destination-title"><strong>Where should this work go?</strong><p>Choose where this follow-up enters project work.</p></div>
  <div className="mt-destination-fields" data-selected={phase>=1}><div><span>Team</span><strong>{phase>=1?'Customer Success':'Choose a team'}<ChevronDown size={13}/></strong></div><div><span>Status</span><strong><i/>{phase>=1?'To do':'Choose a state'}<ChevronDown size={13}/></strong></div></div>
  <div className="mt-task-source"><span>Source</span><strong>{MEETING_TITLE}</strong></div>
  <div className="mt-action-footer" data-complete={phase===3}><span>{phase===3?<><Link2 size={13}/>CS-128 created · Meeting linked</>:<>Customer Success · Follow-up work</>}</span><span className="mt-create-action">{phase===3?<><Check size={13}/>Task created</>:'Create task'}</span></div>
  <div className="mt-task-outcome"><CircleCheck size={15}/><span>{['Keep the commitment and its source together.','Team and status selected.','Ready to create the task.','Assigned to Sam, with the meeting attached.'][phase]}</span></div>
 </div>;
}
function Followup({active,phase}:{active:boolean;phase:number}) {
 return <><div className="mt-mail-envelope"><div className="mt-mail-heading"><Mail size={18}/><strong>Customer follow-up</strong><span className="mt-pill">Draft · Not sent</span></div><div className="mt-mail-meta"><span>To</span><span>Maya Chen <span className="mt-muted">· Northstar Labs</span></span><span>Subject</span><strong>Next steps for Northstar’s SSO pilot</strong></div></div><div className="mt-draft-source"><span className="mt-ai-draft-mark"><img src="/brand/helpin-icon-white.svg" width={13} height={13} alt=""/></span><span>Helpin AI · {active&&phase<2?'Preparing the follow-up':'Prepared from the meeting'}</span></div><div className="mt-email-composition"><p className="mt-email-layout">{FOLLOWUP_DRAFT}</p><p className="mt-email-typed"><StreamingText text={FOLLOWUP_DRAFT} active={active && phase === 1} pending={active && phase === 0} duration={2200} /></p></div><div className="mt-draft-review" data-ready={phase>=2}><Avatar person="sam"/><div><strong>{["Preparing the follow-up","Writing the follow-up","Check the wording and next steps","Ready for Sam’s review"][phase]}</strong><small>Customer draft · Not sent</small></div><Check size={15}/></div></>;
}
function Context(){return <><div className="mt-customer"><span className="mt-company-mark"><Building2 size={24}/></span><div><span className="mt-mini">CUSTOMER RECORD</span><strong>Northstar Labs</strong><small>Enterprise rollout</small></div></div><div className="mt-context-list">{[
  {Icon:Video,label:'Meeting',title:'SSO rollout review',detail:'Summary, transcript & decisions'},
  {Icon:Mail,label:'Contact',title:'Maya Chen',detail:'Security & rollout requirements'},
  {Icon:Building2,label:'Deal',title:'Enterprise rollout',detail:'Security approval pending'},
  {Icon:ListChecks,label:'Task',title:'Share the setup guide',detail:'Sam Rivera · To do'},
].map(({Icon,label,title,detail},i)=><Step key={label} at={i*.8}><span className="mt-context-icon"><Icon size={16}/></span><div><span className="mt-mini">{label}</span><strong>{title}</strong><small>{detail}</small></div><Link2 size={13}/></Step>)}</div><Step at={3} className="mt-context-end"><Link2 size={14}/>One meeting. Connected to the customer and the work.</Step></>;}
const TITLES:Record<Variant,string>={task:'Action items',followup:'Follow-up draft',context:'Customer history'};
const DESCRIPTIONS:Record<Variant,string>={task:'Sam’s commitment to share an Okta setup guide is created as a Customer Success task, assigned to Sam and linked to the source meeting.',followup:'A draft to Maya summarizes the admin-only pilot, setup guide, and security checklist. The email is ready for Sam’s review and has not been sent.',context:'The Northstar Labs customer record connects the SSO rollout meeting, Maya’s contact, the enterprise rollout deal, and Sam’s follow-up task.'};
export function MeetingScene({variant}:{variant:Variant}){
 const {container,active,phase,paused,setPaused}=useMeetingPlayback();
 const [copied,setCopied]=useState(false);
 const [reviewing,setReviewing]=useState(false);
 const [announcement,setAnnouncement]=useState('');
 const copyDraft=async()=>{setPaused(true);try{await navigator.clipboard.writeText('Subject: Next steps for Northstar’s SSO pilot\n\n'+FOLLOWUP_DRAFT);setCopied(true);setAnnouncement('Follow-up draft copied. Nothing has been sent.');}catch{setAnnouncement('Copy is unavailable in this browser.');}};
 return <div ref={container} className={`meeting-scene mt-${variant}`} data-playing={active} data-phase={phase}><div className="mt-toolbar"><span><b className="mt-mark">O</b>OrbitDesk<span className="mt-slash">/</span>{TITLES[variant]}</span><button type="button" aria-label={`${paused?'Play':'Pause'} ${TITLES[variant].toLowerCase()} animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={12}/>:<Pause size={12}/>}</button></div><div className="mt-content" role="img" aria-label={DESCRIPTIONS[variant]}><div aria-hidden="true">{variant==='task'?<Task phase={phase}/>:variant==='followup'?<Followup active={active} phase={phase}/>:<Context/>}</div></div>{variant==='followup'&&<div className="mt-copy-row"><button type="button" aria-pressed={reviewing} onClick={()=>{setPaused(true);setReviewing(true);setAnnouncement("Draft open for review. Nothing has been sent.");}}>{reviewing?"Reviewing draft":"Review draft"}</button><button type="button" onClick={copyDraft}>{copied?<Check size={12}/>:<Copy size={12}/>} {copied?'Copied draft':'Copy draft'}</button><span className="mw-sr-only" role="status">{announcement}</span></div>}</div>;
}
const FINDINGS = [
  { title: 'Find the agreement.', detail: 'Give the next step a clear starting point.', Icon: CircleCheck, state: 'Agreed approach' },
  { title: 'Keep uncertainty visible.', detail: 'Do not let an unresolved question read like a settled decision.', Icon: CircleHelp, state: 'Needs an answer' },
  { title: 'Make the commitment clear.', detail: 'Identify the next step before assigning the work.', Icon: ListChecks, state: 'Suggested next step' },
];
export function MeetingEvidence() {
  const { container, active, phase, paused, setPaused } = useMeetingPlayback();
  return <div className="meeting-bentos" ref={container} data-playing={active} data-phase={phase} role="region" aria-label="Decisions, open questions, and action items from the meeting">
    <div className="mb-toolbar"><span><b className="mt-mark">O</b>Northstar Labs · SSO rollout review</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} meeting insights animation`} aria-pressed={paused} onClick={() => setPaused(!paused)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div className="mb-grid">{MEETING_EVIDENCE.map((entry, index) => {
      const { title, detail, Icon, state } = FINDINGS[index];
      const reading = active && phase === index;
      return <article className="mb-card" key={entry.kind} data-kind={index} data-reading={reading}>
        <div className="mb-copy"><span className="mb-category"><Icon size={14} />{entry.kind}</span><h3>{title}</h3><p>{detail}</p></div>
        <div className="mb-art">
          <div className="mb-source"><span className="mb-source-label"><Quote size={12} />From the conversation</span><blockquote>“{entry.quote}”</blockquote><div className="mb-speaker"><Avatar person={entry.person} /><strong>{entry.name}</strong><span><Link2 size={11} />{entry.time}</span></div></div>
          <div className="mb-finding">
            <div className="mb-finding-state"><Icon size={15} /><span>{state}</span>{index === 0 && <Check className="mb-decision-check" size={14} />}</div>
            <strong>{entry.title}</strong><p>{entry.note}</p>
            {index === 0 ? <div className="mb-pilot"><span>Admin pilot</span><i /><span>Wider rollout</span></div> : index === 1 ? <div className="mb-open"><span />Group-to-role mapping · Unresolved</div> : <div className="mb-assignment"><Avatar person="sam" /><span>Sam Rivera</span></div>}
            <div className="mb-evidence-link"><Link2 size={12} /><span>Check the source</span><span>{entry.time} · Transcript</span></div>
          </div>
        </div>
      </article>;
    })}</div>
  </div>;
}
