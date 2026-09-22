'use client';

import { useEffect, useState } from 'react';
import { Check, CheckCheck, FileText, Link2, Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

const STORY = ['Planned', 'In progress', 'In review', 'Shipped', 'Shipped'];

export function ProjectDelivery() {
  const { container, playing } = useBentoPlayback(16000);
  const [paused, setPaused] = useState(false);
  const [frame, setFrame] = useState(4);
  const active = playing && !paused;

  useEffect(() => {
    if (!active) return;
    let timers: ReturnType<typeof setTimeout>[] = [];
    const start = () => {
      timers.forEach(clearTimeout);
      setFrame(0);
      timers = [1800, 4600, 7400, 10200].map((time, index) =>
        setTimeout(() => setFrame(index + 1), time),
      );
    };
    start();
    const interval = setInterval(start, 16000);
    return () => { timers.forEach(clearTimeout); clearInterval(interval); };
  }, [active]);

  // Pausing also leaves a complete, readable ending.
  const phase = active ? frame : 4;
  const released = phase >= 3;
  const sent = phase === 4;

  return (
    <div className="projects-delivery-art" ref={container} data-playing={active} data-phase={phase}>
      <div className="pdl-flow" role="img" aria-label="Illustrative workflow: Maya requests an admin-only SSO pilot. Coding agent builds the feature and Docs agent updates the guide from the same request. The story moves through work and review to Shipped. Sam reviews the proposed change. The team separately confirms release and publishes the reviewed guide. Sam then approves the customer update before Maya receives it in her original conversation.">
        <div className="pdl-card pdl-request" aria-hidden="true">
          <span className="pdl-label">01 / THE REQUEST</span>
          <div className="pdl-person"><img src="/new/avatars/maya.webp" width={34} height={34} alt="" /><div><strong>Maya Chen</strong><span>Northstar Labs</span></div></div>
          <p className="pdl-quote">“We’d like to test SSO with our admins first.”</p>
          <div className="pdl-card-foot"><Link2 size={13} /><span>PRJ-214 · Admin-only SSO pilot</span></div>
        </div>

        <div className="pdl-connector" data-active={phase === 1} aria-hidden="true"><i /></div>

        <div className="pdl-card pdl-work" data-active={phase === 1 || phase === 2} aria-hidden="true">
          <span className="pdl-label">02 / THE WORK</span>
          <h3>Ready to use. Ready to explain.</h3>
          <div className="pdl-agent"><img src="/new/agents/forge.svg" width={28} height={28} alt="" /><div><strong>Admin-only SSO pilot</strong><span>{released ? 'Release confirmed by the team' : phase === 2 ? 'Proposed change · In review' : 'Coding agent · Preparing the change'}</span></div>{released && <Check size={14} />}</div>
          <div className="pdl-agent"><img src="/new/agents/quill.svg" width={28} height={28} alt="" /><div><strong>Pilot setup guide</strong><span>{released ? 'Reviewed and published' : phase === 2 ? 'Ready for review' : 'Docs agent · Preparing the guide'}</span></div>{released && <Check size={14} />}</div>
          <p className="pdl-scope">The implementation and instructions match the agreed pilot scope.</p><div className="pdl-card-foot pdl-story"><span>PRJ-214</span><span data-done={released}><i />{STORY[phase]}</span></div>
        </div>

        <div className="pdl-connector" data-active={phase >= 3} aria-hidden="true"><i /></div>

        <div className="pdl-card pdl-update" data-sent={sent} aria-hidden="true">
          <span className="pdl-label">03 / BACK TO MAYA</span>
          <div className="pdl-update-heading"><h3>Make the next step clear.</h3><span>{sent ? <CheckCheck size={14} /> : null}{sent ? 'Sent after review' : released ? 'Draft · For review' : 'Awaiting release'}</span></div>
          <p className="pdl-reply">{released ? <>Hi Maya, the admin-only SSO pilot is ready. You can start with the selected administrators we discussed.<br /><br />Here’s the setup guide and pilot checklist. Once your team has tested the pilot, we can review the next rollout step.</> : <>The customer update will be prepared once the team confirms the release and the guide is ready.</>}</p>
          <div className="pdl-guide"><FileText size={14} />Set up your SSO pilot →</div>
          <div className="pdl-card-foot"><img src="/new/avatars/sam.webp" width={21} height={21} alt="" /><span>{sent ? 'Approved by Sam · Sent to Maya' : 'Sam reviews before sending'}</span></div>
        </div>
      </div>
      <div className="pdl-footer"><span>The customer hears what changed—and what to do next.</span><button type="button" className="pdl-playback" aria-label={paused ? 'Play delivery animation' : 'Pause delivery animation'} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    </div>
  );
}
