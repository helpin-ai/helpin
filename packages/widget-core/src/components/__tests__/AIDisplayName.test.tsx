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
