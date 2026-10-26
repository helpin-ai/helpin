// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, it, vi } from 'vitest';
import { TranslatedMessageBubble } from '../TranslatedMessageBubble';
const request = vi.hoisted(() => vi.fn());
const settings = vi.hoisted(() => ({ reading: 'en', detected: '' }));
vi.mock('@/hooks/queries/useSupportTranslation', () => ({
 requestSupportTranslation: request,
 useSupportTranslationOptions: () => ({data:{available:true,preference:{auto_translate_incoming:true,reading_language:settings.reading},conversation:{customer_language:'',translation_mode:'off'},detected_customer_language:settings.detected,languages:{en:'English',es:'Spanish'}}}),
}));
vi.mock('../MessageBubble',()=>({MessageBubble:({translationFooter,translationStatus,translatedContent,message}:{translationFooter:React.ReactNode;translationStatus?:React.ReactNode;translatedContent?:string;message:{content:string}})=><div><p>{translatedContent ?? message.content}</p><div data-footer>{translationFooter}</div><div data-status>{translationStatus}</div></div>}));
(globalThis as unknown as {IS_REACT_ACT_ENVIRONMENT:boolean}).IS_REACT_ACT_ENVIRONMENT=true;
it('offers manual Translate without generating historical translations on mount',async()=>{
 const div=document.createElement('div');const root=createRoot(div);const client=new QueryClient();
 await act(async()=>root.render(<QueryClientProvider client={client}><TranslatedMessageBubble workspaceId="w" message={{id:'old',workspace_id:'w',conversation_id:'c',sender_type:'customer',message_type:'reply',content:'Hola',is_internal:false} as never}/></QueryClientProvider>));
 expect(request).not.toHaveBeenCalled();expect(div.textContent).toContain('Translate');expect(div.textContent).not.toContain('Translating…');
 await act(async()=>root.unmount());client.clear();
});

it('toggles the saved original of an outgoing translation without calling a model', async () => {
 const div=document.createElement('div');const root=createRoot(div);const client=new QueryClient();
 await act(async()=>root.render(<QueryClientProvider client={client}><TranslatedMessageBubble workspaceId="w" message={{id:'sent',workspace_id:'w',conversation_id:'c',sender_type:'user',message_type:'reply',content:'Hola',is_internal:false} as never} cachedTranslation={{purpose:'outgoing_reply',sent_message_id:'sent',source_text:'Hello',translated_text:'Hola',source_language:'en',target_language:'es',status:'ready'} as never}/></QueryClientProvider>));
 expect(div.textContent).toContain('Translated from English');
 const button=div.querySelector('button');expect(button?.textContent).toBe('Show original');
 await act(async()=>button?.click());expect(div.querySelector('p')?.textContent).toBe('Hello');
 await act(async()=>button?.click());expect(div.querySelector('p')?.textContent).toBe('Hola');
 expect(request).not.toHaveBeenCalled();
 await act(async()=>root.unmount());client.clear();
});

it('renders translation preparation below the bubble instead of in its footer',async()=>{
 const div=document.createElement('div');const root=createRoot(div);const client=new QueryClient();
 await act(async()=>root.render(<QueryClientProvider client={client}><TranslatedMessageBubble workspaceId="w" message={{id:'pending',workspace_id:'w',conversation_id:'c',sender_type:'customer',message_type:'reply',content:'Hola',is_internal:false} as never} cachedTranslation={{source_message_id:'pending',source_text:'Hola',target_language:'en',status:'pending'} as never}/></QueryClientProvider>));
 expect(div.querySelector('[data-footer]')?.textContent).not.toContain('Preparing');
 expect(div.querySelector('[data-status]')?.textContent).toContain('Preparing');
 await act(async()=>root.unmount());client.clear();
});

it('translates an AI reply on demand and toggles its original customer-visible text', async () => {
 request.mockReset();
 request.mockResolvedValue({ purpose: 'manual_display', source_message_id: 'ai-reply', source_text: 'Hola', translated_text: 'Hello', source_language: 'es', target_language: 'en', status: 'ready', updated_at: new Date().toISOString() });
 const div = document.createElement('div'); const root = createRoot(div); const client = new QueryClient();
 await act(async () => root.render(<QueryClientProvider client={client}><TranslatedMessageBubble workspaceId="w" message={{ id: 'ai-reply', workspace_id: 'w', conversation_id: 'c', sender_type: 'ai', message_type: 'reply', content: 'Hola', is_internal: false } as never}/></QueryClientProvider>));
 expect(request).not.toHaveBeenCalled();
 expect(div.querySelector('button')?.textContent).toBe('Translate');
 await act(async () => { div.querySelector('button')!.click(); await new Promise(resolve => setTimeout(resolve, 10)); });
 expect(request).toHaveBeenCalledWith('w', 'c', { message_id: 'ai-reply', target_language: 'en' });
 expect(div.textContent).toContain('Translated from Spanish');
 expect(div.querySelector('p')?.textContent).toBe('Hello');
 await act(async () => div.querySelector('button')!.click());
 expect(div.querySelector('p')?.textContent).toBe('Hola');
 expect(div.textContent).toContain('Show translation');
 expect(request).toHaveBeenCalledTimes(1);
 await act(async () => root.unmount()); client.clear();
});


it.each([
 ['ai', 'en', 'Hello! I can help you update your email address. Please open your account settings and select your profile.'],
 ['customer', 'en', 'Hello, I need help updating my email address. I cannot find the option in my account settings.'],
 ['ai', 'es', 'Hola, puedo ayudarte a cambiar la dirección de correo electrónico. Abre la configuración de tu cuenta para continuar.'],
])('hides Translate for a %s message already in %s', async (sender, reading, content) => {
 request.mockReset(); settings.reading = reading;
 const div = document.createElement('div'); const root = createRoot(div); const client = new QueryClient();
 try {
  await act(async () => root.render(<QueryClientProvider client={client}><TranslatedMessageBubble workspaceId="w" message={{id:'same',workspace_id:'w',conversation_id:'c',sender_type:sender,message_type:'reply',content,is_internal:false} as never}/></QueryClientProvider>));
  expect(div.querySelector('button')).toBeNull();
  expect(div.querySelector('p')?.textContent).toBe(content);
  expect(request).not.toHaveBeenCalled();
 } finally { await act(async () => root.unmount()); client.clear(); settings.reading = 'en'; }
});


it.each([
 ...['pending', 'failed', 'expired', 'ready'].map(status => ({ name: `English with ${status} cache and unknown source`, content: 'I can help you update your email address. Please open your account settings and select your profile.', source: 'I can help you update your email address. Please open your account settings and select your profile.', sourceLanguage: 'und', translated: '', status, expected: null })),
 { name: 'regional English evidence', content: 'Hello', source: 'Hello', sourceLanguage: 'en-US', translated: 'Hi', status: 'ready', expected: null },
 { name: 'mixed language evidence overrides local guess', content: 'I can help you update your email address. Please open your account settings and select your profile.', source: 'I can help you update your email address. Please open your account settings and select your profile.', sourceLanguage: 'mul', translated: 'Translation', status: 'ready', expected: 'Show original' },
 { name: 'known same-language text', content: 'Hello', source: 'Hello', sourceLanguage: 'en', translated: 'Hi', status: 'ready', expected: null },
 { name: 'a real display translation', content: 'Hola', source: 'Hola', sourceLanguage: 'es', translated: 'Hello', status: 'ready', expected: 'Show original' },
 { name: 'stale cached evidence', content: 'Hola', source: 'Hello', sourceLanguage: 'en', translated: 'Hello', status: 'ready', expected: 'Translate' },
 { name: 'a failed translation', content: 'Hola', source: 'Hola', sourceLanguage: '', translated: '', status: 'failed', expected: 'Retry' },
])('preserves correct controls for $name', async ({content, source, sourceLanguage, translated, status, expected}) => {
 request.mockReset();
 const div = document.createElement('div'); const root = createRoot(div); const client = new QueryClient();
 try {
  await act(async () => root.render(<QueryClientProvider client={client}><TranslatedMessageBubble workspaceId="w" message={{id:'evidence',workspace_id:'w',conversation_id:'c',sender_type:'ai',message_type:'reply',content,is_internal:false} as never} cachedTranslation={{source_message_id:'evidence',source_text:source,source_language:sourceLanguage,target_language:'en',translated_text:translated,status} as never}/></QueryClientProvider>));
  expect(div.querySelector('button')?.textContent ?? null).toBe(expected);
  if (expected === null) { expect(div.querySelector('[data-status]')?.textContent).toBe(''); expect(div.querySelector('p')?.textContent).toBe(content); }
  if (sourceLanguage === 'en' && source === content) expect(div.querySelector('p')?.textContent).toBe(content);
  expect(request).not.toHaveBeenCalled();
 } finally { await act(async () => root.unmount()); client.clear(); }
});


it.each([
 ['en', 'customer', 'Thanks, it is working now!', null],
 ['en', 'ai', 'Thanks for reaching out!\n\nTo reconnect your Instagram account:\n\n1. Go to Settings > Social Accounts\n2. Click Reconnect next to Instagram', null],
 ['en', 'customer', 'No puedo conectar mi cuenta de Instagram', 'Translate'],
 ['en', 'customer', 'مرحبا، أحتاج مساعدة', 'Translate'],
 ['es', 'customer', 'Thanks, it is working now!', 'Translate'],
])('in a conversation detected as %s, a %s message "%s" shows %s', async (detected, sender, content, expected) => {
 request.mockReset(); settings.detected = detected;
 const div = document.createElement('div'); const root = createRoot(div); const client = new QueryClient();
 try {
  await act(async () => root.render(<QueryClientProvider client={client}><TranslatedMessageBubble workspaceId="w" message={{id:'conv',workspace_id:'w',conversation_id:'c',sender_type:sender,message_type:'reply',content,is_internal:false} as never}/></QueryClientProvider>));
  expect(div.querySelector('button')?.textContent ?? null).toBe(expected);
  expect(request).not.toHaveBeenCalled();
 } finally { await act(async () => root.unmount()); client.clear(); settings.detected = ''; }
});

it('keeps a real display translation in a conversation detected as the reading language', async () => {
 settings.detected = 'en';
 const div = document.createElement('div'); const root = createRoot(div); const client = new QueryClient();
 try {
  await act(async () => root.render(<QueryClientProvider client={client}><TranslatedMessageBubble workspaceId="w" message={{id:'live',workspace_id:'w',conversation_id:'c',sender_type:'customer',message_type:'reply',content:'Hola',is_internal:false} as never} cachedTranslation={{source_message_id:'live',source_text:'Hola',source_language:'es',target_language:'en',translated_text:'Hello',status:'ready'} as never}/></QueryClientProvider>));
  expect(div.querySelector('button')?.textContent).toBe('Show original');
  expect(div.querySelector('p')?.textContent).toBe('Hello');
 } finally { await act(async () => root.unmount()); client.clear(); settings.detected = ''; }
});
