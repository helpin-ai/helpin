'use client';

import { useEffect, useState } from 'react';
import { ArrowUp, Check, FileText, Link2, ListChecks, MoreHorizontal, Pause, Play, Plus, Video } from 'lucide-react';
import { MeetingWorkspace } from './meeting-workspace';
import { useMeetingPlayback } from './use-meeting-playback';
import './meeting-context.css';

const BRIEF = 'Northstar agreed to an admin-only pilot. Group-to-role mapping still needs validation before a wider rollout.\n\nSam owns CS-128: share the reviewed Okta setup guide and pilot checklist. Start there, then confirm the remaining security requirements with Maya.';
function Mark() { return <span className="mc-mark"><img src="/brand/helpin-icon-white.svg" width={18} height={18} alt="" /></span>; }

// Reuses the actual meeting preview beneath the floating Ask Agent presentation.
// The meeting and linked task are supplied as context; this demo does not write data.
export function MeetingContext() {
  const { container, active, phase, paused, setPaused } = useMeetingPlayback();
  const [typed, setTyped] = useState(BRIEF);
  useEffect(() => {
    if (!active || phase === 3) { setTyped(BRIEF); return; }
    if (phase < 2) { setTyped(''); return; }
    let position = 0;
    const timer = setInterval(() => { position = Math.min(position + 4, BRIEF.length); setTyped(BRIEF.slice(0, position)); if (position === BRIEF.length) clearInterval(timer); }, 24);
    return () => clearInterval(timer);
  }, [active, phase]);
  return <div className="meeting-context-preview" ref={container} data-playing={active} data-phase={phase}>
    <div className="mc-playback"><span><Link2 size={13} />The meeting, the customer, and the next step</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} meeting handoff animation`} aria-pressed={paused} onClick={() => setPaused(!paused)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></div>
    <div className="mc-stage">
      <div className="mc-platform" inert aria-hidden="true"><MeetingWorkspace autoplay={false} /></div>
      <div className="mc-dock" role="img" aria-label="Ask Agent uses the attached Northstar Labs meeting and task CS-128 to prepare a teammate handoff. The admin pilot is agreed, role mapping still needs validation, and Sam owns the setup guide and checklist task. This is a briefing; no customer message is sent or task changed.">
        <div aria-hidden="true">
          <header className="mc-dock-header"><Mark /><div><strong>Ask Agent</strong><span>Northstar rollout handoff</span></div><MoreHorizontal size={18} /></header>
          <div className="mc-chat">
            <div className="mc-question"><img src="/new/avatars/aisha.webp" width={27} height={27} alt="" /><div><span>Aisha Patel</span><p>I’m taking the next Northstar call. What did we agree, and what needs attention?</p></div></div>
            <div className="mc-attached"><span><Video size={11} />Rollout review</span><span><ListChecks size={11} />CS-128</span></div>
            <div className="mc-answer-title"><Mark /><strong>Ask Agent</strong><span>{phase < 2 ? 'Reviewing context' : 'Handoff prepared'}</span></div>
            <div className="mc-lookups"><div data-ready={phase >= 1}><FileText size={14} /><span>Meeting summary & decisions</span><Check size={13} /></div><div data-ready={phase >= 2}><ListChecks size={14} /><span>CS-128 · Sam Rivera · To do</span><Check size={13} /></div></div>
            <div className="mc-answer"><p className="mc-answer-layout">{BRIEF}</p><p>{typed}<i data-visible={active && phase === 2 && typed.length < BRIEF.length} /></p></div>
            <div className="mc-next" data-ready={phase >= 3}><span><Check size={13} />Ready for the next conversation</span><p>Decision, open question, owner, and source in view.</p></div>
          </div>
          <div className="mc-composer"><div><Video size={12} />Northstar Labs · Rollout review</div><p>Ask a follow-up…</p><footer><Plus size={14} /><span>Workspace context</span><i><ArrowUp size={14} /></i></footer></div>
        </div>
      </div>
    </div>
  </div>;
}
