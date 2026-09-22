'use client';

import { useState, type CSSProperties, type ReactNode } from 'react';
import { ArrowDown, BookOpen, Check, FileText, GitPullRequest, Globe, LockKeyhole, Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

function Step({children,at=0,className=''}:{children:ReactNode;at?:number;className?:string}) {return <div className={`ks-step ${className}`} style={{'--ks-delay':`${at}s`} as CSSProperties}>{children}</div>;}
function Sources(){return <>
  <h3 className="ks-source-heading">Knowledge for the support agent</h3>
  <p className="ks-source-description">Choose the sources behind its answers.</p>
  <div className="ks-source-list">{[
    {Icon:BookOpen,title:'OrbitDesk Help Center',detail:'Getting started collection'},
    {Icon:Globe,title:'OrbitDesk product website',detail:'Selected product pages'},
    {Icon:FileText,title:'Integration reference',detail:'Uploaded file'},
  ].map(({Icon,title,detail},index)=><Step key={title} at={index*.8}><span className="ks-source-icon"><Icon size={19}/></span><div><strong>{title}</strong><small>{detail}</small></div><span className="ks-ready"><Check size={12}/>Ready</span></Step>)}</div>
  <div className="ks-connector"><ArrowDown size={19}/></div>
  <Step at={2.7} className="ks-available"><img src="/new/agents/echo.svg" width={35} height={35} alt=""/><div><strong>Available to the Support agent</strong><span>Support answers from selected sources</span></div><Check size={16}/></Step>
  <p className="ks-note"><LockKeyhole size={12}/>Internal rollout notes are not selected.</p>
  <div className="ks-update-lifecycle"><span className="ks-caption">HOW AN UPDATE REACHES AN AGENT</span>{[
    ['Draft prepared', 'The live article and agent source stay unchanged.'],
    ['Reviewed and published', 'Your team approves the guidance for readers.'],
    ['Agent source refreshed', 'The selected source index includes the published update.'],
  ].map(([title,detail],index)=><Step key={title} at={3.5+index*1.7}><span>{String(index+1).padStart(2,'0')}</span><div><strong>{title}</strong><p>{detail}</p></div></Step>)}</div>
  <p className="ks-source-maintenance">The index refreshes when you publish or unpublish.</p>
</>;}
function Quill(){return <>
  <div className="ks-quill-heading"><img src="/new/agents/quill.svg" width={43} height={43} alt=""/><div><strong>Docs agent</strong><span>Review the integration setup guide</span></div><span className="ks-badge">Docs agent</span></div>
  <p className="ks-agent-request">Check the integration guide against the released setup flow. Add the missing workspace check and prepare the edit for my review.</p>
  <div className="ks-evidence"><span className="ks-caption">WORKING FROM</span><span><BookOpen size={13}/>Existing integration guide</span><span><GitPullRequest size={13}/>Change #284 · Release confirmed by the team</span></div>
  <div className="ks-review-steps">{[
    ['Reviewed the current guide.', 'The instructions move straight from authorization to syncing.'],
    ['Checked the released change.', 'The setup flow now asks the customer to confirm the destination workspace.'],
    ['Prepared a focused edit.', 'Add the missing check before the sync begins.'],
  ].map(([title,detail],index)=><Step at={.5+index*.9} key={title}><Check size={13}/><div><strong>{title}</strong><p>{detail}</p></div></Step>)}</div>
  <Step at={3.5} className="ks-proposal"><div><FileText size={14}/><strong>Connect your first integration</strong><span>Proposed update</span></div><p>The guide should include the workspace check now shown in the setup flow.</p><div className="ks-removed"><span>−</span><del>After authorization, start the first sync.</del></div><div className="ks-added"><span>+</span>After authorization, confirm which workspace should receive the activity. Then start the first sync.</div></Step>
  <Step at={4.5} className="ks-review"><img src="/new/avatars/sam.webp" width={26} height={26} alt=""/><div><strong>Ready for Sam</strong><span>Proposed change · Not published</span></div><Check size={15}/></Step>
</>;}
export function KnowledgeScene({variant}:{variant:'sources'|'quill'}) {
  const {container,playing,cycle}=useBentoPlayback(14000);
  const [paused,setPaused]=useState(false);
  const description=variant==='sources'?'Selected OrbitDesk public docs, product pages, and an uploaded file are ready for the Support agent. Internal rollout notes are excluded. Updates have three separate steps: draft prepared, reviewed and published, and agent source refreshed. Drafting alone does not change the live article or agent answers.':'Docs agent reviews an integration guide against Change 284, whose release is confirmed by the team. It prepares a missing destination workspace check for Sam. The proposed edit is not published and does not update agent sources.';
  return <div className={`knowledge-scene ks-${variant}`} ref={container} data-playing={playing&&!paused}><div className="ks-toolbar"><span><b className="ks-mark">O</b>OrbitDesk<span>/</span>{variant==='sources'?'Knowledge sources':'Documentation'}</span><button type="button" aria-label={`${paused?'Play':'Pause'} ${variant==='sources'?'knowledge sources':'Docs agent review'} animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={12} aria-hidden="true"/>:<Pause size={12} aria-hidden="true"/>}</button></div><div className="ks-content" key={cycle} role="img" aria-label={description}><div aria-hidden="true">{variant==='sources'?<Sources/>:<Quill/>}</div></div></div>;
}
