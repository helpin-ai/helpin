import { ArrowRight, Braces, Code2, Plug, Terminal } from 'lucide-react';

const INTERFACES = [
  { Icon: Code2, label: 'SDK', code: 'openNewMessage()', outcome: 'Start a conversation' },
  { Icon: Braces, label: 'API', code: 'GET /api/workspaces', outcome: 'Read workspace records' },
  { Icon: Plug, label: 'MCP', code: 'search_documents', outcome: 'Find product knowledge' },
  { Icon: Terminal, label: 'CLI', code: 'helpin doctor', outcome: 'Check your deployment' },
];

export function DeveloperInterfaces() {
  return <div className="developer-interfaces" role="img" aria-label="Four ways to build with Helpin: the SDK starts customer conversations, the API reads workspace records, MCP searches product knowledge, and the CLI checks your deployment.">
    <div className="developer-interfaces-glow" aria-hidden="true" />
    <div className="developer-interfaces-sheet" aria-hidden="true">
      <div className="developer-interfaces-heading"><img src="/brand/helpin-icon-ink.svg" alt="" width={25} height={25}/><strong>Build with Helpin</strong><span>YOUR STACK, CONNECTED</span></div>
      <div className="developer-interface-rows">{INTERFACES.map(({Icon,label,code,outcome}) => <div className="developer-interface-row" key={label}>
        <div className="developer-interface-code"><span><Icon size={15}/>{label}</span><code>{code}</code></div>
        <div className="developer-interface-connector"><i/><ArrowRight size={15}/></div>
        <div className="developer-interface-outcome">{outcome}</div>
      </div>)}</div>
      <div className="developer-interfaces-footer"><span/>Customer history, wherever you build.</div>
    </div>
  </div>;
}
