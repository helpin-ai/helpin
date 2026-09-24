// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { expect, it, vi } from 'vitest';
import { TranslatedMessageBubble } from '../TranslatedMessageBubble';
const request = vi.hoisted(() => vi.fn());
vi.mock('@/hooks/queries/useSupportTranslation', () => ({
 requestSupportTranslation: request,
 useSupportTranslationOptions: () => ({data:{available:true,preference:{auto_translate_incoming:true,reading_language:'en'},languages:{en:'English',es:'Spanish'}}}),
}));
vi.mock('../MessageBubble',()=>({MessageBubble:({translationFooter,translatedContent,message}:{translationFooter:React.ReactNode;translatedContent?:string;message:{content:string}})=><div><p>{translatedContent ?? message.content}</p>{translationFooter}</div>}));
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
