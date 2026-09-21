'use client';

import { useEffect, useState } from 'react';
import { ChevronDown } from 'lucide-react';
import { KnowledgeWorkspace } from '../../products/knowledge/knowledge-previews';
import { useBentoPlayback } from '../useBentoPlayback';
import { AskAgentWindow } from './AskAgentWindow';
import './ask-agent-workspace.css';

/** The existing Knowledge workspace beneath a reusable, interactive agent window. */
export function AskAgentWorkspace() {
  const [open, setOpen] = useState(true);
  const [paused, setPaused] = useState(false);
  const [replay, setReplay] = useState(0);
  const { container, playing, cycle } = useBentoPlayback(24000, open && !paused);
  const [frame, setFrame] = useState(5);
  const active = playing && open && !paused;
  const phase = active ? frame : 5;

  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timers = [1800, 4600, 7600, 10400, 13500].map((delay, index) => setTimeout(() => setFrame(index + 1), delay));
    return () => timers.forEach(clearTimeout);
  }, [active, cycle, replay]);

  return <div className="aaw-scene" ref={container} data-playing={active} data-phase={phase} data-open={open}>
    <div className="aaw-background" aria-hidden="true" inert><KnowledgeWorkspace autoplay={false} apiHref="/new/products/knowledge#knowledge-api" /></div>
    {open && <AskAgentWindow phase={phase} paused={paused} onInteract={() => setPaused(true)} onTogglePlayback={() => setPaused(value => !value)} onMinimize={() => { setPaused(true); setOpen(false); }} onRestart={() => { setPaused(false); setReplay(value => value + 1); }} />}
    <button className="aaw-launcher" type="button" aria-label={open ? 'Minimize Ask Agent preview' : 'Open Ask Agent preview'} aria-expanded={open} onClick={() => { setPaused(true); setOpen(value => !value); }}><img src="/brand/helpin-icon-ink.svg" width={24} height={24} alt="" /><span>Ask Agent</span><ChevronDown size={13} /></button>
  </div>;
}
