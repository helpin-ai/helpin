import { useEffect, useState } from 'react';
import { QuietPageHeader, QuietPrimaryAction, QuietSection, QuietTextAction } from '@/components/design-system/quiet';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog';
import { Sheet, SheetContent } from '@/components/ui/sheet';
import { TooltipProvider } from '@/components/ui/tooltip';
import { HelpinLogo } from '@/components/layout/HelpinLogo';
import { BookOpen01Icon, ChartIncreaseIcon, Mail01Icon, UserGroupIcon, WorkflowSquare01Icon } from '@/lib/icons';
import { SignalsWorkspace, type QueueFilters } from './SignalsWorkspace';
import { PlaybookDetail, PlaybooksWorkspace } from './PlaybooksWorkspace';
import { PlaybookEditor } from './PlaybookEditor';
import { SituationDrawer, type SituationAction } from './SituationDrawer';
import { CustomerWorkspace } from './CustomerWorkspace';
import { AutomationWorkspace } from './AutomationWorkspace';
import { Field, Person, Picker, ResourceLink } from './workspace-ui';
import { ME, customerNames, initialSituations, isAttention, people, playbookSeed, situationSeed, type Playbook, type Scene, type Situation, type View } from './workspace-data';
import '@/index.css';
import './preview.css';
import './workspace.css';

type Route = {view:View;id?:string;tab?:string;focus?:string};
type Modal = {kind:'guide'|'new'} | {kind:'publish';book:Playbook} | {kind:'apply';book:Playbook} | {kind:'decision';situation:Situation;action:SituationAction;patch?:Partial<Situation>} | {kind:'resource';title:string;body:string};
const defaultFilters:QueueFilters={scope:'Mine',state:'Needs attention',category:'All categories'};
function readLocation() {
  const p=new URLSearchParams(location.search);
  const allowed=['signals','playbooks','playbook','edit','customer','flows','agents','activity'];
  return {route:{view:allowed.includes(p.get('view')||'')?p.get('view') as View:'signals',id:p.get('id')||undefined,tab:p.get('tab')||undefined,focus:p.get('focus')||undefined},filters:{scope:p.get('scope')||'Mine',state:p.get('state')||'Needs attention',category:p.get('category')||'All categories'},selected:p.get('situation')};
}
function writeLocation(route:Route,filters:QueueFilters,selected:string|null,replace=false) {
  const url=new URL(location.href);url.search='';url.searchParams.set('view',route.view);
  for(const [key,value] of Object.entries(route))if(key!=='view'&&value)url.searchParams.set(key,value);
  if(route.view==='signals')for(const [key,value] of Object.entries(filters))if(value!==defaultFilters[key as keyof QueueFilters])url.searchParams.set(key,value);
  if(selected)url.searchParams.set('situation',selected);
  history[replace?'replaceState':'pushState'](null,'',url);
}

export function WorkspaceMock() {
  const [situations,setSituations]=useState<Situation[]>(initialSituations);
  const [books,setBooks]=useState<Playbook[]>(()=>structuredClone(playbookSeed));
  const [route,setRoute]=useState<Route>(()=>readLocation().route);
  const [filters,setFilters]=useState<QueueFilters>(()=>readLocation().filters);
  const [selected,setSelected]=useState<string|null>(()=>readLocation().selected);
  const [draft,setDraft]=useState<Playbook|null>(null);
  const [modal,setModal]=useState<Modal|null>(()=>new URLSearchParams(location.search).has('guide')?{kind:'guide'}:null);
  const [scene,setScene]=useState<Scene>('Populated');
  const [dark,setDark]=useState(false);
  const [notice,setNotice]=useState('');
  const [resetKey,setResetKey]=useState(0);
  const active=situations.find(s=>s.id===selected);
  const book=books.find(b=>b.id===route.id);
  const readOnly=scene==='Read-only';
  useEffect(()=>{document.documentElement.classList.toggle('dark',dark);},[dark]);
  useEffect(()=>{const back=()=>{const next=readLocation();setRoute(next.route);setFilters(next.filters);setSelected(next.selected);setModal(null);};window.addEventListener('popstate',back);return()=>window.removeEventListener('popstate',back);},[]);
  useEffect(()=>{if(!notice)return;const timer=window.setTimeout(()=>setNotice(''),4500);return()=>clearTimeout(timer);},[notice]);
  const navigate=(view:View,id?:string,tab?:string,focus?:string)=>{const next={view,id,tab,focus};setRoute(next);setSelected(null);setModal(null);writeLocation(next,filters,null);document.querySelector('.workspace-content')?.scrollTo(0,0);};
  const filter=(patch:Partial<QueueFilters>)=>{const next={...filters,...patch};setFilters(next);writeLocation(route,next,selected,true);};
  const open=(id:string)=>{setSelected(id);writeLocation(route,filters,id,true);};
  const close=()=>{setSelected(null);writeLocation(route,filters,null,true);};
  const resource=(title:string,body:string)=>setModal({kind:'resource',title,body});
  const patch=(id:string,values:Partial<Situation>,event:string)=>setSituations(old=>old.map(s=>s.id===id?{...s,...values,history:[event,...s.history]}:s));
  const reset=()=>{setSituations(initialSituations());setBooks(structuredClone(playbookSeed));setDraft(null);setFilters(defaultFilters);setScene('Populated');setSelected(null);setModal(null);setRoute({view:'signals'});setResetKey(k=>k+1);writeLocation({view:'signals'},defaultFilters,null);setNotice('Sample data reset.');};
  const act=(s:Situation,action:SituationAction,values?:Partial<Situation>)=>{
    if(readOnly)return;
    if(action==='apply-change'){patch(s.id,{...values,status:'Waiting',actionOwner:'',reviewed:true,next:'CRM updated · Continue the customer conversation'},'Exact CRM change approved and applied by you · Just now');setNotice('The proposed fields were updated in the preview.');return;}
    if(action==='send'&&s.status!=='Delivery failed') {patch(s.id,{...values,status:'Waiting',actionOwner:'',reviewed:true,progress:3,next:'Customer reply · Check September 8'},'Approved response sent · Just now');setNotice('Response sent in the preview. Waiting for the customer.');return;}
    if(action==='handoff'){patch(s.id,{...values,status:'Resolved',actionOwner:'',reviewed:true,progress:5,next:'Handoff accepted · First-value goal agreed',outcome:`Handoff accepted by ${values?.owner===ME?'you':values?.owner}. First-value goal: ${values?.goal} Onboarding is still in progress.`},'Success owner accepted the handoff and first-value goal · Just now');setNotice('Handoff accepted. No task or project was created.');return;}
    if(action==='assign'){patch(s.id,{owner:ME,actionOwner:ME,next:'You own the next step · Review the seat request'},'You took ownership · Just now');setNotice('Assigned to you.');return;}
    if(action==='resume'){patch(s.id,{status:s.previousStatus||'Needs attention',actionOwner:ME,next:s.previousStatus==='Waiting'?'Customer reply · Check September 8':'Review the next step before continuing'},'Customer work resumed by you · Just now');setNotice('Customer work resumed.');return;}
    if(action==='reopen'){patch(s.id,{status:'Needs attention',actionOwner:ME,outcome:undefined,progress:Math.min(s.progress,4),next:'Reopened · Review what the customer needs next'},'Situation reopened by you · Just now');setNotice('Reopened for follow-up.');return;}
    setModal({kind:'decision',situation:s,action,patch:values});
  };
  const finishDecision=(s:Situation,action:SituationAction,notes:string,owner:string,due:string,values?:Partial<Situation>,closeSituation=true,renewalProgress?:Situation['renewalProgress'])=>{
    if(action==='pause')patch(s.id,{status:'Paused',previousStatus:s.status,actionOwner:'',next:`Paused · ${notes||'Awaiting the owner’s next decision'}`},`Paused by you: ${notes||'Manual pause'} · Just now`);
    if(action==='reject')patch(s.id,{status:'Needs attention',actionOwner:ME,next:`Changes requested · ${notes}`,reviewed:true},`You withheld approval: ${notes} · Just now`);
    if(action==='resolve')patch(s.id,{status:closeSituation?'Resolved':'Waiting',actionOwner:'',outcome:closeSituation?notes:undefined,renewalProgress,title:renewalProgress?.renewal==='Customer confirmed renewal'?'Renewal confirmed by the customer':renewalProgress?.renewal==='Customer will not renew'?'Customer will not renew':renewalProgress?.blocker==='Customer confirmed resolution'?'Blocker resolved; renewal awaiting confirmation':s.title,progress:closeSituation?((s.configuration||books.find(b=>b.id===s.playbook))?.steps.length||1):renewalProgress?.blocker==='Customer confirmed resolution'?4:s.progress,next:closeSituation?'Outcome recorded · Just now':'Renewal confirmation needed · Check September 8',reviewed:true},`Outcome recorded by you: ${notes} · Just now`);
    if(action==='task')patch(s.id,{task:`CRM-${300+situations.filter(item=>item.task).length} · ${notes}`,taskOwner:owner,taskDue:due},`Follow-up task created with your approval · ${owner} · Due ${due}`);
    if(action==='send')patch(s.id,{...values,status:'Waiting',actionOwner:'',reviewed:true,progress:3,next:'Customer reply · Check September 8'},'Email reconnected; approved response sent once · Just now');
    setModal(null);setNotice(action==='task'?'Follow-up task linked. The customer situation remains open.':action==='send'?'Preview connection restored. The response was sent once.':'Decision saved in the preview.');
  };
  const create=(template:Playbook)=>{const next={...structuredClone(template),template:template.template||template.id,id:`custom-${books.length+1}`,name:`${template.name} — custom`,version:0,enabled:false};setDraft(next);navigate('edit',next.id);};
  const publish=(published:Playbook)=>{const next={...published,enabled:published.version===0?true:published.enabled,version:published.version+1};setBooks(old=>old.some(b=>b.id===next.id)?old.map(b=>b.id===next.id?next:b):[...old,next]);setDraft(null);navigate('playbook',next.id);setNotice(`Published version ${next.version} in the preview. Existing customer work is unchanged.`);};
  const apply=(chosen:Playbook,company:string)=>{
    if(situations.some(s=>s.company===company&&s.playbook===chosen.id&&s.status!=='Resolved'))return;
    const template=situationSeed.find(s=>s.playbook===(chosen.template||chosen.id)&&s.status==='Needs approval')||situationSeed[1];
    const next:Situation={...structuredClone(template),id:`${company.toLowerCase()}-${Date.now()}`,company,owner:ME,actionOwner:ME,status:'Needs approval',title:`${chosen.name} is ready to review`,playbook:chosen.id,author:`${company} customer team`,quote:'Please help us agree the right next step for our team.',source:'Example customer conversation',time:'Just now',goal:chosen.goal,task:undefined,history:['Manually enrolled by you · Just now','Sample customer context prepared for review'],next:'Your approval · Review the prepared next step',draft:template.kind==='handoff'?'':'Hello team,\n\nLet’s confirm your priorities and agree a practical next step together. Would a short call this week work for you?\n\nMuhammad'};
    next.configuration=structuredClone(chosen);
    setSituations(old=>[next,...old]);setModal(null);open(next.id);setNotice(`${company} added to the playbook in the preview.`);
  };
  const explore=(item:GuideItem)=>{setScene('Populated');if(item.filters){const next={...defaultFilters,...item.filters};setFilters(next);setRoute({view:'signals'});setSelected(item.situation||null);setModal(null);writeLocation({view:'signals'},next,item.situation||null);}else{navigate(item.view,item.id,item.tab);if(item.situation){setSelected(item.situation);writeLocation({view:item.view,id:item.id,tab:item.tab},filters,item.situation,true);}}};
  const navItems:[string,View,typeof ChartIncreaseIcon][]=[['Signals','signals',ChartIncreaseIcon],['Playbooks','playbooks',BookOpen01Icon],['Companies','customer',UserGroupIcon],['Flows','flows',WorkflowSquare01Icon],['Agents','agents',UserGroupIcon],['Activity','activity',ChartIncreaseIcon]];
  const currentLabel=route.view==='edit'?'Edit playbook':route.view==='playbook'?'Playbooks':navItems.find(([,v])=>v===route.view)?.[0]||'CRM';
  const automation=['flows','agents','activity'].includes(route.view);
  return <TooltipProvider><div className="preview-root work-preview">
    <div className="preview-bar"><span><strong>Design preview</strong><span className="preview-disclaimer"> · Sample data · Changes stay in this tab</span></span><div className="preview-controls"><QuietTextAction onClick={()=>setModal({kind:'guide'})}>Explore screens</QuietTextAction><a href="/design-preview/blueprint/" target="_blank" rel="noreferrer">Blueprint ↗</a><Picker label="Preview state" value={scene} options={['Populated','Empty','Loading','Error','Read-only']} onChange={v=>setScene(v as Scene)}/><QuietTextAction onClick={()=>setDark(v=>!v)}>{dark?'Light':'Dark'}</QuietTextAction><QuietTextAction onClick={reset}>Reset</QuietTextAction></div></div>
    <div className="app-frame"><aside className="app-sidebar" aria-label="Workspace navigation"><div className="brand-row"><HelpinLogo className="justify-start" imageClassName="size-7"/></div><div className="workspace-switch"><span className="workspace-monogram">H</span><span>Helpin workspace<small>Sales & customer success</small></span></div><nav className="module-nav" aria-label="Modules">{[['CRM',ChartIncreaseIcon],['Projects',WorkflowSquare01Icon],['Support',Mail01Icon],['Docs',BookOpen01Icon]].map(([name,Icon])=>{const Symbol=Icon as typeof Mail01Icon;return <button key={String(name)} title={String(name)} aria-label={String(name)} className={name==='CRM'?'module-active':''} onClick={()=>name==='CRM'?navigate('signals'):resource(String(name),'This is an existing Helpin module outside this CRM design mockup. Relevant work is linked from a customer situation only when it is needed. No live records are connected.')}><Symbol className="size-[17px]"/></button>;})}</nav>
      <div className="sidebar-label">CRM</div><nav aria-label="CRM">{navItems.slice(0,3).map(([label,view,Icon])=><button key={view} className={`nav-item ${(view==='playbooks'&&['playbooks','playbook','edit'].includes(route.view)||route.view===view)?'nav-selected':''}`} onClick={()=>navigate(view)}><Icon className="size-4"/>{label}{view==='signals'&&<span className="nav-count">{situations.filter(s=>isAttention(s)&&s.actionOwner===ME).length}</span>}</button>)}</nav><div className="sidebar-label automation-label">AUTOMATION</div><nav aria-label="Automation">{navItems.slice(3).map(([label,view,Icon])=><button key={view} className={`nav-item ${route.view===view?'nav-selected':''}`} onClick={()=>navigate(view)}><Icon className="size-4"/>{label}</button>)}</nav><div className="sidebar-bottom"><div className="sidebar-design-help"><button onClick={()=>setModal({kind:'guide'})}>Explore the complete design →</button><p>3 customer journeys<br/>One connected workspace</p></div><div className="profile"><span className="avatar">MA</span><span>Muhammad Azhar<small>Helpin workspace</small></span></div></div></aside>
    <main className="signals-page workspace-content"><div className="page-topline"><span>{automation?'Automation':'CRM'}<span className="breadcrumb-slash"> / </span><button onClick={()=>navigate(route.view==='playbook'||route.view==='edit'?'playbooks':route.view)}>{currentLabel}</button>{route.id&&<><span className="breadcrumb-slash"> / </span><span>{route.view==='customer'?route.id:book?.name||'Draft'}</span></>}</span><span>Sample snapshot · Sep 5, 2026</span></div><div key={resetKey}>
      {route.view==='signals'&&<SignalsWorkspace situations={situations} filters={filters} setFilters={filter} onOpen={open} onPlaybooks={()=>navigate('playbooks')} scene={scene} onRetry={()=>setScene('Populated')}/>}
      {route.view==='playbooks'&&<PlaybooksWorkspace books={books} situations={situations} onOpen={id=>navigate('playbook',id)} onNew={()=>setModal({kind:'new'})} scene={scene} onRetry={()=>setScene('Populated')}/>}
      {route.view==='playbook'&&book&&<PlaybookDetail key={`${book.id}-${route.tab}`} book={book} situations={situations} initialTab={route.tab} readOnly={readOnly} onEdit={()=>{setDraft(null);navigate('edit',book.id);}} onApply={()=>setModal({kind:'apply',book})} onToggle={()=>{if(!readOnly){setBooks(old=>old.map(b=>b.id===book.id?{...b,enabled:!b.enabled}:b));setNotice(`${book.enabled?'Disabled':'Enabled'} new enrollments. Active customer work is unchanged.`);}}} onOpen={open} onFlow={()=>navigate('flows',book.id)}/>}
      {route.view==='edit'&&(draft||book)&&<PlaybookEditor key={(draft||book)!.id} book={(draft||book)!} readOnly={readOnly} onCancel={()=>{setDraft(null);navigate(book?'playbook':'playbooks',book?.id);}} onPreview={next=>setModal({kind:'publish',book:next})}/>}
      {(route.view==='playbook'||route.view==='edit')&&!book&&!draft&&<QuietSection title="This playbook isn’t in the sample"><QuietTextAction onClick={()=>navigate('playbooks')}>Back to playbooks</QuietTextAction></QuietSection>}
      {route.view==='customer'&&(route.id?<CustomerWorkspace key={`${route.id}-${route.focus}`} company={route.id} focus={route.focus} situations={situations} books={books} onOpen={open} onBook={id=>navigate('playbook',id)} onResource={resource}/>:<><QuietPageHeader title="Companies" description="Customer relationships and the work moving them forward."/><table className="work-table work-top-gap" aria-label="Companies"><thead><tr><th>Company</th><th>Account owner</th><th>Active situations</th><th>Needs attention</th></tr></thead><tbody>{customerNames.map(company=>{const members=situations.filter(s=>s.company===company);return <tr key={company} onClick={()=>navigate('customer',company)}><td><button><strong>{company}</strong></button></td><td><Person name={company==='Northstar'?'Sara Ahmed':ME}/></td><td>{members.filter(s=>s.status!=='Resolved').length}</td><td>{members.filter(isAttention).length||'None'}</td></tr>;})}</tbody></table></>)}
      {automation&&<AutomationWorkspace key={`${route.view}-${route.id}`} view={route.view} book={book} books={books} situations={situations} onNavigate={navigate} onOpen={open}/>}
    </div></main></div>
    <Sheet open={!!active} onOpenChange={value=>{if(!value)close();}}><SheetContent showCloseButton={false} className="signal-sheet work-preview-sheet" overlayClassName="signal-overlay">{active&&<SituationDrawer key={active.id} situation={active} book={books.find(b=>b.id===active.playbook)} readOnly={readOnly} onClose={close} onPatch={(values,event)=>{if(!readOnly)patch(active.id,values,event);}} onAction={(action,values)=>act(active,action,values)} onCustomer={()=>navigate('customer',active.company,undefined,active.id)} onBook={()=>navigate('playbook',active.playbook||undefined,'Customers')} onResource={resource}/>}</SheetContent></Sheet>
    <Dialog open={!!modal} onOpenChange={value=>{if(!value)setModal(null);}}><DialogContent className={`work-modal ${modal?.kind==='guide'?'work-guide':''}`}>
      {modal?.kind==='guide'&&<><DialogTitle>Explore the complete design</DialogTitle><DialogDescription>Connected desktop screens. Decisions and configuration changes use sample data only.</DialogDescription><div className="guide-groups">{['Daily work','Customer journeys','Manage the process','Shared automation'].map(group=><section key={group}><h3>{group}</h3>{guide.filter(item=>item.group===group).map(item=><ResourceLink key={item.title} title={item.title} meta={item.meta} onClick={()=>explore(item)}/>)}</section>)}</div></>}
      {modal?.kind==='resource'&&<><DialogTitle>{modal.title}</DialogTitle><DialogDescription className="resource-body">{modal.body}</DialogDescription><QuietTextAction onClick={()=>setModal(null)}>Back to work</QuietTextAction></>}
      {modal?.kind==='new'&&<><DialogTitle>Create a playbook</DialogTitle><DialogDescription>Start with a complete customer process, then configure it for your team.</DialogDescription><div>{playbookSeed.map(template=><ResourceLink key={template.id} title={template.name} meta={template.description} onClick={()=>create(template)}/>)}</div></>}
      {modal?.kind==='publish'&&<PublishPreview key={`${modal.book.id}-${modal.book.version}`} book={modal.book} activeCount={situations.filter(s=>s.playbook===modal.book.id&&s.status!=='Resolved').length} onCancel={()=>setModal(null)} onPublish={()=>publish(modal.book)}/>}
      {modal?.kind==='apply'&&<ApplyPreview book={modal.book} situations={situations} onCancel={()=>setModal(null)} onApply={company=>apply(modal.book,company)}/>}
      {modal?.kind==='decision'&&<DecisionPreview situation={modal.situation} action={modal.action} onCancel={()=>setModal(null)} onConfirm={(notes,owner,due,closeSituation,renewalProgress)=>finishDecision(modal.situation,modal.action,notes,owner,due,modal.patch,closeSituation,renewalProgress)}/>}
    </DialogContent></Dialog>
    <div className={`preview-toast ${notice?'visible':''}`} role="status">{notice}</div>
  </div></TooltipProvider>;
}

function PublishPreview({book,activeCount,onCancel,onPublish}:{book:Playbook;activeCount:number;onCancel:()=>void;onPublish:()=>void}) {
  const [tested,setTested]=useState(false);
  return <><DialogTitle>Review before publishing</DialogTitle><DialogDescription>{book.name} · Version {book.version+1}</DialogDescription><div className="modal-scroll"><Field label="Customer outcome"><span>{book.goal}</span></Field><Field label="Entry"><span>{book.trigger}</span></Field><Field label="Owner"><Person name={book.owner}/></Field><Field label="Approval"><span>{book.approval} · Exact response before sending</span></Field><Field label="Process"><span>{book.steps.length} milestones · Follow up after {book.delay}</span></Field><Field label="Stops when"><span>{book.stop}</span></Field><p className="work-small">{activeCount} active customer {activeCount===1?'process keeps':'processes keep'} the configuration they started with. This version applies to new enrollments.</p><QuietSection title="Sample check" className="flush-section" action={<QuietTextAction onClick={()=>setTested(true)}>{tested?'Run again':'Run sample check'}</QuietTextAction>}>{tested?<ul className="check-list"><li>Matching sample customer enters once.</li><li>Agent prepares a draft; sending waits for human approval.</li><li>A customer reply cancels the pending follow-up.</li><li>No task, ticket, or project is created by default.</li></ul>:<p className="work-prose">Walk through the proposed policy with sample customer data. No messages or real enrollments are created.</p>}</QuietSection></div><div className="modal-footer"><QuietTextAction onClick={onCancel}>Keep editing</QuietTextAction><QuietPrimaryAction disabled={!tested} onClick={onPublish}>Publish version {book.version+1}</QuietPrimaryAction></div></>;
}
function ApplyPreview({book,situations,onCancel,onApply}:{book:Playbook;situations:Situation[];onCancel:()=>void;onApply:(company:string)=>void}) {
  const eligible=customerNames.filter(company=>!situations.some(s=>s.company===company&&s.playbook===book.id&&s.status!=='Resolved'));
  const [company,setCompany]=useState(eligible.includes('Aster')?'Aster':eligible[0]||'');
  return <><DialogTitle>Apply to a customer</DialogTitle><DialogDescription>{book.name}</DialogDescription><Field label="Customer"><Picker label="Customer to enroll" value={company} options={eligible} onChange={setCompany}/></Field><Field label="Responsible"><Person name={ME}/></Field><Field label="Customer outcome"><span>{book.goal}</span></Field><Field label="First step"><span>Review the customer’s context and prepared next step.</span></Field><p className="work-small">Customers already active in this playbook are excluded. Applying it won’t send a message or create a PM task.</p><div className="modal-footer"><QuietTextAction onClick={onCancel}>Cancel</QuietTextAction><QuietPrimaryAction disabled={!company} onClick={()=>onApply(company)}>Apply playbook</QuietPrimaryAction></div></>;
}
function DecisionPreview({situation:s,action,onCancel,onConfirm}:{situation:Situation;action:SituationAction;onCancel:()=>void;onConfirm:(notes:string,owner:string,due:string,closeSituation:boolean,renewalProgress?:Situation['renewalProgress'])=>void}) {
  const [notes,setNotes]=useState(action==='task'?`Follow up on ${s.company}’s next step`:'');
  const [owner,setOwner]=useState(s.owner||ME);
  const [due,setDue]=useState('2026-09-08');
  const [blocker,setBlocker]=useState(s.renewalProgress?.blocker||'Not confirmed');
  const [renewal,setRenewal]=useState(s.renewalProgress?.renewal||'Still in progress');
  const renewalOutcome=action==='resolve'&&s.kind==='renewal';
  const confirm=()=>onConfirm(renewalOutcome?`Blocker: ${blocker}. Renewal: ${renewal}. ${notes}`:notes,owner,due,!renewalOutcome||renewal!=='Still in progress',renewalOutcome?{blocker,renewal}:undefined);
  const title=action==='pause'?'Pause this customer situation':action==='reject'?'Request a different next step':action==='resolve'?'Record the customer outcome':action==='task'?'Create a follow-up task':'Reconnect and send';
  const description=action==='pause'?'Scheduled actions stop here. Linked tasks and other customer situations stay unchanged.':action==='reject'?'Your feedback stays with this situation. The same proposal will not be sent automatically.':action==='resolve'?'Record what actually happened—not just that an agent finished running.':action==='task'?'Use a task for a specific commitment that needs its own owner or deadline.':'The previous attempt did not deliver a message. This preview simulates reconnecting your email and sending the approved response once.';
  return <>
    <DialogTitle>{title}</DialogTitle>
    <DialogDescription>{s.company} · {description}</DialogDescription>
    {renewalOutcome&&<>
      <Field label="Customer blocker"><Picker label="Blocker outcome" value={blocker} options={['Not confirmed','Customer confirmed resolution','Unresolved']} onChange={setBlocker}/></Field>
      <Field label="Renewal"><Picker label="Renewal outcome" value={renewal} options={['Still in progress','Customer confirmed renewal','Customer will not renew']} onChange={setRenewal}/></Field>
      <p className="work-small">Resolving the blocker does not complete the renewal. Keep this situation open until its commercial outcome is known.</p>
    </>}
    {action==='send'?<Field label="Email account"><span>Muhammad Azhar · Workspace email</span></Field>:<>
      <Field label={action==='task'?'Task name':action==='pause'?'Reason (optional)':action==='reject'?'What should change?':'Confirmed outcome'}>
        <textarea aria-label={action==='task'?'Task name':action==='resolve'?'Confirmed outcome':'Decision reason'} value={notes} onChange={e=>setNotes(e.target.value)} rows={3} placeholder={action==='resolve'?'Describe the result and the evidence that confirms it…':action==='reject'?'Tell the owner what needs to change…':'Add context…'}/>
      </Field>
      {action==='task'&&<><Field label="Owner"><Picker label="Task owner" value={owner} options={people} onChange={setOwner}/></Field><Field label="Due"><input aria-label="Task due date" type="date" value={due} onChange={e=>setDue(e.target.value)}/></Field><Field label="Related to"><span>{s.company} · {s.title}</span></Field></>}
    </>}
    <div className="modal-footer"><QuietTextAction onClick={onCancel}>Cancel</QuietTextAction><QuietPrimaryAction disabled={!['pause','send'].includes(action)&&(!notes.trim()||action==='task'&&!due)} onClick={confirm}>{action==='pause'?'Pause work':action==='reject'?'Request changes':action==='task'?'Create & link task':action==='resolve'?'Record outcome':'Simulate reconnect & send'}</QuietPrimaryAction></div>
  </>;
}
type GuideItem={group:string;title:string;meta:string;view:View;id?:string;tab?:string;situation?:string;filters?:Partial<QueueFilters>};
const guide:GuideItem[]=[
  {group:'Daily work',title:'Signals · My next actions',meta:'Scannable queue with visible filters',view:'signals',filters:defaultFilters},
  {group:'Daily work',title:'Approval queue',meta:'Review work without a separate inbox',view:'signals',filters:{state:'Needs approval',scope:'All'}},
  {group:'Daily work',title:'Review a CRM change',meta:'Cedar · Exact editable fields, with no playbook required',view:'signals',situation:'cedar-deal'},
  {group:'Daily work',title:'Waiting and completed work',meta:'Keep progress visible without cluttering today’s queue',view:'signals',filters:{state:'All work',scope:'All'}},
  {group:'Daily work',title:'Unassigned customer work',meta:'Find requests that need an owner',view:'signals',filters:{state:'All open',scope:'Unassigned'}},
  {group:'Customer journeys',title:'Buying-intent follow-up',meta:'Harbor · Edit and approve the exact response',view:'signals',situation:'harbor-buyer'},
  {group:'Customer journeys',title:'Sales-to-success handoff',meta:'Lumen · Accept goals, promises, and responsibility',view:'signals',situation:'lumen-handoff'},
  {group:'Customer journeys',title:'Renewal-risk recovery',meta:'Northstar · Relevant existing work, not duplicate tasks',view:'signals',situation:'northstar-risk'},
  {group:'Customer journeys',title:'Recover a failed delivery',meta:'Orbit · Know whether anything was sent',view:'signals',situation:'orbit-delivery'},
  {group:'Manage the process',title:'Playbooks catalogue',meta:'Ownership, enrollment, and attention at a glance',view:'playbooks'},
  {group:'Manage the process',title:'Playbook overview',meta:'Customer outcome, process, and operating rules',view:'playbook',id:'renewal-recovery'},
  {group:'Manage the process',title:'Configure and publish',meta:'Entry, steps, permissions, stop conditions, and preview',view:'edit',id:'renewal-recovery'},
  {group:'Manage the process',title:'Customer progress',meta:'Participating customers and their actual next steps',view:'playbook',id:'renewal-recovery',tab:'Customers'},
  {group:'Manage the process',title:'Confirmed outcomes',meta:'Customer results backed by confirmation',view:'playbook',id:'renewal-recovery',tab:'Outcomes'},
  {group:'Manage the process',title:'Connected customer record',meta:'Northstar · Two situations, one relationship',view:'customer',id:'Northstar'},
  {group:'Shared automation',title:'Connected flow',meta:'The execution behind a business-facing playbook',view:'flows',id:'renewal-recovery'},
  {group:'Shared automation',title:'CRM Assistant',meta:'One shared agent with controlled tool access',view:'agents'},
  {group:'Shared automation',title:'Execution activity',meta:'Run results are separate from customer outcomes',view:'activity'},
];
