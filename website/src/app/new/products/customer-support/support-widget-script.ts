import type { Message, WidgetConfig } from '@helpin-ai/widget-core';

export const DEMO_DURATION = 32000;
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
  { id: 'question', role: 'customer', content: 'Our export is still incomplete. Any progress since yesterday?' },
  { id: 'answer', role: 'ai', senderAvatar: '/brand/helpin-icon-ink.svg', content: 'I found your earlier report: 10,000 contacts exported instead of 18,400. Let me check the latest logs.', aiReplyKind: 'answer' },
  { id: 'followup', role: 'customer', content: 'The smaller report worked, but we still need the full list.' },
  { id: 'handoff', role: 'ai', senderAvatar: '/brand/helpin-icon-ink.svg', content: 'The full export is still stopping at 10,000 rows. Your filters look right. I’m passing your report and the findings to Sam.' },
  { id: 'joined', role: 'system', content: 'Sam Rivera joined the conversation', systemEventType: 'teammate_joined', senderName: 'Sam Rivera', senderAvatar: '/new/avatars/sam.webp' },
  { id: 'teammate', role: 'agent', content: 'Hi Maya, I have your earlier messages and the logs. I’ve linked them to the engineering task. We’ll update you here when the fix is ready.', senderName: 'Sam Rivera', senderAvatar: '/new/avatars/sam.webp' },
  { id: 'released', role: 'system', senderName: 'Release status', content: 'After the team confirms the release' },
  { id: 'resolved', role: 'ai', senderAvatar: '/brand/helpin-icon-ink.svg', content: 'The export fix has been released, Maya. Please run your report again and let us know whether all 18,400 contacts are included.' },
];

export const DEMO_MESSAGES: Message[] = SCRIPT.map(message => ({
  ...message, conversationId: 'website-demo', isInternal: false, createdAt: '2026-09-20T09:14:00Z',
}));

// Deliberately local: no SDK client, widget key, session, or network adapter.
export function demoFrame(elapsed: number, createdAt: string) {
  const messages: Message[] = [];
  let stream: { id: string; duration: number; elapsed: number } | null = null;
  const starts = [0, 1800, 6100, 9200, 13000, 14800, 19000, 23400];
  const ends = [0, 3400, 6100, 12200, 13000, 17500, 19000, 26500];
  for (let index = 0; index < DEMO_MESSAGES.length; index += 1) {
    if (elapsed < starts[index]) continue;
    const message = DEMO_MESSAGES[index];
    const streaming = elapsed < ends[index];
    if (streaming) stream = { id: message.id, duration: ends[index] - starts[index], elapsed: elapsed - starts[index] };
    messages.push({ ...message, createdAt, isStreaming: streaming, sources: streaming ? undefined : message.sources });
  }
  return {
    messages,
    stream,
    isAIThinking: (elapsed >= 500 && elapsed < 1800) || (elapsed >= 7000 && elapsed < 9200) || (elapsed >= 22000 && elapsed < 23400),
    aiProgressLabel: elapsed < 6100 ? 'Checking the earlier conversation…' : elapsed < 19000 ? 'Searching connected export logs…' : 'Preparing the approved follow-up…',
    isTyping: elapsed >= 13500 && elapsed < 14800,
    stage: elapsed < 6100 ? 'Earlier customer report found' : elapsed < 9200 ? 'Investigating with connected logs' : elapsed < 13000 ? 'Export logs checked · Error found' : elapsed < 19000 ? 'Sam takes over · Findings attached' : elapsed < 23400 ? 'Fix released · Follow-up approved' : 'Customer update approved by Sam',
    handedOff: elapsed >= 13000 && elapsed < 23400,
  };
}
