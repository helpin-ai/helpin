'use client';

import { useState } from 'react';
import { BookOpen, Check, Code2, GitPullRequest, ListChecks, MessageSquare, Pause, Play } from 'lucide-react';
import { ProjectIntake } from './project-intake';
import { SprintPlanning, RoadmapPlanning } from './planning-scenes';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowAgent, WorkflowSources } from '../../_components/WorkflowParts';
import './project-scene-story.css';

type Variant = 'context' | 'sprint' | 'roadmap' | 'agents';
const BEATS = { context: [0,1200,3000,4800,7400,9200], sprint: [0,2400,5500,8200,10600], roadmap: [0,2400,4800,7600], agents: [0,1200,3000,4800,6600,8500,10400] };
function Agents({ phase }: { phase: number }) {
  return <div className="ps-agent-story"><div className="ps-agent-task"><span className="ps-key">PRJ-214</span><h3>Add an admin-only SSO pilot</h3><span><MessageSquare size={12} />Maya’s rollout request attached</span></div>
    <WorkflowAgent name="Scribe" role="Coding task planner" status={phase === 0 ? 'Thinking…' : phase < 4 ? 'Reading the scope and codebase' : phase < 6 ? 'Coordinating implementation and review' : 'Ready for Sam’s review'} working={phase > 0 && phase < 6} />
    <div className="ps-agent-stage">{phase > 0 && phase < 4 && <WorkflowSources sources={[{icon:ListChecks,label:'Task requirements',detail:'Pilot access for selected admins only'},{icon:Code2,label:'Connected repository',detail:'Enrollment controls and role mapping'},{icon:BookOpen,label:'Team instructions',detail:'Access-boundary tests required'}]} phase={phase} />}
    {phase >= 4 && <div className="wf-outcome"><div className="ps-agent-run"><img src="/new/agents/forge.svg" width={30} height={30} alt="" /><div><strong>Forge</strong><span>{phase < 5 ? 'Adding pilot controls and tests…' : 'Pilot controls and tests ready'}</span></div>{phase >= 5 && <Check size={14} />}</div>{phase >= 5 && <div className="ps-agent-run"><img src="/new/agents/lens.svg" width={30} height={30} alt="" /><div><strong>Lens</strong><span>{phase < 6 ? 'Reviewing access boundaries…' : 'Review findings attached'}</span></div>{phase >= 6 && <Check size={14} />}</div>}{phase >= 6 && <div className="ps-human-review"><GitPullRequest size={18} /><div><strong>Pull request ready for Sam</strong><span>Awaiting approval</span></div><img src="/new/avatars/sam.webp" width={27} height={27} alt="" /></div>}</div>}</div>
  </div>;
}
export function ProjectScene({ variant }: { variant: Variant }) {
  const [paused,setPaused] = useState(false);
  const { container, playing, phase, cycle, reducedMotion } = useWorkflowPlayback({ paused, beats: BEATS[variant], duration: variant === 'sprint' ? 16500 : 15500, resetKey: variant });
  return <div className={`project-scene ps-${variant}`} ref={container} data-playing={playing} data-phase={phase}><div className="ps-toolbar"><span><span className="ps-workspace">O</span>OrbitDesk<span className="ps-toolbar-separator">/</span>{variant==='context'?'Requests':variant==='roadmap'?'Roadmap':variant==='sprint'?'Sprints':'Projects'}</span>{!reducedMotion && <button type="button" aria-label={`${paused?'Play':'Pause'} ${variant} animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={12}/>:<Pause size={12}/>}</button>}</div><div className="ps-scene-content" key={cycle}>{variant==='context'?<ProjectIntake phase={phase}/>:variant==='sprint'?<SprintPlanning phase={phase}/>:variant==='roadmap'?<RoadmapPlanning phase={phase}/>:<Agents phase={phase}/>}</div></div>;
}
