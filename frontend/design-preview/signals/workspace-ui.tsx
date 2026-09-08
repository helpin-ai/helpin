import type { ReactNode } from 'react';
import { QuietEmptyState, QuietPrimaryAction, QuietSection, QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ArrowRight01Icon, CheckmarkCircle02Icon } from '@/lib/icons';
import { ME, ownerName, type Scene, type Step } from './workspace-data';

export function Picker({ label, value, options, onChange, disabled }: { label: string; value: string; options: string[]; onChange: (value: string) => void; disabled?: boolean }) {
  return <Select value={value} onValueChange={onChange} disabled={disabled} size="ui"><SelectTrigger aria-label={label} className="preview-picker"><SelectValue /></SelectTrigger><SelectContent>{options.map(o => <SelectItem key={o} value={o}>{o}</SelectItem>)}</SelectContent></Select>;
}
export function Person({ name }: { name: string }) {
  return <span className="owner-cell">{name && <span className={`avatar avatar-small ${name === ME ? 'avatar-you' : 'avatar-other'}`}>{name.split(' ').map(w=>w[0]).slice(0,2).join('')}</span>}<span>{ownerName(name)}</span></span>;
}
export function LineTabs({ value, onChange, items, label }: { value: string; onChange: (v: string) => void; items: { value: string; label: string; count?: number | string }[]; label: string }) {
  return <Tabs value={value} onValueChange={onChange} className="workspace-tabs"><TabsList variant="quiet" aria-label={label}>{items.map(item=><TabsTrigger key={item.value} value={item.value}>{item.label}{item.count !== undefined && <span className="tab-count">{item.count}</span>}</TabsTrigger>)}</TabsList></Tabs>;
}
export function Field({ label, children, hint }: { label: string; children: ReactNode; hint?: string }) {
  return <div className="work-field"><div><span className="field-label">{label}</span>{hint && <small>{hint}</small>}</div><div className="field-value">{children}</div></div>;
}
export function TextField({ label, value, onChange, disabled }: { label: string; value: string; onChange: (v:string)=>void; disabled?: boolean }) {
  return <QuietUnderlineInput aria-label={label} value={value} onChange={e=>onChange(e.target.value)} disabled={disabled} />;
}
export function StateText({ value }: { value: string }) {
  const tone = /failed|attention|approval|held|Missing|Paused/.test(value) ? 'text-quiet-accent' : /Resolved|Enabled|Confirmed|Succeeded|Accepted/.test(value) ? 'text-quiet-positive' : 'text-quiet-text-tertiary';
  return <span className={`work-state ${tone}`}>{value}</span>;
}
export function Progress({ steps, progress, onStep }: { steps: Step[]; progress: number; onStep?: (index: number)=>void }) {
  return <ol className="process-steps">{steps.map((step,i)=><li key={`${step.name}-${i}`} className={i<progress ? 'step-done' : i===progress ? 'step-current' : ''}><span className="step-marker">{i<progress ? <CheckmarkCircle02Icon className="size-3.5" /> : i+1}</span><div className="step-body"><div className="step-title-line"><strong>{step.name}</strong><span>{step.actor}</span>{onStep && <QuietTextAction onClick={()=>onStep(i)} aria-label={`Edit ${step.name}`}>Edit</QuietTextAction>}</div><p>{step.detail}</p></div></li>)}</ol>;
}
export function ResourceLink({ title, meta, onClick }: { title: string; meta?: string; onClick: ()=>void }) {
  return <button className="work-resource" onClick={onClick}><span>{title}{meta && <small>{meta}</small>}</span><ArrowRight01Icon className="size-3.5" /></button>;
}
export function SurfaceState({ scene, onRetry, emptyTitle='No work matches this view', children }: { scene: Scene; onRetry: ()=>void; emptyTitle?: string; children: ReactNode }) {
  if(scene==='Loading') return <div className="work-loading" aria-label="Loading preview">{[0,1,2,3].map(n=><div key={n}><i /><i /></div>)}</div>;
  if(scene==='Error') return <QuietEmptyState title="We couldn’t load this view" description="Your work hasn’t changed. Try loading it again." action={<QuietPrimaryAction onClick={onRetry}>Try again</QuietPrimaryAction>} />;
  if(scene==='Empty') return <QuietEmptyState title={emptyTitle} description="There are no matching items in this sample view." action={<QuietTextAction onClick={onRetry}>Show sample data <ArrowRight01Icon className="size-3.5" /></QuietTextAction>} />;
  return <>{scene==='Read-only' && <p className="read-only-note">You can view this workspace. Editing requires permission.</p>}{children}</>;
}
export { QuietSection };
