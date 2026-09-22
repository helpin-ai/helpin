'use client';

import { useState } from 'react';
import { ArrowDown, BookOpen, Check, FileText, Globe, HelpCircle, Pause, Play, Sparkles } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-knowledge.css';

export function SupportKnowledge({ variant = 'support' }: { variant?: 'support' | 'knowledge' }) {
  const knowledge = variant === 'knowledge';
  const copy = knowledge ? {
    question: 'The smaller export worked, but we still need the full contact list. What should we do next?',
    gap: 'The guide explains how to export—not what to do when the export is incomplete.',
    detail: '14 conversations ask the same thing. Add guidance for checking the result and reporting missing contacts, beyond the smaller-export workaround.',
    title: 'What to do when an export is incomplete',
    outline: ['Compare the exported total with the expected total.', 'Check the selected data and filters.', 'Share the export details with support when records are still missing.'],
    source: 'Prepared from 14 related conversations and the existing guide',
    review: 'Awaiting Sam’s review · Not published',
    caption: 'Repeated questions become one gap, with every conversation attached.',
  } : {
    question: 'Why does my export stop before all the contacts are included?',
    gap: 'Missing troubleshooting guidance',
    detail: 'Helpin grouped 14 conversations about incomplete exports. The export guide explains the steps, but not what to check when contacts are missing.',
    title: 'When an export is missing contacts',
    outline: ['Check your selection and filters.', 'Compare the expected and exported totals.', 'Share the report details with support.'],
    source: 'Drafted from 14 related conversations',
    review: 'Awaiting review · Not published',
    caption: 'Repeated questions become one gap and one draft.',
  };
  const { container, playing, cycle } = useBentoPlayback(9500);
  const [paused, setPaused] = useState(false);
  return <div className="support-knowledge-art" ref={container} data-playing={playing && !paused}>
    <div className="support-knowledge-toolbar"><span><span className="support-knowledge-mark">O</span>OrbitDesk<span className="support-knowledge-toolbar-divider">/</span>Knowledge</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} knowledge improvement animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    <div className="support-knowledge-scene" key={cycle} role="img" aria-label={knowledge ? "Maya and 13 other customers ask what to do when an export is incomplete. Helpin groups the 14 conversations into one coverage gap: the existing export guide lacks troubleshooting guidance. Helpin prepares a general article draft without copying private customer details. Sam has not yet reviewed or published it; agent knowledge has not changed." : "Helpin uses selected docs, website pages, and files as knowledge sources. Maya asks why her export stops before all contacts are included. Helpin groups her conversation with 13 similar ones into one coverage gap for a missing troubleshooting guide, and drafts an article for Sam to review. The draft is not published."}>
      <div aria-hidden="true">
        <div className="knowledge-sources"><span>Selected knowledge sources</span><div>{[{ Icon: BookOpen, label: 'Help articles' }, { Icon: Globe, label: 'Website pages' }, { Icon: FileText, label: 'Files' }].map(({ Icon, label }) => <span key={label}><Icon size={14} /><span>{label}</span><Check size={12} /></span>)}</div></div>
        <div className="knowledge-question"><img src="/new/avatars/maya.webp" width={28} height={28} alt="" /><div><span>Maya Chen · Northstar Labs</span><p>{copy.question}</p></div></div>
        {knowledge && <div className="knowledge-context-labels"><span><Check size={12} />14 related conversations</span><span><BookOpen size={12} />Export your contacts</span></div>}<div className="knowledge-gap"><span className="knowledge-gap-icon"><HelpCircle size={17} /></span><div><strong>{copy.gap}</strong><p>{copy.detail}</p></div><span className="knowledge-gap-badge">Coverage gap</span></div>
        <div className="knowledge-connector"><span /><ArrowDown size={16} /></div>
        <div className="knowledge-draft">
          <div className="knowledge-draft-heading"><span><img src="/brand/helpin-icon-white.svg" width={14} height={14} alt="" /></span><strong>{knowledge ? "Troubleshooting article" : "Suggested article"}</strong><span className="knowledge-draft-status">Draft</span></div>
          <h3>{copy.title}</h3>
          <div className="knowledge-outline">{copy.outline.map((line,index)=><span key={line}><span>{String(index+1).padStart(2,"0")}</span>{line}</span>)}</div>
          <div className="knowledge-evidence"><Sparkles size={13} /><span>{copy.source}</span></div>
        </div>
        <div className="knowledge-review"><img src="/new/avatars/sam.webp" width={24} height={24} alt="" /><div><strong>{copy.review}</strong><span>Sam Rivera · Reviewer</span></div><span className="knowledge-review-check"><Check size={13} /></span></div>
      </div>
    </div>
    <p className="knowledge-caption">{copy.caption}</p>
  </div>;
}
