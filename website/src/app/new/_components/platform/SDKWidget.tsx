'use client';

import { useEffect, useRef, useState } from 'react';
import type { Message, MountWidgetOptions, WidgetConfig } from '@helpin-ai/widget-core';
import '@helpin-ai/widget-core/src/styles/widget.css';
import './sdk-widget.css';

type WidgetRuntime = typeof import('@helpin-ai/widget-core');
const OPENING_MESSAGE = 'Can you help with our rollout?';
const CONFIG: WidgetConfig = {
  workspaceId: 'website-sdk-example',
  workspaceName: 'OrbitDesk',
  visitorName: 'Maya Chen',
  branding: { logoUrl: '/brand/helpin-icon-white.svg', primaryColor: '#0f7a50', welcomeMessage: 'Hi Maya. How can we help?', widgetPosition: 'bottom-right', showBranding: true, colorScheme: 'light' },
  features: { aiEnabled: true, aiFirst: true, showTalkToHuman: false, fileUploads: false, preChatForm: false, requirePhone: false, csatRating: false, forceIdentify: false },
  availability: { isOnline: true, statusText: 'Online', replyTimeText: '' },
};

export function SDKWidget() {
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
    config: CONFIG, isOpen: true, messages, showLauncher: false, initialView: 'conversation',
    connectionStatus: 'connected', contactCaptureCompleted: true,
    activeConversation: { id: 'website-sdk-example', subject: 'Rollout support', status: 'open', flowState: 'ai_handling' },
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
  });

  return <div className="sdk-widget-example">
    <div className="sdk-widget-viewport" id="sdk-widget-window" data-ready={ready}>
      {!ready && <div className="sdk-widget-fallback"><img src="/brand/helpin-icon-ink.svg" width={28} height={28} alt="" /><strong>Talk to your team</strong><p>Open a conversation from your application, then keep talking in the Helpin widget.</p><noscript>Enable JavaScript to try the widget.</noscript></div>}
      <div className="sdk-widget-mount" ref={target} />
    </div>
    <p className="sdk-widget-note">Try the composer. Messages stay on this page.</p>
  </div>;
}
