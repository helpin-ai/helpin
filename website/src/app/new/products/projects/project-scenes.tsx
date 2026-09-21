'use client';

import { useEffect, useState, type CSSProperties, type ReactNode } from 'react';
import { Check, GitPullRequest, MessageSquare, Pause, Play } from 'lucide-react';
import { ProjectIntake } from './project-intake';
import { SprintPlanning, RoadmapPlanning } from './planning-scenes';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

type Variant = 'context'|'sprint'|'roadmap'|'agents';
const SCENES: Record<Variant,{label:string;description:string}> = {
  context: { label: 'customer request', description: 'Illustrative request-to-task workflow in OrbitDesk. Maya at Northstar Labs asks for Slack alerts when syncs fail. Ask Agent prepares requirements, Engineering as the team, High priority, Sam as owner, and Sprint 24. Sam approves the plan, then task ORB-491 is created with the original conversation and company linked. The conversation also shows the linked task.' },
  sprint: { label: 'sprint planning', description: 'Illustrative sprint planning and closeout. Two selected backlog tasks join six existing tasks in Sprint 24. Later, six of eight tasks are completed. The two unfinished tasks carry into Sprint 25, retaining the original sprint history.' },
  roadmap: { label: 'roadmap', description: 'Illustrative roadmap: the objective Make integration failures easier to act on groups three scheduled epics: Sync alerts, Webhook reliability, and Rollout readiness, with dates and health visible.' },
  agents: { label: 'project agents', description: 'Illustrative agent workflow: Planning agent prepares a plan for sync alerts, Coding task planner refines task ORB-491, Coding agent prepares changes and tests, and Code reviewer returns review findings. Sam reviews the proposed change, which is not merged.' },
};
function Step({children,at=0,className=''}:{children:ReactNode;at?:number;className?:string}) { return <div className={`ps-step ${className}`} style={{'--ps-delay':`${at}s`} as CSSProperties}>{children}</div>; }
function Agents() { return <>
  <div className="ps-agent-task"><span className="ps-key">ORB-491</span><h3>Add Slack alerts for failed syncs</h3><span><MessageSquare size={12} />Maya’s request stays attached</span></div>
  <div className="ps-agent-steps">{[
    {name:'Planning agent',file:'atlas',action:'Shape the plan',detail:'Requirements and scope'},
    {name:'Coding task planner',file:'scribe',action:'Refine the task',detail:'Implementation details'},
    {name:'Coding agent',file:'forge',action:'Prepare the change',detail:'Code and regression tests'},
    {name:'Code reviewer',file:'lens',action:'Review the work',detail:'Findings and verification'},
  ].map((agent,index)=><Step at={.4+index*.8} key={agent.name}><img src={`/new/agents/${agent.file}.svg`} width={34} height={34} alt="" /><div><span>{agent.name}</span><strong>{agent.action}</strong><small>{agent.detail}</small></div><Check size={14} /></Step>)}</div>
  <Step at={4} className="ps-human-review"><GitPullRequest size={18} /><div><strong>Ready for Sam’s review</strong><span>Proposed changes · Not merged</span></div><img src="/new/avatars/sam.webp" width={27} height={27} alt="" /></Step>
</>; }
export function ProjectScene({variant}:{variant:Variant}) {
  const planning = variant === 'sprint' || variant === 'roadmap' || variant === 'context';
  const {container,playing,cycle}=useBentoPlayback(variant === 'sprint' ? 18000 : planning ? 14000 : 10000);
  const [paused,setPaused]=useState(false);
  const active = playing && !paused;
  const [frame, setFrame] = useState(3);
  useEffect(() => {
    if (!active || !planning) return;
    setFrame(0);
    const times = variant === 'sprint' ? [2000, 5500, 8500, 11000] : [2000, 4500, 7000];
    const timers = times.map((time, index) => setTimeout(() => setFrame(index + 1), time));
    return () => timers.forEach(clearTimeout);
  }, [active, planning, cycle, variant]);
  const phase = active ? frame : variant === 'sprint' ? 4 : 3;
  return <div className={`project-scene ps-${variant}`} ref={container} data-playing={active} data-phase={planning ? phase : undefined}><div className="ps-toolbar"><span><span className="ps-workspace">O</span>OrbitDesk<span className="ps-toolbar-separator">/</span>{variant==='context'?'Requests':variant==='roadmap'?'Roadmap':variant==='sprint'?'Sprints':'Projects'}</span><button type="button" aria-label={`${paused?'Play':'Pause'} ${SCENES[variant].label} animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={12} aria-hidden="true"/>:<Pause size={12} aria-hidden="true"/>}</button></div><div className="ps-scene-content" key={cycle} role="img" aria-label={SCENES[variant].description}><div aria-hidden="true">{variant==='context'?<ProjectIntake phase={phase}/>:variant==='sprint'?<SprintPlanning phase={phase}/>:variant==='roadmap'?<RoadmapPlanning phase={phase}/>:<Agents/>}</div></div></div>;
}
