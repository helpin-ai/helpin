'use client';

import { useEffect, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { KnowledgeWorkspace } from '../../products/knowledge/knowledge-previews';
import { useBentoPlayback } from '../useBentoPlayback';
import { AskAgentWindow } from './AskAgentWindow';
import { REVIEW_RUNS, RUN_COMPLETE_PHASE, RUN_CYCLE_MS, RUN_PHASE_TIMES, type DemoRunId } from './ask-agent-demo';
import './ask-agent-workspace.css';

/** The existing Knowledge workspace beneath a reusable, interactive agent window. */
export function AskAgentWorkspace() {
  const assistantName = 'Helpin AI';
  const [open, setOpen] = useState(true);
  const [paused, setPaused] = useState(false);
  const [replay, setReplay] = useState(0);
  const [selected, setSelected] = useState<DemoRunId>('review');
  const { container, playing, reducedMotion } = useBentoPlayback(0, open);
  const [frame, setFrame] = useState(0);
  const active = playing && open && !paused;
  const phase = reducedMotion ? RUN_COMPLETE_PHASE : frame;

  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timers = RUN_PHASE_TIMES.map((delay, index) => setTimeout(() => setFrame(index + 1), delay));
    // Selection and replay reset this timeout too, so a clicked run gets its full story.
    const next = selected === 'chat' ? undefined : setTimeout(() => {
      const index = REVIEW_RUNS.findIndex(run => run.id === selected);
      setFrame(0);
      setSelected(REVIEW_RUNS[(index + 1) % REVIEW_RUNS.length].id);
    }, RUN_CYCLE_MS);
    return () => { timers.forEach(clearTimeout); clearTimeout(next); };
  }, [active, replay, selected]);

  function startRun(value: DemoRunId) {
    setFrame(0);
    setSelected(value);
    setPaused(false);
    setReplay(value => value + 1);
  }

  return <div className="aaw-scene" ref={container} data-playing={active} data-phase={phase} data-open={open}>
    <div className="aaw-background" aria-hidden="true" inert><KnowledgeWorkspace autoplay={false} apiHref="/products/knowledge#knowledge-api" /></div>
    {open && <AskAgentWindow selected={selected} onSelectRun={startRun} streaming={!reducedMotion} playing={active} phase={phase} paused={paused} onInteract={() => setPaused(true)} onTogglePlayback={() => setPaused(value => !value)} onMinimize={() => { setPaused(true); setOpen(false); }} />}
    <button className="aaw-launcher" type="button" aria-label={`${open ? 'Minimize' : 'Open'} ${assistantName} preview`} aria-expanded={open} onClick={() => { if (open) { setPaused(true); setOpen(false); } else { setOpen(true); startRun(selected); } }}><img src="/brand/helpin-icon-ink.svg" width={24} height={24} alt="" /><span>{assistantName}</span><ChevronDown size={13} /></button>
  </div>;
}
