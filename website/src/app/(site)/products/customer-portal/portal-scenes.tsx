'use client';

import { useState, type ReactNode } from 'react';
import { ArrowUpRight, Check, DoorOpen, FileText, Lock, Paperclip, Pause, Play, Search, Sparkles, ThumbsDown, ThumbsUp, UserPlus } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import '../../_components/docs-paths/docs-paths.css';
import '../customer-support/support-desk-scenes.css';
import './portal-scenes.css';

// App-styled scenes for the customer portal page. Labels follow the product
// (frontend/src/components/customer-portal, settings/CustomerPortalSettings, crm/CompanyPortalPanel,
// support/reports/ReportPortalIntake). Names, companies and figures are fictional.

const MAYA = '/new/avatars/maya.webp';
const SAM = '/new/home/teammate-sam.svg';

function Playback({ paused, onToggle }: { paused: boolean; onToggle: () => void }) {
  return <button type="button" className="dp-playback" onClick={onToggle} aria-pressed={paused}>{paused ? <Play size={12} /> : <Pause size={12} />}{paused ? 'Play' : 'Pause'}</button>;
}

function PortalHeader({ active }: { active: 'My requests' | 'New request' | 'Team' | 'Help articles' }) {
  return <div className="cp-header">
    <span className="cp-brand"><span className="cp-brand-mark">O</span>OrbitDesk Support</span>
    <nav aria-hidden="true">{['My requests', 'All requests', 'Copied to me', 'Help articles', 'Team'].map(item => <span key={item} data-active={item === active}>{item}</span>)}</nav>
    <span className="cp-new">New request</span>
  </div>;
}

function Browser({ children }: { children: ReactNode }) {
  return <div className="cp-browser">
    <div className="cp-chrome" aria-hidden="true"><i /><i /><i /><span><Lock size={11} />help.orbitdesk.example/requests</span></div>
    {children}
  </div>;
}

const REQUESTS = [
  { title: 'Webhook retries after deploy', type: 'Integrations', status: 'Active', tone: 'current', meta: 'Reply by today at 14:30', selected: true },
  { title: 'SSO for the Berlin team', type: 'Account access', status: 'Waiting on you', tone: 'blocker', meta: 'Your team replied' },
  { title: 'Invoices export is missing VAT', type: 'Billing', status: 'Resolved', tone: 'positive', meta: '2 days ago' },
  { title: 'Add a second workspace', type: 'Billing', status: 'Resolved', tone: 'positive', meta: 'Last week' },
];

export function PortalHeroScene() {
  return <figure className="cp-figure" aria-label="The OrbitDesk customer portal on its help center domain: a list of requests with their status, and an open request that shows when to expect a reply.">
    <Browser>
      <PortalHeader active="My requests" />
      <div className="cp-hero-body">
        <div className="cp-list">
          <div className="cp-search"><Search size={13} />Search requests…</div>
          {REQUESTS.map(request => <div key={request.title} className="cp-req" data-selected={request.selected}>
            <span className="cp-req-main"><b>{request.title}</b><small>{request.type} · {request.meta}</small></span>
            <span className="cp-status" data-tone={request.tone}>{request.status}</span>
          </div>)}
        </div>
        <div className="cp-detail">
          <header><strong>Webhook retries after deploy</strong><span className="cp-status" data-tone="current">Active</span></header>
          <p className="cp-detail-meta">Integrations · Requested by Maya Chen · Copied: Jonas Weber</p>
          <p className="cp-replyby">Expect a reply by today at 14:30</p>
          <div className="sd-msg"><img src={MAYA} width={24} height={24} alt="" /><div><b>Maya Chen</b><p>Since this morning’s deploy, every webhook to our endpoint is retried three times before it’s accepted.</p></div></div>
          <div className="cp-composer">Write a reply, or drop files here…<Paperclip size={13} /></div>
        </div>
      </div>
    </Browser>
  </figure>;
}

// Stages: 0 Ask AI answers from help articles · 1 the customer writes a request instead · 2 related articles appear
const SELF_BEATS = [0, 5200, 7400];
const SELF_STATUS = [
  'Ask AI answers a question from the help articles and links the source.',
  'Maya starts a new request.',
  'Related help articles appear while she writes.',
];

export function SelfServiceScene() {
  const [paused, setPaused] = useState(false);
  const { container, phase, cycle, playing } = useWorkflowPlayback({ beats: SELF_BEATS, duration: 13500, paused });
  return <div className="dp-root sd-root" ref={container} data-playing={playing}>
    <div className="dp-frame cp-frame">
      <PortalHeader active={phase === 0 ? 'Help articles' : 'New request'} />
      <div className="cp-self" key={cycle}>
        {phase === 0 ? <div className="cp-ask">
          <p className="cp-ask-title"><Sparkles size={14} />Ask AI <small>Answers from the help articles</small></p>
          <div className="cp-ask-q">How do I add our VAT number to invoices?</div>
          <div className="cp-ask-a">
            <p>Open <b>Billing → Company details</b> and enter your VAT number. It appears on every invoice created after you save it.</p>
            <span className="cp-source"><FileText size={12} />Add your VAT number to invoices<ArrowUpRight size={11} aria-hidden="true" /></span>
            <span className="cp-ask-actions"><ThumbsUp size={12} aria-label="Helpful" /><ThumbsDown size={12} aria-label="Not helpful" /><span>Contact support</span></span>
          </div>
        </div> : <div className="sd-form">
          <label>What is your request about?<span className="sd-select">Billing</span></label>
          <label>Summary<span className="sd-input">Invoices export is missing VAT</span></label>
          {phase >= 2 && <div className="sd-suggest">
            <b>Related articles</b>
            <small>These might answer your question. They open in a new tab.</small>
            <span><FileText size={13} />Add your VAT number to invoices</span>
            <span><FileText size={13} />Export invoices as CSV</span>
          </div>}
          <div className="sd-form-actions"><span className="sd-attach"><Paperclip size={13} />Attach files</span><span className="sd-btn">Send request</span></div>
        </div>}
      </div>
    </div>
    <Playback paused={paused} onToggle={() => setPaused(value => !value)} />
    <span className="dp-sr-only" role="status">{SELF_STATUS[phase]}</span>
  </div>;
}

export function SelfServiceReport() {
  return <figure className="cp-report" aria-label="The Portal self-service report with fictional figures.">
    <p className="cp-report-title">Portal self-service <small>Last 30 days</small></p>
    <div className="cp-report-grid">
      <div><span>Likely answered by an article</span><b>156</b><small>Opened a suggested article and sent no request within 30 minutes</small></div>
      <div><span>Opened an article</span><b>412</b><small>Of 980 forms that showed suggestions</small></div>
      <div><span>Asked after reading</span><b>96</b><small>Sent a request after opening an article</small></div>
      <div><span>Nothing to suggest</span><b>41</b><small>Requests sent when no article matched. Coverage gaps collects them.</small></div>
    </div>
  </figure>;
}

// Stages: 0 request types · 1 "Bug report" chosen, its fields appear · 2 file attached and a colleague copied
const TYPE_BEATS = [0, 2400, 5600];
const TYPES = ['Billing', 'Bug report', 'Account access', 'Integrations'];
const TYPE_STATUS = [
  'The customer chooses what the request is about.',
  'Bug report asks for the area and the steps to reproduce.',
  'A screenshot is attached and a colleague is copied.',
];

export function RequestTypesScene() {
  const [paused, setPaused] = useState(false);
  const { container, phase, cycle, playing } = useWorkflowPlayback({ beats: TYPE_BEATS, duration: 12500, paused });
  return <div className="dp-root sd-root" ref={container} data-playing={playing}>
    <div className="dp-frame cp-frame">
      <PortalHeader active="New request" />
      <div className="sd-form cp-types" key={cycle}>
        <label>What is your request about?<span className="sd-select">{phase >= 1 ? 'Bug report' : 'Choose one'}</span></label>
        {phase === 0 && <div className="cp-menu" role="listbox" aria-label="Request types">{TYPES.map(type => <span key={type} role="option" aria-selected={type === 'Bug report'} data-hover={type === 'Bug report'}>{type}</span>)}</div>}
        {phase >= 1 && <>
          <label>Which area is affected?<span className="sd-select">Exports</span></label>
          <label>Steps to reproduce<span className="sd-input sd-textarea">Export contacts with more than 10,000 rows. The file stops at row 10,000.</span></label>
        </>}
        {phase >= 2 && <div className="cp-extras">
          <span className="cp-file"><Paperclip size={12} />export-screenshot.png</span>
          <span className="cp-cc"><UserPlus size={12} />Copied: Jonas Weber</span>
        </div>}
        {phase >= 1 && <div className="sd-form-actions"><span className="sd-attach"><Paperclip size={13} />Attach files</span><span className="sd-btn">Send request</span></div>}
      </div>
    </div>
    <Playback paused={paused} onToggle={() => setPaused(value => !value)} />
    <span className="dp-sr-only" role="status">{TYPE_STATUS[phase]}</span>
  </div>;
}

// Stages: 0 waiting, with the reply-by promise · 1 the team replies, the request updates live
const PROMISE_BEATS = [0, 4200];
const PROMISE_STATUS = [
  'The request shows when to expect a reply.',
  'Sam replies. The reply appears as new and the request now waits on the customer.',
];

export function ReplyByScene() {
  const [paused, setPaused] = useState(false);
  const { container, phase, cycle, playing } = useWorkflowPlayback({ beats: PROMISE_BEATS, duration: 11500, paused });
  const replied = phase >= 1;
  return <div className="dp-root sd-root" ref={container} data-playing={playing}>
    <div className="dp-frame cp-frame">
      <PortalHeader active="My requests" />
      <div className="sd-thread cp-promise" key={cycle}>
        <header><strong>Webhook retries after deploy</strong><span className="cp-status" data-tone={replied ? 'blocker' : 'current'}>{replied ? 'Waiting on you' : 'Active'}</span></header>
        {replied ? <p className="cp-detail-meta">Integrations · Your team replied</p> : <p className="cp-replyby">Expect a reply by today at 14:30</p>}
        <div className="sd-msg"><img src={MAYA} width={24} height={24} alt="" /><div><b>Maya Chen</b><p>Since this morning’s deploy, every webhook to our endpoint is retried three times before it’s accepted.</p></div></div>
        {replied && <div className="sd-msg sd-msg-team cp-new-reply"><img src={SAM} width={24} height={24} alt="" /><div><b>Sam Rivera <small>Support</small><span className="cp-new-badge">New reply</span></b><p>Thanks, Maya. Your endpoint now takes longer than the delivery timeout to respond, so each delivery is retried. The delivery log shows every attempt; once responses are faster, the retries stop.</p></div></div>}
      </div>
    </div>
    <Playback paused={paused} onToggle={() => setPaused(value => !value)} />
    <span className="dp-sr-only" role="status">{PROMISE_STATUS[phase]}</span>
  </div>;
}

const TEAM = [
  { name: 'Maya Chen', email: 'maya@northstarlabs.example', role: 'Admin', note: 'Sees company requests and manages people', photo: MAYA },
  { name: 'Jonas Weber', email: 'jonas@northstarlabs.example', role: 'Member', note: 'Sees every company request' },
  { name: 'Inès Laurent', email: 'ines@northstarlabs.example', role: 'Requester', note: 'Sees only their own requests' },
];

export function CompanyAccessScene() {
  return <figure className="cp-access" aria-label="The portal Team tab with admin, member and requester roles, and the company access panel in the Helpin CRM.">
    <div className="dp-frame cp-frame cp-frame-auto">
      <PortalHeader active="Team" />
      <div className="cp-team">
        <div className="cp-team-head"><b>Northstar Labs</b><span className="cp-invite"><UserPlus size={12} />Invite a colleague</span></div>
        {TEAM.map(person => <div key={person.name} className="cp-member">
          {person.photo ? <img src={person.photo} width={28} height={28} alt="" /> : <span className="cp-member-initials">{person.name.split(' ').map(part => part[0]).join('')}</span>}
          <span className="cp-member-main"><b>{person.name}</b><small>{person.email}</small></span>
          <span className="cp-role"><b>{person.role}</b><small>{person.note}</small></span>
        </div>)}
      </div>
    </div>
    <div className="cp-crm">
      <p className="cp-crm-title"><DoorOpen size={14} />Customer portal <small>Northstar Labs · CRM company</small></p>
      <dl>
        <div><dt>Company access</dt><dd><span className="sd-toggle" aria-label="On" /></dd></div>
        <div><dt>Company admins</dt><dd>Maya Chen</dd></div>
        <div><dt>Admins can invite colleagues</dt><dd><Check size={13} />On</dd></div>
      </dl>
    </div>
  </figure>;
}

export function PortalInboxScene() {
  return <figure className="cp-inbox" aria-label="A portal request in the Helpin team inbox with a private suggestion from the support agent and the customer's rating.">
    <div className="dp-frame cp-frame cp-frame-auto">
      <div className="dp-bar"><span className="dp-logo">O</span><span className="dp-bar-title">Inbox</span></div>
      <div className="cp-inbox-body">
        <div className="cp-inbox-row"><img src={MAYA} width={28} height={28} alt="" /><span className="sd-row-main"><b>Maya Chen</b><small>Invoices export is missing VAT</small></span><span className="cp-channel"><DoorOpen size={11} />Portal</span><span className="sd-chip" data-tone="idle">FRT 22m</span></div>
        <div className="cp-suggestion">
          <p><Sparkles size={13} /><b>Private suggestion</b><small>Team only · customers never see it</small></p>
          <p className="cp-suggestion-text">Exports created after Monday’s update leave the VAT column out. Suggest re-running the export after the fix in EXP-151, and link “Export invoices as CSV”.</p>
        </div>
        <div className="cp-rated"><Check size={13} />Resolved by Sam Rivera · Rated <b>Very satisfied</b></div>
      </div>
    </div>
  </figure>;
}
