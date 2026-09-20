'use client';

import { useState } from 'react';
import { BookOpen, Check, Inbox, PanelRight, Pause, Play, Sparkles } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

export function SupportHeroScene() {
  const { container, playing, cycle } = useBentoPlayback(8500);
  const [paused, setPaused] = useState(false);

  return <div className="support-hero-art" ref={container} data-playing={playing && !paused}>
    <div className="support-hero-art-label"><span>FROM QUESTION TO A HELPFUL REPLY</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} support inbox animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={13} aria-hidden="true" /> : <Pause size={13} aria-hidden="true" />}</button></div>
    <div className="support-hero-window">
      <div className="support-hero-window-bar"><span><Inbox size={16} aria-hidden="true" /> Inbox <span className="support-hero-divider">/</span> Okta SSO before rollout</span><PanelRight size={16} aria-hidden="true" /></div>
      <div className="support-hero-window-body" key={cycle}>
        <div className="support-hero-thread">
          <div className="support-hero-message"><div className="support-hero-author"><img src="/new/avatars/maya.webp" alt="" width={28} height={28} /><strong>Maya Chen</strong><span>Northstar Labs</span></div><p>Can we start with an admin-only Okta pilot before inviting the rest of our team?</p></div>
          <div className="support-hero-note"><span>Sam · Internal note</span><p>Setup guide linked. Role mapping is with engineering.</p></div>
          <div className="support-hero-draft"><div className="support-hero-draft-label"><Sparkles size={15} aria-hidden="true" /><strong>Helpin AI draft</strong></div><p>Yes, let’s start with your admins. Here’s the Okta setup guide. Sam will follow up on role mapping before your wider rollout.</p><div className="support-hero-source"><BookOpen size={13} aria-hidden="true" /> Okta setup guide</div><div className="support-hero-review"><Check size={13} aria-hidden="true" /> Ready for your review</div></div>
        </div>
        <div className="support-hero-customer"><span className="support-hero-rail-label">CUSTOMER CONTEXT</span><div className="support-hero-identity"><span className="support-hero-monogram">MC</span><strong>Maya Chen</strong><span>Northstar Labs</span></div><div className="support-hero-rail-item"><span>Assigned to</span><strong>Sam Rivera</strong></div><div className="support-hero-rail-item"><span>Linked project</span><strong>SSO Enterprise Readiness</strong><small><span className="support-live-dot" /> In progress</small></div><div className="support-hero-rail-item"><span>Knowledge</span><strong><BookOpen size={13} aria-hidden="true" /> Okta setup guide</strong></div></div>
      </div>
    </div>
    <p className="support-hero-art-caption"><span className="support-live-dot" /> The conversation, the context, and the next step. <span>Illustrative demo</span></p>
  </div>;
}
