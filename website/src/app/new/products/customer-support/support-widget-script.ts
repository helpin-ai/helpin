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
  { id: 'question', role: 'customer', content: 'How do I export just the contacts I selected?' },
  { id: 'answer', role: 'ai', senderAvatar: '/brand/helpin-icon-ink.svg', content: 'Select your contacts, then choose Export → Selected contacts.', aiReplyKind: 'answer', sources: [{ docId: 'export-guide', title: 'Export your contacts', snippet: 'Select contacts, then choose Export → Selected contacts.', confidence: 1, language: 'en' }] },
  { id: 'followup', role: 'customer', content: 'It stops at 10,000 rows. We need all 18,400 for our report.' },
  { id: 'handoff', role: 'ai', senderAvatar: '/brand/helpin-icon-ink.svg', content: 'I checked the export logs. Pagination stops after 10,000 rows; your filters are correct. I’m handing Sam the findings.' },
  { id: 'joined', role: 'system', content: 'Sam Rivera joined the conversation', systemEventType: 'teammate_joined', senderName: 'Sam Rivera', senderAvatar: '/new/avatars/sam.webp' },
  { id: 'teammate', role: 'agent', content: 'Hi Maya. I’ve linked your report and the logs to EXP-142 so engineering can fix the export.', senderName: 'Sam Rivera', senderAvatar: '/new/avatars/sam.webp' },
  { id: 'resolved', role: 'ai', senderAvatar: '/brand/helpin-icon-ink.svg', content: 'The export fix is live. Please try your report again — all 18,400 contacts should now be included.' },
];

export const DEMO_MESSAGES: Message[] = SCRIPT.map(message => ({
  ...message, conversationId: 'website-demo', isInternal: false, createdAt: '2026-09-20T09:14:00Z',
}));

// Deliberately local: no SDK client, widget key, session, or network adapter.
export function demoFrame(elapsed: number, createdAt: string) {
  const messages: Message[] = [];
  let stream: { id: string; duration: number; elapsed: number } | null = null;
  const starts = [0, 1800, 6100, 9200, 13000, 14800, 23400];
  const ends = [0, 3400, 6100, 12200, 13000, 17500, 26500];
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
    aiProgressLabel: elapsed < 6100 ? 'Checking the export guide…' : elapsed < 19000 ? 'Searching connected export logs…' : 'Preparing the approved follow-up…',
    isTyping: elapsed >= 13500 && elapsed < 14800,
    stage: elapsed < 6100 ? 'An answer from your docs' : elapsed < 9200 ? 'Investigating with connected logs' : elapsed < 13000 ? 'Export logs checked · Error found' : elapsed < 19000 ? 'Sam takes over · Findings attached' : elapsed < 23400 ? 'Fix released · Follow-up approved' : 'Helpin AI follows up with Maya',
    handedOff: elapsed >= 13000 && elapsed < 23400,
  };
}
