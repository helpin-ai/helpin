'use client';

import { useState } from 'react';
import { Inbox, PanelRight, Sparkles } from 'lucide-react';
import { SupportWorkspace } from './support-workspace';
import './support-inbox-showcase.css';

const DETAILS = [
  { title: 'A place for every conversation', copy: 'Shared and team inboxes, with dedicated queues for AI handling and resolved conversations.', icon: Inbox },
  { title: 'Customer context, always beside you', copy: 'Contact details, conversation routing, tags, and linked tasks stay alongside the thread.', icon: PanelRight },
  { title: 'The tools to move it forward', copy: 'Reply, leave a note, or try a Helpin AI draft grounded in the conversation and linked work.', icon: Sparkles },
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
