'use client';

import { useEffect, useState } from 'react';
import { Check, CheckCheck, FileText, Link2, Pause, Play } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowClick } from '../../_components/WorkflowParts';
import { StreamingText } from '../../_components/StreamingText';

const STORY = ['Planned', 'In progress', 'In review', 'Shipped', 'Shipped'];

export function ProjectDelivery() {
  const [paused, setPaused] = useState(false);
  const { container, playing: active, phase } = useWorkflowPlayback({ paused, beats: [0,1800,5000,7900,11200], duration: 17000 });
  const released = phase >= 3;
  const sent = phase === 4;

  return (
    <div className="projects-delivery-art" ref={container} data-playing={active} data-phase={phase}>
      <div className="pdl-flow" role="region" aria-label="Illustrative workflow: Maya requests an admin-only SSO pilot. Coding agent builds the feature and Docs agent updates the guide from the same request. The story moves through work and review to Shipped. Sam reviews the proposed change. The team separately confirms release and publishes the reviewed guide. Echo prepares the customer update after the release is confirmed. Sam approves it before Echo sends it to Maya in her original conversation.">
        <div className="pdl-card pdl-request">
          <span className="pdl-label">01 / THE REQUEST<button type="button" className="pdl-playback" aria-label={paused ? 'Play delivery animation' : 'Pause delivery animation'} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12}/> : <Pause size={12}/>}</button></span>
          <div className="pdl-person"><img src="/new/avatars/maya.webp" width={34} height={34} alt="" /><div><strong>Maya Chen</strong><span>Northstar Labs</span></div></div>
          <p className="pdl-quote">“We’d like to test SSO with our admins first.”</p>
          <div className="pdl-card-foot"><Link2 size={13} /><span>PRJ-214 · Admin-only SSO pilot</span></div>
        </div>

        <div className="pdl-connector" data-active={phase === 1} aria-hidden="true"><i /></div>

        <div className="pdl-card pdl-work" data-active={phase === 1 || phase === 2} aria-hidden="true">
          <span className="pdl-label">02 / THE WORK</span>
          <h3>Prepare the change and guide.</h3>
          <div className="pdl-agent"><img src="/new/agents/forge.svg" width={28} height={28} alt="" /><div><strong>Admin-only SSO pilot</strong><span>{released ? 'Release confirmed by the team' : phase === 2 ? 'Proposed change · In review' : 'Forge · Reading the task and codebase'}</span></div>{released && <Check size={14} />}</div>
          <div className="pdl-agent"><img src="/new/agents/quill.svg" width={28} height={28} alt="" /><div><strong>Pilot setup guide</strong><span>{released ? 'Reviewed and published' : phase === 2 ? 'Ready for review' : 'Quill · Reading the guide and scope'}</span></div>{released && <Check size={14} />}</div>
          <div className="pdl-scope">{phase === 2 ? <span className="pdl-approve">Approve changes<WorkflowClick delay={1300}/></span> : <span>Customer report · Repository · Setup guide</span>}</div><div className="pdl-card-foot pdl-story"><span>PRJ-214</span><span data-done={released}><i />{STORY[phase]}</span></div>
        </div>

        <div className="pdl-connector" data-active={phase >= 3} aria-hidden="true"><i /></div>

        <div className="pdl-card pdl-update" data-sent={sent} aria-hidden="true">
          <span className="pdl-label">03 / BACK TO MAYA</span>
          <div className="pdl-update-heading"><h3>Echo updates Maya.</h3><span>{sent ? <CheckCheck size={14} /> : null}{sent ? 'Sent by Echo' : released ? 'Draft · For review' : 'Awaiting release'}</span></div>
          <p className="pdl-reply">{released ? <><StreamingText active={active && phase === 3} duration={1900} text="Hi Maya, the admin-only SSO pilot is ready. Here’s the setup guide and checklist. Let us know how your pilot goes." /></> : <>The customer update will be prepared once the team confirms the release and the guide is ready.</>}</p>
          <div className="pdl-guide"><FileText size={14} />Set up your SSO pilot →</div>
          <div className="pdl-card-foot"><img src="/new/avatars/sam.webp" width={21} height={21} alt="" /><span className={phase === 3 ? 'pdl-approve' : undefined}>{sent ? 'Approved by Sam · Echo sent the update' : 'Approve customer update'}{phase === 3 && <WorkflowClick delay={1600}/>}</span></div>
        </div>
      </div>

    </div>
  );
}
