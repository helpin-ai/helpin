'use client';

import { useRef, useState, type KeyboardEvent } from 'react';
import { ArrowDown, ArrowRight, Check, CheckCheck, Inbox, ListFilter, Mail, Pause, Play, Route, Tag, Users } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-inbox-features.css';

const FEATURES = [
  {
    id: 'email', icon: Mail, label: 'Email forwarding',
    title: 'Keep your support address. Give your team a shared inbox.',
    copy: 'Forward email from your existing address into Helpin. Questions arrive as conversations your team can assign, annotate, and answer alongside live chat.',
    points: ['Forward into the shared inbox or a specific team inbox.', 'Test your forwarding setup and see when it is verified.', 'Configure a verified sender address for replies from your domain.'],
    description: 'Email sent to support@orbitdesk.example is forwarded into the shared inbox. The forwarding setup is verified, and Maya’s email arrives as a conversation.',
  },
  {
    id: 'routing', icon: Route, label: 'Routing & assignment',
    title: 'Get each question to the team that can answer it.',
    copy: 'Give billing, technical support, and other teams their own inboxes. Use routing rules or AI triage to suggest a destination, and enable automatic moves when you’re ready.',
    points: ['Route by message content or sender details.', 'Keep team inbox access limited to its members.', 'Assign work manually or use round-robin assignment.'],
    description: 'A customer asks for an invoice update. A rule matches the word invoice and routes the conversation to Billing. Round-robin assignment selects Sam Rivera.',
  },
  {
    id: 'organize', icon: Tag, label: 'Tags & saved views',
    title: 'Turn a busy inbox into a clear view of the work.',
    copy: 'Tag conversations by topic, request, or product area. Combine filters into saved views so your team can return to the conversations that need attention.',
    points: ['Create and apply the tags that fit your workflow.', 'Filter conversations by tags, owner, and status.', 'Save personal views or share them with your team.'],
    description: 'Maya’s CSV export issue is tagged Bug and Exports. A saved team view filters open conversations tagged Exports and shows that issue alongside another export question.',
  },
] as const;

function Illustration({ variant }: { variant: string }) {
  if (variant === 'email') return <>
    <div className="inbox-proof-heading"><Mail size={16} /><span>Forwarding setup</span><span className="inbox-proof-status"><Check size={12} /> Verified</span></div>
    <div className="inbox-proof-step inbox-proof-address"><span>YOUR EXISTING ADDRESS</span><strong>support@orbitdesk.example</strong></div>
    <div className="inbox-proof-connector inbox-proof-step"><ArrowDown size={18} /><span>Email forwarding</span></div>
    <div className="inbox-proof-step inbox-proof-destination"><Inbox size={20} /><div><strong>Shared inbox</strong><span>OrbitDesk</span></div><CheckCheck size={16} /></div>
    <div className="inbox-proof-step inbox-proof-email"><div className="inbox-proof-person"><img src="/new/avatars/maya.webp" width={28} height={28} alt="" /><div><strong>Maya Chen</strong><span>Northstar Labs · Email</span></div><small>New</small></div><strong>Can you help with our CSV export?</strong><p>The export stops before all our rows are included.</p><div className="inbox-proof-foot"><Mail size={13} /> The email is now a shared conversation.</div></div>
  </>;
  if (variant === 'routing') return <>
    <div className="inbox-proof-heading"><Route size={16} /><span>Routing & assignment</span><span className="inbox-proof-status"><Check size={12} /> Enabled</span></div>
    <div className="inbox-proof-step inbox-proof-question"><span>NEW CONVERSATION</span><p>“Can you update the company name on our invoice?”</p></div>
    <div className="inbox-proof-step inbox-proof-rule"><span>WHEN THE MESSAGE CONTAINS</span><strong>invoice</strong><ArrowRight size={16} /><div><span>MOVE TO</span><strong>Billing</strong></div></div>
    <div className="inbox-proof-connector inbox-proof-step"><ArrowDown size={18} /><span>Round-robin assignment</span></div>
    <div className="inbox-proof-step inbox-proof-owner"><img src="/new/avatars/sam.webp" width={38} height={38} alt="" /><div><strong>Sam Rivera</strong><span>Billing inbox · Assigned</span></div><CheckCheck size={17} /></div>
    <div className="inbox-proof-step inbox-proof-result"><Check size={14} /> The right inbox. A clear owner.</div>
  </>;
  return <>
    <div className="inbox-proof-heading"><ListFilter size={16} /><span>Organize conversations</span><span className="inbox-proof-status"><Users size={12} /> Team view</span></div>
    <div className="inbox-proof-step inbox-proof-question"><span>MAYA CHEN · NORTHSTAR LABS</span><p>CSV export stops before all rows are included</p><div className="inbox-proof-tags"><span><Tag size={11} /> Bug</span><span><Tag size={11} /> Exports</span></div></div>
    <div className="inbox-proof-step inbox-proof-view"><div><ListFilter size={16} /><strong>Open export issues</strong></div><div className="inbox-proof-filters"><span>Status: Open</span><span>Tag: Exports</span></div></div>
    <div className="inbox-proof-step inbox-proof-results"><div><span className="support-live-dot" /><strong>CSV export stops early</strong><small>Maya</small></div><div><span className="support-live-dot" /><strong>Missing date column</strong><small>Alex</small></div></div>
    <div className="inbox-proof-step inbox-proof-result"><Check size={14} /> A saved view your team can come back to.</div>
  </>;
}

function InboxProof({ feature, paused, onToggle }: { feature: typeof FEATURES[number]; paused: boolean; onToggle: () => void }) {
  const { container, playing, cycle } = useBentoPlayback(8500);
  return <div className="inbox-feature-proof" ref={container} data-playing={playing && !paused}>
    <div className="inbox-proof-toolbar"><span>OrbitDesk</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} inbox feature animation`} aria-pressed={paused} onClick={onToggle}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    <div key={cycle} className={`inbox-proof-scene inbox-proof-${feature.id}`} role="img" aria-label={feature.description}><div aria-hidden="true"><Illustration variant={feature.id} /></div></div>
  </div>;
}

export function SupportInboxFeatures() {
  const [selected, setSelected] = useState(0);
  const [paused, setPaused] = useState(false);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const feature = FEATURES[selected];

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next = index;
    if (event.key === 'ArrowRight') next = (index + 1) % FEATURES.length;
    else if (event.key === 'ArrowLeft') next = (index + FEATURES.length - 1) % FEATURES.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = FEATURES.length - 1;
    else return;
    event.preventDefault();
    setSelected(next);
    tabs.current[next]?.focus();
  }

  return <div className="inbox-features">
    <div className="inbox-feature-tabs" role="tablist" aria-label="Explore inbox features">{FEATURES.map(({ id, icon: Icon, label }, index) => <button key={id} ref={element => { tabs.current[index] = element; }} type="button" role="tab" id={`inbox-tab-${id}`} aria-controls={`inbox-panel-${id}`} aria-selected={selected === index} tabIndex={selected === index ? 0 : -1} onClick={() => setSelected(index)} onKeyDown={event => onKeyDown(event, index)}><Icon size={17} aria-hidden="true" />{label}</button>)}</div>
    {FEATURES.map(({ id, title, copy, points }, index) => <div key={id} role="tabpanel" id={`inbox-panel-${id}`} aria-labelledby={`inbox-tab-${id}`} hidden={selected !== index} tabIndex={0}>
      {selected === index && <div className="inbox-feature-panel">
        <div className="inbox-feature-copy"><h3>{title}</h3><p>{copy}</p><ul>{points.map(point => <li key={point}><Check size={15} aria-hidden="true" /><span>{point}</span></li>)}</ul>{id === 'routing' && <p className="inbox-feature-note">Round-robin assignment is available on eligible plans.</p>}</div>
        <InboxProof key={id} feature={feature} paused={paused} onToggle={() => setPaused(value => !value)} />
      </div>}
    </div>)}
  </div>;
}
