'use client';

import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { Check, Inbox, PanelRight, Sparkles, X } from 'lucide-react';
import './support-inbox-showcase.css';

const DETAILS = [
  { title: 'A place for every conversation', copy: 'Shared and team inboxes, with dedicated queues for AI handling and resolved conversations.', icon: Inbox, area: { left: '4.5%', top: '7.5%', width: '13.2%', height: '63%' }, pin: { left: '17.7%', top: '32%' } },
  { title: 'Customer context, always beside you', copy: 'Contact details, conversation routing, tags, and linked tasks stay alongside the thread.', icon: PanelRight, area: { left: '79.3%', top: '5.5%', width: '20.2%', height: '89%' }, pin: { left: '79.3%', top: '39%' } },
  { title: 'The tools to move it forward', copy: 'Reply, leave a note, or try a Helpin AI draft grounded in the conversation and linked work.', icon: Sparkles, area: { left: '39.9%', top: '78.4%', width: '38.8%', height: '20%' }, pin: { left: '77.6%', top: '80.9%' } },
] as const;

export function SupportInboxShowcase() {
  const [selected, setSelected] = useState<number | null>(null);
  const [preview, setPreview] = useState<number | null>(null);
  const [draftOpen, setDraftOpen] = useState(false);
  const [draftUsed, setDraftUsed] = useState(false);
  const draftPanel = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement | null>(null);
  const active = preview ?? selected;

  useEffect(() => {
    if (draftOpen) draftPanel.current?.focus({ preventScroll: true });
  }, [draftOpen]);

  function openDraft(button: HTMLButtonElement) {
    trigger.current = button;
    setDraftUsed(false);
    setDraftOpen(true);
  }
  function closeDraft() {
    setDraftOpen(false);
    trigger.current?.focus({ preventScroll: true });
  }

  return <figure className="support-inbox-showcase">
    <div className="support-inbox-stage">
      <div className="support-inbox-stage-heading"><span><Inbox size={15} aria-hidden="true" />One inbox. The whole customer story.</span><div className="support-inbox-stage-actions"><button type="button" onClick={event => openDraft(event.currentTarget)} aria-expanded={draftOpen} aria-controls="support-ai-draft"><Sparkles size={14} aria-hidden="true" />Try an AI draft</button></div></div>
      <div className="support-inbox-capture">
        <img src="/new/support/inbox-orbitdesk-sep20-1440.webp" srcSet="/new/support/inbox-orbitdesk-sep20-960.webp 960w, /new/support/inbox-orbitdesk-sep20-1440.webp 1440w, /new/support/inbox-orbitdesk-sep20-1920.webp 1920w, /new/support/inbox-orbitdesk-sep20-3840.webp 3840w" sizes="(max-width: 760px) calc(100vw - 72px), (max-width: 1240px) calc(100vw - 112px), 1128px" width={3840} height={2016} alt="Illustrative OrbitDesk inbox: Maya at Northstar Labs reports an incomplete CSV export. Sam’s internal note, task EXP-142, AI-applied tags, routing, and customer details stay beside the thread." loading="lazy" decoding="async" />
        <div className="support-inbox-annotations" aria-hidden="true">{DETAILS.map(({ area, pin }, index) => <div key={index}><span className="support-inbox-region" data-active={active === index} style={area as CSSProperties} /><span className="support-inbox-pin" data-active={active === index} style={pin as CSSProperties}>{index + 1}</span></div>)}</div>
        <button className="support-inbox-ai-trigger" type="button" onClick={event => openDraft(event.currentTarget)} aria-label="Open Helpin AI draft for Maya" aria-expanded={draftOpen} aria-controls="support-ai-draft"><Sparkles aria-hidden="true" /><span>AI Tools</span></button>
        <div id="support-ai-draft" hidden={!draftOpen} className="support-inbox-draft" ref={draftPanel} tabIndex={-1} role="region" aria-label="Interactive Helpin AI reply draft" onKeyDown={event => { if (event.key === 'Escape') { event.preventDefault(); closeDraft(); } }}>
          <div className="support-inbox-draft-header"><span className="support-inbox-draft-mark"><img src="/brand/helpin-icon-white.svg" width={16} height={16} alt="" /></span><div><strong>{draftUsed ? 'Reply draft' : 'Helpin AI draft'}</strong><span>{draftUsed ? 'Ready for your review' : 'Suggested reply to Maya'}</span></div><button type="button" onClick={closeDraft} aria-label="Close AI draft"><X size={15} aria-hidden="true" /></button></div>
          <div className="support-inbox-draft-body"><p>Thanks, Maya. We’ve reproduced the export stopping at 10,000 rows and linked your report to EXP-142. Your “Active customers, created this year” filters are included in the investigation.</p><p>We’ll update you here once we have a confirmed fix or workaround. There’s no need to resend those details.</p></div>
          <div className="support-inbox-draft-context"><Check size={12} aria-hidden="true" />Conversation history + linked task</div>
          <div className="support-inbox-draft-actions"><span aria-live="polite">{draftUsed ? 'Added to the demo reply. Not sent.' : 'Review before sending.'}</span><button type="button" onClick={() => setDraftUsed(true)} aria-disabled={draftUsed}>{draftUsed ? <><Check size={12} aria-hidden="true" />Draft added</> : 'Use this draft'}</button></div>
        </div>
      </div>
    </div>
    <figcaption><div className="support-inbox-callouts">{DETAILS.map(({ title, copy, icon: Icon }, index) => <button key={title} type="button" aria-pressed={selected === index} aria-label={`Highlight area ${index + 1}: ${title}`} data-active={active === index} onMouseEnter={() => setPreview(index)} onMouseLeave={() => setPreview(null)} onFocus={() => setPreview(index)} onBlur={() => setPreview(null)} onClick={() => setSelected(value => value === index ? null : index)}><span className="support-inbox-callout-number">{index + 1}</span><span className="support-inbox-callout-copy"><span className="support-inbox-callout-title"><Icon size={15} aria-hidden="true" />{title}</span><span>{copy}</span></span></button>)}</div></figcaption>
  </figure>;
}
