'use client';

import { useId, useRef, useState, type KeyboardEvent } from 'react';
import { Code2, Plug, Terminal } from 'lucide-react';
import { GITHUB_URL } from './ui';

const EXAMPLES = [
  {
    id: 'cli', label: 'CLI', icon: Terminal, title: 'Set up and manage your instance.',
    description: 'Install with guided setup. Configure your instance, manage services, inspect logs, and check readiness with the Helpin CLI.',
    filename: 'helpin · local setup', note: 'Example session with the Helpin CLI installed.',
    lines: [
      ['$ helpin install', 'command'],
      ['✓ Docker, Compose, and memory checks passed', 'success'],
      ['✓ Release downloaded and verified', 'success'],
      ['✓ Helpin services are ready', 'success'],
      ['', 'plain'],
      ['$ helpin status', 'command'],
      ['$ helpin logs', 'command'],
      ['$ helpin doctor', 'command'],
    ],
    href: `${GITHUB_URL}/blob/develop/community/README.md`, link: 'Read the installation guide',
  },
  {
    id: 'mcp', label: 'MCP', icon: Plug, title: 'Bring Helpin into your AI tools.',
    description: 'Connect an MCP client to a Helpin workspace. Choose the scopes and tools it can use, then work with your customer and product context.',
    filename: 'mcp.json', note: 'Connect your client, sign in, and choose a workspace.',
    lines: [
      ['{', 'plain'], ['  "mcpServers": {', 'plain'], ['    "helpin": {', 'plain'],
      ['      "url": "https://mcp.helpin.ai/mcp"', 'success'],
      ['    }', 'plain'], ['  }', 'plain'], ['}', 'plain'],
    ],
    href: `${GITHUB_URL}/blob/develop/docs/public-mcp-server.md`, link: 'Read the MCP guide',
  },
  {
    id: 'sdk', label: 'SDK', icon: Code2, title: 'Connect the product your customers use.',
    description: 'Add the Helpin widget to your app and pass customer identity into the conversation. Use the JavaScript SDK or a framework integration.',
    filename: 'your-app.ts', note: 'Use your installation’s widget key and public URL.',
    lines: [
      ["import { helpinClient } from '@helpin-ai/sdk-js';", 'plain'], ['', 'plain'],
      ['const helpin = helpinClient({', 'plain'], ['  widgetKey: YOUR_WIDGET_KEY,', 'success'],
      ['  host: YOUR_HELPIN_URL,', 'success'], ['  supportOnly: true,', 'success'],
      ['});', 'plain'], ['', 'plain'], ['helpin?.open();', 'command'],
    ],
    href: `${GITHUB_URL}/tree/develop/packages/sdk-js`, link: 'Explore the SDK',
  },
] as const;

export function DeveloperWorkbench() {
  const [selected, setSelected] = useState(0);
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
  const id = `workbench-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  const example = EXAMPLES[selected];
  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next: number;
    if (event.key === 'ArrowRight') next = (index + 1) % EXAMPLES.length;
    else if (event.key === 'ArrowLeft') next = (index + EXAMPLES.length - 1) % EXAMPLES.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = EXAMPLES.length - 1;
    else return;
    event.preventDefault(); setSelected(next); tabRefs.current[next]?.focus();
  }
  return <div className="developer-workbench">
    <div className="developer-tabs" role="tablist" aria-label="Developer examples">
      {EXAMPLES.map(({id: key,label,icon:Icon},i) => <button key={key} ref={el=>{tabRefs.current[i]=el;}} type="button" role="tab" id={`${id}-tab-${key}`} aria-controls={`${id}-panel`} aria-selected={selected===i} tabIndex={selected===i?0:-1} onClick={()=>setSelected(i)} onKeyDown={e=>onKeyDown(e,i)}><Icon size={17} aria-hidden="true" />{label}</button>)}
    </div>
    <div id={`${id}-panel`} role="tabpanel" aria-labelledby={`${id}-tab-${example.id}`} tabIndex={0} className="developer-example">
      <div className="developer-example-copy"><h3>{example.title}</h3><p>{example.description}</p><a className="inline-link" href={example.href} target="_blank" rel="noopener noreferrer">{example.link} →</a></div>
      <div className="developer-terminal"><div className="developer-terminal-bar"><span aria-hidden="true"><i /><i /><i /></span><span>{example.filename}</span></div><pre><code>{example.lines.map(([line,tone],i)=><span className={`developer-code-line ${tone}`} key={`${example.id}-${i}`}>{line || ' '}{'\n'}</span>)}</code></pre><p>{example.note}</p></div>
    </div>
  </div>;
}
