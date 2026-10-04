'use client';

import { useEffect, useRef, useState } from 'react';
import type { Message, MountWidgetOptions, WidgetConfig } from '@helpin-ai/widget-core';
import '@helpin-ai/widget-core/src/styles/widget.css';
import './sdk-widget.css';
import { useWorkflowPlayback } from '../useWorkflowPlayback';
import { revealWidgetPreviewText } from '../../products/customer-support/support-widget-stream';

type WidgetRuntime = typeof import('@helpin-ai/widget-core');
const OPENING_MESSAGE = 'My export is missing contacts. Can you help?';
const CONFIG: WidgetConfig = {
  workspaceId: 'website-sdk-example',
  workspaceName: 'OrbitDesk',
  visitorName: 'Maya Chen',
  branding: { logoUrl: '/brand/helpin-icon-white.svg', primaryColor: '#0f7a50', welcomeMessage: 'Hi Maya. How can we help?', widgetPosition: 'bottom-right', showBranding: true, colorScheme: 'light' },
  features: { aiEnabled: true, aiFirst: true, showTalkToHuman: false, fileUploads: false, preChatForm: false, requirePhone: false, csatRating: false, forceIdentify: false },
  availability: { isOnline: true, statusText: 'Online', replyTimeText: '' },
};

export function SDKWidget() {
  const { container, phase, playing, cycle } = useWorkflowPlayback({beats:[0,1200,3000,4900,8000],duration:13000});
  const target = useRef<HTMLDivElement>(null);
  const runtime = useRef<WidgetRuntime | null>(null);
  const [ready, setReady] = useState(false);
  const [messages, setMessages] = useState<Message[]>(() => [{
    id: 'sdk-opening', conversationId: 'website-sdk-example', role: 'customer',
    content: OPENING_MESSAGE, createdAt: new Date().toISOString(), isInternal: false,
  }]);
  function send(content: string) {
    setMessages(previous => [...previous, {
      id: `sdk-${Date.now()}-${previous.length}`, conversationId: 'website-sdk-example',
      role: 'customer', content, createdAt: new Date().toISOString(), isInternal: false,
    }]);
  }
  const options: MountWidgetOptions = {
    config: CONFIG, isOpen: true, messages: phase >= 3 ? [messages[0], {id: 'sdk-answer',conversationId: 'website-sdk-example',role: 'ai',content: 'I checked the export guide and your logs. The export stopped after the first page. I can bring your team in with the missing row count and these findings.',createdAt: messages[0].createdAt,isInternal: false,senderName: 'Echo',isStreaming: playing && phase === 3}, ...messages.slice(1)] : messages, showLauncher: false, initialView: 'conversation',
    isAIThinking: phase === 1 || phase === 2,
    aiProgressLabel: phase === 1 ? 'Reading the export guide' : 'Checking the connected export logs',
    connectionStatus: 'connected', contactCaptureCompleted: true,
    activeConversation: { id: 'website-sdk-example', subject: 'Export support', status: 'open', flowState: 'ai_handling' },
    onSendMessage: send,
  };
  const latestOptions = useRef(options);
  latestOptions.current = options;

  useEffect(() => {
    const container = target.current;
    if (!container) return;
    let cancelled = false;
    import('@helpin-ai/widget-core').then(module => {
      if (cancelled) return;
      runtime.current = module;
      module.mountWidget(container, latestOptions.current);
      setReady(true);
    }).catch(() => { /* Keep the server-rendered fallback if loading fails. */ });
    return () => {
      cancelled = true;
      runtime.current?.unmountWidget(container);
      runtime.current = null;
    };
  }, []);

  useEffect(() => {
    if (!ready || !target.current) return;
    runtime.current?.mountWidget(target.current, options);
    revealWidgetPreviewText(target.current, playing && phase === 3 ? {id:`sdk-answer-${cycle}`,duration:2200,elapsed:0} : null);
  });

  useEffect(() => {
    if (!ready || !target.current) return;
    const container = target.current;
    const updatePlaceholder = () => {
      const composer = container.querySelector<HTMLTextAreaElement>('textarea.helpin-compose-input');
      if (composer) composer.placeholder = 'Tell us what you expected to see…';
    };
    updatePlaceholder();
    const observer = new MutationObserver(updatePlaceholder);
    observer.observe(container, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, [ready]);

  return <div className="sdk-widget-example" ref={container} data-playing={playing} data-phase={phase}>
    <button type="button" className="sdk-demo-launcher" onClick={() => target.current?.querySelector<HTMLTextAreaElement>('textarea')?.focus({ preventScroll: true })}>Get help with this export</button>
    <div className="sdk-widget-viewport" id="sdk-widget-window" data-ready={ready}>
      {!ready && <div className="sdk-widget-fallback"><img src="/brand/helpin-icon-ink.svg" width={28} height={28} alt="" /><strong>Talk to your team</strong><p>Open a conversation from your application, then keep talking in the Helpin widget.</p><noscript>Enable JavaScript to try the widget.</noscript></div>}
      <div className="sdk-widget-mount" ref={target} />
    </div>
  </div>;
}
