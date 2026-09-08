import { useEffect, useRef, useState } from 'react';
import { QuietPageHeader, QuietSearchInput, QuietTextAction, QuietIconAction, QuietPrimaryAction } from '@/components/design-system/quiet';
import { Sheet, SheetContent } from '@/components/ui/sheet';
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { TooltipProvider } from '@/components/ui/tooltip';
import { HelpinLogo } from '@/components/layout/HelpinLogo';
import { ArrowRight01Icon, Cancel01Icon, ChartIncreaseIcon, UserGroupIcon, Mail01Icon, Search01Icon, Settings02Icon, BookOpen01Icon, CheckmarkCircle02Icon, Menu01Icon, WorkflowSquare01Icon } from '@/lib/icons';
import { categories, seed, type Signal } from './data';
import '@/index.css';
import './preview.css';
import { SignalDetail } from './SignalDetail';
import { signalCopy } from './presentation';

const me = 'Muhammad Azhar';
const scopes = ['Mine', 'My teams', 'Unassigned', 'All'];
type Scene = 'Populated' | 'Empty' | 'Loading' | 'Error' | 'Read-only';

function Picker({ label, value, options, onChange, disabled = false }: { label: string; value: string; options: string[]; onChange: (value: string) => void; disabled?: boolean }) {
  return <Select value={value} onValueChange={onChange} disabled={disabled} size="ui"><SelectTrigger aria-label={label} className="preview-picker"><SelectValue /></SelectTrigger><SelectContent>{options.map(option => <SelectItem key={option} value={option}>{option}</SelectItem>)}</SelectContent></Select>;
}

export function App() {
  const params = new URLSearchParams(location.search);
  const [signals, setSignals] = useState<Signal[]>(() => structuredClone(seed));
  const [scope, setScope] = useState(scopes.includes(params.get('scope') || '') ? params.get('scope')! : 'Mine');
  const [category, setCategory] = useState(['All categories', ...categories].includes(params.get('category') || '') ? params.get('category')! : 'All categories');
  const [state, setState] = useState(['Open', 'Handled', 'Dismissed', 'All states'].includes(params.get('state') || '') ? params.get('state')! : 'Open');
  const [query, setQuery] = useState(params.get('q') || '');
  const [review, setReview] = useState('Any review state');
  const [priority, setPriority] = useState('Any priority');
  const [selected, setSelected] = useState<string | null>(null);
  const [scene, setScene] = useState<Scene>('Populated');
  const [dark, setDark] = useState(false);
  const [navOpen, setNavOpen] = useState(false);
  const [notice, setNotice] = useState('');
  const [resource, setResource] = useState<{ title: string; body: string } | null>(null);
  const opener = useRef<HTMLButtonElement | null>(null);
  const active = signals.find(s => s.id === selected);
  const readOnly = scene === 'Read-only';

  useEffect(() => { document.documentElement.classList.toggle('dark', dark); }, [dark]);
  useEffect(() => {
    const url = new URL(location.href);
    for (const [key, value] of Object.entries({ scope, category, state, q: query })) {
      if (value) url.searchParams.set(key, value);
      else url.searchParams.delete(key);
    }
    history.replaceState(null, '', url);
  }, [scope, category, state, query]);
  useEffect(() => { if (!notice) return; const timer = window.setTimeout(() => setNotice(''), 5000); return () => window.clearTimeout(timer); }, [notice]);

  const inScope = (s: Signal, value = scope) => value === 'All' || (value === 'Mine' ? s.owner === me : value === 'Unassigned' ? !s.owner : [me, 'Sara Ahmed'].includes(s.owner));
  const matching = (s: Signal) => (state === 'All states' || s.state === state) && (review === 'Any review state' || (review === 'Reviewed' ? s.reviewed : !s.reviewed)) && (priority === 'Any priority' || (priority === 'High priority' ? s.priority >= 15 : s.priority < 15)) && `${s.company} ${s.title} ${signalCopy[s.id]?.title || ''} ${s.quote}`.toLowerCase().includes(query.toLowerCase());
  const base = scene === 'Empty' ? [] : signals.filter(s => inScope(s) && matching(s));
  const rows = base.filter(s => category === 'All categories' || s.category === category).sort((a, b) => b.priority - a.priority || a.id.localeCompare(b.id));
  const count = (c: string) => scene === 'Loading' || scene === 'Error' ? '—' : c === 'All categories' ? base.length : base.filter(s => s.category === c).length;
  const patch = (id: string, values: Partial<Signal>, event: string) => setSignals(old => old.map(s => s.id === id ? { ...s, ...values, history: [...s.history, event] } : s));
  const closeDetail = () => { setSelected(null); };
  const openDetail = (s: Signal, button: HTMLButtonElement) => { opener.current = button; setSelected(s.id); };
  const reset = () => { setSignals(structuredClone(seed)); setScope('Mine'); setCategory('All categories'); setState('Open'); setReview('Any review state'); setPriority('Any priority'); setQuery(''); setScene('Populated'); closeDetail(); setNotice('Sample data reset.'); };
  const placeholder = (title: string) => setResource({ title, body: 'This destination belongs to the existing Helpin application. Navigation is intentionally not connected in this design preview. No live records will be changed.' });

  return <TooltipProvider><div className="preview-root">
    <div className="preview-bar"><span><strong>Design preview</strong><span className="preview-disclaimer"> · Sample data · Changes stay in this tab</span></span><div className="preview-controls"><Picker label="Preview state" value={scene} options={['Populated', 'Empty', 'Loading', 'Error', 'Read-only']} onChange={v => setScene(v as Scene)} /><QuietTextAction onClick={() => setDark(!dark)}>{dark ? 'Light' : 'Dark'}</QuietTextAction><QuietTextAction onClick={reset}>Reset</QuietTextAction></div></div>
    <div className="app-frame">
      <aside className={`app-sidebar ${navOpen ? 'sidebar-visible' : ''}`} aria-label="Workspace navigation">
        <div className="brand-row"><HelpinLogo className="justify-start" imageClassName="size-7" /><QuietIconAction className="mobile-only" aria-label="Close navigation" onClick={() => setNavOpen(false)}><Cancel01Icon /></QuietIconAction></div>
        <button className="workspace-switch" onClick={() => placeholder('Workspace switcher')}><span className="workspace-monogram">H</span><span>Helpin workspace<small>CRM</small></span><span className="muted">⌄</span></button>
        <nav className="module-nav" aria-label="Modules">{[['CRM', ChartIncreaseIcon], ['Projects', WorkflowSquare01Icon], ['Support', Mail01Icon], ['Docs', BookOpen01Icon]].map(([name, Icon]) => { const Symbol = Icon as typeof Mail01Icon; return <button key={String(name)} aria-label={String(name)} title={String(name)} className={name === 'CRM' ? 'module-active' : ''} onClick={() => placeholder(String(name))}><Symbol className="size-[17px]" /></button>; })}</nav>
        <div className="sidebar-label">WORKSPACE</div>
        <nav aria-label="CRM">{[['Overview', ChartIncreaseIcon], ['Contacts', UserGroupIcon], ['Companies', WorkflowSquare01Icon], ['Deals', ChartIncreaseIcon], ['Signals', ChartIncreaseIcon], ['Review', CheckmarkCircle02Icon]].map(([name, Icon]) => { const Symbol = Icon as typeof Mail01Icon; return <button key={String(name)} className={`nav-item ${name === 'Signals' ? 'nav-selected' : ''}`} aria-current={name === 'Signals' ? 'page' : undefined} onClick={() => name === 'Signals' ? setNavOpen(false) : placeholder(String(name))}><Symbol className="size-4" />{String(name)}{name === 'Signals' && <span className="nav-count">{signals.filter(s => s.state === 'Open' && s.owner === me).length}</span>}</button>; })}</nav>
        <div className="sidebar-bottom"><div className="sidebar-utilities"><QuietIconAction aria-label="Pipelines" onClick={() => placeholder('Pipelines')}><WorkflowSquare01Icon /></QuietIconAction><QuietIconAction aria-label="Email accounts" onClick={() => placeholder('Email accounts')}><Mail01Icon /></QuietIconAction><QuietIconAction aria-label="CRM settings" onClick={() => placeholder('CRM settings')}><Settings02Icon /></QuietIconAction></div><button className="nav-item" onClick={() => placeholder('Workspace search')}><Search01Icon className="size-4" />Search<span className="nav-count">⌘ K</span></button><div className="profile"><span className="avatar">MA</span><span>Muhammad Azhar<small>Helpin workspace</small></span></div></div>
      </aside>
      <main className="signals-page">
        <div className="mobile-heading"><QuietIconAction aria-label="Open navigation" onClick={() => setNavOpen(true)}><Menu01Icon /></QuietIconAction><span>Helpin / CRM</span></div>
        <div className="page-topline"><span>CRM <span className="breadcrumb-slash">/</span> Signals</span><span>Sample snapshot · Sep 5, 2026</span></div>
        <QuietPageHeader title="Signals" description="Know what needs attention. Decide what happens next." actions={<QuietTextAction onClick={() => placeholder('Automation review')}>Review approvals <ArrowRight01Icon className="size-3.5" /></QuietTextAction>} />
        <nav className="category-nav" aria-label="Signal categories">{['All categories', ...categories].map(c => <button key={c} aria-pressed={category === c} onClick={() => setCategory(c)}>{c}<span>{count(c)}</span></button>)}</nav>
        <div className="queue-toolbar">
          <QuietSearchInput aria-label="Search signals" placeholder="Search customers or signals…" value={query} onChange={e => setQuery(e.target.value)} containerClassName="signal-search" />
          <div className="queue-filters">
            <Picker label="Assignment" value={scope} options={scopes} onChange={setScope} />
            <Picker label="Work state" value={state} options={['Open', 'Handled', 'Dismissed', 'All states']} onChange={setState} />
            <Picker label="Review state" value={review} options={['Any review state', 'Needs review', 'Reviewed']} onChange={setReview} />
            <Picker label="Priority" value={priority} options={['Any priority', 'High priority', 'Normal priority']} onChange={setPriority} />
          </div>
        </div>
        {readOnly && <p className="permission-note">Read-only access. You can inspect signals and evidence; decisions and assignment are unavailable.</p>}
        <div className="signal-table-wrap" aria-busy={scene === 'Loading'}>
          <table className="signals-grid" aria-label="Signal queue">
            <colgroup><col style={{ width: '44%' }} /><col style={{ width: '18%' }} /><col style={{ width: '13%' }} /><col style={{ width: '15%' }} /><col style={{ width: '10%' }} /></colgroup>
            <thead><tr>{['Signal', 'Customer', 'Category', 'Owner', 'Priority'].map(label => <th key={label} scope="col">{label}</th>)}</tr></thead>
            <tbody>
              {scene === 'Error' ? <tr><td colSpan={5}><div className="empty-state" role="alert"><h2>We couldn’t load signals</h2><p>Your filters are still here. Try loading the queue again.</p><QuietPrimaryAction onClick={() => setScene('Populated')}>Retry</QuietPrimaryAction></div></td></tr> : scene === 'Loading' ? <tr><td colSpan={5}><div className="loading-rows">{[1, 2, 3, 4].map(n => <div key={n}><i /><i /></div>)}</div></td></tr> : rows.length ? rows.map(s => <tr key={s.id} className="signal-table-row" data-testid="signal-row" onClick={e => { const button = e.currentTarget.querySelector('button'); if (button) openDetail(s, button); }}>
                <td><button className="table-signal-title" aria-label={`${signalCopy[s.id]?.title || s.title} · ${s.company}`}><strong>{signalCopy[s.id]?.title || s.title}</strong></button></td>
                <td className="table-customer">{s.company}</td>
                <td className="table-category">{s.category}</td>
                <td><span className="owner-cell">{s.owner ? <><span className="avatar avatar-small">{s.owner === me ? 'MA' : 'SA'}</span><span>{s.owner === me ? 'You' : s.owner}</span></> : <span className="muted">Unassigned</span>}</span></td>
                <td><span className={s.priority >= 15 ? 'risk-text priority-cell' : 'priority-cell muted'}><span className={`priority-mark ${s.priority >= 15 ? 'priority-high' : ''}`} aria-hidden="true">{s.priority >= 15 ? '↑' : '−'}</span>{s.priority >= 15 ? 'High' : 'Normal'}</span></td>
              </tr>) : <tr><td colSpan={5}><div className="empty-state"><CheckmarkCircle02Icon className="size-6 muted" /><h2>{query || category !== 'All categories' || review !== 'Any review state' || priority !== 'Any priority' ? 'No signals match these filters' : scope === 'Mine' && state === 'Open' ? 'No open signals assigned to you' : 'No signals in this view'}</h2><p>{scope === 'Mine' ? 'Check your team’s queue or find work that still needs an owner.' : 'Choose another category or clear the filters to see more signals.'}</p><div className="empty-actions"><QuietTextAction onClick={() => { setScope('My teams'); setScene('Populated'); }}>View my teams</QuietTextAction><QuietTextAction onClick={() => { setScope('Unassigned'); setScene('Populated'); }}>View unassigned</QuietTextAction><QuietTextAction onClick={() => { setQuery(''); setCategory('All categories'); setReview('Any review state'); setPriority('Any priority'); }}>Clear filters</QuietTextAction></div></div></td></tr>}
            </tbody>
          </table>
        </div>
        <div className="queue-footer"><span>{scene === 'Error' || scene === 'Loading' ? 'Counts are unavailable while the queue is not loaded.' : 'Each row is one signal. Decisions apply only to that signal.'}</span><QuietTextAction onClick={() => placeholder('Deal health')}>View deal health <ArrowRight01Icon className="size-3" /></QuietTextAction></div>
      </main>
    </div>
    <Sheet open={!!active} onOpenChange={open => { if (!open) closeDetail(); }}><SheetContent showCloseButton={false} className="signal-sheet" overlayClassName="signal-overlay" onCloseAutoFocus={event => { event.preventDefault(); opener.current?.focus(); }}>
      {active && <SignalDetail key={active.id} signal={active} readOnly={readOnly} onClose={closeDetail}
        onUpdate={(values, event) => patch(active.id, values, event)}
        onNotice={setNotice} onResource={setResource}
        onCompany={() => placeholder(`${active.company} · Company record`)} />}
    </SheetContent></Sheet>
    <Dialog open={!!resource} onOpenChange={open => { if (!open) setResource(null); }}><DialogContent><DialogTitle className="pr-7">{resource?.title}</DialogTitle><DialogDescription className="whitespace-pre-line leading-relaxed">{resource?.body}</DialogDescription></DialogContent></Dialog>
    <div className={`preview-toast ${notice ? 'visible' : ''}`} role="status">{notice}</div>
  </div></TooltipProvider>;
}
