'use client';

import { useEffect, useRef, useState } from 'react';
import type { MountWidgetOptions } from '@helpin-ai/widget-core';
import { Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import { DEMO_CONFIG, DEMO_DURATION, DEMO_MESSAGES, demoFrame } from './support-widget-script';
import '@helpin-ai/widget-core/src/styles/widget.css';
import './support-widget-demo.css';
import { revealWidgetPreviewText } from './support-widget-stream';

type WidgetRuntime = typeof import('@helpin-ai/widget-core');

export function SupportHeroScene() {
  const { container, playing } = useBentoPlayback(DEMO_DURATION);
  const mount = useRef<HTMLDivElement>(null);
  const runtime = useRef<WidgetRuntime | null>(null);
  const [ready, setReady] = useState(false);
  const [paused, setPaused] = useState(false);
  const [elapsed, setElapsed] = useState(DEMO_DURATION);
  const timestamp = useRef('2026-09-20T09:14:00Z');
  const active = playing && !paused;
  const frame = demoFrame(active ? elapsed : DEMO_DURATION, timestamp.current);
  const options: MountWidgetOptions = {
    config: DEMO_CONFIG,
    isOpen: true,
    showLauncher: false,
    initialView: 'conversation',
    connectionStatus: 'connected',
    messages: frame.messages,
    isAIThinking: frame.isAIThinking,
    aiProgressLabel: frame.aiProgressLabel,
    isTyping: frame.isTyping,
    typingAgentName: 'Sam',
    typingAgentAvatar: '/new/avatars/sam.webp',
    activeTeammate: frame.handedOff ? { userId: 'sam-demo', name: 'Sam Rivera', avatarUrl: '/new/avatars/sam.webp', status: 'online' } : undefined,
    activeConversation: { id: 'website-demo', subject: 'CSV export stops early', status: 'open', flowState: frame.handedOff ? 'assigned_to_human' : 'ai_handling' },
    contactCaptureCompleted: true,
  };
  const currentOptions = useRef(options);
  currentOptions.current = options;

  useEffect(() => {
    const target = mount.current;
    if (!target) return;
    let cancelled = false;
    timestamp.current = new Date().toISOString();
    // The package bundles its own Preact renderer. Mount it imperatively rather
    // than rendering Preact components through React's renderer.
    import('@helpin-ai/widget-core').then(module => {
      if (cancelled) return;
      runtime.current = module;
      module.mountWidget(target, currentOptions.current);
      setReady(true);
    }).catch(() => {
      // The complete server-rendered transcript remains visible if loading fails.
    });
    return () => {
      cancelled = true;
      runtime.current?.unmountWidget(target);
      runtime.current = null;
    };
  }, []);

  useEffect(() => {
    const target = mount.current;
    if (!ready || !target) return;
    // The shared list follows new messages. Also keep the last reply in view
    // when this inline container reflows (especially in the static state).
    let frameId = 0;
    const observer = new ResizeObserver(() => {
      cancelAnimationFrame(frameId);
      frameId = requestAnimationFrame(() => {
        const list = target.querySelector<HTMLElement>('.helpin-message-list');
        if (list) list.scrollTop = list.scrollHeight;
      });
    });
    observer.observe(target);
    return () => {
      observer.disconnect();
      cancelAnimationFrame(frameId);
    };
  }, [ready]);

  useEffect(() => {
    if (!ready || !active) return;
    setElapsed(0);
    const started = performance.now();
    let frameId = 0;
    let previousFrame = '';
    const advance = (now: number) => {
      const elapsed = (now - started) % DEMO_DURATION;
      const next = demoFrame(elapsed, timestamp.current);
      const last = next.messages.at(-1);
      const signature = `${last?.id}:${last?.content.length}:${last?.isStreaming}:${next.isAIThinking}:${next.isTyping}:${next.stage}`;
      // Update only at story boundaries. CSS streams the text between them.
      if (signature !== previousFrame) {
        previousFrame = signature;
        setElapsed(elapsed);
      }
      frameId = requestAnimationFrame(advance);
    };
    frameId = requestAnimationFrame(advance);
    return () => cancelAnimationFrame(frameId);
  }, [ready, active]);

  useEffect(() => {
    if (ready && mount.current) {
      runtime.current?.mountWidget(mount.current, options);
      revealWidgetPreviewText(mount.current, active ? frame.stream : null);
    }
  });

  return <div className="support-hero-art support-widget-art" ref={container} data-playing={active} data-stage={frame.stage}>
    <div className="support-hero-art-label"><span>FROM THE CUSTOMER’S SIDE</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} support widget animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={13} aria-hidden="true" /> : <Pause size={13} aria-hidden="true" />}</button></div>
    <div className="support-widget-shell" role="img" aria-label="Illustrative OrbitDesk chat: Maya asks how to export selected contacts. Helpin AI answers from the export guide. Maya reports that the export stops at 10,000 rows. Helpin AI searches connected export logs, finds the pagination error, and hands Sam the findings. Sam links the report to EXP-142. Once the fix is reviewed, tested and released, Helpin AI sends Maya the approved follow-up.">
      <div className="support-widget-workspace" aria-hidden="true"><span className="support-workspace-logo">O</span><strong>OrbitDesk</strong><span>Customer support</span></div>
      <div className="support-widget-viewport" data-ready={ready}>
        <div className="support-widget-fallback" aria-hidden="true"><div className="support-widget-fallback-header"><img src="/brand/helpin-icon-ink.svg" width={24} height={24} alt="" /><strong>Helpin AI <small>OrbitDesk support</small></strong></div><div className="support-widget-transcript">{DEMO_MESSAGES.map(message => <div key={message.id} className={`support-widget-fallback-message support-widget-fallback-${message.role}`}><span>{message.role === 'customer' ? 'Maya Chen' : message.senderName || 'Helpin AI'}</span><p>{message.content}</p>{message.sources && <small>Source: {message.sources.map(source => source.title).join(', ')}</small>}</div>)}</div></div>
        <div className="support-widget-mount" ref={mount} inert aria-hidden="true" />
      </div>
    </div>
    <p className="support-hero-art-caption"><span className="support-live-dot" /><span className="support-widget-stage" aria-hidden="true">{frame.stage}</span><span>Illustrative conversation</span></p>
  </div>;
}
