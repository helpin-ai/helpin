'use client';

import { useState, type CSSProperties } from 'react';
import { Check, Link2, MessageSquare, Pause, Play, Square } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

export function ProjectHero() {
  const { container, playing, cycle } = useBentoPlayback(9500);
  const [paused, setPaused] = useState(false);
  const delay = (seconds: number) => ({ '--hero-delay': `${seconds}s` }) as CSSProperties;
  return <div className="project-hero-art" ref={container} data-playing={playing && !paused}>
    <div className="project-hero-art-label"><span>FROM CUSTOMER REQUEST TO CLEAR TASK</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} project planning animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div className="project-hero-task" key={cycle} role="img" aria-label="OrbitDesk task ORB-491, Add Slack alerts for failed syncs, is planned and owned by Sam. Maya’s original request from Northstar Labs stays attached. Atlas has drafted requirements for Sam to review before work starts.">
      <div aria-hidden="true">
        <div className="pht-toolbar"><span className="ps-workspace">O</span><strong>OrbitDesk</strong><span>/</span><span>Projects</span><span className="pht-task-key">ORB-491</span></div>
        <div className="pht-body">
          <div className="pht-type"><span/>Feature request</div>
          <h2>Add Slack alerts for failed syncs</h2>
          <div className="pht-properties"><span className="pht-status">Planned</span><span>High priority</span><span><img src="/new/avatars/sam.webp" width={22} height={22} alt=""/>Sam Rivera</span></div>
          <div className="pht-request pht-reveal" style={delay(.35)}><div><img src="/new/avatars/maya.webp" width={28} height={28} alt=""/><span><strong>Maya Chen</strong><small>Northstar Labs</small></span><MessageSquare size={14}/></div><p>“Can you alert our team in Slack when a sync fails? We only notice when a customer reports it.”</p></div>
          <div className="pht-requirements pht-reveal" style={delay(1.3)}><span>WHAT NEEDS TO BE TRUE</span><div><Square size={13}/>Notify the team when a sync fails</div><div><Square size={13}/>Include the affected account and error</div><div><Square size={13}/>Link directly to the sync details</div></div>
          <div className="pht-agent pht-reveal" style={delay(2.3)}><img src="/new/agents/atlas.svg" width={34} height={34} alt=""/><div><strong>Atlas prepared the task outline</strong><span>Requirements ready for Sam’s review</span></div><Check size={15}/></div>
        </div>
        <div className="pht-footer"><Link2 size={13}/><span>The original request stays attached</span><span>1 conversation</span></div>
      </div>
    </div>
  </div>;
}
