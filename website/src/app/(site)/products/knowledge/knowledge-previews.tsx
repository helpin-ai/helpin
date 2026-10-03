'use client';

import '../../_components/product-previews/preview-navigation.css';

import { useEffect, useId, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { ArrowLeft, ArrowUpRight, BookOpen, Braces, Check, ChevronDown, ChevronRight, CircleHelp, Code2, Copy, FileText, KeyRound, LayoutGrid, LifeBuoy, Link2, ListChecks, MessageSquare, Pause, Play, Plus, Rocket, Search, Settings, Shield, Users, Wallet, Zap } from 'lucide-react';
import { useWorkflowPlayback } from '../../_components/useWorkflowPlayback';
import { StreamingText } from '../../_components/StreamingText';
import { WorkflowClick } from '../../_components/WorkflowParts';
import './knowledge-previews.css';

// Isolated presentational versions of the platform's CollectionCard / DocumentsTable
// and help-center TopBar / ArticleShell / APIRequestPanel. Demo actions are local;
// these previews never use app authentication, customer data, or live API requests.
function useKnowledgePlayback(autoplay = true) {
  const [paused, setPaused] = useState(false);
  const flow = useWorkflowPlayback({beats: [0,2200,4800,7600,11000], duration: 20000, paused: paused || !autoplay});
  return { ...flow, active: autoplay && flow.playing, phase: autoplay ? flow.phase : 4, paused, setPaused };
}

function Playback({ paused, onClick, name }: { paused: boolean; onClick: () => void; name: string }) {
  return <button type="button" className="kp-playback" onClick={onClick} aria-label={`${paused ? 'Play' : 'Pause'} ${name} animation`} aria-pressed={paused}>{paused ? <Play size={13} /> : <Pause size={13} />}</button>;
}
function Mark() { return <span className="kp-mark" aria-hidden="true">O</span>; }
const COLLECTIONS = [
  { title: 'Getting Started', count: 8, Icon: Rocket },
  { title: 'Workspace & Team', count: 6, Icon: Users },
  { title: 'Security & SSO', count: 7, Icon: Shield },
  { title: 'Projects & Tasks', count: 5, Icon: ListChecks },
  { title: 'Integrations', count: 6, Icon: Link2 },
  { title: 'AI & Automation', count: 5, Icon: Zap },
  { title: 'Billing & Plans', count: 7, Icon: Wallet },
  { title: 'Troubleshooting', count: 4, Icon: LifeBuoy },
];
const DOCUMENTS = [
  ['Set up SSO with Okta', 'Security & SSO', '1 hour ago'],
  ['Map groups to workspace roles', 'Security & SSO', '2 hours ago'],
  ['Invite your team', 'Workspace & Team', 'yesterday'],
  ['Connect your first integration', 'Getting Started', 'yesterday'],
  ['Configure SCIM provisioning', 'Security & SSO', 'yesterday'],
  ['Connect your GitHub repository', 'Integrations', '2 days ago'],
  ['Choose when agents need approval', 'AI & Automation', '2 days ago'],
  ['Understand workspace permissions', 'Workspace & Team', '3 days ago'],
  ['Create your first project', 'Projects & Tasks', '4 days ago'],
  ['Manage your subscription', 'Billing & Plans', '4 days ago'],
  ['Resolve a failed sync', 'Troubleshooting', '5 days ago'],
];

export function KnowledgeWorkspace({ apiHref = '#knowledge-api', autoplay = true }: { apiHref?: string; autoplay?: boolean }) {
  const { container, active, phase, paused, setPaused } = useKnowledgePlayback(autoplay);
  const [collection, setCollection] = useState('');
  const [manualBrowse, setManualBrowse] = useState(false);
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState('recent');
  const searchId = useId();
  const visibleCollection = collection || (!manualBrowse && phase >= 2 && phase < 4 ? COLLECTIONS[2].title : '');
  const filtered = DOCUMENTS.filter(([title, group]) => (!visibleCollection || visibleCollection === group) && title.toLowerCase().includes(query.toLowerCase()));
  if (sort === 'title') filtered.sort((a, b) => a[0].localeCompare(b[0]));
  const interact = () => { setPaused(true); setManualBrowse(true); };
  return <div ref={container} className="kp-preview kp-workspace" data-playing={active} data-phase={phase} role="region" aria-label="OrbitDesk Knowledge workspace preview">
    <div className="kp-workspace-nav preview-sidebar">
      <div className="kp-workspace-brand"><Mark /><strong>OrbitDesk</strong><ChevronDown size={12} /></div>
      <div className="kp-nav-columns"><div className="kp-app-rail" aria-hidden="true">{[[LayoutGrid, 'Projects'], [MessageSquare, 'Support'], [BookOpen, 'Docs'], [Users, 'CRM'], [Zap, 'Automate'], [Settings, 'Settings']].map(([Icon, label]) => { const I = Icon as typeof BookOpen; return <span key={String(label)} data-selected={label === 'Docs'}><I size={17} /><small>{String(label)}</small></span>; })}</div>
        <div className="kp-space-nav"><div className="kp-space-title"><span>HC</span><strong>Help Center<small>48 docs · external</small></strong></div><span className="kp-dark-action"><Plus size={12} />New document<ChevronDown size={11} /></span><div className="kp-nav-muted"><FileText size={12} />Recent docs</div><div className="kp-nav-muted"><FileText size={12} />My documents</div><button type="button" className="kp-nav-all" data-selected={!visibleCollection} aria-pressed={!visibleCollection} onClick={() => { interact(); setCollection(''); setQuery(''); }}>All docs in space <span>48</span></button><p className="kp-caption">IN THIS SPACE</p>{COLLECTIONS.map(({ title, count, Icon }) => <button type="button" key={title} data-selected={visibleCollection === title} aria-pressed={visibleCollection === title} onClick={() => { interact(); setCollection(title); }}><Icon size={12} /><span>{title}</span><small>{count}</small></button>)}</div></div>
      <div className="kp-user"><img src="/new/avatars/sam.webp" width={25} height={25} alt="" /><span>Sam Rivera<small>sam@orbitdesk.example</small></span></div>
    </div>
    <div className="kp-library">
      <div className="kp-library-top"><span><ArrowLeft size={13} />OrbitDesk · Knowledge</span><Playback name="library" paused={paused} onClick={() => { if (paused) { setCollection(''); setQuery(''); setManualBrowse(false); } setPaused(!paused); }} /></div>
      <div className="kp-library-title"><h3>Help your customers get started.</h3><div aria-hidden="true"><span><Plus size={12} />Collection</span><span className="kp-dark-action"><Plus size={12} />Document</span></div></div>
      <p className="kp-library-description">Setup guides, troubleshooting, and answers to everyday questions.</p><div className="kp-reference-strip"><div><strong>Building with our API?</strong><p>Explore the endpoints, check the examples, and try a request.</p></div><a href={apiHref}><Code2 size={12} />Open API reference →</a><small>Import an OpenAPI URL or file to add an interactive API reference beneath your help-center header.</small></div>
      <div className="kp-collections">{COLLECTIONS.map(({ title, count, Icon }, i) => <button type="button" key={title} className="kp-collection" aria-pressed={visibleCollection === title} data-selected={visibleCollection === title} data-highlight={!collection && !query && phase === 1 && i === 2} onClick={() => { interact(); setCollection(collection === title ? '' : title); }}><Icon size={18} /><span><strong>{title}</strong><small>{count} docs</small></span>{phase === 1 && i === 2 && active && <WorkflowClick delay={900}/>}</button>)}</div>
      <div className="kp-doc-toolbar"><label htmlFor={searchId}><Search size={14} /><input id={searchId} aria-label="Search documents" value={query} placeholder="Find a guide…" onChange={e => { interact(); setQuery(e.target.value); }} /></label><span>{visibleCollection || '48 documents'}</span><select aria-label="Sort documents" value={sort} onChange={e => { interact(); setSort(e.target.value); }}><option value="recent">Last updated</option><option value="title">Title A–Z</option></select></div>
      <div className="kp-document-list">{filtered.slice(0, 7).map(([title, group, updated], i) => <div className="kp-document" key={title} data-highlight={!query && !collection && phase >= 2 && phase <= 3 && i === 0}><FileText size={17} /><div><strong>{title}</strong><small>{group}</small></div><span>Updated {updated}</span><Check className="kp-doc-check" size={13} aria-label="Published" /></div>)}{filtered.length === 0 && <p className="kp-empty">No documents match your search.</p>}</div>
      <div className="kp-library-bottom"><BookOpen size={12} /><span>{collection || query ? `${filtered.length} matching demo documents` : 'Published guides, organized in one place'}</span><span className="kp-library-status" data-highlight={phase === 4}><Check size={12} />Public help center</span></div>
    </div>
  </div>;
}

function PublicHeader({ api = false, children }: { api?: boolean; children?: ReactNode }) {
  return <div className="kp-public-header"><div className="kp-public-brand"><Mark /><strong>OrbitDesk</strong></div><div className="kp-public-tabs"><a href="#knowledge-library" aria-current={!api ? 'page' : undefined}>Help guides</a><a href="#knowledge-api" aria-current={api ? 'page' : undefined}>API reference</a></div>{children}<span className="kp-website-label" aria-hidden="true">Go to Website<ArrowUpRight size={12} /></span></div>;
}
const ARTICLE_STEPS = [
  ['Choose the tool.', 'Open Settings → Integrations and select the service you want to connect.'],
  ['Review access.', 'Sign in to the service and review the permissions requested.'],
  ['Confirm the destination.', 'Choose the OrbitDesk workspace that should receive the activity.'],
  ['Start syncing.', 'Start the sync, then check the connection status in Integrations.'],
];
const ARTICLES = ['What is OrbitDesk?', 'Connect your first integration', 'Invite your team', 'Workspace permissions', 'Import customer records', 'Export customer data'];
export function KnowledgeReader() {
  const { container, active, phase, paused, setPaused } = useKnowledgePlayback();
  const [selected, setSelected] = useState<number | null>(null);
  const [query, setQuery] = useState('');
  const searchInput = useRef<HTMLInputElement>(null);
  const current = selected ?? (phase >= 2 ? 2 : 0);
  const choose = (index: number) => { setPaused(true); setSelected(index); };
  const results = ARTICLE_STEPS.map(([title], index) => ({ title, index })).filter(({ title }) => title.toLowerCase().includes(query.toLowerCase()));
  return <div ref={container} className="kp-preview kp-reader" data-playing={active} data-phase={phase} role="region" aria-label="OrbitDesk public help center preview">
    <PublicHeader><label className="kp-public-search"><Search size={14} /><input ref={searchInput} aria-label="Find a step in this guide" placeholder="What are you trying to do?" value={query} onKeyDown={e => { if (e.key === 'Escape') setQuery(''); }} onChange={e => { setPaused(true); setQuery(e.target.value); }} /></label></PublicHeader>
    <div className="kp-reader-body"><aside className="kp-reader-nav" aria-label="Example help center navigation"><strong>New to OrbitDesk?<ChevronDown size={12} /></strong><strong>Getting Started<ChevronDown size={12} /></strong>{ARTICLES.map((title, i) => <span key={title} data-selected={i === 1}>{title}</span>)}{['Integrations & API', 'Projects & tasks', 'AI agents', 'Troubleshooting'].map(title => <strong key={title}>{title}<ChevronRight size={12} /></strong>)}</aside>
      <article className="kp-article"><div className="kp-article-eyebrow">Getting Started<Playback name="reader" paused={paused} onClick={() => { setSelected(null); setPaused(!paused); }} /></div><h3>Connect your first integration</h3><p className="kp-article-intro">Bring activity from another tool into OrbitDesk. Choose the connection, check its access, and confirm where the data should go.</p>
        {query && <div className="kp-guide-results"><strong>Steps in this guide</strong>{results.length ? results.map(({ title, index }) => <button type="button" key={title} onClick={() => { choose(index); setQuery(''); searchInput.current?.focus({ preventScroll: true }); }}>{title}<ArrowUpRight size={12} /></button>) : <p>No matching steps. Try “destination” or “sync”.</p>}</div>}
        <div className="kp-article-steps">{ARTICLE_STEPS.map(([title, body], i) => <div className="kp-article-step" key={title} data-selected={current === i}><h4>{title}</h4><p>{body}</p>{i === 2 && <div className="kp-callout"><CircleHelp size={15} /><span>Check the workspace before starting the sync.</span></div>}</div>)}</div>
      </article><aside className="kp-toc" aria-label="Steps in this guide"><strong>In this guide</strong>{ARTICLE_STEPS.map(([title], i) => <button type="button" key={title} aria-current={current === i ? 'step' : undefined} onClick={() => choose(i)}>{title}</button>)}<div className="kp-reader-note"><BookOpen size={16} /><span>One guide.<br />A clear next step.</span></div></aside>
    </div>
    {selected === null && !query && phase >= 1 && <div className="kp-reader-ai" data-phase={phase}><header><img src="/brand/helpin-icon-white.svg" className="kp-reader-ai-mark" width={25} height={25} alt=""/><strong>Helpin AI</strong><span>{phase < 3 ? 'Reading the guide…' : 'Answer ready'}</span></header><div className="kp-reader-question">Which workspace will receive the data?</div><div className="kp-reader-source"><BookOpen size={14}/><div><strong>Connect your first integration</strong><span>Confirm the destination · Step 3</span></div>{phase >= 2 && <Check size={13}/>}</div>{phase >= 3 && <div className="kp-reader-answer"><StreamingText text="Choose the destination workspace after authorizing the integration, before you start the first sync. That workspace will receive the activity." active={active && phase === 3} duration={2000}/><span><BookOpen size={12}/>Confirm the destination ↗</span></div>}<div className="kp-reader-input">Ask a follow-up…<ArrowUpRight size={13}/></div></div>}
  </div>;
}

const API_LANGUAGES = ['cURL', 'JavaScript', 'Python'] as const;
type Language = typeof API_LANGUAGES[number];
function codeSample(language: Language, page: string, limit: string) {
  const url = `https://api.orbitdesk.example/v1/integrations?page=${page || '1'}&limit=${limit || '20'}`;
  if (language === 'JavaScript') return `const response = await fetch(\n  "${url}",\n  { headers: {\n    Authorization: "Bearer YOUR_API_KEY"\n  }}\n);\nconst data = await response.json();`;
  if (language === 'Python') return `import requests\n\nresponse = requests.get(\n  "${url}",\n  headers={\n    "Authorization": "Bearer YOUR_API_KEY"\n  }\n)\ndata = response.json()`;
  return `curl "${url}" \\\n  -H "Authorization: Bearer YOUR_API_KEY"`;
}
const EXAMPLE_RESPONSE = '{\n  "data": [\n    {\n      "id": "int_042",\n      "provider": "slack",\n      "status": "connected"\n    }\n  ]\n}';
export function KnowledgeAPI() {
  const { container, active, phase, paused, setPaused } = useKnowledgePlayback();
  const [language, setLanguage] = useState<Language>('cURL');
  const [page, setPage] = useState('1');
  const [limit, setLimit] = useState('20');
  const [response, setResponse] = useState(false);
  const [announcement, setAnnouncement] = useState('');
  const [copied, setCopied] = useState(false);
  const languageId = useId();
  const copyReference = async () => {
    setPaused(true);
    try {
      await navigator.clipboard.writeText('OrbitDesk API example · v1.0.0, OpenAPI 3.1.0\nGET /v1/integrations\nBearer authentication required. Query parameters: page (integer, default 1), limit (integer, default 20).\n\n' + codeSample('cURL', page, limit) + '\n\nExample response:\n' + EXAMPLE_RESPONSE);
      setCopied(true);
      setAnnouncement('API example copied for your AI tool.');
    } catch {
      setAnnouncement('Copy is unavailable in this browser. You can select the request example below.');
    }
  };
  const shown = response || (!paused && phase >= 3);
  const interact = () => { setPaused(true); setResponse(false); };
  const reset = () => { interact(); setPage('1'); setLimit('20'); setLanguage('cURL'); setResponse(false); setAnnouncement('Demo request reset.'); };
  return <div ref={container} className="kp-preview kp-api" data-playing={active} data-phase={phase} role="region" aria-label="OrbitDesk interactive API reference preview">
    <PublicHeader api><span className="kp-public-search kp-search-decoration" aria-hidden="true"><Search size={14} />Search for articles…<span>⌘ K</span></span></PublicHeader>
    <div className="kp-api-body"><aside className="kp-api-nav" aria-label="Example endpoint navigation"><p className="kp-caption">API REFERENCES</p><strong><Braces size={14} />OrbitDesk API</strong><p className="kp-caption">ON THIS PAGE</p><span>Overview</span><span>Authentication</span>{[['INTEGRATIONS', 'List integrations', 'Create integration', 'Get integration', 'Update integration'], ['WORKSPACES', 'List workspaces', 'Get workspace'], ['CUSTOMERS', 'List customers', 'Create customer']].map(([group, ...items]) => <div key={group}><p className="kp-caption">{group}</p>{items.map((title) => <span key={title} data-selected={title === 'List integrations'}><small data-method={title.startsWith('Create') ? 'post' : title.startsWith('Update') ? 'patch' : 'get'}>{title.startsWith('Create') ? 'POST' : title.startsWith('Update') ? 'PATCH' : 'GET'}</small>{title}</span>)}</div>)}</aside>
      <article className="kp-api-content"><div className="kp-api-versions"><span>v1.0.0</span><span>OpenAPI 3.1.0</span><button type="button" className="kp-copy-ai" onClick={copyReference}>{copied ? <Check size={12} /> : <Copy size={12} />}{copied ? 'Copied' : 'Copy for AI'}</button><Playback name="API" paused={paused} onClick={() => { setResponse(false); setPaused(!paused); }} /></div><h3>OrbitDesk API</h3><p>Connect integrations, manage workspaces, and keep customer records in sync.</p><div className="kp-base-url"><strong>Base URLs</strong><code>https://api.orbitdesk.example</code><small>OrbitDesk API</small></div><h4><KeyRound size={16} />Authenticate your requests.</h4><div className="kp-auth"><strong>BearerAuth <small>BEARER</small></strong><p>Include your API key as a bearer token in the request header.</p></div><h4 className="kp-endpoint-heading"><span className="kp-method">GET</span>List integrations</h4><div className="kp-endpoint-path"><code>/v1/integrations</code></div><p>List the integrations connected to your workspace.</p><h4>Parameters</h4><table className="kp-parameter-table"><thead><tr><th>Name</th><th>Type</th><th>Description</th></tr></thead><tbody><tr><td>page</td><td>integer</td><td>Page number. Default: 1.</td></tr><tr><td>limit</td><td>integer</td><td>Results per page. Default: 20.</td></tr></tbody></table></article>
      <div className="kp-request"><div className="kp-request-heading"><span className="kp-method">GET</span><code>/v1/integrations</code></div><div className="kp-request-title"><strong>Build your request.</strong><button type="button" onClick={reset}>Reset</button></div><div className="kp-readonly-field">Server<code>https://api.orbitdesk.example</code></div><div className="kp-readonly-field">Authentication<span><KeyRound size={12} />Bearer YOUR_API_KEY</span></div><form onSubmit={e => { e.preventDefault(); interact(); setResponse(true); setAnnouncement('Example response ready: 200 OK. Slack integration connected.'); }}><p className="kp-caption">PARAMETERS</p><label className="kp-param-input">Page<input type="number" min="1" max="100" required value={page} onChange={e => { interact(); setPage(e.target.value); }} /><small>QUERY</small></label><label className="kp-param-input">Results per page<input type="number" min="1" max="100" required value={limit} onChange={e => { interact(); setLimit(e.target.value); }} /><small>QUERY</small></label><button type="submit" className="kp-send" data-highlight={active && phase === 2}><Play size={12} />{response ? 'Run demo again' : 'Run demo request'}{active && phase === 2 && <WorkflowClick delay={1000}/>}</button></form>
        <div className="kp-code"><div className="kp-code-tabs" role="tablist" aria-label="Request language">{API_LANGUAGES.map((item, i) => <button key={item} type="button" role="tab" id={`${languageId}-${i}`} aria-controls={`${languageId}-code`} aria-selected={language === item} tabIndex={language === item ? 0 : -1} onClick={() => { interact(); setLanguage(item); }} onKeyDown={e => { if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return; e.preventDefault(); const next = e.key === 'Home' ? 0 : e.key === 'End' ? 2 : (i + (e.key === 'ArrowRight' ? 1 : 2)) % 3; interact(); setLanguage(API_LANGUAGES[next]); document.getElementById(`${languageId}-${next}`)?.focus(); }}>{item}</button>)}</div><pre id={`${languageId}-code`} role="tabpanel" aria-labelledby={`${languageId}-${API_LANGUAGES.indexOf(language)}`} tabIndex={0}><code>{codeSample(language, page, limit)}</code></pre></div>
        <div className="kp-response-title"><strong>Example response</strong><span data-ready={shown}>{shown ? '200 OK' : 'Ready to run'}</span></div><pre className="kp-response" data-ready={shown}><code>{shown ? EXAMPLE_RESPONSE : 'Run the request to see the response.'}</code></pre><span className="kp-sr-only" role="status">{announcement}</span>
      </div>
    </div>
  </div>;
}
