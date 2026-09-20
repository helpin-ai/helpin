'use client';

import { useEffect, useState } from 'react';
import { Check, CheckCheck, FileText, GitPullRequest, Link2, MessageSquare, Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

export function ProjectDelivery() {
  const { container, playing, cycle } = useBentoPlayback(16000);
  const [paused, setPaused] = useState(false);
  const [frame, setFrame] = useState(3);
  const active = playing && !paused;
  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timers = [2000, 5000, 7500].map((time, index) => setTimeout(() => setFrame(index + 1), time));
    return () => timers.forEach(clearTimeout);
  }, [active, cycle]);
  const phase = active ? frame : 3;
  return <div className="project-delivery" ref={container} data-playing={active} data-phase={phase}>
    <div className="pd-top"><span className="ps-key">ORB-491 / RELEASE FOLLOW-UP</span><span className="pd-complete"><Check size={12} aria-hidden="true" />Shipped</span><button type="button" className="pd-playback" aria-label={`${paused ? 'Play' : 'Pause'} customer follow-up animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div key={cycle} role="img" aria-label="Illustrative follow-up for released task ORB-491. The task links to Maya’s original Slack alert request at Northstar Labs. Ask Agent prepares a reply using the shipped work and setup guide. Sam approves it, then the configured workflow sends the update in the original conversation. Approval and sending depend on the team’s configured workflow."><div aria-hidden="true">
      <h3>Slack alerts are ready for Northstar.</h3>
      <div className="pd-evidence"><span><GitPullRequest size={14} />Released change</span><span><FileText size={14} />Setup guide</span></div>
      <div className="pd-original"><div><img src="/new/avatars/maya.webp" width={28} height={28} alt="" /><span><strong>Maya Chen</strong><small>Northstar Labs · Original request</small></span><Link2 size={14} /></div><p>“Can you alert our team in Slack when a sync fails?”</p><span><Link2 size={11} />Linked task · ORB-491</span></div>
      <div className="pd-reply" data-ready={phase > 0} data-sent={phase === 3}><div><MessageSquare size={15} /><strong>Customer update</strong><span>{phase === 3 ? 'Sent' : phase === 2 ? 'Approved' : 'Draft'}</span></div><p>Hi Maya — Slack alerts for failed syncs are ready. Each alert includes the affected account, the failure reason, and a link to investigate. Here’s the setup guide to get started.</p><span className="pd-guide"><FileText size={12} />Set up Slack sync alerts</span></div>
      <div className="pd-owner" data-approved={phase >= 2}><img src={phase >= 2 ? '/new/avatars/sam.webp' : '/brand/helpin-icon-black.svg'} width={24} height={24} alt="" /><span>{phase >= 2 ? 'Approved by Sam' : 'Ask Agent prepares the update for review'}</span>{phase >= 2 && <Check size={13} />}</div>
      <div className="pd-sent" data-sent={phase === 3}><CheckCheck size={14} /><span>{phase === 3 ? 'Sent in Maya’s original conversation' : 'Sending follows your configured approvals'}</span></div>
    </div></div>
  </div>;
}
