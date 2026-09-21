'use client';
import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { ArrowDown, ArrowRight, BookOpen, Bot, Braces, Check, CheckCheck, Copy, Database, FileCheck2, Globe, HardDrive, LockKeyhole, MessagesSquare, Pause, Play, Plug, Server, ShieldCheck, Terminal, Workflow } from 'lucide-react';
import { useBentoPlayback } from '../useBentoPlayback';
import { CodeContent } from './PlatformProof';
import { SDKWidget } from './SDKWidget';

function Step({children,at=0,className=''}:{children:ReactNode;at?:number;className?:string}) {return <div className={`platform-step ${className}`} style={{'--step-delay':`${at}s`} as CSSProperties}>{children}</div>;}
function MotionFrame({children,label,description,className=''}:{children:ReactNode;label:string;description:string;className?:string}) {
  const {container,playing,cycle}=useBentoPlayback(11000);
  const [paused,setPaused]=useState(false);
  return <div ref={container} className={`platform-scene ${className}`} data-playing={playing&&!paused}><div className="platform-scene-toolbar"><span><b className="platform-orbit-mark">O</b>OrbitDesk<span className="platform-divider">/</span>{label}</span><button type="button" aria-label={`${paused?'Play':'Pause'} ${label.toLowerCase()} animation`} aria-pressed={paused} onClick={()=>setPaused(!paused)}>{paused?<Play size={12}/>:<Pause size={12}/>}</button></div><div key={cycle} className="platform-scene-content" role="img" aria-label={description}><div aria-hidden="true">{children}</div></div></div>;
}
export function InfrastructureScene(){return <MotionFrame className="platform-infrastructure" label="Your infrastructure" description="Helpin Community runs support, docs, and agents on your infrastructure. The installation connects the application to PostgreSQL, S3-compatible file storage, and Agent Runtime. You configure external model and email providers."><div className="infra-boundary"><span className="platform-micro"><Server size={13}/>YOUR DEPLOYMENT</span><Step at={.2} className="infra-app"><img src="/brand/helpin-icon-white.svg" width={30} height={30} alt=""/><div><strong>Helpin Community</strong><span>Support · Docs · Agents</span></div><span className="infra-beta">Beta</span></Step><div className="infra-connectors"><i/><i/><i/></div><div className="infra-nodes">{[{Icon:Database,title:'PostgreSQL',detail:'Customer history'},{Icon:HardDrive,title:'File storage',detail:'S3-compatible'},{Icon:Workflow,title:'Agent Runtime',detail:'Agent execution'}].map(({Icon,title,detail},i)=><Step at={1+i*.6} key={title}><Icon size={21}/><strong>{title}</strong><small>{detail}</small></Step>)}</div><Step at={3.3} className="infra-owned"><LockKeyhole size={13}/>Data and application services on your infrastructure</Step></div><div className="infra-external"><span className="platform-micro">CONNECTIONS YOU CONFIGURE</span><div><span><Bot size={14}/>Model providers</span><span><MessagesSquare size={14}/>Email services</span></div></div></MotionFrame>;}
const MODES={local:{label:'Local evaluation',title:'Start on your own machine.',description:'Use the local bundle defaults to explore support, docs, and agents.',rows:[['Dashboard & widget','localhost:8085'],['Public help center','localhost:8086'],['Object storage','localhost:9005']],steps:['Check Docker and available resources','Install the verified Community bundle','Create an account and workspace'],note:'Docker Compose · Start with 8 GiB RAM and 20 GiB free disk for evaluation.'},server:{label:'Public server',title:'Put your installation on your domains.',description:'Configure public origins, then connect your DNS and HTTPS reverse proxy.',rows:[['Dashboard & widget','app.orbitdesk.example'],['Public help center','help.orbitdesk.example'],['Attachments','files.orbitdesk.example']],steps:['Set the public domains and trusted proxy source','Apply the generated Caddy configuration on your host','Run doctor and verify the customer journey'],note:'The CLI prepares configuration. Your team manages DNS, HTTPS, and the host proxy.'}};
export function DeploymentExplorer(){const [mode,setMode]=useState<'local'|'server'>('local');const selected=MODES[mode];return <div className="deployment-explorer"><div className="platform-selector" role="group" aria-label="Deployment setup"><button type="button" aria-pressed={mode==='local'} aria-controls="deployment-details" onClick={()=>setMode('local')}><Terminal size={14}/>Local evaluation</button><button type="button" aria-pressed={mode==='server'} aria-controls="deployment-details" onClick={()=>setMode('server')}><Globe size={14}/>Public server</button></div><div id="deployment-details" className="deployment-details"><div className="deployment-copy"><span className="platform-micro">{selected.label}</span><h3>{selected.title}</h3><p>{selected.description}</p><ol>{selected.steps.map(s=><li key={s}>{s}</li>)}</ol></div><div className="deployment-hosts"><span className="platform-micro">YOUR ENTRY POINTS</span>{selected.rows.map(([label,address])=><div key={label}><span>{label}</span><code>{address}</code><Check size={14}/></div>)}<p><Server size={15}/>{selected.note}</p></div></div></div>;}

const SNIPPETS = {
 JavaScript: {install: 'npm install @helpin-ai/sdk-js', file:'support.ts',code:`import { helpinClient } from '@helpin-ai/sdk-js';

const client = helpinClient({
  widgetKey: 'YOUR_PUBLIC_WIDGET_KEY',
  host: 'https://client.helpin.ai',
  supportOnly: true,
  autoBoot: false,
});

client?.openNewMessage(
  'Can you help with our rollout?'
);`, note:'Use your widget’s public key and host. For self-hosting, set host to your installation’s public widget URL.'},
 React: {install: 'npm install @helpin-ai/react @helpin-ai/sdk-js', file:'SupportButton.tsx',code:`import { useHelpin } from '@helpin-ai/react';

// Inside your configured HelpinProvider.
export function SupportButton() {
  const { openNewMessage } = useHelpin();

  return (
    <button onClick={() => openNewMessage(
      'Can you help with our rollout?'
    )}>
      Talk to us
    </button>
  );
}`,note:'Initialize the client and wrap your application in HelpinProvider. The hook connects your own UI to the widget.'},
 'Next.js': {install: 'npm install @helpin-ai/nextjs @helpin-ai/sdk-js', file:'SupportButton.tsx',code:`'use client';

import { useHelpin } from '@helpin-ai/nextjs';

// Inside your configured HelpinProvider.
export function SupportButton() {
  const { openNewMessage } = useHelpin();

  return (
    <button onClick={() => openNewMessage(
      'Can you help with our rollout?'
    )}>Talk to us</button>
  );
}`,note:'Use a client component under HelpinProvider. Browser initialization stays separate from server rendering.'},
};
export function SDKExplorer(){const [language,setLanguage]=useState<keyof typeof SNIPPETS>('JavaScript');const [copyState,setCopyState]=useState('Copy example');const timer=useRef<ReturnType<typeof setTimeout>|null>(null);useEffect(()=>()=>{if(timer.current)clearTimeout(timer.current)},[]);const example=SNIPPETS[language];async function copy(){try{await navigator.clipboard.writeText(example.code);setCopyState('Copied');}catch{setCopyState('Select the code to copy');}if(timer.current)clearTimeout(timer.current);timer.current=setTimeout(()=>setCopyState('Copy example'),2500);}return <div className="sdk-explorer"><div className="sdk-topbar"><div className="platform-selector" role="group" aria-label="SDK example language">{(Object.keys(SNIPPETS) as (keyof typeof SNIPPETS)[]).map(name=><button type="button" key={name} aria-pressed={language===name} aria-controls="sdk-example" onClick={()=>{setLanguage(name);setCopyState('Copy example')}}>{name}</button>)}</div><button className="platform-copy" type="button" onClick={copy}><Copy size={13}/><span aria-live="polite">{copyState}</span></button></div><div className="sdk-install"><Terminal size={13}/><code>{example.install}</code></div><div className="sdk-body"><div className="sdk-code"><div><span className="sdk-file-dot"/>{example.file}</div><pre id="sdk-example" tabIndex={0} aria-label={`${language} SDK code example`}><code><CodeContent code={example.code}/></code></pre></div><div className="sdk-result"><span className="platform-micro">OPEN A CONVERSATION FROM YOUR APP</span><SDKWidget/><p>{example.note}</p></div></div></div>;}
export function DeveloperHeroScene(){return <MotionFrame className="developer-hero-scene" label="Connected workspace" description="Your application supplies customer history through the SDK. Helpin connects conversations, knowledge, and agents around the customer. Selected external tools can be made available through MCP."><div className="dev-source"><Braces size={22}/><div><strong>Your application</strong><span>Customer identity & conversations</span></div><code>SDK</code></div><div className="dev-flow-line"><i/><ArrowDown size={17}/></div><Step at={.8} className="dev-hub"><img src="/brand/helpin-icon-white.svg" width={29} height={29} alt=""/><div><strong>Helpin</strong><span>The customer history stays connected</span></div></Step><div className="dev-capabilities">{[{Icon:MessagesSquare,label:'Conversations'},{Icon:BookOpen,label:'Knowledge'},{Icon:Bot,label:'Agents'}].map(({Icon,label},i)=><Step at={1.8+i*.5} key={label}><Icon size={20}/><span>{label}</span></Step>)}</div><Step at={3.6} className="dev-tool-connection"><Plug size={16}/><span>Selected external tools</span><span>MCP</span><Check size={14}/></Step></MotionFrame>;}
export function MCPScene({direction}:{direction:'inbound'|'outbound'}) {
 const inbound=direction==='inbound';
 return <MotionFrame className="mcp-workflow" label={inbound?'Helpin MCP':'External MCP'} description={inbound?'An external AI client asks for the export guide. Its workspace-bound connection permits document search. Helpin returns the guide; other actions are not granted.':'A Helpin agent checks a linked export issue through an external MCP server. Only the selected read tool is available, and the issue status returns to the agent.'}>
   <div className="mcp-endpoints"><div>{inbound?<Terminal size={21}/>:<img src="/new/agents/echo.svg" width={34} height={34} alt=""/>}<strong>{inbound?'Your AI client':'Helpin agent'}</strong></div><span className="mcp-wire"><i/><ArrowRight size={18}/></span><div>{inbound?<img src="/brand/helpin-icon-white.svg" width={28} height={28} alt=""/>:<Plug size={23}/>}<strong>{inbound?'Helpin workspace':'External server'}</strong></div></div>
   <div className="mcp-request"><span className="platform-micro">THE REQUEST</span><p>{inbound?'Find the guide for exporting contacts.':'Check the status of the linked export issue.'}</p></div>
   <div className="mcp-execution"><Step at={.8}><LockKeyhole size={14}/><div><strong>{inbound?'Check workspace access':'Use the selected tool'}</strong><span>{inbound?'Docs read · OrbitDesk':'Connected tracker · Read access'}</span></div><Check size={13}/></Step><Step at={2}><Braces size={14}/><div><strong>{inbound?'Search the documentation':'Read the linked issue'}</strong><span>{inbound?'search_documents':'EXP-142 · Customer export'}</span></div><Check size={13}/></Step></div>
   <Step at={3.2} className="mcp-result"><BookOpen size={17}/><div><strong>{inbound?'Export your contacts':'EXP-142 · In progress'}</strong><span>{inbound?'Guide returned to your AI client':'Issue status returned to the agent'}</span></div><ArrowRight size={14}/></Step>
   <div className="mcp-footnote"><ShieldCheck size={14}/>{inbound?'One workspace. Only the access granted.':'Only the tools selected for this agent.'}</div>
 </MotionFrame>;
}
export function EventScene(){return <MotionFrame className="event-workflow" label="Event to agent run" description="Where Automation is enabled, an incoming event is checked against configured rules. A matching rule records a trigger execution and can start an agent run. Activity and run results remain separate."><div className="event-source"><span><Workflow size={17}/></span><div><strong>Repository event received</strong><small>Pull request updated</small></div><span className="platform-micro">WEBHOOK</span></div><div className="event-timeline">{[{Icon:FileCheck2,title:'Check the configured rule',body:'Repository and event conditions match'},{Icon:CheckCheck,title:'Record the trigger execution',body:'A traceable reason for starting work'},{Icon:Bot,title:'Start the selected agent',body:'Tools and instructions follow its configuration'},{Icon:BookOpen,title:'Bring the result back',body:'Inspect the run, interactions, and artifacts'}].map(({Icon,title,body},i)=><Step at={.6+i*.9} key={title}><span><Icon size={16}/></span><div><strong>{title}</strong><small>{body}</small></div></Step>)}</div></MotionFrame>;}
