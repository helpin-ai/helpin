import { BookOpen, Bot, CalendarDays, Check, Database, FileCheck2, HardDrive, Kanban, Layers3, MessagesSquare, Network, Server, Users, Workflow } from 'lucide-react';

const MODULES = [
  { Icon: MessagesSquare, label: 'Support' }, { Icon: Kanban, label: 'Projects' },
  { Icon: Users, label: 'CRM' }, { Icon: CalendarDays, label: 'Meetings' },
  { Icon: BookOpen, label: 'Knowledge' }, { Icon: Bot, label: 'Agents' },
];

/** Static, resolution-independent illustrations; no simulated settings or live deployment. */
export function SelfHostingArt() {
  return <div className="pricing-deployment-art pricing-host-art" role="img" aria-label="Helpin’s six product modules connected to a database, agent runtime, and file storage inside your own infrastructure."><div aria-hidden="true">
    <div className="pricing-infra-heading"><Server size={15} /><span>The workspace you operate</span><span className="pricing-infra-boundary">Hosted by your team</span></div>
    <div className="pricing-infra-boundary-box">
      <div className="pricing-infra-modules">{MODULES.map(({Icon,label}) => <div key={label}><Icon size={19} strokeWidth={1.5} /><span>{label}</span></div>)}</div>
      <div className="pricing-infra-spine" />
      <div className="pricing-infra-core"><span><img src="/brand/helpin-icon-white.svg" width={29} height={29} alt="" /></span><div><strong>Helpin</strong><small>Customer history and the work behind it.</small></div><Layers3 size={20} /></div>
      <svg className="pricing-infra-wires" viewBox="0 0 480 48" preserveAspectRatio="none"><path d="M240 0V22H80V48M240 22V48M240 22H400V48" /><circle cx="240" cy="22" r="3" /></svg>
      <div className="pricing-infra-services">{[{Icon:Database,title:'Database',detail:'PostgreSQL'},{Icon:Workflow,title:'Agent runtime',detail:'Agent execution'},{Icon:HardDrive,title:'File storage',detail:'S3-compatible'}].map(({Icon,title,detail}) => <div key={title}><Icon size={20} strokeWidth={1.5} /><strong>{title}</strong><small>{detail}</small></div>)}</div>
      <div className="pricing-infra-footer"><span><i />Your data</span><span><i />Services you choose to connect</span><span><i />Your deployment</span></div>
    </div>
  </div></div>;
}

export function EnterprisePlanningArt() {
  return <div className="pricing-deployment-art pricing-enterprise-art" role="img" aria-label="An Enterprise deployment brief brings together your infrastructure requirements, commercial licensing, and rollout plan for a conversation with the Helpin team."><div aria-hidden="true">
    <div className="pricing-brief-back" />
    <div className="pricing-brief">
      <div className="pricing-brief-top"><span><img src="/brand/helpin-icon-ink.svg" width={22} height={22} alt="" />Helpin</span><span>Enterprise</span></div>
      <div className="pricing-brief-heading"><small>LET’S PLAN IT TOGETHER</small><strong>Your deployment brief</strong><p>A practical starting point for the conversation.</p></div>
      <div className="pricing-brief-items">{[{Icon:Network,title:'Your environment',detail:'Where Helpin will run and which systems it needs to connect.'},{Icon:FileCheck2,title:'Your requirements',detail:'The capabilities, access, and operating arrangements your team needs.'},{Icon:Users,title:'Your rollout',detail:'Who will use the workspace, where to begin, and what needs to happen before launch.'}].map(({Icon,title,detail},i) => <div key={title}><span><Icon size={19} strokeWidth={1.5} /></span><div><strong>{title}</strong><small>{detail}</small></div><span className="pricing-brief-number">0{i+1}</span></div>)}</div>
      <div className="pricing-brief-footer"><span className="pricing-brief-team"><img src="/brand/helpin-icon-white.svg" width={18} height={18} alt="" /></span><span><strong>Talk it through with Helpin</strong><small>Agree on the requirements before the rollout.</small></span><Check size={16} /></div>
    </div>
  </div></div>;
}
