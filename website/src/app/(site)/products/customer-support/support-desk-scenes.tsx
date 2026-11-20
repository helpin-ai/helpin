'use client';

import { useState } from 'react';
import { Angry, Check, Clock3, FileText, Frown, Laugh, Meh, Paperclip, Pause, Play, Smile, UserRound } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import '../../_components/docs-paths/docs-paths.css';
import './support-desk-scenes.css';

// App-styled scenes for the Support page. Labels follow the product:
// SLA policies (frontend/src/components/settings/sla), the customer portal
// (frontend/src/components/customer-portal) and Reports (frontend/src/components/support/reports).
// Names, figures and companies are fictional.

const MAYA = '/new/avatars/maya.webp';
const SAM = '/new/home/teammate-sam.svg';

function Playback({ paused, onToggle }: { paused: boolean; onToggle: () => void }) {
  return <button type="button" className="dp-playback" onClick={onToggle} aria-pressed={paused}>{paused ? <Play size={12} /> : <Pause size={12} />}{paused ? 'Play' : 'Pause'}</button>;
}

function Bar({ title }: { title: string }) {
  return <div className="dp-bar"><span className="dp-logo">O</span><span className="dp-bar-title">{title}</span></div>;
}

// Stages: 0 policy and timers · 1 target nears the warning window · 2 "SLA due soon" warning and auto-assignment · 3 reply sent, target met
const SLA_BEATS = [0, 2200, 4200, 8400];
const SLA_STATUS = [
  'An SLA policy for urgent customers sets first-response, next-response and resolution targets in business hours.',
  'The next-response target on conversation 142 is close to due.',
  'Helpin shows an SLA due soon warning and assigns the conversation to Sam Rivera.',
  'Sam replies and the next-response target is met.',
];

export function SlaScene() {
  const [paused, setPaused] = useState(false);
  const { container, phase, cycle, playing } = useWorkflowPlayback({ beats: SLA_BEATS, duration: 14000, paused });
  const chip = phase >= 3 ? { text: 'NRT met', tone: 'met' } : phase >= 1 ? { text: 'NRT 9m', tone: 'soon' } : { text: 'NRT 14m', tone: 'idle' };
  return <div className="dp-root sd-root" ref={container} data-playing={playing}>
    <div className="dp-frame sd-frame sd-frame-sla">
      <Bar title="SLA policies" />
      <div className="sd-sla" key={cycle}>
        <div className="sd-policy" role="group" aria-label="SLA policy">
          <header><strong>Urgent customers</strong><span className="sd-pill sd-pill-on">Enabled</span></header>
          <dl>
            <div><dt>Applies to</dt><dd>Priority: Urgent, High · Company tier: Enterprise</dd></div>
            <div><dt>Targets</dt><dd className="sd-targets"><span>First response <b>30m</b></span><span>Next response <b>1h</b></span><span>Resolution <b>8h</b></span></dd></div>
            <div><dt>Count business hours only</dt><dd><span className="sd-toggle" aria-label="On" /></dd></div>
            <div><dt>Warn before a target is missed</dt><dd>10 minutes</dd></div>
            <div><dt>If nobody is assigned at the warning, assign to</dt><dd className="sd-person"><img src={SAM} width={18} height={18} alt="" />Sam Rivera</dd></div>
          </dl>
        </div>
        <div className="sd-list" role="group" aria-label="Inbox">
          <div className="sd-row" data-active={phase >= 1}>
            <img className="sd-avatar" src={MAYA} width={28} height={28} alt="" />
            <span className="sd-row-main"><b>Maya Chen</b><small>#142 · CSV export missing contacts</small></span>
            <span className="sd-chip" data-tone={chip.tone}><Clock3 size={11} />{chip.text}</span>
            <span className="sd-owner">{phase >= 2 ? <img src={SAM} width={20} height={20} alt="Sam Rivera" /> : <UserRound size={14} aria-label="Unassigned" />}</span>
          </div>
          <div className="sd-row">
            <span className="sd-initials">AM</span>
            <span className="sd-row-main"><b>Alex Morgan</b><small>#139 · Invoice for last month</small></span>
            <span className="sd-chip" data-tone="idle"><Clock3 size={11} />FRT 3h 57m</span>
            <span className="sd-owner"><UserRound size={14} aria-label="Unassigned" /></span>
          </div>
          <div className="sd-row">
            <span className="sd-initials" data-tone="2">NW</span>
            <span className="sd-row-main"><b>Noah Williams</b><small>#137 · Webhook deliveries are retrying</small></span>
            <span className="sd-chip" data-tone="idle"><Clock3 size={11} />NRT 2h 10m</span>
            <span className="sd-owner"><img src={SAM} width={20} height={20} alt="Sam Rivera" /></span>
          </div>
        </div>
        {phase === 2 && <div className="sd-toast" role="presentation">
          <span className="sd-toast-icon"><Clock3 size={15} /></span>
          <div><strong>SLA due soon in #142</strong><p>The next response target of Urgent customers is due in 9m. Assigned to Sam Rivera.</p></div>
        </div>}
        {phase >= 3 && <p className="sd-met"><Check size={13} />Sam replied · Next response: met</p>}
      </div>
    </div>
    <Playback paused={paused} onToggle={() => setPaused(value => !value)} />
    <span className="dp-sr-only" role="status">{SLA_STATUS[phase]}</span>
  </div>;
}

// Stages: 0 new request with a request type · 1 related help articles appear · 2 the team replies · 3 the customer rates the support
const PORTAL_BEATS = [0, 2400, 5800, 9200];
const PORTAL_STATUS = [
  'Maya opens a new billing request in the customer portal.',
  'Related help articles appear while she writes.',
  'Sam from the support team replies in the portal.',
  'The request is resolved and Maya rates the support as very satisfied.',
];
const RATINGS = [
  { icon: Angry, label: 'Very unsatisfied' },
  { icon: Frown, label: 'Unsatisfied' },
  { icon: Meh, label: 'Neutral' },
  { icon: Smile, label: 'Satisfied' },
  { icon: Laugh, label: 'Very satisfied' },
];

export function PortalScene() {
  const [paused, setPaused] = useState(false);
  const { container, phase, cycle, playing } = useWorkflowPlayback({ beats: PORTAL_BEATS, duration: 15000, paused });
  const thread = phase >= 2;
  return <div className="dp-root sd-root" ref={container} data-playing={playing}>
    <div className="dp-frame sd-frame">
      <Bar title="OrbitDesk Support" />
      <div className="sd-portal" key={cycle}>
        <nav className="sd-portal-nav" aria-hidden="true"><span data-active={!thread}>New request</span><span data-active={thread}>My requests</span><span>All requests</span><span>Copied to me</span><span>Help articles</span></nav>
        {!thread ? <div className="sd-form">
          <label>What is your request about?<span className="sd-select">Billing</span></label>
          <label>Summary<span className="sd-input">Invoices export is missing VAT</span></label>
          <label>Details<span className="sd-input sd-textarea">The CSV from Billing → Invoices has had no VAT column since Monday.</span></label>
          {phase >= 1 && <div className="sd-suggest">
            <b>Related articles</b>
            <small>These might answer your question. They open in a new tab.</small>
            <span><FileText size={13} />Add your VAT number to invoices</span>
            <span><FileText size={13} />Export invoices as CSV</span>
          </div>}
          <div className="sd-form-actions"><span className="sd-attach"><Paperclip size={13} />Attach files</span><span className="sd-btn">Send request</span></div>
        </div> : <div className="sd-thread">
          <header><strong>Invoices export is missing VAT</strong><span className="sd-pill">{phase >= 3 ? 'Resolved' : 'New reply'}</span></header>
          <div className="sd-msg"><img src={MAYA} width={24} height={24} alt="" /><div><b>Maya Chen</b><p>The CSV from Billing → Invoices has had no VAT column since Monday.</p></div></div>
          <div className="sd-msg sd-msg-team"><img src={SAM} width={24} height={24} alt="" /><div><b>Sam Rivera <small>Support</small></b><p>Thanks, Maya. Exports created after Monday’s update left the VAT column out. That’s fixed, and your next export will include it.</p></div></div>
          {phase >= 3 && <div className="sd-rating">
            <b>How was the support you received?</b>
            <div className="sd-faces" role="radiogroup" aria-label="Your rating">{RATINGS.map(({ icon: Icon, label }, index) => <span key={label} role="radio" aria-checked={index === 4} aria-label={label} data-selected={index === 4}><Icon size={17} /></span>)}</div>
            <small>Your rating goes to the team that handled this request.</small>
          </div>}
        </div>}
      </div>
    </div>
    <Playback paused={paused} onToggle={() => setPaused(value => !value)} />
    <span className="dp-sr-only" role="status">{PORTAL_STATUS[phase]}</span>
  </div>;
}

const RATING_SPLIT = [2, 3, 6, 27, 62];

export function ReportsCard() {
  return <figure className="dp-root sd-root sd-reports-figure" aria-label="Helpin support reports for the last 30 days with fictional figures">
    <div className="dp-frame sd-frame sd-reports-frame">
      <div className="dp-bar"><span className="dp-logo">O</span><span className="dp-bar-title">Reports</span><span className="sd-period">Last 30 days · All inboxes</span></div>
      <div className="sd-reports">
        <div className="sd-report-block"><h4>Volume and speed</h4><div className="sd-metrics">
          <div><span>New conversations</span><b>1,284</b><small>12% vs previous period</small></div>
          <div><span>Resolved</span><b>1,236</b><small>9% vs previous period</small></div>
          <div><span>First response (team)</span><b>9m</b><small>Median · 90th percentile 41m</small></div>
        </div></div>
        <div className="sd-report-block"><h4>AI agent</h4><div className="sd-metrics">
          <div><span>AI resolution rate</span><b>58%</b></div>
          <div><span>AI involvement</span><b>83%</b><small>1,066 of 1,284 new conversations</small></div>
          <div><span>Handed to the team</span><b>21%</b><small>270 handoffs</small></div>
        </div></div>
        <div className="sd-reports-split">
          <div className="sd-report-block"><h4>Customer satisfaction <small>312 ratings</small></h4>
            <div className="sd-csat"><span>Resolved by AI <b>4.5</b></span><span>Resolved by team <b>4.7</b></span></div>
            <div className="sd-dist" aria-hidden="true">{RATING_SPLIT.map((share, index) => <i key={index} style={{ width: `${share}%` }} data-level={index} />)}</div>
          </div>
          <div className="sd-report-block"><h4>SLA targets <small>418 conversations</small></h4>
            <div className="sd-sla-rates"><span>First response <b>97%</b></span><span>Next response <b>94%</b></span><span>Resolution <b>91%</b></span></div>
          </div>
        </div>
      </div>
    </div>
  </figure>;
}
