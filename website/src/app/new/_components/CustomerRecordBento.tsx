'use client';

import { useId, type CSSProperties, type ReactNode, type SVGProps } from 'react';
import { CalendarDays, Check, Code2, FileText, GitBranch, GitPullRequest, Link2, Mail, Mic, ShieldCheck, Sparkles, Tag, Users, type LucideIcon } from 'lucide-react';
import { useBentoPlayback } from './useBentoPlayback';

export type RecordVariant = 'conversations' | 'meetings' | 'projects' | 'deals' | 'docs' | 'email-calendar' | 'coding';
const LABELS: Record<RecordVariant, [string, string]> = {
  conversations: ['Customer conversations', 'Maya at Northstar Labs asks about setting up Okta SSO, testing with a pilot group, and who owns the rollout. Helpin AI answers each follow-up with linked guides and project context, applies relevant tags, and keeps the conversation connected to the same rollout project.'],
  meetings: ['Customer meetings', 'Northstar Labs’ security review connects the recording, speaker notes, and two next steps: validate the Okta setup and share the guide.'],
  projects: ['Customer projects', 'SSO Enterprise Readiness connects seven customer requests to tasks: Okta mapping in review, SCIM planned, and role mapping shipped. Eight of twelve tasks are complete.'],
  coding: ['Coding agents with customer context', 'Forge works on the Okta SAML mapping task with Maya’s request for Northstar Labs’ rollout attached, prepares a role-mapping code change, and returns pull request 728 for Sam to review.'],
  deals: ['Customer deals', 'Northstar Labs’ $42,000 renewal is in negotiation. Its SSO approval requirement is connected to Maya’s security review.'],
  docs: ['Customer documentation', 'A published Okta setup guide with three steps is connected to PR 728, shared with Maya at Northstar Labs, and used in a Helpin AI reply.'],
  'email-calendar': ['Customer email and calendar', 'Northstar Labs’ timeline connects Maya’s rollout email, the security review meeting, and the Okta guide shared by Helpin AI.'],
};
const at = (seconds: number) => ({ '--cr-delay': `${seconds}s` }) as CSSProperties;

function Glyph({ icon: Icon, x, y, size = 24, color = '#72dfad' }: { icon: LucideIcon; x: number; y: number; size?: number; color?: string }) {
  return <g transform={`translate(${x} ${y})`} color={color}><Icon width={size} height={size} strokeWidth={1.7} aria-hidden="true" /></g>;
}
function Panel({ id, x, y, width, height, stacked = false }: { id: string; x: number; y: number; width: number; height: number; stacked?: boolean }) {
  return <>{stacked && <rect x={x + 9} y={y - 9} width={width} height={height} rx={16} fill="#101b18" stroke="#243c32" />}<rect className="cr-panel" x={x} y={y} width={width} height={height} rx={16} fill={`url(#${id}-surface)`} /></>;
}
function Reveal({ start, children }: { start: number; children: ReactNode }) {
  return <g className="cr-reveal" style={at(start)}>{children}</g>;
}
// Discrete character reveals preserve SVG shaping; punctuation gets a natural pause.
function TypedText({ children, start, pace = .025, ...props }: SVGProps<SVGTextElement> & { children: string; start: number; pace?: number }) {
  let delay = start;
  return <text {...props} className={`${props.className ?? ''} cr-typed`}>
    {Array.from(children).map((character, index) => {
      const characterDelay = delay;
      delay += pace * (/[.,?!]/.test(character) ? 3 : character === ' ' ? .6 : 1);
      return <tspan key={index} className="cr-character" style={at(characterDelay)}>{character}</tspan>;
    })}
  </text>;
}
function Checkmark({ x, y }: { x: number; y: number }) {
  return <g><circle cx={x} cy={y} r={9} fill="#143c2c" stroke="#59bd8d" /><path className="cr-check-stroke" d={`M${x-4} ${y} l3 3 5 -6`} fill="none" stroke="#72dfad" strokeWidth={1.7} strokeLinecap="round" strokeLinejoin="round" pathLength={1} /></g>;
}
function Avatar({ x, y, initial = 'M' }: { x: number; y: number; initial?: string }) {
  const clipId = `cr-avatar-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  const portrait = initial === 'S' ? 'sam' : 'maya';
  return <g>
    <defs><clipPath id={clipId}><circle cx={x} cy={y} r={17} /></clipPath></defs>
    <image href={`/new/avatars/${portrait}.webp`} x={x - 17} y={y - 17} width={34} height={34} preserveAspectRatio="xMidYMid slice" clipPath={`url(#${clipId})`} />
    <circle cx={x} cy={y} r={17} fill="none" stroke="#476454" />
  </g>;
}
function HelpinMark({ x, y, size = 26 }: { x: number; y: number; size?: number }) {
  return <g className="cr-helpin-mark"><rect x={x} y={y} width={size} height={size} rx={7} fill="#1b4835" stroke="#376b50" /><image href="/brand/helpin-icon-white.svg" x={x + 5} y={y + 5} width={size - 10} height={size - 10} /></g>;
}
function Connection({ d, start = 0 }: { d: string; start?: number }) {
  return <g fill="none"><path d={d} className="cr-wire" /><path d={d} pathLength={100} className="cr-signal" style={at(start)} /></g>;
}

const CONVERSATION_TURNS = [
  { question: 'How do I set up Okta SSO?', answer: 'Create a SAML app, then map your roles.', source: 'Set up SSO with Okta →', tag: 'Setup' },
  { question: 'Can we test with a small group?', answer: 'Yes. Start with a pilot group in Okta.', source: 'Test your SSO setup →', tag: 'Pilot' },
  { question: 'Who’s handling our rollout?', answer: 'Sam owns it. Okta mapping is in review.', source: 'View the rollout project →', tag: 'Rollout' },
];

function ConversationTurn({ turn, index }: { turn: typeof CONVERSATION_TURNS[number]; index: number }) {
  const offset = index * 10;
  return <g className="cr-exchange" transform={`translate(0 ${index * 240})`}>
    <Reveal start={offset + .15}><rect x={54} y={105} width={288} height={44} rx={12} fill="#1d2a27" /></Reveal>
    <TypedText start={offset + .35} pace={.043} x={70} y={133} className="cr-body">{turn.question}</TypedText>
    <Reveal start={offset + 1.85}><HelpinMark x={60} y={163} /><text x={90} y={182} className="cr-ai-label">Helpin AI</text><text x={163} y={182} className="cr-caption">{index === 0 ? 'First response' : 'Follow-up'}</text></Reveal>
    <Reveal start={offset + 1.85}><rect x={87} y={196} width={418} height={73} rx={12} fill="#153b2b" stroke="#367458" /></Reveal>
    <g className="cr-typing" style={at(offset + 1.85)} aria-hidden="true">{[0,1,2].map(i => <circle key={i} cx={104+i*12} cy={221} r={2.5} style={at(i*.14)} />)}</g>
    <TypedText start={offset + 2.9} pace={.033} x={104} y={223} className="cr-body">{turn.answer}</TypedText>
    <Reveal start={offset + 4.35}><Glyph icon={FileText} x={105} y={241} size={16} /></Reveal>
    <TypedText start={offset + 4.4} pace={.022} x={129} y={254} className="cr-source-link">{turn.source}</TypedText>
    <Reveal start={offset + 5.05}><Glyph icon={Tag} x={56} y={291} size={18} color="#a9bdb0" /></Reveal>
    {[
      { name: 'SSO', x: 83, width: 59, start: 5.05 },
      { name: 'Okta', x: 150, width: 65, start: 5.28 },
      { name: turn.tag, x: 223, width: 74, start: 5.51 },
    ].map(({name,x,width,start})=><Reveal key={name} start={offset + start}><rect x={x} y={287} width={width} height={27} rx={8} fill="#163027" stroke="#315e49" /><text x={x+width/2} y={306} textAnchor="middle" className="cr-accent">{name}</text></Reveal>)}
    <TypedText start={offset + 5.65} x={304} y={306} className="cr-caption">Tagged by AI</TypedText>
  </g>;
}

function Conversations({ id }: { id: string }) {
  return <>
    <Panel id={id} x={32} y={24} width={536} height={352} stacked />
    <Avatar x={69} y={61} /><text x={98} y={57} className="cr-title">Maya Chen</text><text x={98} y={79} className="cr-muted">Northstar Labs</text>
    <rect x={432} y={44} width={114} height={27} rx={13} fill="#173f2d" /><circle cx={445} cy={58} r={3} fill="#79e6af" /><text x={458} y={63} className="cr-accent">AI handling</text>
    <defs><clipPath id={`${id}-thread-window`}><rect x={44} y={94} width={512} height={229} /></clipPath></defs>
    <g clipPath={`url(#${id}-thread-window)`}>
      <g className="cr-thread-scroll">{CONVERSATION_TURNS.map((turn, index) => <ConversationTurn key={turn.question} turn={turn} index={index} />)}</g>
    </g>
    <path d="M54 330 H546" stroke="#294236" />
    <Reveal start={6.15}><Glyph icon={Link2} x={57} y={346} size={18} /></Reveal>
    <TypedText start={6.2} pace={.02} x={83} y={360} className="cr-accent">SSO Enterprise Readiness</TypedText>
    <Reveal start={6.8}><Checkmark x={435} y={354} /><text x={452} y={360} className="cr-caption">Context saved</text></Reveal>
  </>;
}

function Meetings({ id }: { id: string }) {
  const bars = [9,16,24,13,31,40,25,16,32,45,27,18,36,23,13,28,41,29,16,34,24,11,18,9];
  return <>
    <Panel id={id} x={74} y={34} width={452} height={328} stacked />
    <Glyph icon={Mic} x={98} y={56} /><text x={134} y={77} className="cr-title">Security &amp; rollout review</text><text x={98} y={102} className="cr-muted">Northstar Labs · 42 min</text><Glyph icon={Sparkles} x={354} y={89} size={16} /><text x={377} y={102} className="cr-caption">AI meeting notes</text>
    <rect x={96} y={119} width={408} height={58} rx={10} fill="#101e18" />
    {bars.map((height, i) => <rect key={i} className="cr-wave" x={116 + i * 15.8} y={148 - height / 2} width={4} height={height} rx={2} fill="#62ce98" style={at(i * .045)} />)}
    <rect className="cr-highlight" x={135} y={192} width={366} height={31} rx={6} style={at(.6)} /><rect className="cr-highlight" x={135} y={234} width={366} height={31} rx={6} style={at(1.8)} />
    <Reveal start={.6}><Avatar x={113} y={208} /><text x={141} y={213} className="cr-body">SSO is required before rollout.</text></Reveal>
    <Reveal start={1.8}><Avatar x={113} y={250} initial="S" /><text x={141} y={255} className="cr-body">We’ll validate the Okta setup.</text></Reveal>
    <path d="M98 280 H502" stroke="#294236" />
    <Reveal start={3}><Checkmark x={109} y={307} /><text x={129} y={312} className="cr-muted">Validate Okta setup</text></Reveal>
    <Reveal start={3.75}><Checkmark x={109} y={339} /><text x={129} y={344} className="cr-muted">Share the setup guide</text><text x={500} y={344} textAnchor="end" className="cr-accent">2 action items</text></Reveal>
  </>;
}

function Projects({ id }: { id: string }) {
  return <>
    <Panel id={id} x={62} y={43} width={476} height={312} stacked />
    <Reveal start={.1}><rect x={340} y={25} width={181} height={29} rx={10} fill="#143b29" stroke="#376b4f" /><Glyph icon={Users} x={353} y={31} size={17} /><text x={380} y={45} className="cr-caption">7 customer requests</text></Reveal>
    <Glyph icon={FileText} x={87} y={64} /><text x={123} y={85} className="cr-title">SSO Enterprise Readiness</text>
    <text x={87} y={122} className="cr-muted">8 / 12 tasks complete</text><text x={510} y={122} textAnchor="end" className="cr-accent">67%</text>
    <rect x={87} y={135} width={423} height={7} rx={3.5} fill="#24362e" /><rect className="cr-progress" x={87} y={135} width={283} height={7} rx={3.5} fill="#61dca1" />
    {[
      ['Okta SAML mapping','In review','#e8c77d'],
      ['SCIM provisioning','Planned','#a9b9b0'],
      ['Role mapping','Shipped','#72dfad'],
    ].map(([name,status,color],i)=><Reveal key={name} start={.65+i*.85}><rect className="cr-highlight" x={87} y={168+i*48} width={423} height={35} rx={6} style={at(.65+i*.85)} /><path d={`M87 ${163+i*48} H510`} stroke="#294236" />{i===2?<Checkmark x={98} y={185+i*48} />:<circle cx={98} cy={185+i*48} r={8} fill="none" stroke="#789d88" />}<text x={119} y={191+i*48} className="cr-body">{name}</text><text x={508} y={191+i*48} textAnchor="end" className="cr-status" fill={color}>{status}</text></Reveal>)}
    <Reveal start={2.9}><Glyph icon={Users} x={87} y={315} size={20} /><text x={116} y={332} className="cr-muted">Maya + 6 customers waiting · 3 linked issues</text></Reveal>
  </>;
}

function Coding({ id }: { id: string }) {
  return <>
    <Panel id={id} x={32} y={24} width={536} height={352} stacked />
    <Glyph icon={Code2} x={54} y={42} size={22} /><text x={86} y={59} className="cr-title">Okta SAML mapping</text>
    <text x={544} y={58} textAnchor="end" className="cr-caption">HLP-142</text>
    <rect x={54} y={77} width={492} height={48} rx={10} fill="#192d23" stroke="#2c4d3a" />
    <Avatar x={78} y={101} /><text x={104} y={96} className="cr-caption">Maya · Northstar Labs</text>
    <text x={104} y={115} className="cr-body">SSO is blocking our rollout.</text>
    <text x={530} y={105} textAnchor="end" className="cr-accent">7 requests</text>
    <Connection d="M300 125 V148" start={.3} />
    <rect x={54} y={148} width={492} height={137} rx={12} fill="#0c1812" stroke="#30503d" />
    <HelpinMark x={68} y={160} /><text x={105} y={177} className="cr-ai-label">Forge · Coding agent</text>
    <g className="cr-coding-working"><circle className="cr-coding-pulse" cx={457} cy={173} r={3} fill="#dfc47d" /><text x={469} y={177} className="cr-caption">Working</text></g>
    <g className="cr-coding-ready"><Checkmark x={435} y={172} /><text x={451} y={177} className="cr-accent">Changes ready</text></g>
    <path d="M68 197 H532" stroke="#253f30" />
    <text x={72} y={222} className="cr-code-muted">18</text><text x={104} y={222} className="cr-code">const member = &#123;</text>
    <Reveal start={1.25}><rect className="cr-code-insert" x={65} y={230} width={470} height={24} rx={4} /><text x={72} y={246} className="cr-code-added">+</text><text x={104} y={246} className="cr-code-added">  role: mapOktaGroup(group),</text></Reveal>
    <text x={72} y={272} className="cr-code-muted">20</text><text x={104} y={272} className="cr-code">&#125;;</text>
    <Connection d="M300 285 V310" start={2.65} />
    <Reveal start={3.3}>
      <rect x={54} y={310} width={492} height={48} rx={10} fill="#193226" stroke="#38654b" />
      <Glyph icon={GitPullRequest} x={68} y={322} size={22} /><text x={104} y={330} className="cr-body">PR #728 · Okta role mapping</text>
      <Glyph icon={GitBranch} x={104} y={338} size={12} /><text x={122} y={349} className="cr-caption">orbitdesk / platform</text>
      <Avatar x={414} y={334} initial="S" /><text x={438} y={331} className="cr-accent">Ready for review</text><text x={438} y={349} className="cr-caption">Assigned to Sam</text>
    </Reveal>
  </>;
}

function Deals({ id }: { id: string }) {
  return <>
    <Panel id={id} x={84} y={42} width={432} height={287} stacked />
    <text x={108} y={80} className="cr-title">Northstar Labs · Renewal</text>
    <text x={108} y={133} className="cr-amount">$42,000</text><text x={491} y={128} textAnchor="end" className="cr-accent">Negotiation</text>
    <Connection d="M119 170 H481" start={.3} />
    {['Lead','Qualified','Proposal','Negotiation'].map((label,i)=><g key={label}><circle className="cr-stage-ring" cx={119+i*120.5} cy={170} r={10} fill="none" stroke="#72dfad" style={at(.3+i*.4)} /><circle cx={119+i*120.5} cy={170} r={5} fill={i===3?'#6bdfaa':'#365744'} /><text x={119+i*120.5} y={194} textAnchor="middle" className="cr-caption">{label}</text></g>)}
    <Reveal start={1.8}><rect x={107} y={217} width={386} height={85} rx={12} fill="#2a271a" stroke="#655437" /><Glyph icon={ShieldCheck} x={124} y={233} color="#e8c77d" size={20} /><text x={155} y={248} className="cr-status" fill="#e8c77d">From the security review</text><text x={124} y={278} className="cr-body">SSO approval required before renewal.</text></Reveal>
    <Connection d="M300 302 V337" start={2.8} />
    <Reveal start={3.25}><Panel id={id} x={138} y={337} width={324} height={45} /><Glyph icon={Mic} x={154} y={349} size={20} /><text x={184} y={366} className="cr-muted">Maya · Security review</text></Reveal>
  </>;
}

function Docs({ id }: { id: string }) {
  return <>
    <Panel id={id} x={109} y={34} width={388} height={298} stacked />
    <Glyph icon={FileText} x={132} y={58} /><text x={168} y={79} className="cr-title">Set up SSO with Okta</text>
    <rect x={132} y={101} width={86} height={27} rx={13} fill="#193b2c" /><text x={175} y={120} textAnchor="middle" className="cr-accent">Published</text>
    {['Create a SAML application','Map roles and attributes','Test with your pilot team'].map((label,i)=><Reveal key={label} start={.3+i*.85}><circle cx={144} cy={159+i*45} r={12} fill="#173b2b" /><circle className="cr-draw-ring" cx={144} cy={159+i*45} r={12} fill="none" stroke="#62ce98" strokeWidth={1.2} pathLength={1} /><text x={144} y={165+i*45} textAnchor="middle" className="cr-accent">{i+1}</text><text x={168} y={165+i*45} className="cr-body">{label}</text></Reveal>)}
    <path d="M132 281 H473" stroke="#294236" /><Glyph icon={GitPullRequest} x={132} y={296} size={18} /><text x={161} y={311} className="cr-muted">Updated after PR #728</text>
    <Connection d="M305 332 V358 H257" start={2.5} />
    <Reveal start={3.3}><Glyph icon={Sparkles} x={323} y={349} size={17} /><text x={347} y={363} className="cr-accent">Used in AI reply</text></Reveal>
    <Reveal start={3}><Panel id={id} x={40} y={328} width={217} height={56} /><Avatar x={70} y={356} /><text x={97} y={351} className="cr-body">Shared with Maya</text><text x={97} y={372} className="cr-caption">Northstar Labs</text></Reveal>
  </>;
}

function Timeline({ id }: { id: string }) {
  const items = [
    { icon: Mail, title: 'SSO rollout next steps', detail: 'Email from Maya', time: '09:14' },
    { icon: CalendarDays, title: 'Security & renewal review', detail: 'Meeting · 42 minutes', time: '10:00' },
    { icon: Check, title: 'Okta setup guide shared', detail: 'Helpin AI · Follow-up', time: '11:08' },
  ];
  return <>
    <Panel id={id} x={46} y={35} width={508} height={334} stacked />
    <text x={73} y={74} className="cr-title">Northstar Labs</text><text x={525} y={73} textAnchor="end" className="cr-muted">Email &amp; calendar</text>
    <path d="M94 133 V313" className="cr-wire" fill="none" /><path d="M94 133 V313" className="cr-timeline-progress" pathLength={1} fill="none" stroke="#72dfad" strokeWidth={2} />
    {items.map(({icon,title,detail,time},i)=><Reveal key={title} start={.5+i*1.1}><circle cx={94} cy={133+i*90} r={7} fill="#173b2b" stroke="#64cf9b" /><rect x={117} y={102+i*90} width={412} height={69} rx={12} fill="#14251e" stroke="#2c4438" /><Glyph icon={icon} x={133} y={124+i*90} size={23} /><text x={170} y={129+i*90} className="cr-body">{title}</text><text x={170} y={153+i*90} className="cr-muted">{detail}</text><text x={514} y={129+i*90} textAnchor="end" className="cr-caption">{time}</text></Reveal>)}
  </>;
}

export function CustomerRecordBento({ variant }: { variant: RecordVariant }) {
  const id = `cr-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  const { container, playing, cycle } = useBentoPlayback(variant === 'conversations' ? 30000 : 7500);
  const [title, description] = LABELS[variant];
  const Scene = { conversations: Conversations, meetings: Meetings, projects: Projects, coding: Coding, deals: Deals, docs: Docs, 'email-calendar': Timeline }[variant];
  return <div ref={container} className={`record-bento-art cr-scene cr-${variant}`} data-playing={playing}>
    <svg key={cycle} className="cr-svg" viewBox="0 0 600 400" width={600} height={400} role="img" aria-labelledby={`${id}-title ${id}-description`}>
      <title id={`${id}-title`}>{title}</title><desc id={`${id}-description`}>{description}</desc>
      <defs><linearGradient id={`${id}-surface`} x1="0" y1="0" x2="1" y2="1"><stop stopColor="#182520" /><stop offset="1" stopColor="#0c1511" /></linearGradient></defs>
      <Scene id={id} />
    </svg>
  </div>;
}
