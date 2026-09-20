'use client';

import { useState } from 'react';
import { BookOpen, Check, GitPullRequest, PanelRight, Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

export function SupportHeroScene() {
  const { container, playing, cycle } = useBentoPlayback(8500);
  const [paused, setPaused] = useState(false);

  return <div className="support-hero-art" ref={container} data-playing={playing && !paused}>
    <div className="support-hero-art-label"><span>ANSWER. HAND OFF. KEEP IT MOVING.</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} support inbox animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={13} aria-hidden="true" /> : <Pause size={13} aria-hidden="true" />}</button></div>
    <div className="support-hero-window">
      <div className="support-hero-window-bar"><span className="support-hero-workspace"><span className="support-workspace-logo" aria-hidden="true">O</span><strong>OrbitDesk</strong><span className="support-hero-divider">/</span><span>Shared inbox</span></span><PanelRight size={16} aria-hidden="true" /></div>
      <div className="support-hero-window-body" key={cycle} role="img" aria-label="Illustrative support workflow: Maya asks about an Okta admin pilot and incorrect roles. Helpin AI answers from the setup guide. The role-mapping issue is linked to SSO Enterprise Readiness, and Sam receives the conversation with a handoff note."><div className="support-hero-window-content" aria-hidden="true">
        <div className="support-hero-thread">
          <div className="support-hero-message"><div className="support-hero-author"><img src="/new/avatars/maya.webp" alt="" width={28} height={28} /><strong>Maya Chen</strong><span>Northstar Labs</span></div><p>Can we pilot Okta with just our admins? The group mapping gives them the wrong role.</p></div>
          <div className="support-hero-answer"><div className="support-hero-answer-label"><span className="support-hero-ai-mark"><img src="/brand/helpin-icon-ink.svg" width={14} height={14} alt="" /></span><strong>Helpin AI</strong></div><p>Yes. Use an admin-only group for the pilot, following the Okta setup guide. I’m passing the role-mapping issue to our team.</p><div className="support-hero-source"><BookOpen size={13} aria-hidden="true" /> Okta setup guide</div><div className="support-hero-sent"><Check size={13} aria-hidden="true" /> Answer sent to Maya</div></div>
          <div className="support-hero-linked"><GitPullRequest size={14} /><div><strong>SSO / 142 · Role mapping</strong><span>Conversation linked · In progress</span></div><Check size={13} /></div>
          <div className="support-hero-note"><span><img src="/new/avatars/sam.webp" width={20} height={20} alt="" /> Handed to Sam · Internal note</span><p>Admin pilot guide shared. Incorrect roles need investigation. Original conversation attached.</p></div>
        </div>
        <div className="support-hero-customer"><span className="support-hero-rail-label">CUSTOMER CONTEXT</span><div className="support-hero-identity"><img src="/new/avatars/maya.webp" width={44} height={44} alt="" /><strong>Maya Chen</strong><span>Northstar Labs</span></div><div className="support-hero-rail-item support-hero-assigned"><span>Assigned to</span><strong>Sam Rivera</strong></div><div className="support-hero-rail-item"><span>Linked project</span><strong>SSO Enterprise Readiness</strong><small><span className="support-live-dot" /> In progress</small></div><div className="support-hero-rail-item"><span>Knowledge</span><strong><BookOpen size={13} aria-hidden="true" /> Okta setup guide</strong></div></div>
      </div></div>
    </div>
    <p className="support-hero-art-caption"><span className="support-live-dot" /> The conversation, the context, and the next step. <span>Illustrative demo</span></p>
  </div>;
}
