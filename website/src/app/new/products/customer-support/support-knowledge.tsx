'use client';

import { useState } from 'react';
import { ArrowDown, BookOpen, Check, FileText, Globe, HelpCircle, Pause, Play, Sparkles } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-knowledge.css';

export function SupportKnowledge() {
  const { container, playing, cycle } = useBentoPlayback(9500);
  const [paused, setPaused] = useState(false);
  return <div className="support-knowledge-art" ref={container} data-playing={playing && !paused}>
    <div className="support-knowledge-toolbar"><span><span className="support-knowledge-mark">O</span>OrbitDesk<span className="support-knowledge-toolbar-divider">/</span>Knowledge</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} knowledge improvement animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    <div className="support-knowledge-scene" key={cycle} role="img" aria-label="Helpin uses selected docs, website pages, and files as knowledge sources. Maya asks why her CSV export stops at 10,000 rows. A coverage gap identifies a missing troubleshooting guide, and Helpin suggests an article draft for Sam to review. The draft is not published.">
      <div aria-hidden="true">
        <div className="knowledge-sources"><span>Selected knowledge sources</span><div>{[{ Icon: BookOpen, label: 'Help articles' }, { Icon: Globe, label: 'Website pages' }, { Icon: FileText, label: 'Files' }].map(({ Icon, label }) => <span key={label}><Icon size={14} /><span>{label}</span><Check size={12} /></span>)}</div></div>
        <div className="knowledge-question"><img src="/new/avatars/maya.webp" width={28} height={28} alt="" /><div><span>Maya Chen · Northstar Labs</span><p>Why does my export stop before all the contacts are included?</p></div></div>
        <div className="knowledge-gap"><span className="knowledge-gap-icon"><HelpCircle size={17} /></span><div><strong>Missing troubleshooting guidance</strong><p>The export guide explains the steps, but not what to check when contacts are missing.</p></div><span className="knowledge-gap-badge">Coverage gap</span></div>
        <div className="knowledge-connector"><span /><ArrowDown size={16} /></div>
        <div className="knowledge-draft">
          <div className="knowledge-draft-heading"><span><img src="/brand/helpin-icon-white.svg" width={14} height={14} alt="" /></span><strong>Suggested article</strong><span className="knowledge-draft-status">Draft</span></div>
          <h3>When an export is missing contacts</h3>
          <div className="knowledge-outline"><span><span>01</span>Check your selection and filters.</span><span><span>02</span>Compare the expected and exported totals.</span><span><span>03</span>Share the report details with support.</span></div>
          <div className="knowledge-evidence"><Sparkles size={13} /><span>Draft informed by the customer’s conversation</span></div>
        </div>
        <div className="knowledge-review"><img src="/new/avatars/sam.webp" width={24} height={24} alt="" /><div><strong>Awaiting review · Not published</strong><span>Sam Rivera · Reviewer</span></div><span className="knowledge-review-check"><Check size={13} /></span></div>
      </div>
    </div>
    <p className="knowledge-caption">Give the next customer a clearer place to start.</p>
  </div>;
}
