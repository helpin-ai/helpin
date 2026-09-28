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
export function InfrastructureScene(){return <MotionFrame className="platform-infrastructure" label="Your infrastructure" description="Helpin runs support, projects, CRM, meetings, knowledge and agents on your infrastructure. The installation connects the application to PostgreSQL, S3-compatible file storage, Agent Runtime, and supporting queue and workflow services. You configure external model and email providers."><h3 className="self-hosting-art-heading">Helpin, operated by your team.</h3><div className="infra-boundary"><span className="platform-micro"><Server size={13}/>SERVICES YOU OPERATE</span><Step at={.2} className="infra-app"><img src="/brand/helpin-icon-white.svg" width={30} height={30} alt=""/><div><strong>Helpin</strong><span>Customer history, teamwork, and AI agents.</span></div><span className="infra-beta">0.2 beta</span></Step><div className="infra-connectors"><i/><i/><i/></div><div className="infra-nodes">{[{Icon:Database,title:'PostgreSQL',detail:'Customer records and product data'},{Icon:HardDrive,title:'File storage',detail:'Documents and attachments'},{Icon:Workflow,title:'Agent runtime',detail:'Agent execution'}].map(({Icon,title,detail},i)=><Step at={1+i*.6} key={title}><Icon size={21}/><strong>{title}</strong><small>{detail}</small></Step>)}</div><Step at={3.3} className="infra-owned"><LockKeyhole size={13}/>Data and application services on your infrastructure</Step></div><div className="infra-external"><span className="platform-micro">SERVICES YOU CHOOSE TO CONNECT</span><div><span><Bot size={14}/>AI providers</span><span><MessagesSquare size={14}/>Email services</span><span><Plug size={14}/>Optional integrations</span></div></div></MotionFrame>;}
const MODES = {
 local: {
  label: 'Local evaluation', title: 'Start with a customer story you can test.',
  description: 'Use sample records to explore a conversation, a linked task, and an agent’s answer. Check whether the history helps the next person understand what needs to happen.',
  panel: 'Your local installation', rows: [['Dashboard and widget', 'localhost:8085'], ['Help center', 'localhost:8086'], ['Object storage', 'localhost:9005']],
  steps: [['Prepare your machine.', 'Check the requirements for the release.'], ['Follow the installation guide.', 'Set up the local services.'], ['Set up your evaluation.', 'Configure the connections needed for the workflow you want to evaluate.']],
  note: 'Explore with sample data before bringing in customer records.',
  detail: 'Suggested evaluation prompt: “What has this customer already tried, and what is the next step in the linked task?”',
  reminder: 'Connect an AI provider to try the agent’s answer.',
 },
 server: {
  label: 'Public server', title: 'Prepare Helpin for your team.',
  description: 'Set the public addresses, review access, and test the customer-facing workflows before launch.',
  panel: 'Your domains. Your deployment.', rows: [['Dashboard', 'app.orbitdesk.example'], ['Help center', 'docs.orbitdesk.example'], ['File service', 'files.orbitdesk.example']],
  steps: [['Configure your public addresses.', 'Choose where your team, customers, and published content connect.'], ['Connect the services you need.', 'Set up the relevant email, AI, and integration access.'], ['Check the complete workflow.', 'Test sign-in, customer conversations, agent permissions, and follow-up.']],
  note: 'Make the route from customer question to team action work end to end.',
  detail: 'Configure the widget for your deployment’s public address and permitted website origins. Grant external tools only the access the workflow needs.',
  reminder: 'Illustrative addresses. Your team manages DNS, the proxy, and certificates.',
 },
};
export function DeploymentExplorer(){const [mode,setMode]=useState<'local'|'server'>('local');const selected=MODES[mode];return <div className="deployment-explorer"><div className="platform-selector" role="group" aria-label="Deployment setup"><button type="button" aria-pressed={mode==='local'} aria-controls="deployment-details" onClick={()=>setMode('local')}><Terminal size={14}/>Local evaluation</button><button type="button" aria-pressed={mode==='server'} aria-controls="deployment-details" onClick={()=>setMode('server')}><Globe size={14}/>Public server</button></div><div id="deployment-details" className="deployment-details"><div className="deployment-copy"><span className="platform-micro">{selected.label}</span><h3>{selected.title}</h3><p>{selected.description}</p><ol>{selected.steps.map(([title,body])=><li key={title}><div><strong>{title}</strong><span>{body}</span></div></li>)}</ol></div><div className="deployment-hosts"><span className="platform-micro">{selected.panel}</span>{selected.rows.map(([label,address])=><div key={label}><span>{label}</span><code>{address}</code><Check size={14}/></div>)}<p><Server size={15}/>{selected.note}</p><p className="deployment-detail">{selected.detail}</p><p className="deployment-reminder">{selected.reminder}</p></div></div></div>;}

const SNIPPETS = {
 JavaScript: {install: 'npm install @helpin-ai/sdk-js@0.0.45', file:'support.ts',code:`import { helpinClient } from '@helpin-ai/sdk-js';

const client = helpinClient({
  widgetKey: 'YOUR_PUBLIC_WIDGET_KEY',
  host: 'https://client.helpin.ai',
  autoBoot: false,
});

client?.openNewMessage(
  'My export is missing contacts. Can you help?'
);`, note:'Use your widget’s public key and host. For self-hosting, set host to your installation’s public widget URL.'},
 React: {install: 'npm install @helpin-ai/react@0.0.45 @helpin-ai/sdk-js@0.0.45', file:'SupportButton.tsx',code:`import { useHelpin } from '@helpin-ai/react';

// Inside your configured HelpinProvider.
export function SupportButton() {
  const { openNewMessage } = useHelpin();

  return (
    <button onClick={() => openNewMessage(
      'My export is missing contacts. Can you help?'
    )}>
      Get help with this export
    </button>
  );
}`,note:'Initialize the client and wrap your application in HelpinProvider. The hook connects your own UI to the widget.'},
 'Next.js': {install: 'npm install @helpin-ai/nextjs@0.0.45 @helpin-ai/sdk-js@0.0.45', file:'SupportButton.tsx',code:`'use client';

import { useHelpin } from '@helpin-ai/nextjs';

// Inside your configured HelpinProvider.
export function SupportButton() {
  const { openNewMessage } = useHelpin();

  return (
    <button onClick={() => openNewMessage(
      'My export is missing contacts. Can you help?'
    )}>Get help with this export</button>
  );
}`,note:'Use a client component under HelpinProvider. Browser initialization stays separate from server rendering.'},
 Vue: {install: 'npm install @helpin-ai/vue@0.0.45 @helpin-ai/sdk-js@0.0.45', file:'SupportButton.vue', code:`<script setup lang="ts">
import { useHelpin } from '@helpin-ai/vue';

// Inside an app that installs HelpinPlugin.
const helpin = useHelpin();
</script>

<template>
  <button @click="helpin.openNewMessage(
    'My export is missing contacts. Can you help?'
  )">Get help with this export</button>
</template>`,note:'Create the client and install HelpinPlugin in main.ts. The composable connects your own UI to the widget.'},
};
export function SDKExplorer(){const [language,setLanguage]=useState<keyof typeof SNIPPETS>('JavaScript');const [copyState,setCopyState]=useState('Copy example');const timer=useRef<ReturnType<typeof setTimeout>|null>(null);useEffect(()=>()=>{if(timer.current)clearTimeout(timer.current)},[]);const example=SNIPPETS[language];async function copy(){try{await navigator.clipboard.writeText(example.code);setCopyState('Copied');}catch{setCopyState('Select the code to copy');}if(timer.current)clearTimeout(timer.current);timer.current=setTimeout(()=>setCopyState('Copy example'),2500);}return <div className="sdk-explorer"><div className="sdk-topbar"><div className="platform-selector" role="group" aria-label="SDK example language">{(Object.keys(SNIPPETS) as (keyof typeof SNIPPETS)[]).map(name=><button type="button" key={name} aria-pressed={language===name} aria-controls="sdk-example" onClick={()=>{setLanguage(name);setCopyState('Copy example')}}>{name}</button>)}</div><button className="platform-copy" type="button" onClick={copy}><Copy size={13}/><span aria-live="polite">{copyState}</span></button></div><div className="sdk-install"><Terminal size={13}/><code>{example.install}</code></div><div className="sdk-body"><div className="sdk-code"><h3 className="developer-code-heading">Open a conversation from your app.</h3><div><span className="sdk-file-dot"/>{example.file}</div><pre id="sdk-example" tabIndex={0} aria-label={`${language} SDK code example`}><code><CodeContent code={example.code}/></code></pre></div><div className="sdk-result"><span className="platform-micro">A QUESTION FROM INSIDE YOUR PRODUCT</span><SDKWidget/><p>{example.note}</p></div></div></div>;}
export function DeveloperHeroScene(){return <MotionFrame className="developer-hero-scene" label="Connected workspace" description="Your application supplies customer history through the SDK. Helpin connects conversations, knowledge, and agents around the customer. Selected external tools can be made available through MCP."><div className="dev-source"><Braces size={22}/><div><strong>Your product</strong><span>Where the customer starts the conversation.</span></div><code>Helpin SDK</code></div><div className="dev-flow-line"><i/><ArrowDown size={17}/></div><Step at={.8} className="dev-hub"><img src="/brand/helpin-icon-white.svg" width={29} height={29} alt=""/><div><strong>Helpin</strong><span>The conversation, customer history, and work—connected.</span></div></Step><div className="dev-capabilities">{[{Icon:MessagesSquare,label:'Conversations'},{Icon:BookOpen,label:'Knowledge'},{Icon:Bot,label:'Agents'}].map(({Icon,label},i)=><Step at={1.8+i*.5} key={label}><Icon size={20}/><span>{label}</span></Step>)}</div><Step at={3.6} className="dev-tool-connection"><Plug size={16}/><div><strong>Your connected systems</strong><span>The tools you choose for each agent.</span></div><span>MCP</span><Check size={14}/></Step></MotionFrame>;}
export function MCPScene({direction}:{direction:'inbound'|'outbound'}) {
 const inbound=direction==='inbound';
 return <MotionFrame className="mcp-workflow" label={inbound?'Helpin MCP':'External MCP'} description={inbound?'An external AI client asks for the export guide. Its workspace-bound connection permits document search. Helpin returns the guide; other actions are not granted.':'A Helpin agent checks a linked export issue through an external MCP server. Only the selected read tool is available, and the issue status returns to the agent.'}>
   <h3 className="developer-scene-heading">{inbound ? 'Find the guidance without switching tools.' : 'Check the work before drafting the update.'}</h3><div className="mcp-endpoints"><div>{inbound?<Terminal size={21}/>:<img src="/brand/helpin-icon-white.svg" width={34} height={34} alt=""/>}<strong>{inbound?'Your AI client':'Ask Agent'}</strong></div><span className="mcp-wire"><i/><ArrowRight size={18}/></span><div>{inbound?<img src="/brand/helpin-icon-white.svg" width={28} height={28} alt=""/>:<Plug size={23}/>}<strong>{inbound?'OrbitDesk':'Your issue tracker'}</strong></div></div>
   <div className="mcp-request"><span className="platform-micro">THE REQUEST</span><p>{inbound?'Find our guide to exporting contacts.':'Check the linked export issue before preparing Maya’s update.'}</p></div>
   <div className="mcp-execution"><Step at={.8}><LockKeyhole size={14}/><div><strong>{inbound?'Documentation access granted':'Read an issue'}</strong><span>{inbound?'OrbitDesk':'Read access · Editing not enabled'}</span></div><Check size={13}/></Step><Step at={2}><Braces size={14}/><div><strong>{inbound?'Search Helpin documentation.':'Read the linked issue'}</strong><span>{inbound?'Documentation search':'EXP-142 · Customer export'}</span></div><Check size={13}/></Step></div>
   <Step at={3.2} className="mcp-result"><BookOpen size={17}/><div><strong>{inbound?'Export your contacts':'EXP-142 · In progress'}</strong><span>{inbound?'The guide explains how to select contacts and start an export.':'The issue is still marked In progress. I’ll prepare a status update without saying the fix has been released.'}</span></div><ArrowRight size={14}/></Step>
   <div className="mcp-footnote"><ShieldCheck size={14}/>{inbound?'A relevant guide, returned to your AI client.':'Draft ready for review'}</div>
   {!inbound && <p className="developer-demo-caption">An informed update, without permission to change the issue.</p>}
 </MotionFrame>;
}
export function EventScene(){return <MotionFrame className="event-workflow" label="Event to agent run" description="Illustrative workflow: a GitHub pull request opened for EXP-142 matches a configured rule. Its trigger record starts a code review. The reviewer flags the failed-page case for Sam. The review is complete, but the human decision is pending and nothing is merged."><h3 className="developer-scene-heading">A pull request starts a review—not a release.</h3><div className="event-source"><span><Workflow size={17}/></span><div><strong>Event received</strong><small>A pull request linked to EXP-142 was opened.</small></div><span className="platform-micro">GITHUB</span></div><div className="event-timeline">{[{Icon:FileCheck2,title:'Rule checked',body:'The configured repository and event conditions match.'},{Icon:CheckCheck,title:'Trigger recorded',body:'The matching event is recorded as the reason for this run.'},{Icon:Bot,title:'Agent started',body:'The code reviewer examines the proposed export changes.'},{Icon:BookOpen,title:'Result returned',body:'Review findings are ready for Sam. Nothing has been merged.'}].map(({Icon,title,body},i)=><Step at={.6+i*.9} key={title}><span><Icon size={16}/></span><div><strong>{title}</strong><small>{body}</small></div></Step>)}</div><Step at={4.5} className="developer-review-finding"><p>The change retrieves the remaining contacts. Check the failure case before approving it: what happens when a later page cannot be fetched?</p><strong>Review completed · Human decision pending</strong></Step><p className="developer-demo-caption">An event starts the review. Your team decides what happens next.</p></MotionFrame>;}
