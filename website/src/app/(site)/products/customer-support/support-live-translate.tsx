'use client';

import { useState } from 'react';
import { Check, Languages, Pause, Play, Send } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { StreamingText } from '../../_components/StreamingText';
import { WorkflowClick } from '../../_components/WorkflowParts';
import './support-live-translate.css';

// Mirrors LiveTranslateBar, TranslatedMessageBubble and ReplyComposer:
// new messages translate for the reader; outgoing replies translate after Send.
const MESSAGE = { original: '¿Cómo puedo descargar mis facturas?', translated: 'How can I download my invoices?' };
const REPLY = { original: 'Open Billing, then choose Download invoice.', translated: 'Abre Facturación y selecciona Descargar factura.' };
const BEATS = [0, 1200, 2700, 5000, 7300, 8900, 10500];

function Conversation({ phase, playing, onInspect }: { phase: number; playing: boolean; onInspect: () => void }) {
  const [originalMessage, setOriginalMessage] = useState(false);
  const [originalReply, setOriginalReply] = useState(false);
  const incomingTranslated = phase >= 2;
  const sent = phase >= 6;
  return <div className="ops-scene lt-scene">
    <div className="lt-customer"><span>L</span><strong>Lucía Martín</strong><small>Chat</small></div>
    <div className="lt-language-bar"><Languages size={13} aria-hidden="true" /><strong>Live translate</strong><span>Spanish → English</span></div>
    <div className="lt-messages">
      <div className="lt-message lt-incoming" data-translating={phase === 1}>
        <p lang={!incomingTranslated || originalMessage ? 'es' : 'en'}>{incomingTranslated && !originalMessage ? MESSAGE.translated : MESSAGE.original}</p>
        {phase === 1 ? <small className="lt-working"><i />Translating to English…</small> : incomingTranslated ? <div className="lt-message-footer"><span>{originalMessage ? 'Original · Spanish' : 'Translated from Spanish'}</span><button type="button" aria-label={originalMessage ? 'Show translated customer message' : 'Show original customer message'} onClick={() => { onInspect(); setOriginalMessage(value => !value); }}>{originalMessage ? 'Show translation' : 'Show original'}</button></div> : <small>Just now</small>}
      </div>
      {phase >= 5 && <div className="lt-message lt-outgoing" data-translating={!sent}>
        <p lang={sent && !originalReply ? 'es' : 'en'}>{sent && !originalReply ? REPLY.translated : REPLY.original}</p>
        {sent ? <>
          <div className="lt-message-footer"><span>{originalReply ? 'Original · English' : 'Translated from English'}</span><button type="button" aria-label={originalReply ? 'Show translated reply' : 'Show original reply'} onClick={() => { onInspect(); setOriginalReply(value => !value); }}>{originalReply ? 'Show translation' : 'Show original'}</button></div>
          <small className="lt-sent"><Check size={10} aria-hidden="true" />Sent in Spanish</small>
        </> : <small className="lt-working"><i />Translating to Spanish…</small>}
      </div>}
    </div>
    <div className="lt-composer" aria-label="Reply composer" data-writing={phase === 3 || phase === 4}>
      <div className="lt-input">{phase === 3 || phase === 4 ? <StreamingText text={REPLY.original} active={playing} duration={1400} /> : <span>Reply in English…</span>}</div>
      <div className="lt-composer-tools"><span><Languages size={11} />Reply in Spanish</span><span className="lt-send" aria-hidden="true" data-ready={phase === 3 || phase === 4}>Send<Send size={11} />{phase === 4 && playing && <WorkflowClick />}</span></div>
    </div>
  </div>;
}

export function SupportLiveTranslate() {
  const [paused, setPaused] = useState(false);
  const { container, playing, phase, cycle, reducedMotion } = useWorkflowPlayback({ paused, beats: BEATS, duration: 16000 });
  return <div ref={container} className="ops-art lt-art" data-playing={playing} data-phase={phase} role="region" aria-label="Live translation conversation demo">
    <div className="ops-art-toolbar"><span><span className="ops-workspace-mark">O</span>OrbitDesk</span>{!reducedMotion && <button type="button" aria-label={`${paused ? 'Play' : 'Pause'} live translate animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button>}</div>
    <Conversation key={cycle} phase={phase} playing={playing} onInspect={() => setPaused(true)} />
  </div>;
}
