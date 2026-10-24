// @vitest-environment jsdom
import { act, useEffect } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useDockPins } from '../useDockPins';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
const api = vi.hoisted(()=>({listPins:vi.fn(),setPin:vi.fn(),error:vi.fn()}));
vi.mock('@/lib/services/dockChatService',()=>({dockChatService:api}));
vi.mock('sonner',()=>({toast:{error:api.error}}));
let state: ReturnType<typeof useDockPins>;
function Harness({ user = 'me', workspace = 'ws' }: { user?: string; workspace?: string }) { const value=useDockPins(workspace,user,true);useEffect(()=>{ state=value; },[value]);return <span>{value.chats.map(chat=>chat.id).join(',')}</span>; }
let root: ReturnType<typeof createRoot>;
let container: HTMLDivElement;
let client: QueryClient;
const saved = { chats:[{id:'old-pinned-chat'}],runs:[] };
beforeEach(()=>{vi.clearAllMocks();container=document.createElement('div');document.body.append(container);root=createRoot(container);client=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});api.listPins.mockResolvedValue({data:saved,error:null});api.setPin.mockResolvedValue({data:{pinned:false},error:null});});
afterEach(()=>{act(()=>root.unmount());client.clear();container.remove();});
async function render(user='me',workspace='ws') {await act(async()=>{root.render(<QueryClientProvider client={client}><Harness user={user} workspace={workspace}/></QueryClientProvider>);});await act(async()=>{await new Promise(resolve=>setTimeout(resolve,25));});}
it('loads persisted pins independently of history and never carries them across accounts or workspaces',async()=>{
 await render();expect(state.chats.map(c=>c.id)).toEqual(['old-pinned-chat']);
 api.listPins.mockResolvedValue({data:{chats:[],runs:[]},error:null});
 await render('other');expect(state.chats).toEqual([]);
 await render('me','different');expect(state.chats).toEqual([]);
 await render();expect(state.chats.map(c=>c.id)).toEqual(['old-pinned-chat']);
});
it('refreshes confirmed pin changes and keeps the previous state when saving fails',async()=>{
 await render();
 api.setPin.mockResolvedValueOnce({data:null,error:'network unavailable'});
 await act(async()=>state.togglePin('chat','old-pinned-chat',false));
 await act(async()=>{await new Promise(resolve=>setTimeout(resolve,25));});
 expect(api.error).toHaveBeenCalled();expect(state.chats).toHaveLength(1);
 api.listPins.mockResolvedValue({data:{chats:[],runs:[]},error:null});
 await act(async()=>state.togglePin('chat','old-pinned-chat',false));
 await act(async()=>{await new Promise(resolve=>setTimeout(resolve,25));});
 expect(api.setPin).toHaveBeenLastCalledWith('ws','chat','old-pinned-chat',false);expect(state.chats).toEqual([]);
});
