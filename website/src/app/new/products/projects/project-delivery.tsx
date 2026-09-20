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
      <div className="pdl-flow" role="img" aria-label="Illustrative workflow: Maya requests Slack alerts. Forge builds the feature and Quill updates the guide from the same request. The story moves through work and review to Shipped. Sam approves the release, documentation, and customer update before Maya receives the news in her original conversation.">
        <div className="pdl-card pdl-request" aria-hidden="true">
          <span className="pdl-label">01 / THE REQUEST</span>
          <div className="pdl-person"><img src="/new/avatars/maya.webp" width={34} height={34} alt="" /><div><strong>Maya Chen</strong><span>Northstar Labs</span></div></div>
          <p className="pdl-quote">“Can you alert our team in Slack when a sync fails?”</p>
          <div className="pdl-card-foot"><Link2 size={13} /><span>Linked to ORB-491</span></div>
        </div>

        <div className="pdl-connector" data-active={phase === 1} aria-hidden="true"><i /></div>

        <div className="pdl-card pdl-work" data-active={phase === 1 || phase === 2} aria-hidden="true">
          <span className="pdl-label">02 / THE WORK</span>
          <h3>Build it. Document it.</h3>
          <div className="pdl-agent"><img src="/new/agents/forge.svg" width={28} height={28} alt="" /><div><strong>Slack sync alerts</strong><span>Forge · {released ? 'Feature released' : phase === 2 ? 'Ready for review' : 'Building the feature'}</span></div>{released && <Check size={14} />}</div>
          <div className="pdl-agent"><img src="/new/agents/quill.svg" width={28} height={28} alt="" /><div><strong>Setup guide</strong><span>Quill · {released ? 'Guide published' : phase === 2 ? 'Ready for review' : 'Updating the docs'}</span></div>{released && <Check size={14} />}</div>
          <div className="pdl-card-foot pdl-story"><span>ORB-491</span><span data-done={released}><i />{STORY[phase]}</span></div>
        </div>

        <div className="pdl-connector" data-active={phase >= 3} aria-hidden="true"><i /></div>

        <div className="pdl-card pdl-update" data-sent={sent} aria-hidden="true">
          <span className="pdl-label">03 / BACK TO MAYA</span>
          <div className="pdl-update-heading"><h3>Close the loop.</h3><span>{sent ? <CheckCheck size={14} /> : null}{sent ? 'Sent' : 'Draft'}</span></div>
          <p className="pdl-reply">Hi Maya — Slack sync alerts are live. Here’s how to get your team set up.</p>
          <div className="pdl-guide"><FileText size={14} />Set up Slack sync alerts</div>
          <div className="pdl-card-foot"><img src="/new/avatars/sam.webp" width={21} height={21} alt="" /><span>{sent ? 'Approved by Sam · Sent to Maya' : 'Sam reviews before sending'}</span></div>
        </div>
      </div>
      <div className="pdl-footer"><span>One request. Every step connected.</span><button type="button" className="pdl-playback" aria-label={paused ? 'Play delivery animation' : 'Pause delivery animation'} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    </div>
  );
}
