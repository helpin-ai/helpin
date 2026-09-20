import type { Message, WidgetConfig } from '@helpin-ai/widget-core';

export const DEMO_DURATION = 22000;
export const DEMO_CONFIG: WidgetConfig = {
  workspaceId: 'website-orbitdesk-demo',
  workspaceName: 'OrbitDesk',
  visitorName: 'Maya Chen',
  branding: {
    primaryColor: '#0f7a50',
    welcomeMessage: 'Hi Maya. How can we help?',
    widgetPosition: 'bottom-right',
    showBranding: true,
    colorScheme: 'light',
  },
  features: {
    aiEnabled: true, aiFirst: true, showTalkToHuman: false,
    fileUploads: false, preChatForm: false, requirePhone: false,
    csatRating: false, forceIdentify: false,
  },
  availability: { isOnline: true, statusText: 'Online', replyTimeText: '' },
};

const SCRIPT: Omit<Message, 'conversationId' | 'isInternal' | 'createdAt'>[] = [
  { id: 'question', role: 'customer', content: 'Can we pilot Okta with just our admins?' },
  { id: 'answer', role: 'ai', senderAvatar: '/brand/helpin-icon-ink.svg', content: 'Yes. Start with an admin-only group, following the Okta setup guide.', aiReplyKind: 'answer', sources: [{ docId: 'okta-guide', title: 'Okta setup guide', snippet: 'Start your pilot with an admin-only group.', confidence: 1, language: 'en' }] },
  { id: 'followup', role: 'customer', content: 'Thanks! The group mapping gives our admins the wrong role. Can you check?' },
  { id: 'handoff', role: 'ai', senderAvatar: '/brand/helpin-icon-ink.svg', content: 'I’m passing that to Sam with your pilot details, so you won’t need to explain it again.' },
  { id: 'joined', role: 'system', content: 'Sam Rivera joined the conversation', systemEventType: 'teammate_joined', senderName: 'Sam Rivera', senderAvatar: '/new/avatars/sam.webp' },
  { id: 'teammate', role: 'agent', content: 'Hi Maya. I’ve linked this to our SSO project. I’ll follow up here after engineering checks the role mapping.', senderName: 'Sam Rivera', senderAvatar: '/new/avatars/sam.webp' },
];

export const DEMO_MESSAGES: Message[] = SCRIPT.map(message => ({
  ...message, conversationId: 'website-demo', isInternal: false, createdAt: '2026-09-20T09:14:00Z',
}));

// Deliberately local: no SDK client, widget key, session, or network adapter.
export function demoFrame(elapsed: number, createdAt: string) {
  const messages: Message[] = [];
  const starts = [0, 1800, 6100, 8500, 12400, 14200];
  const ends = [0, 3400, 6100, 10600, 12400, 16800];
  for (let index = 0; index < DEMO_MESSAGES.length; index += 1) {
    if (elapsed < starts[index]) continue;
    const message = DEMO_MESSAGES[index];
    const streaming = elapsed < ends[index];
    const progress = streaming ? Math.max(1, Math.floor(message.content.length * (elapsed - starts[index]) / (ends[index] - starts[index]))) : message.content.length;
    messages.push({ ...message, createdAt, content: message.content.slice(0, progress), isStreaming: streaming, sources: streaming ? undefined : message.sources });
  }
  return {
    messages,
    isAIThinking: (elapsed >= 500 && elapsed < 1800) || (elapsed >= 7000 && elapsed < 8500),
    isTyping: elapsed >= 13000 && elapsed < 14200,
    stage: elapsed < 6100 ? 'An answer from your knowledge' : elapsed < 12400 ? 'A handoff with the context' : 'Your team carries the work forward',
    handedOff: elapsed >= 12400,
  };
}
