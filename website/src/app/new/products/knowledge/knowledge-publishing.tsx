'use client';

import { useState } from 'react';
import { ArrowRight, Check, Globe, LockKeyhole, Pause, Play, Server } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './knowledge-publishing.css';

// Local product examples. No requests are sent to a workspace or an API.
const HOSTING = [
  {label:'Helpin subdomain',url:'orbitdesk.helpin.center',detail:'Publish on a hosted Helpin address.',entry:'Helpin subdomain',path:'Direct request'},
  {label:'Custom domain',url:'docs.orbitdesk.example',detail:'Use a dedicated docs domain with your brand.',entry:'Your docs domain',path:'Custom domain'},
  {label:'Reverse proxy',url:'orbitdesk.example/docs',detail:'Keep documentation under a path on your main website.',entry:'Your website /docs',path:'Public URL preserved'},
];
export function PublishingDemo(){
  const [mode,setMode]=useState(2);
  const [paused,setPaused]=useState(false);
  const {container,playing,cycle}=useBentoPlayback(10000);
  const option=HOSTING[mode];
  return <div className="kp-demo" ref={container} data-playing={playing&&!paused}><div className="kp-controls"><div role="group" aria-label="Example publishing URL"><span className="kp-label">PUBLISH YOUR WAY</span><div>{HOSTING.map((item,index)=><button type="button" key={item.label} aria-pressed={index===mode} aria-controls="knowledge-delivery-route" onClick={()=>setMode(index)}>{item.label}</button>)}</div></div><button className="kp-pause" type="button" aria-label={`${paused?'Play':'Pause'} publishing animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={13}/>:<Pause size={13}/>}</button></div>
    <div className="kp-route" id="knowledge-delivery-route" key={`${mode}-${cycle}`}>
      <div className="kp-address"><span><Globe size={16}/>{option.entry}</span><strong><LockKeyhole size={14}/>{option.url}</strong><p>{option.detail}</p></div>
      <div className="kp-connection" aria-hidden="true"><span/><i/><ArrowRight size={18}/></div>
      <div className="kp-delivery"><span className="kp-node-icon"><Server size={22}/></span><span className="kp-label">HELPIN DELIVERY</span><h3>Ready-to-read HTML</h3><p>Server rendered.<br/>{' '}Cached and compressed.</p><span className="kp-delivery-pill"><Check size={12}/>{option.path}</span></div>
      <div className="kp-connection kp-connection-second" aria-hidden="true"><span/><i/><ArrowRight size={18}/></div>
      <div className="kp-output"><div><b className="ks-mark">O</b><strong>OrbitDesk Docs</strong><span><Check size={11}/>HTML</span></div><span className="kp-output-eyebrow">GETTING STARTED</span><h4>Connect your first integration</h4><p>Choose your service, authorize access, and confirm your workspace.</p><div className="kp-output-lines" aria-hidden="true"><span/><span/><span/></div><span className="kp-output-ready"><Check size={12}/>Article content in the initial response</span></div>
    </div><div className="kp-canonical"><span>CANONICAL URL</span><code>https://{option.url}/articles/connect-your-first-integration</code><Check size={13} aria-hidden="true"/></div>
  </div>;
}
