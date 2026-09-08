export const ME = 'Muhammad Azhar';
export const people = [ME, 'Sara Ahmed', 'Daniel Kim', 'Omar Farooq'];
export const motions = ['Prospecting', 'Conversion', 'Onboarding', 'Adoption', 'Expansion', 'Renewal', 'Retention'];
export type View = 'signals' | 'playbooks' | 'playbook' | 'edit' | 'customer' | 'flows' | 'agents' | 'activity';
export type Scene = 'Populated' | 'Empty' | 'Loading' | 'Error' | 'Read-only';
export type WorkStatus = 'Needs approval' | 'Needs attention' | 'Delivery failed' | 'Waiting' | 'Paused' | 'Resolved';
export type Step = { name: string; actor: 'Agent' | 'Person' | 'Approval' | 'Wait'; detail: string };
export type CRMChange = {kind:'Create deal'|'Change stage'|'Enrich contact';target:string;fields:{label:string;before:string;after:string}[];applied?:boolean};
export type Playbook = {
  id: string; name: string; category: string; description: string; goal: string; owner: string;
  enabled: boolean; version: number; trigger: string; audience: string; approval: string;
  delay: string; stop: string; steps: Step[]; template?: string;
};
export type Situation = {
  id: string; company: string; title: string; category: string; owner: string; actionOwner: string;
  status: WorkStatus; priority: 'High' | 'Normal'; next: string; playbook: string | null;
  kind: 'buyer' | 'handoff' | 'renewal' | 'manual'; quote: string; author: string; source: string;
  time: string; draft: string; goal: string; progress: number; history: string[];
  reviewed: boolean; task?: string; taskOwner?: string; taskDue?: string; outcome?: string; previousStatus?: WorkStatus; configuration?: Playbook; change?:CRMChange; renewalProgress?:{blocker:string;renewal:string};
};

export const playbookSeed: Playbook[] = [
  { id: 'buying-intent', name: 'Buying-intent follow-up', category: 'Sales', description: 'Turn a customer’s interest into an agreed next step.',
    goal: 'A qualified next step is agreed with the customer.', owner: ME, enabled: true, version: 3,
    trigger: 'A qualifying buying-intent signal appears', audience: 'Known contacts with buying intent', approval: 'Situation owner', delay: '2 business days', stop: 'Customer replies, opts out, or the opportunity is closed',
    steps: [
      { name: 'Understand the request', actor: 'Agent', detail: 'Review the conversation, deal, and existing commitments.' },
      { name: 'Prepare the next step', actor: 'Agent', detail: 'Draft a relevant response. Suggest a deal update only when needed.' },
      { name: 'Review and send', actor: 'Approval', detail: 'The owner can edit the exact response before approving it.' },
      { name: 'Wait for the customer', actor: 'Wait', detail: 'A reply cancels the scheduled nudge. Check again after 2 business days.' },
      { name: 'Confirm the next step', actor: 'Person', detail: 'Record the agreed next step or why the opportunity is not being pursued.' },
    ] },
  { id: 'customer-handoff', name: 'Sales-to-success handoff', category: 'Customer success', description: 'Carry customer goals and promises into the right hands.',
    goal: 'The success owner accepts a complete handoff and first-value goal.', owner: 'Sara Ahmed', enabled: true, version: 2,
    trigger: 'A deal is marked closed won', audience: 'New customers with a closed-won deal', approval: 'Customer success owner', delay: '1 business day', stop: 'Handoff accepted, or the deal is reopened',
    steps: [
      { name: 'Capture the customer’s goals', actor: 'Agent', detail: 'Bring together sold scope, stakeholders, promises, and open dependencies.' },
      { name: 'Prepare the handoff', actor: 'Agent', detail: 'Flag missing details. Reuse existing onboarding work where relevant.' },
      { name: 'Confirm the receiving owner', actor: 'Approval', detail: 'Sales keeps responsibility until the success owner accepts.' },
      { name: 'Agree the first-value milestone', actor: 'Person', detail: 'Record the customer’s desired result and agreed timing.' },
      { name: 'Accept the handoff', actor: 'Person', detail: 'Finish the handoff without claiming that onboarding is complete.' },
    ] },
  { id: 'renewal-recovery', name: 'Renewal-risk recovery', category: 'Customer success', description: 'Address the real blocker and keep the renewal moving.',
    goal: 'The customer’s blocker is resolved and renewal is confirmed.', owner: 'Sara Ahmed', enabled: true, version: 4,
    trigger: 'Renewal is approaching or a credible risk appears', audience: 'Active customers with a renewal in the next 60 days', approval: 'Customer success owner', delay: '1 business day', stop: 'Renewal confirmed, contract ended, or the process is manually closed',
    steps: [
      { name: 'Understand the renewal risk', actor: 'Agent', detail: 'Review the customer’s concern, contract, and relevant support context.' },
      { name: 'Prepare a recovery response', actor: 'Agent', detail: 'Explain the next step using verified commitments, not promises.' },
      { name: 'Review the customer response', actor: 'Approval', detail: 'Get the responsible person’s approval before contacting the customer.' },
      { name: 'Follow the blocker through', actor: 'Wait', detail: 'Link relevant work. Recheck when its status or the customer’s response changes.' },
      { name: 'Confirm the renewal outcome', actor: 'Person', detail: 'Track blocker resolution separately from confirmed renewal.' },
    ] },
];

export const situationSeed: Situation[] = [
  { id: 'northstar-risk', company: 'Northstar', title: 'Integration issue is blocking renewal', category: 'Retention', owner: 'Sara Ahmed', actionOwner: ME,
    status: 'Needs approval', priority: 'High', next: 'Your approval · Recovery response ready', playbook: 'renewal-recovery', kind: 'renewal',
    quote: 'We need the sync issue resolved before we can commit to another year. Can you send us a recovery plan by September 8?', author: 'Anna Lee', source: 'Support conversation', time: 'Today, 9:14 AM',
    draft: 'Hi Anna,\n\nI understand the sync issue is holding up your renewal. Omar is investigating it, and I’ll send you a confirmed recovery plan by September 8.\n\nI’ll keep you updated here as we verify the fix.\n\nMuhammad',
    goal: 'Resolve the sync blocker and confirm the September renewal.', progress: 2, reviewed: false, task: 'ENG-248 · Investigate CRM sync failures',
    history: ['Recovery response prepared · 9:18 AM', 'Your approval requested by Sara · 9:16 AM', 'Customer raised a renewal blocker · 9:14 AM'] },
  { id: 'harbor-buyer', company: 'Harbor', title: 'Pricing requested for a 40-seat rollout', category: 'Conversion', owner: ME, actionOwner: ME,
    status: 'Needs approval', priority: 'High', next: 'Review response and proposed next step', playbook: 'buying-intent', kind: 'buyer',
    quote: 'We’re looking at 40 seats for our operations team. Could you walk us through the options and help us arrange a demo next week?', author: 'James Wilson', source: 'Email', time: 'Today, 8:42 AM',
    draft: 'Hi James,\n\nHappy to help you explore a 40-seat rollout. Let’s use a demo to understand your operations workflow and walk through the right plan together.\n\nWould Tuesday at 2 PM work for your team?\n\nMuhammad',
    goal: 'Agree a discovery call with the operations buying team.', progress: 2, reviewed: false,
    history: ['Response prepared for your review · 8:46 AM', 'Matched to buying-intent follow-up · 8:43 AM', 'James requested a demo and pricing · 8:42 AM'] },
  { id: 'lumen-handoff', company: 'Lumen', title: 'Sales handoff is ready to accept', category: 'Onboarding', owner: 'Daniel Kim', actionOwner: ME,
    status: 'Needs approval', priority: 'High', next: 'Your acceptance · Customer goals captured', playbook: 'customer-handoff', kind: 'handoff',
    quote: 'Our first goal is to launch the support workspace for all 12 agents by September 18. We’ll need help importing our existing knowledge base.', author: 'Maya Chen', source: 'Sales call', time: 'Yesterday, 3:30 PM',
    draft: '', goal: 'Launch the support workspace for 12 agents by September 18.', progress: 2, reviewed: false,
    history: ['Handoff prepared with two commitments · Today, 8:20 AM', 'Daniel requested your acceptance · Today, 8:18 AM', 'Annual agreement signed · Yesterday, 4:10 PM'] },
  { id: 'orbit-delivery', company: 'Orbit', title: 'Follow-up could not be delivered', category: 'Conversion', owner: ME, actionOwner: ME,
    status: 'Delivery failed', priority: 'High', next: 'Email connection needs your attention', playbook: 'buying-intent', kind: 'buyer',
    quote: 'Could you share a few times for a product walkthrough? Our team is available next week.', author: 'Leo Park', source: 'Email', time: 'Yesterday, 2:05 PM',
    draft: 'Hi Leo,\n\nWe’d be happy to show you around. Would Wednesday at 11 AM work for your team?\n\nMuhammad',
    goal: 'Arrange a product walkthrough.', progress: 2, reviewed: true,
    history: ['Send stopped: email connection expired. No message was sent · 9:02 AM', 'Response approved by you · 9:01 AM', 'Response prepared · 8:55 AM'] },
  { id: 'beacon-owner', company: 'Beacon', title: 'Additional seats need an owner', category: 'Expansion', owner: '', actionOwner: '',
    status: 'Needs attention', priority: 'Normal', next: 'Assign someone to review the request', playbook: null, kind: 'manual',
    quote: 'We’re adding five people to the marketing team. Who should we speak with about getting them access?', author: 'Nina Patel', source: 'Email', time: 'Today, 10:06 AM', draft: '',
    goal: 'Help the new marketing teammates get access.', progress: 0, reviewed: false, history: ['No account owner found · 10:07 AM', 'Customer requested five additional seats · 10:06 AM'] },
  { id: 'northstar-expansion', company: 'Northstar', title: '20 more seats requested', category: 'Expansion', owner: ME, actionOwner: '',
    status: 'Waiting', priority: 'Normal', next: 'Outreach held while renewal risk is open', playbook: 'buying-intent', kind: 'buyer',
    quote: 'Our new operations group may need 20 more seats. Can we discuss this once the sync issue is sorted?', author: 'Anna Lee', source: 'Email', time: 'Yesterday, 11:20 AM', draft: '',
    goal: 'Understand the operations team’s expansion needs.', progress: 1, reviewed: true, history: ['Expansion outreach held by contact policy · Today, 9:16 AM', 'Anna asked about 20 more seats · Yesterday, 11:20 AM'] },
  { id: 'atlas-waiting', company: 'Atlas', title: 'Waiting for kickoff availability', category: 'Onboarding', owner: ME, actionOwner: '',
    status: 'Waiting', priority: 'Normal', next: 'Customer reply · Check September 8', playbook: 'customer-handoff', kind: 'handoff',
    quote: 'Please give us a couple of days to get the right people together for the kickoff.', author: 'Sophie Martin', source: 'Email', time: 'Yesterday, 10:15 AM', draft: '',
    goal: 'Agree the first-value milestone with the customer team.', progress: 3, reviewed: true, history: ['Next check scheduled for September 8', 'Customer asked for time to coordinate · Yesterday, 10:15 AM'] },
  { id: 'ember-renewed', company: 'Ember', title: 'Renewal confirmed for another year', category: 'Renewal', owner: 'Sara Ahmed', actionOwner: '',
    status: 'Resolved', priority: 'Normal', next: 'Renewal confirmed · September 4', playbook: 'renewal-recovery', kind: 'renewal',
    quote: 'The reporting issue is resolved. We’ve signed the renewal and are happy to continue for another year.', author: 'Isabel Cruz', source: 'Email', time: 'Yesterday, 4:20 PM', draft: '',
    goal: 'Resolve the reporting blocker and confirm renewal.', progress: 5, reviewed: true, outcome: 'Blocker resolved and renewal confirmed by the customer.', history: ['Renewal confirmed · September 4', 'Customer verified the fix · September 3'] },
  { id: 'acme-agreed', company: 'Acme', title: 'Discovery call agreed with the buying team', category: 'Conversion', owner: ME, actionOwner: '',
    status: 'Resolved', priority: 'Normal', next: 'Next step agreed · September 4', playbook: 'buying-intent', kind: 'buyer',
    quote: 'Tuesday at 2 works. I’ve invited our head of operations and IT lead so we can cover everyone’s questions.', author: 'Ben Taylor', source: 'Email', time: 'Yesterday, 1:15 PM', draft: '',
    goal: 'Agree a discovery call with the buying team.', progress: 5, reviewed: true, outcome: 'Discovery call agreed with operations and IT.', history: ['Next step confirmed from customer reply · September 4', 'Response sent after owner approval · September 3'] },
  { id: 'fern-handoff', company: 'Fern', title: 'Customer handoff accepted', category: 'Onboarding', owner: ME, actionOwner: '',
    status: 'Resolved', priority: 'Normal', next: 'Handoff accepted · September 3', playbook: 'customer-handoff', kind: 'handoff',
    quote: 'The onboarding goal and scope are clear. I’ve accepted the handoff and will coordinate the next customer session.', author: ME, source: 'Handoff decision', time: 'September 3', draft: '',
    goal: 'Transfer the customer’s goals and commitments to their success owner.', progress: 5, reviewed: true, outcome: 'Handoff accepted. Onboarding is still in progress.', history: ['Handoff accepted by you · September 3', 'Customer goals and commitments confirmed · September 3'] },
];

situationSeed.push(
  {id:'cedar-deal',company:'Cedar',title:'A new opportunity is ready to review',category:'Prospecting',owner:ME,actionOwner:ME,status:'Needs approval',priority:'Normal',next:'Create deal · Review the proposed fields',playbook:null,kind:'manual',quote:'We’re comparing options for our customer operations team and would like to discuss your product.',author:'Alex Morgan',source:'Email',time:'Today, 10:20 AM',draft:'',goal:'Qualify the customer’s needs and agree whether to pursue the opportunity.',progress:0,reviewed:false,history:['Deal creation suggested from the customer conversation · 10:24 AM'],change:{kind:'Create deal',target:'Cedar · Customer operations',fields:[{label:'Deal name',before:'No deal',after:'Cedar · Customer operations'},{label:'Pipeline',before:'Not set',after:'New business'},{label:'Stage',before:'Not set',after:'Discovery'}]}},
  {id:'aster-stage',company:'Aster',title:'The buying team has started evaluation',category:'Conversion',owner:'Daniel Kim',actionOwner:ME,status:'Needs approval',priority:'Normal',next:'Change deal stage · Discovery → Evaluation',playbook:null,kind:'manual',quote:'The demo covered what we needed. We’d like to start an evaluation with our IT team next week.',author:'Sam Rivera',source:'Email',time:'Today, 9:50 AM',draft:'',goal:'Support the buying team through their evaluation.',progress:0,reviewed:false,history:['Stage change proposed for your approval · 9:53 AM'],change:{kind:'Change stage',target:'Aster · Team rollout',fields:[{label:'Stage',before:'Discovery',after:'Evaluation'}]}},
  {id:'beacon-contact',company:'Beacon',title:'A customer contact has a new role',category:'Expansion',owner:ME,actionOwner:ME,status:'Needs approval',priority:'Normal',next:'Update contact · Confirm the source and change',playbook:null,kind:'manual',quote:'I’m now leading the marketing operations team. Please use my new title on the account.',author:'Nina Patel',source:'Email',time:'Today, 9:35 AM',draft:'',goal:'Keep the customer’s stakeholder information accurate.',progress:0,reviewed:false,history:['Contact update suggested from Nina’s message · 9:38 AM'],change:{kind:'Enrich contact',target:'Nina Patel · Beacon',fields:[{label:'Job title',before:'Marketing Manager',after:'Head of Marketing Operations'}]}}
);
export const customerNames = ['Northstar', 'Harbor', 'Lumen', 'Orbit', 'Beacon', 'Atlas', 'Ember', 'Acme', 'Fern', 'Aster', 'Cedar'];
export const isAttention = (s: Situation) => ['Needs approval', 'Needs attention', 'Delivery failed'].includes(s.status);
export const ownerName = (name: string) => name === ME ? 'You' : name || 'Unassigned';
export const playbookFor = (id: string | null, books: Playbook[]) => books.find(p => p.id === id);
export const initialSituations = ():Situation[] => situationSeed.map(s=>({...structuredClone(s),configuration:structuredClone(playbookSeed.find(b=>b.id===s.playbook))}));
