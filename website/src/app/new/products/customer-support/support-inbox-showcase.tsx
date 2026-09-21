'use client';

import { useState } from 'react';
import { Inbox, PanelRight, Sparkles } from 'lucide-react';
import { SupportWorkspace } from './support-workspace';
import './support-inbox-showcase.css';

const DETAILS = [
  { title: 'Route to the right team', copy: 'Keep chat and email in shared inboxes. Use tags and saved views to focus on what needs attention.', icon: Inbox },
  { title: 'Pick up with the full history', copy: 'See the customer, company, owner and linked task beside the conversation.', icon: PanelRight },
  { title: 'Reply with the findings', copy: 'Review an AI draft based on the investigation, add a note, and keep the customer informed.', icon: Sparkles },
];

export function SupportInboxShowcase() {
  const [selected, setSelected] = useState<number | null>(null);
  const [preview, setPreview] = useState<number | null>(null);
  const active = preview ?? selected;
  return <figure className="support-inbox-showcase">
    <div className="support-inbox-stage"><SupportWorkspace highlight={active} /></div>
    <figcaption><div className="support-inbox-callouts">{DETAILS.map(({ title, copy, icon: Icon }, index) => <button key={title} type="button" aria-pressed={selected === index} aria-label={`Highlight area ${index + 1}: ${title}`} data-active={active === index} onMouseEnter={() => setPreview(index)} onMouseLeave={() => setPreview(null)} onFocus={() => setPreview(index)} onBlur={() => setPreview(null)} onClick={() => setSelected(value => value === index ? null : index)}><span className="support-inbox-callout-number">{index + 1}</span><span className="support-inbox-callout-copy"><span className="support-inbox-callout-title"><Icon size={15} aria-hidden="true" />{title}</span><span>{copy}</span></span></button>)}</div></figcaption>
  </figure>;
}
