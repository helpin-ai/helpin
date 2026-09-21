'use client';

import { useEffect, useState } from 'react';
import { Check, FileSearch, LockKeyhole, Pause, Play, ShieldCheck, Sparkles } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-identity.css';

export function SupportIdentity() {
  const { container, playing } = useBentoPlayback(17000);
  const [paused, setPaused] = useState(false);
  const [frame, setFrame] = useState(4);
  const active = playing && !paused;

  useEffect(() => {
    if (!active) return;
    let timers: ReturnType<typeof setTimeout>[] = [];
    const start = () => {
      timers.forEach(clearTimeout);
      setFrame(0);
      timers = [1900, 4600, 7600, 10800].map((time, index) =>
        setTimeout(() => setFrame(index + 1), time),
      );
    };
    start();
    const interval = setInterval(start, 17000);
    return () => { timers.forEach(clearTimeout); clearInterval(interval); };
  }, [active]);

  const phase = active ? frame : 4;
  const verified = phase >= 1;
  const found = phase >= 2;
  const drafted = phase >= 3;
  const readyForReview = phase === 4;

  return (
    <div className="support-identity-art" ref={container} data-playing={active} data-phase={phase}>
      <div className="sia-toolbar"><span><i>O</i>OrbitDesk <span>· Illustrative workflow</span></span><button type="button" aria-label={paused ? 'Play identity and tools animation' : 'Pause identity and tools animation'} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
      <div role="img" aria-label="Example configured workflow: OrbitDesk signs Maya’s widget identity on its server and Helpin verifies the HMAC signature. The team asks Ask Agent to investigate her incomplete CSV export with a selected MCP log tool. The tool applies its own account access rules and returns a row-limit error. Ask Agent attaches the findings to the customer request and prepares a reply for review; it has not been sent.">
        <div aria-hidden="true">
          <div className="sia-customer">
            <div className="sia-customer-top"><img src="/new/avatars/maya.webp" width={34} height={34} alt="" /><div><strong>Maya Chen</strong><span>Northstar Labs</span></div><span className="sia-verified" data-ready={verified}><ShieldCheck size={13} />{verified ? 'Identity verified' : 'Checking'}</span></div>
            <p>“Can you check why our export is incomplete?”</p>
            <div className="sia-proof"><LockKeyhole size={12} /><span>Server-signed identity</span><span>HMAC</span></div>
          </div>
          <div className="sia-wire" data-active={phase === 1}><i /></div>
          <div className="sia-investigation" data-active={verified}>
            <div className="sia-agent-heading"><img src="/brand/helpin-icon-white.svg" width={21} height={21} alt="" /><div><strong>Ask Agent</strong><span>Investigate the reported export</span></div><span className="sia-status">{found ? 'Finding ready' : verified ? 'Checking logs' : 'Ready'}</span></div>
            <div className="sia-tool"><FileSearch size={17} /><div><strong>Account export logs</strong><span>Your MCP server · Selected tool</span></div>{found ? <Check size={15} /> : <i className="sia-working" />}</div>
            <div className="sia-result" data-ready={found}><span>EXPORT LOG · NORTHSTAR LABS</span><p>The export stopped before retrieving the remaining contacts.</p><small>Customer access checked by your tool</small></div>
          </div>
          <div className="sia-wire" data-active={phase === 3}><i /></div>
          <div className="sia-response" data-ready={drafted}>
            <div className="sia-response-heading"><span><Sparkles size={13} />Customer reply</span><span>{readyForReview ? <Check size={13} /> : null}{readyForReview ? 'Ready for review' : 'Draft'}</span></div>
            <p>Findings attached to the customer’s request</p>
            <div className="sia-review"><img src="/new/avatars/sam.webp" width={21} height={21} alt="" /><span>{readyForReview ? 'Customer reply ready for review' : 'Preparing a reply for Sam’s review'}</span></div>
          </div>
        </div>
      </div>
      <p className="sia-caption">Verified identity. Limited access. An informed next step.</p>
    </div>
  );
}
