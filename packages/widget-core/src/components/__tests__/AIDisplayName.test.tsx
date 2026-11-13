import {describe,it,expect,vi} from 'vitest';
import {render} from '@testing-library/preact';
import {ConversationView} from '../ConversationView';
import type {WidgetConfig} from '../../types';
const config:WidgetConfig={workspaceId:'demo',aiDisplayName:'Echo · Demo assistant',branding:{primaryColor:'#000000',welcomeMessage:'Hello',widgetPosition:'bottom-right',showBranding:false},features:{aiEnabled:true,aiFirst:true,showTalkToHuman:false,fileUploads:false,preChatForm:false,requirePhone:false,csatRating:false,forceIdentify:false},availability:{isOnline:true,statusText:'Online',replyTimeText:''}};
describe('AI display name',()=>{
 it('uses configured identity without changing a human sender',()=>{
  const {container}=render(<ConversationView config={config} conversation={{id:'c',subject:'Help',status:'open'}} messages={[{id:'a',conversationId:'c',role:'ai',content:'Welcome',isInternal:false,createdAt:'2026-01-01T00:00:00Z'},{id:'h',conversationId:'c',role:'agent',senderName:'Alex',content:'I can help',isInternal:false,createdAt:'2026-01-01T00:01:00Z'}]} onSendMessage={vi.fn()} onBack={vi.fn()} />);
  expect(container.textContent).toContain('Echo · Demo assistant · AI');
  expect(container.textContent).toContain('Alex');
  expect(container.textContent).not.toContain('Helpin AI');
 });
});

describe('configured AI avatar', () => {
 const branded = {...config, workspaceName: 'Demo support', branding: {...config.branding, avatarUrl: 'https://example.com/avatar.png', logoUrl: 'https://example.com/logo.png'}};
 it.each(['intro', 'reply', 'legacy handoff'])('uses the identity avatar for the %s and keeps the AI label', (state) => {
  const messages = state === 'intro' ? [] : [{id:'a',conversationId:'c',role: state === 'reply' ? 'ai' as const : 'system' as const,senderName:'Helpin AI',content:'Hello',isInternal:false,createdAt:'2026-01-01T00:00:00Z'}];
  const {container}=render(<ConversationView config={branded} messages={messages} onSendMessage={vi.fn()} onBack={vi.fn()} />);
  expect(container.querySelector('.helpin-conversation-header img')?.getAttribute('src')).toBe(branded.branding.avatarUrl);
  expect(container.querySelector('.helpin-message-agent-header img')?.getAttribute('src')).toBe(branded.branding.avatarUrl);
  expect(container.querySelector('.helpin-message-agent-name')?.textContent).toBe('Echo · Demo assistant · AI');
 });
 it('keeps a human teammate avatar and name after the AI reply', () => {
  const {container}=render(<ConversationView config={branded} messages={[
   {id:'a',conversationId:'c',role:'ai',content:'Hello',isInternal:false,createdAt:'2026-01-01T00:00:00Z'},
   {id:'h',conversationId:'c',role:'agent',senderName:'Alex',senderAvatar:'https://example.com/alex.png',content:'I can help',isInternal:false,createdAt:'2026-01-01T00:01:00Z'},
  ]} onSendMessage={vi.fn()} onBack={vi.fn()} />);
  expect(container.querySelector('.helpin-conversation-title')?.textContent).toBe('Alex');
  expect(container.querySelector('.helpin-conversation-header img')?.getAttribute('src')).toBe('https://example.com/alex.png');
  expect([...container.querySelectorAll('.helpin-message-agent-header img')].map(i=>i.getAttribute('src'))).toEqual([branded.branding.avatarUrl,'https://example.com/alex.png']);
 });
 it('keeps the default AI icon when no identity avatar is configured', () => {
  const {container}=render(<ConversationView config={config} messages={[]} onSendMessage={vi.fn()} onBack={vi.fn()} />);
  expect(container.querySelector('.helpin-message-agent-header svg')).not.toBeNull();
 });
});
