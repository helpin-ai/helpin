'use client';

import { StreamingText } from '../../_components/StreamingText';

import { ArrowUp, Check, FileText, Link2, ListChecks, MoreHorizontal, Pause, Play, Plus, Video } from 'lucide-react';
import { MeetingWorkspace } from './meeting-workspace';
import { useMeetingPlayback } from './use-meeting-playback';
import './meeting-context.css';

const BRIEF = 'Northstar agreed to an admin-only SSO pilot before a wider rollout.\n\nGroup-to-role mapping still needs validation. Sam owns CS-128 to send the setup guide and pilot checklist. The task is currently marked To do.\n\nBefore discussing the next rollout step, confirm the guide’s status and review the remaining security requirements with Maya.';
function Mark() { return <span className="mc-mark"><img src="/brand/helpin-icon-white.svg" width={18} height={18} alt="" /></span>; }

// Reuses the actual meeting preview beneath the floating Ask Agent presentation.
// The meeting and linked task are supplied as context; this demo does not write data.
export function MeetingContext() {
  const { container, active, phase, paused, setPaused } = useMeetingPlayback();
  return <div className="meeting-context-preview" ref={container} data-playing={active} data-phase={phase}>

    <div className="mc-stage">
      <div className="mc-platform" inert aria-hidden="true"><MeetingWorkspace autoplay={false} /></div>
      <div className="mc-dock" role="region" aria-label="Helpin AI uses the attached Northstar Labs meeting and task CS-128 to prepare a teammate handoff. The admin pilot is agreed, role mapping still needs validation, and Sam owns the setup guide and checklist task. This is a briefing; no customer message is sent or task changed.">
        <div>
          <header className="mc-dock-header"><Mark /><div><strong>Helpin AI</strong><span>Northstar rollout handoff</span></div><button type="button" className="mc-inline-playback" aria-label={`${paused ? 'Play' : 'Pause'} meeting briefing animation`} aria-pressed={paused} onClick={() => setPaused(!paused)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button></header>
          <div className="mc-chat">
            <div className="mc-question"><img src="/new/avatars/aisha.webp" width={27} height={27} alt="" /><div><span>Aisha Patel</span><p>Brief me for Northstar’s next call. What did we promise, and where does the work stand?</p></div></div>
            <div className="mc-attached"><span><Video size={11} />Rollout review · Meeting decisions</span><span><ListChecks size={11} />CS-128 · Current task status</span></div>
            <div className="mc-answer-title"><Mark /><strong>Helpin AI</strong><span>{phase < 2 ? 'Reviewing context' : 'Brief ready'}</span></div>
            <div className="mc-lookups"><div data-ready={phase >= 1}><FileText size={14} /><span>Meeting summary & decisions</span><Check size={13} /></div>{phase >= 1 && <div data-ready={phase >= 2}><ListChecks size={14} /><span>CS-128 · Sam Rivera · To do</span><Check size={13} /></div>}</div>
            <div className="mc-answer"><p className="mc-answer-layout">{BRIEF}</p><p><StreamingText text={BRIEF} active={active && phase === 2} pending={phase < 2} duration={2400} /></p></div>
            {phase >= 3 && <div className="mc-next" data-ready={true}><span><Check size={13} />Ready for the next conversation</span><p>Agreement · Open question · Owner · Task status</p></div>}
          </div>
          <div className="mc-composer"><div><Video size={12} />Northstar Labs · Rollout review</div><p>Ask about the next step…</p><footer><Plus size={14} /><span>Workspace context</span><i><ArrowUp size={14} /></i></footer></div>
        </div>
      </div>
    </div>
  </div>;
}
