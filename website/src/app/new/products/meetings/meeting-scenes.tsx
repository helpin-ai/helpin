'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { ArrowDown, CircleHelp, Building2, Check, CircleCheck, Copy, FileText, Link2, ListChecks, Mail, Pause, Play, Quote, Video } from 'lucide-react';
import { useMeetingPlayback } from './use-meeting-playback';
import { FOLLOWUP_DRAFT, MEETING_EVIDENCE } from './meeting-demo-data';
import './meeting-evidence.css';

type Variant = 'task' | 'followup' | 'context';
function Step({children,at=0,className=''}:{children:ReactNode;at?:number;className?:string}){return <div className={`mt-step ${className}`} data-stage={Math.min(3,Math.ceil(at))}>{children}</div>;}
function Avatar({person}:{person:'maya'|'sam'}){return <img className="mt-avatar" src={`/new/avatars/${person}.webp`} width={28} height={28} alt=""/>;}
function Task({phase}:{phase:number}){return <><span className="mt-mini">ACTION ITEM · FROM THE TRANSCRIPT</span><div className="mt-source"><Quote size={16}/><p>“I’ll send the reviewed setup guide and pilot checklist.”</p><div><Avatar person="sam"/><span>Sam Rivera</span><span>28:14</span></div></div><div className="mt-connector"><ArrowDown size={18}/></div><Step at={1} className="mt-task-card"><div className="mt-task-id"><ListChecks size={16}/><span>SSO Enterprise Readiness</span><span className="mt-task-review">Reviewed by Sam</span></div><strong>Share the Okta setup guide and pilot checklist</strong><div className="mt-task-meta"><span><Avatar person="sam"/>Sam Rivera</span><span><i/>To do</span><span>Customer Success</span></div><Step at={2.3} className="mt-linked"><Link2 size={13}/>Northstar Labs · SSO rollout review</Step></Step><Step at={3} className="mt-complete"><CircleCheck size={15}/><span><strong>{['Action item found in the transcript','Task destination selected','Reviewed by Sam Rivera','CS-128 created · Assigned to Sam'][phase]}</strong><small>{phase===3?'Reviewed action item · Meeting linked':'Customer Success · To do · Sam Rivera'}</small></span></Step></>;}
function Followup({active,phase}:{active:boolean;phase:number}) {
 const [typed,setTyped]=useState(FOLLOWUP_DRAFT);
 useEffect(()=>{
  if(!active){setTyped(FOLLOWUP_DRAFT);return;}
  if(phase===0){setTyped('');return;}
  if(phase!==1){setTyped(FOLLOWUP_DRAFT);return;}
  let position=0;
  const timer=setInterval(()=>{position=Math.min(position+4,FOLLOWUP_DRAFT.length);setTyped(FOLLOWUP_DRAFT.slice(0,position));if(position===FOLLOWUP_DRAFT.length)clearInterval(timer);},24);
  return ()=>clearInterval(timer);
 },[active,phase]);
 return <><div className="mt-mail-heading"><Mail size={18}/><strong>Customer follow-up</strong><span className="mt-pill">Draft</span></div><div className="mt-mail-meta"><span>To</span><span>Maya Chen <span className="mt-muted">· Northstar Labs</span></span><span>Subject</span><strong>Next steps for your SSO pilot</strong></div><div className="mt-draft-source"><span className="mt-ai-draft-mark"><img src="/brand/helpin-icon-white.svg" width={13} height={13} alt=""/></span><span>Helpin AI · {active&&phase<2?'Preparing the follow-up':'Draft from the meeting context'}</span></div><div className="mt-email-composition"><p className="mt-email-layout">{FOLLOWUP_DRAFT}</p><p className="mt-email-typed">{typed}<span className="mt-typing-caret" data-visible={active&&phase===1&&typed.length<FOLLOWUP_DRAFT.length}/></p></div><Step at={3} className="mt-draft-review"><Avatar person="sam"/><div><strong>Ready for Sam’s review</strong><small>Draft prepared · Not sent</small></div><Check size={15}/></Step></>;
}
function Context(){return <><div className="mt-customer"><span className="mt-company-mark"><Building2 size={24}/></span><div><span className="mt-mini">CUSTOMER RECORD</span><strong>Northstar Labs</strong><small>Enterprise rollout</small></div></div><div className="mt-context-list">{[
  {Icon:Video,label:'Meeting',title:'SSO rollout review',detail:'Summary, transcript & decisions'},
  {Icon:Mail,label:'Contact',title:'Maya Chen',detail:'Security & rollout requirements'},
  {Icon:Building2,label:'Deal',title:'Enterprise rollout',detail:'Security approval pending'},
  {Icon:ListChecks,label:'Task',title:'Share the setup guide',detail:'Sam Rivera · To do'},
].map(({Icon,label,title,detail},i)=><Step key={label} at={i*.8}><span className="mt-context-icon"><Icon size={16}/></span><div><span className="mt-mini">{label}</span><strong>{title}</strong><small>{detail}</small></div><Link2 size={13}/></Step>)}</div><Step at={3} className="mt-context-end"><Link2 size={14}/>One meeting. Connected to the customer and the work.</Step></>;}
const TITLES:Record<Variant,string>={task:'Action items',followup:'Follow-up draft',context:'Customer context'};
const DESCRIPTIONS:Record<Variant,string>={task:'Sam’s commitment to share an Okta setup guide is reviewed and created as a Customer Success task, assigned to Sam and linked to the source meeting.',followup:'A draft to Maya summarizes the admin-only pilot, setup guide, and security checklist. The email is ready for Sam’s review and has not been sent.',context:'The Northstar Labs customer record connects the SSO rollout meeting, Maya’s contact, the enterprise rollout deal, and Sam’s follow-up task.'};
export function MeetingScene({variant}:{variant:Variant}){
 const {container,active,phase,paused,setPaused}=useMeetingPlayback();
 const [copied,setCopied]=useState(false);
 const [announcement,setAnnouncement]=useState('');
 const copyDraft=async()=>{setPaused(true);try{await navigator.clipboard.writeText('Subject: Next steps for your SSO pilot\n\n'+FOLLOWUP_DRAFT);setCopied(true);setAnnouncement('Follow-up draft copied. Nothing has been sent.');}catch{setAnnouncement('Copy is unavailable in this browser.');}};
 return <div ref={container} className={`meeting-scene mt-${variant}`} data-playing={active} data-phase={phase}><div className="mt-toolbar"><span><b className="mt-mark">O</b>OrbitDesk<span className="mt-slash">/</span>{TITLES[variant]}</span><button type="button" aria-label={`${paused?'Play':'Pause'} ${TITLES[variant].toLowerCase()} animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={12}/>:<Pause size={12}/>}</button></div><div className="mt-content" role="img" aria-label={DESCRIPTIONS[variant]}><div aria-hidden="true">{variant==='task'?<Task phase={phase}/>:variant==='followup'?<Followup active={active} phase={phase}/>:<Context/>}</div></div>{variant==='followup'&&<div className="mt-copy-row"><span>Review before sending</span><button type="button" onClick={copyDraft}>{copied?<Check size={12}/>:<Copy size={12}/>} {copied?'Copied draft':'Copy draft'}</button><span className="mw-sr-only" role="status">{announcement}</span></div>}</div>;
}
const FINDINGS = [
  { title: 'Know what was decided.', detail: 'Keep the agreement and the words behind it together.', Icon: CircleCheck, state: 'Decision captured' },
  { title: 'Keep open questions visible.', detail: 'See what still needs an answer before work moves on.', Icon: CircleHelp, state: 'Needs clarification' },
  { title: 'Find the next step and its owner.', detail: 'Review the commitment before turning it into a task.', Icon: ListChecks, state: 'Suggested action item' },
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
          <div className="mb-finding">
            <div className="mb-finding-state"><Icon size={15} /><span>{state}</span>{index === 0 && <Check className="mb-decision-check" size={14} />}</div>
            <strong>{entry.title}</strong><p>{entry.note}</p>
            {index === 0 ? <div className="mb-pilot"><span>Admin pilot</span><i /><span>Wider rollout</span></div> : index === 1 ? <div className="mb-open"><span />Group-to-role mapping · Unresolved</div> : <div className="mb-assignment"><Avatar person="sam" /><span>Sam Rivera</span><span>Review before creating</span></div>}
            <div className="mb-source"><span className="mb-source-label"><Quote size={12} />From the conversation</span><blockquote>“{entry.quote}”</blockquote><div className="mb-speaker"><Avatar person={entry.person} /><strong>{entry.name}</strong><span><Link2 size={11} />{entry.time}</span></div></div>
          </div>
        </div>
      </article>;
    })}</div>
  </div>;
}
