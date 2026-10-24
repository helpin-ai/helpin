// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { DockRoster } from '../DockRoster';
import { TooltipProvider } from '@/components/ui/tooltip';
import { useAuthStore } from '@/stores/authStore';
import type { DockChat, DockRunSummary } from '@/lib/dockTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
vi.mock('@/components/ui/dropdown-menu', () => ({
 DropdownMenu: ({children}: { children: React.ReactNode }) => <>{children}</>, DropdownMenuTrigger: ({children}: { children: React.ReactNode }) => <>{children}</>,
 DropdownMenuContent: ({children}: { children: React.ReactNode }) => <div>{children}</div>,
 DropdownMenuItem: ({children,onSelect}: { children: React.ReactNode; onSelect: () => void }) => <button onClick={onSelect}>{children}</button>,
}));
const chat = {id:'chat', user_id:'someone-else',title:'Shared discussion',visibility:'workspace',updated_at:new Date().toISOString()} as DockChat;
const run = {run:{id:'run',status:'completed',target_type:'task',target_id:'task',input:{}},agent:{id:'agent',name:'Research'},last_activity_at:new Date().toISOString()} as DockRunSummary;
const onTogglePin = vi.fn();
const base = { workspaceId:'ws', selectedChatId:null,selectedRunId:null,chats:[chat],runs:[run],onTabChange:vi.fn(),onSelectChat:vi.fn(),onSelectRun:vi.fn(),onNewChat:vi.fn(),onRenameChat:vi.fn(),onArchiveChat:vi.fn(),onRetryChats:vi.fn(),onRetryRuns:vi.fn(),onTogglePin };
const container = document.createElement('div'); document.body.append(container);
let root: ReturnType<typeof createRoot>;
beforeEach(()=>{vi.stubGlobal('ResizeObserver',class { observe() {} unobserve() {} disconnect() {} });});
afterEach(()=>{act(()=>root.unmount());container.innerHTML='';vi.clearAllMocks();vi.unstubAllGlobals();});
it('lets readers pin a shared chat without offering owner actions and explains sharing', async()=>{
 useAuthStore.setState({user:{id:'me'} as never});root=createRoot(container);
 await act(async()=>root.render(<TooltipProvider><DockRoster {...base} tab="chats" pinnedChatIds={[]} pinnedRunIds={[]} /></TooltipProvider>));
 expect(container.textContent).toContain('Pin');expect(container.textContent).not.toContain('Rename');expect(container.textContent).not.toContain('Archive');
 await act(async()=>container.querySelector<HTMLElement>('[role="img"]')?.focus());
 expect(document.querySelector('[role="tooltip"]')?.textContent).toContain('everyone in this workspace');
 await act(async()=>Array.from(container.querySelectorAll('button')).find(b=>b.textContent==='Pin')?.click());
 expect(onTogglePin).toHaveBeenCalledWith('chat','chat',true);
 expect(base.onSelectChat).not.toHaveBeenCalled();
});
it.each(['chats','agents'] as const)('shows a pinned %s item once and allows unpinning',async(tab)=>{
 root=createRoot(container);
 await act(async()=>root.render(<TooltipProvider><DockRoster {...base} tab={tab} pinnedChatIds={['chat']} pinnedRunIds={['run']} /></TooltipProvider>));
 expect(container.querySelectorAll('.agent-dock-roster-row')).toHaveLength(1);
 expect(container.querySelector('section')?.textContent).toContain('Pinned');
 await act(async()=>Array.from(container.querySelectorAll('button')).find(b=>b.textContent==='Unpin')?.click());
 expect(onTogglePin).toHaveBeenCalledWith(tab==='chats'?'chat':'run',tab==='chats'?'chat':'run',false);
 expect(container.querySelector('button button')).toBeNull();
});
