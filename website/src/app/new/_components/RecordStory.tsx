'use client';

import { useEffect, useRef, useState } from 'react';

// Section two visual. At rest, three tool fragments hold three versions of the same
// customer: a support thread, a roadmap row, a CRM card. When the section enters view
// they travel into the record and the week draws down the timeline. One authored moment,
// transforms and opacity only, plays once. Reduced motion shows the finished record.
const CARDS = 7;

const MERGE_MS = 800;
const STEP_MS = 420;

export function RecordStory() {
  const ref = useRef<HTMLDivElement>(null);
  const [phase, setPhase] = useState<'rest' | 'merge' | 'play'>('rest');
  const [n, setN] = useState(0);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) { setPhase('play'); setN(CARDS); return; }
    setPhase('rest'); setN(0);
    const timers: number[] = [];
    const start = () => {
      setPhase('merge');
      timers.push(window.setTimeout(() => {
        setPhase('play');
        let cur = 0;
        const id = window.setInterval(() => {
          cur += 1; setN(cur);
          if (cur >= CARDS) window.clearInterval(id);
        }, STEP_MS);
        timers.push(id);
      }, MERGE_MS));
    };
    if (typeof IntersectionObserver === 'undefined') { start(); return; }
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) { start(); io.disconnect(); }
    }, { threshold: 0.45 });
    io.observe(el);
    return () => { io.disconnect(); timers.forEach((t) => { window.clearTimeout(t); window.clearInterval(t); }); };
  }, []);

  const progress = n / CARDS;

  return (
    <div className={`rs-stage ${phase}`} ref={ref}>
      {/* Three versions of the same story, before Helpin. */}
      <div className="rs-frag rs-frag-a" aria-hidden="true">
        <span className="rs-frag-l">Support inbox</span>
        <b>Northstar Labs · Does SSO work with Okta?</b>
        <span>Open · assigned to Sam</span>
      </div>
      <div className="rs-frag rs-frag-b" aria-hidden="true">
        <span className="rs-frag-l">Roadmap</span>
        <b>Okta SAML mapping</b>
        <span>Backlog · no customer linked</span>
      </div>
      <div className="rs-frag rs-frag-c" aria-hidden="true">
        <span className="rs-frag-l">CRM</span>
        <b>Northstar Labs · $48k ARR</b>
        <span>Renewal in 22 days · last touch 3 weeks ago</span>
      </div>

      <div className="rs-panel" role="img" aria-label="Northstar Labs customer record overview: an AI summary of the SSO evaluation, two open conversations, the SSO Enterprise Readiness project at eight of twelve tasks, the security and renewal review meeting, the enterprise renewal deal, the published Okta article, and recent activity ending with Maya notified">
        <div className="rs-head">
          <div className="rs-av">NL</div>
          <div>
            <b>Northstar Labs</b>
            <span>Enterprise · $42k ARR · Renewal in 63 days</span>
          </div>
          <span className="rs-live">One record</span>
        </div>
        <div className="rs-tabs" aria-hidden="true">
          {['Overview', 'Conversations', 'Meetings', 'Projects', 'Deals', 'Docs', 'Activity'].map((t, i) => <span key={t} className={i === 0 ? 'on' : undefined}>{t}</span>)}
        </div>
        <div className="rs-ov" aria-hidden="true" style={{ ['--rs-p' as string]: progress }}>
          <div className="rs-col">
            <div className={`rs-card summary ${n >= 1 ? 'on' : ''}`} data-card="summary">
              <span className="rs-card-l">AI summary</span>
              <p>Northstar Labs needs SSO before its production rollout.</p>
              <p>Maya has asked about Okta twice, and the requirement came up again during the latest security review.</p>
              <p>The linked SSO project is in progress.</p>
            </div>
            <div className={`rs-card ${n >= 2 ? 'on' : ''}`} data-card="conversations">
              <div className="rs-card-h"><span className="rs-card-l">Conversations</span><span className="rs-count">2 open</span></div>
              <b>Maya R.</b>
              <blockquote>Does SSO work with Okta? Security needs the setup steps before we migrate.</blockquote>
              <span>Last reply 18m ago</span>
            </div>
            <div className={`rs-card ${n >= 3 ? 'on' : ''}`} data-card="projects">
              <span className="rs-card-l">Projects</span>
              <b>SSO Enterprise Readiness</b>
              <div className="rs-bar"><i className={n >= 3 ? 'on' : undefined} style={{ width: '66.7%' }} /></div>
              <span>8 / 12 tasks complete · 7 customer requests · 3 engineering issues</span>
              <ul className="rs-tasks">
                <li><span>Okta SAML mapping</span><em className="prog">In progress</em></li>
                <li><span>SCIM provisioning</span><em>Planned</em></li>
                <li><span>Role mapping</span><em className="done">Shipped</em></li>
              </ul>
            </div>
          </div>
          <div className="rs-col">
            <div className={`rs-card ${n >= 4 ? 'on' : ''}`} data-card="meetings">
              <span className="rs-card-l">Meetings</span>
              <b>Security &amp; renewal review</b>
              <span>2 days ago · 42 min · 3 decisions · 2 action items</span>
              <div className="rs-signal"><span className="rs-card-l">Key signal</span>SSO approval is required before production rollout.</div>
            </div>
            <div className={`rs-card ${n >= 5 ? 'on' : ''}`} data-card="deal">
              <span className="rs-card-l">Deal</span>
              <b>Enterprise renewal · $42,000</b>
              <span>Negotiation · Renewal in 63 days</span>
              <div className="rs-signal"><span className="rs-card-l">Buyer signal</span>Security approval depends on SSO readiness.</div>
            </div>
            <div className={`rs-card ${n >= 6 ? 'on' : ''}`} data-card="docs">
              <span className="rs-card-l">Docs</span>
              <b>Set up SSO with Okta</b>
              <span><em className="done">Published</em> · Updated after Change #728</span>
            </div>
            <div className={`rs-card ${n >= 7 ? 'on' : ''}`} data-card="activity">
              <span className="rs-card-l">Recent activity</span>
              <ul className="rs-act">
                <li>Change #728 merged</li>
                <li>SSO documentation updated</li>
                <li>Maya notified</li>
              </ul>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
