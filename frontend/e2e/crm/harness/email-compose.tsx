import { useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Toaster } from '@/components/ui/sonner';
import { CRMEmailComposerDialog } from '@/components/crm/CRMEmailComposerDialog';
import { CRMEmailReplyComposer } from '@/components/crm/CRMEmailReplyComposer';
import { CRMEmailSignatureSettings } from '@/components/crm/CRMEmailSignature';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useAuthStore } from '@/stores/authStore';
import type { CRMEmailAccount } from '@/lib/crmTypes';
import type { Workspace, User } from '@/lib/types';
import '@/index.css';
useWorkspaceStore.setState({ currentWorkspace: {id:'ws-email',slug:'email-test',name:'Email test'} as Workspace });
useAuthStore.setState({ user:{id:'owner'} as User });
const params=new URLSearchParams(window.location.search);
if(params.has('dark')) document.documentElement.classList.add('dark');
const account={id:'mailbox',workspace_id:'ws-email',member_id:'owner',email_address:'waqar@contentstudio.io',provider:'gmail',can_send:true,is_active:true,status:'connected',signature:'Waqar Azeem\nContentStudio',sync_state:{},has_synced_data:true,created_at:'',updated_at:''} as CRMEmailAccount;
function Harness(){
 const [open,setOpen]=useState(true);
 const [next,setNext]=useState(false);
 const [reply,setReply]=useState('');
 const [sending,setSending]=useState(false);
 return <div className="min-h-screen bg-background p-6 text-foreground">
  {params.has('settings') ? <CRMEmailSignatureSettings workspaceId="ws-email" account={account}/> : params.has('reply') ? <div className="max-w-2xl"><CRMEmailReplyComposer workspaceId="ws-email" content={reply} signature={account.signature} sending={sending} onChange={setReply} onSubmit={async(data)=>{setSending(true);try {const result=await fetch('/mock-reply',{method:'POST',body:JSON.stringify(data)});if(!result.ok)throw new Error('Failed');setReply('');}finally{setSending(false);}}}/></div> : <>
  <button onClick={()=>{setNext(true);setOpen(true);}}>Compose another</button>
  {open && <CRMEmailComposerDialog workspaceId="ws-email" accounts={params.has('empty') ? [] : [account,{...account,id:'other',email_address:'other@example.com',signature:'Other sender'}]} open draft={{to:[next?'second@example.com':'amna@contentstudio.io'],dealId:'deal-1',dealName:'ContentStudio renewal'}} onOpenChange={setOpen}/>}
  </>}
  <Toaster/>
 </div>;
}
const client=new QueryClient({defaultOptions:{queries:{retry:false},mutations:{retry:false}}});
createRoot(document.getElementById('root')!).render(<QueryClientProvider client={client}><TooltipProvider><Harness/></TooltipProvider></QueryClientProvider>);
