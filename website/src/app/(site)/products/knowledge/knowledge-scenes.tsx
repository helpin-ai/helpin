'use client';

import { useState } from 'react';
import { BookOpen, Check, FileText, Globe, LockKeyhole, Pause, Play, Search } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { WorkflowAgent, WorkflowClick } from '../../_components/WorkflowParts';
import { StreamingText } from '../../_components/StreamingText';
import { WorkScene } from '../../_components/WorkScene';
import './knowledge-source-story.css';

export function KnowledgeScene({ variant }: { variant: 'sources' | 'quill' }) {
  if (variant === 'quill') return <WorkScene variant="docs" />;
  return <SourceStory />;
}
function SourceStory() {
  const [paused, setPaused] = useState(false);
  const { container, playing, phase, cycle, reducedMotion } = useWorkflowPlayback({ paused, beats: [0, 1300, 2800, 4400, 6100, 8000], duration: 13900 });
  return <div className="knowledge-scene ks-sources ks-story" ref={container} data-playing={playing} data-phase={phase}>
    <div className="ks-toolbar"><span><b className="ks-mark">O</b>OrbitDesk<span>/</span>Knowledge sources</span>{!reducedMotion && <button type="button" aria-label={`${paused ? 'Play' : 'Pause'} knowledge sources animation`} aria-pressed={paused} onClick={() => setPaused(!paused)}>{paused ? <Play size={12} /> : <Pause size={12} />}</button>}</div>
    <div className="ks-story-body" key={cycle}>
      <WorkflowAgent name="Echo" role="Support agent" status={phase < 3 ? 'Choose its knowledge sources' : phase < 5 ? 'Finding the latest published guide' : 'Answer ready'} working={phase >= 3 && phase < 5} />
      {phase < 3 ? <div className="ks-source-picker">
        {[{ Icon: BookOpen, title: 'Public help articles', detail: 'OrbitDesk Help Center', selected: phase >= 1 }, { Icon: Globe, title: 'Product pages', detail: 'Selected website pages', selected: phase >= 2 }, { Icon: LockKeyhole, title: 'Internal rollout notes', detail: 'Private team space', selected: false }].map(({ Icon, title, detail, selected }, i) => <div key={title} data-selected={selected}><Icon size={18} /><div><strong>{title}</strong><small>{detail}</small></div><span className="ks-source-checkbox">{selected && <Check size={12} />}{phase === i && i < 2 && playing && <WorkflowClick />}</span></div>)}
      </div> : <div className="ks-answer-flow wf-outcome">
        <div className="ks-customer-question">Where is the export button now?</div>
        <div className="ks-reading-guide"><Search size={15} /><div><strong>Export your contacts</strong><small>{phase < 4 ? 'Searching selected sources…' : 'Latest published update · Actions menu'}</small></div>{phase >= 4 && <Check size={14} />}</div>
        {phase >= 5 && <div className="ks-sourced-answer"><p><StreamingText active={playing} duration={1800} text="Open Contacts, select the people you need, then choose Actions → Export selected contacts." /></p><span><BookOpen size={12} />Export your contacts</span></div>}
      </div>}
      <div className="ks-source-footer">{phase < 3 ? <><LockKeyhole size={12} />Only selected sources are available to Echo</> : <><FileText size={12} />Published guidance · Source refreshed</>}</div>
    </div>
  </div>;
}
