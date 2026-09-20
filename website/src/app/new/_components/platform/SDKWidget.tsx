'use client';

import { useEffect, useRef, useState } from 'react';
import type { Message, MountWidgetOptions, WidgetConfig } from '@helpin-ai/widget-core';
import { ArrowUpRight, MessagesSquare } from 'lucide-react';
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
  const launcher = useRef<HTMLButtonElement>(null);
  const focusComposer = useRef(false);
  const [ready, setReady] = useState(false);
  const [isOpen, setOpen] = useState(true);
  const [messages, setMessages] = useState<Message[]>([]);
  function send(content: string) {
    setMessages(previous => [...previous, {
      id: `sdk-${Date.now()}-${previous.length}`, conversationId: 'website-sdk-example',
      role: 'customer', content, createdAt: new Date().toISOString(), isInternal: false,
    }]);
  }
  const options: MountWidgetOptions = {
    config: CONFIG, isOpen, messages, showLauncher: false, initialView: 'conversation',
    connectionStatus: 'connected', contactCaptureCompleted: true,
    activeConversation: { id: 'website-sdk-example', subject: 'Rollout support', status: 'open', flowState: 'ai_handling' },
    onSendMessage: send,
    onClose: () => { setOpen(false); launcher.current?.focus(); },
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
    // Preact commits separately from React. Move focus only after an explicit
    // launch, never when this below-the-fold example hydrates.
    if (!focusComposer.current || !isOpen) return;
    const frame = requestAnimationFrame(() => {
      target.current?.querySelector<HTMLTextAreaElement>('textarea')?.focus({ preventScroll: true });
      focusComposer.current = false;
    });
    return () => cancelAnimationFrame(frame);
  });

  function runExample() {
    // Match SDK openNewMessage(content): a new conversation with the first
    // message sent. This isolated renderer has no transport or live inbox key.
    setMessages([{ id: 'sdk-opening', conversationId: 'website-sdk-example', role: 'customer', content: OPENING_MESSAGE, createdAt: new Date().toISOString(), isInternal: false }]);
    focusComposer.current = true;
    setOpen(true);
  }

  return <div className="sdk-widget-example">
    <div className="sdk-widget-launch"><div><b className="platform-orbit-mark">O</b><strong>OrbitDesk</strong></div><button type="button" ref={launcher} onClick={runExample} disabled={!ready} aria-controls="sdk-widget-window"><MessagesSquare size={13} />Run example<ArrowUpRight size={12} /></button></div>
    <div className="sdk-widget-viewport" id="sdk-widget-window" data-ready={ready} data-open={isOpen}>
      {!ready && <div className="sdk-widget-fallback"><img src="/brand/helpin-icon-ink.svg" width={28} height={28} alt="" /><strong>Talk to OrbitDesk</strong><p>Open a conversation from your application, then keep talking in the Helpin widget.</p><noscript>Enable JavaScript to try the widget.</noscript></div>}
      {ready && !isOpen && <div className="sdk-widget-closed"><MessagesSquare size={28} /><strong>A conversation is one click away.</strong><p>Run the example to open the widget.</p></div>}
      <div className="sdk-widget-mount" ref={target} />
    </div>
    <p className="sdk-widget-note">Try the composer. Messages stay on this page.</p>
  </div>;
}
