import { useState } from 'react';
import { QuietEmptyState, QuietPageHeader, QuietSearchInput, QuietTextAction } from '@/components/design-system/quiet';
import { ArrowRight01Icon } from '@/lib/icons';
import { ME, isAttention, motions, type Scene, type Situation } from './workspace-data';
import { LineTabs, Person, Picker, SurfaceState } from './workspace-ui';

export type QueueFilters = { scope: string; state: string; category: string };
export function SignalsWorkspace({ situations, filters, setFilters, onOpen, onPlaybooks, scene, onRetry }: {
  situations: Situation[]; filters: QueueFilters; setFilters: (patch: Partial<QueueFilters>)=>void;
  onOpen: (id:string)=>void; onPlaybooks: ()=>void; scene: Scene; onRetry: ()=>void;
}) {
  const [query,setQuery]=useState('');
  const [review,setReview]=useState('Any review state');
  const [priority,setPriority]=useState('Any priority');
  const inScope=(s:Situation)=>filters.scope==='All' || (filters.scope==='Mine' ? s.owner===ME || s.actionOwner===ME : filters.scope==='Unassigned' ? !s.owner || (isAttention(s)&&!s.actionOwner) : true);
  const stateMatches=(s:Situation)=>filters.state==='All work' || (filters.state==='Needs attention' ? isAttention(s) && (filters.scope!=='Mine' || s.actionOwner===ME || (!s.actionOwner&&s.owner===ME)) : filters.state==='All open' ? s.status!=='Resolved' : s.status===filters.state);
  const base=situations.filter(s=>inScope(s)&&stateMatches(s)&&(review==='Any review state'||s.reviewed===(review==='Reviewed'))&&(priority==='Any priority'||s.priority===priority)&&`${s.company} ${s.title} ${s.next} ${s.quote}`.toLowerCase().includes(query.toLowerCase()));
  const rows=base.filter(s=>filters.category==='All categories'||s.category===filters.category).sort((a,b)=>Number(b.priority==='High')-Number(a.priority==='High'));
  const count=(category:string)=>scene==='Error'||scene==='Loading'?'—':scene==='Empty'?0:category==='All categories'?base.length:base.filter(s=>s.category===category).length;
  return <>
    <QuietPageHeader title="Signals" description="The customer work that needs a human next step." actions={<QuietTextAction onClick={onPlaybooks}>Manage playbooks <ArrowRight01Icon className="size-3.5" /></QuietTextAction>} />
    <LineTabs label="Signal categories" value={filters.category} onChange={category=>setFilters({category})} items={['All categories',...motions].map(c=>({value:c,label:c==='All categories'?'All':c,count:count(c)}))} />
    <div className="queue-toolbar"><QuietSearchInput aria-label="Search signals" placeholder="Search customers or situations…" value={query} onChange={e=>setQuery(e.target.value)} containerClassName="signal-search" /><div className="queue-filters">
      <Picker label="Assignment" value={filters.scope} options={['Mine','My teams','Unassigned','All']} onChange={scope=>setFilters({scope})} />
      <Picker label="Work state" value={filters.state} options={['Needs attention','Needs approval','All open','Waiting','Paused','Resolved','Delivery failed','All work']} onChange={state=>setFilters({state})} />
      <Picker label="Evidence review" value={review} options={['Any review state','Needs review','Reviewed']} onChange={setReview} />
      <Picker label="Priority" value={priority} options={['Any priority','High','Normal']} onChange={setPriority} />
    </div></div>
    <SurfaceState scene={scene} onRetry={onRetry}><div className="signal-table-wrap"><table className="signals-grid work-signals-table" aria-label="Customer situations"><colgroup><col style={{width:'43%'}}/><col style={{width:'16%'}}/><col style={{width:'12%'}}/><col style={{width:'19%'}}/><col style={{width:'10%'}}/></colgroup><thead><tr>{['Signal','Customer','Category','Owner','Priority'].map(h=><th key={h}>{h}</th>)}</tr></thead><tbody>
      {rows.map(s=><tr key={s.id} className="signal-table-row" data-testid="situation-row" onClick={()=>onOpen(s.id)}><td><button className="table-signal-title" onClick={e=>{e.stopPropagation();onOpen(s.id);}}><strong>{s.title}</strong></button><span className={`work-next ${s.status==='Delivery failed'?'text-quiet-accent':''}`}>{s.next}</span></td><td><button className="table-company" onClick={e=>{e.stopPropagation();onOpen(s.id);}}>{s.company}</button></td><td className="table-category">{s.category}</td><td><Person name={s.owner}/></td><td><span className={s.priority==='High'?'priority-cell text-quiet-accent':'priority-cell muted'}><span className="priority-mark" aria-hidden="true">{s.priority==='High'?'↑':'−'}</span>{s.priority}</span></td></tr>)}
    </tbody></table>{!rows.length&&<QuietEmptyState title="Nothing needs your attention in this view" description="Waiting work and completed outcomes are still available. You can also check your team’s queue." action={<div className="flex gap-5"><QuietTextAction onClick={()=>{setFilters({scope:'All',state:'All work',category:'All categories'});setQuery('');setReview('Any review state');setPriority('Any priority');}}>View all work</QuietTextAction><QuietTextAction onClick={()=>setFilters({scope:'Unassigned',state:'All open',category:'All categories'})}>View unassigned</QuietTextAction></div>} />}</div></SurfaceState>
  </>;
}
