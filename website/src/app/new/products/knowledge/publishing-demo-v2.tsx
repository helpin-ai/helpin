'use client';

import { useEffect, useId, useState } from 'react';
import { ArrowRight, Check, FileCode2, Globe, LockKeyhole, Network, Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './publishing-demo-v2.css';

const DELIVERY_STATES = ['Requesting your article', 'Rendering on Helpin', 'Delivering the HTML', 'Your article is ready'];
const ROUTE = 'M 0 154 H 22 Q 40 154 40 136 V 83 Q 40 65 58 65 H 80';

const MODES = [
  { label: 'Helpin subdomain', host: 'orbitdesk.helpin.center', path: '', routing: 'Hosted address', detail: 'A home for your docs, ready to publish.' },
  { label: 'Custom domain', host: 'docs.orbitdesk.example', path: '', routing: 'Your custom domain', detail: 'Your brand, right down to the address.' },
  { label: 'Reverse proxy', host: 'orbitdesk.example', path: '/docs', routing: 'Your reverse proxy', detail: 'Your docs, on the website your customers already know.' },
];

export function PublishingDemoV2() {
  const [mode,setMode] = useState(2);
  const [paused,setPaused] = useState(false);
  const {container,playing,cycle} = useBentoPlayback(14000);
  const storyId = useId();
  const [frame,setFrame] = useState(3);
  const [selection,setSelection] = useState(0);
  const current = MODES[mode];
  const active = playing && !paused;
  const phase = active ? frame : 3;
  // Keep the article and controls mounted: only the delivery state changes.
  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timers = [1400, 3300, 5300].map((delay,index) => setTimeout(() => setFrame(index + 1), delay));
    return () => timers.forEach(clearTimeout);
  }, [active, cycle, selection]);
  return <div className="kp2-demo" ref={container} data-playing={active} data-phase={phase}>
    <div className="kp2-toolbar"><div className="kp2-modes" role="group" aria-label="Example publishing URL">{MODES.map((item,index)=><button key={item.label} type="button" aria-pressed={index===mode} aria-controls={storyId} onClick={()=>{setMode(index);setSelection(value=>value+1);}}>{item.label}</button>)}</div><button className="kp2-pause" type="button" aria-label={`${paused?'Play':'Pause'} publishing animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={13}/>:<Pause size={13}/>}</button></div>
    <div className="kp2-story" id={storyId}>
      <div className="kp2-address"><span>YOUR PUBLIC ADDRESS</span><div><LockKeyhole size={17}/><strong><span>{current.host}</span><em>{current.path}</em></strong></div><p>{current.detail}</p></div>
      <div className="kp2-stage">
        <div className="kp2-routing" role="img" aria-label={`A request to ${current.host}${current.path} reaches Helpin through ${current.routing.toLowerCase()}. Helpin delivers server-rendered HTML with caching and compression. The article is available in the initial response.`}>
          <div className="kp2-routing-content" aria-hidden="true">
            <div className="kp2-node kp2-node-request" data-current={phase===0} data-complete={phase>=1}><span className="kp2-node-icon"><Network size={18}/></span><div><span>01 · REQUEST</span><strong>{current.routing}</strong><small>{current.host}{current.path}</small></div><i><Check size={12}/></i></div>
            <div className="kp2-vertical"><span/><i/></div>
            <div className="kp2-node kp2-node-helpin" data-current={phase===1} data-complete={phase>=2}><span className="kp2-helpin-icon"><img src="/brand/helpin-icon-white.svg" width={23} height={23} alt=""/></span><div><span>02 · DELIVERY</span><strong>Rendered by Helpin</strong><small>orbitdesk.helpin.center</small></div><i><Check size={12}/></i></div>
            <div className="kp2-services"><span data-ready={phase>=1}>Server rendering</span><span data-ready={phase>=2}>HTML cache</span><span data-ready={phase>=2}>Compression</span></div>
            <div className="kp2-response" data-ready={phase===3}><span><FileCode2 size={16}/></span><div><strong>Ready to read</strong><small>Article HTML in the first response</small></div><Check size={14}/></div>
          </div>
        </div>
        <div className="kp2-bridge" aria-hidden="true"><svg viewBox="0 0 80 320" preserveAspectRatio="none"><path className="kp2-track" d={ROUTE}/><path className="kp2-progress" pathLength="100" d={ROUTE}/>{active && phase===2 && <path className="kp2-packet" pathLength="100" d={ROUTE}/>}</svg><ArrowRight size={15}/><span className="kp2-mobile-packet"/></div>
        <div className="kp2-browser" data-ready={phase===3}><div className="kp2-browser-bar"><span className="kp2-browser-dots" aria-hidden="true"><i/><i/><i/></span><span><LockKeyhole size={10}/>{current.host}<b>{current.path}</b></span><Globe size={12} aria-hidden="true"/></div><div className="kp2-load-track" aria-hidden="true"><span/></div><div className="kp2-document"><div className="kp2-doc-header"><span><b>O</b>OrbitDesk<span>Docs</span></span><span className="kp2-rendered" data-ready={phase===3}><Check size={11}/>Server rendered</span></div><div className="kp2-doc-body"><aside aria-hidden="true"><span>GETTING STARTED</span><i>Overview</i><strong>Connect an integration</strong><i>Invite your team</i><i>Workspace settings</i><span>BUILD WITH ORBITDESK</span><i>API reference</i></aside><div className="kp2-article"><span className="kp2-article-eyebrow">Getting started</span><h3>Connect your first integration</h3><p>Bring your tools into OrbitDesk and keep the context together.</p><div className="kp2-article-steps"><div><span>1</span><strong>Choose your integration</strong></div><div><span>2</span><strong>Review access</strong></div><div><span>3</span><strong>Confirm your workspace</strong></div></div><div className="kp2-article-callout"><Check size={12}/>Ready for your first sync.</div></div></div></div><div className="kp2-browser-footer"><span className="kp2-delivery-status" data-ready={phase===3}><i>{phase===3 ? <Check size={12}/> : <span/>}</i>{DELIVERY_STATES[phase]}</span><span>Your URL stays yours.</span></div></div>
      </div><div className="kp2-public-path" data-ready={phase===3}><span><Globe size={13}/>PUBLIC LINKS</span><p>Articles, canonical URLs, and sitemaps use <strong>{current.host}{current.path}</strong></p><Check size={13}/></div>
    </div>
  </div>;
}
