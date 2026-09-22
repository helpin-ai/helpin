import { Users, Building2 } from 'lucide-react';
import { SIGNUP_URL } from '../new/_components/ui';

export const PLANS = [
  {
    name: 'Starter',
    price: 99,
    annual: 79,
    description: 'A shared starting point for support, product, and customer teams.',
    Icon: Users,
    seats: 'Unlimited',
    aiUsage: 'Standard',
    cta: 'Start free trial',
    href: `${SIGNUP_URL}?plan=starter`,
    popular: false,
    features: [
      'Support, projects, CRM, meetings and knowledge',
      'Unlimited users',
      '10 teams',
      '500 documents',
      '5,000 contacts',
      'Built-in AI agents',
      'GitHub integration',
      'Public help center + custom domain',
    ],
  },
  {
    name: 'Growth',
    price: 299,
    annual: 239,
    description: 'For teams ready to build repeatable workflows around their agents.',
    Icon: Building2,
    seats: 'Unlimited',
    aiUsage: '3× Starter',
    cta: 'Start free trial',
    href: `${SIGNUP_URL}?plan=growth`,
    popular: true,
    features: [
      'Everything in Starter, plus:',
      'Unlimited teams',
      'Unlimited documents',
      'Unlimited contacts',
      'Create agents for your process.',
      'Run them on a schedule.',
      'Connect the steps through automation.',
      'AI conversation routing',
      'Remove Helpin branding',
      'Priority support',
    ],
  },
];

export const COMPARISON_FEATURES = [
  { name: 'Your team', category: true },
  { name: 'Users', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Teams', starter: '10', growth: 'Unlimited' },
  { name: 'Mobile access', starter: true, growth: true },

  { name: 'Planning and delivery', category: true },
  { name: 'Tasks & stories', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Epics', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Sprints', starter: true, growth: true },
  { name: 'Board, list, and roadmap views', starter: true, growth: true },
  { name: 'Custom fields', starter: true, growth: true },

  { name: 'Customer support', category: true },
  { name: 'Live chat widget', starter: true, growth: true },
  { name: 'Shared inbox', starter: true, growth: true },
  { name: 'Team inboxes', starter: true, growth: true },
  { name: 'Saved replies', starter: true, growth: true },
  { name: 'Email forwarding', starter: true, growth: true },
  { name: 'Round-robin assignment', starter: false, growth: true },
  { name: 'SLA management', starter: false, growth: true },
  { name: 'AI conversation routing', starter: false, growth: true },

  { name: 'Customer relationships', category: true },
  { name: 'Contacts', starter: '5,000', growth: 'Unlimited' },
  { name: 'Deals and pipelines', starter: true, growth: true },
  { name: 'Gmail sync', starter: true, growth: true },
  { name: 'Buyer signal detection', starter: true, growth: true },
  { name: 'Deal automation', starter: false, growth: true },

  { name: 'Docs and help center', category: true },
  { name: 'Documents', starter: '500', growth: 'Unlimited' },
  { name: 'Internal docs', starter: true, growth: true },
  { name: 'Public help center', starter: true, growth: true },
  { name: 'Custom domain', starter: true, growth: true },
  { name: 'AI article translation', starter: false, growth: true },

  { name: 'Meetings', category: true },
  { name: 'Notes, summaries and action items', starter: true, growth: true },

  { name: 'Agent capabilities and usage', category: true },
  { name: 'Included AI usage', starter: 'Standard monthly allowance', growth: '3× Starter allowance' },
  { name: 'Built-in agents', starter: true, growth: true },
  { name: 'Custom agents', starter: false, growth: true },
  { name: 'Agent scheduling', starter: false, growth: true },
  { name: 'Extra AI usage', starter: 'Exact metered usage', growth: 'Exact metered usage' },

  { name: 'Automation and connections', category: true },
  { name: 'GitHub integration', starter: true, growth: true },
  { name: 'Import tools', starter: true, growth: true },
  { name: 'Module access controls', starter: true, growth: true },
  { name: 'Automation flows', starter: false, growth: true },

  { name: 'Your customer-facing experience', category: true },
  { name: 'Widget branding', starter: 'Helpin', growth: 'Removed' },

  { name: 'Help from our team', category: true },
  { name: 'Standard support', starter: true, growth: true },
  { name: 'Priority support', starter: false, growth: true },
];

export const FAQS = [
  {
    "q": "Can we use Helpin without a Cloud subscription?",
    "a": "Yes. Operate the open-source edition yourself and configure the services it uses. Your team remains responsible for the installation and its operating costs."
  },
  {
    "q": "How does the open-source edition differ from Enterprise?",
    "a": "The product modules are open source. Certain capabilities use a separate Enterprise license. Check the licensing terms for the functionality you intend to deploy."
  },
  {
    "q": "Do we buy each product module separately?",
    "a": "No. Modules are bundled; capacity and advanced features differ."
  },
  {
    "q": "What happens when our AI allowance runs out?",
    "a": "Without overage enabled, new paid AI work is blocked when the remaining allowance cannot cover it. Wait for the monthly renewal or enable optional metered overage. Review the rates in AI usage and charges before enabling it."
  },
  {
    "q": "What determines AI consumption?",
    "a": "The model and work performed. Settings displays usage."
  },
  {
    "q": "Can we change Cloud plans?",
    "a": "Use billing settings; check timing and adjustments before confirming."
  },
  {
    "q": "Is annual billing available?",
    "a": "Yes. The plan cards show yearly totals."
  },
  {
    "q": "Do we need a card for the trial?",
    "a": "No. Try Cloud without adding a card on a 14-day Growth trial. After the trial, choose Starter or Growth to continue. Until you choose a paid plan, workspace access is limited."
  },
  {
    "q": "Can we choose the models our agents use?",
    "a": "Configure the model for the agent’s work, along with its instructions, tools, and approval settings. Check which connections are available in your deployment."
  },
  {
    "q": "Will Helpin cost less than our current tools?",
    "a": "Compare the subscriptions, integrations, and operating work you would actually replace. A useful evaluation should show whether Helpin improves your workflow as well as how the costs compare."
  },
  {
    "q": "Does unused AI allowance carry forward?",
    "a": "No; it renews monthly."
  },
  {
    "q": "What does “per workspace” mean?",
    "a": "Each workspace has separate billing."
  },
  {
    "q": "Will adding teammates increase our subscription?",
    "a": "No seat-based charge applies."
  },
  {
    "q": "How should we evaluate access and data handling?",
    "a": "Review workspace permissions, agent tool access, and the connected services your workflows use. For self-hosting, also review the infrastructure and operational responsibilities your team will take on."
  }
];

export const WORKFLOW_COMPARISON = [
  { tool: 'Jira / Linear / ClickUp', workflow: 'Project planning', description: 'Keep the reason behind the task within reach.', domain: 'linear.app' },
  { tool: 'Intercom / Zendesk / Freshdesk', workflow: 'Customer support', description: 'Answer with the earlier conversation in view.', domain: 'intercom.com' },
  { tool: 'HubSpot / Salesforce / Pipedrive', workflow: 'Customer relationships', description: 'See what needs attention before the next call.', domain: 'hubspot.com' },
  { tool: 'Notion / Confluence / Slite', workflow: 'Knowledge and docs', description: 'Turn what you learn into guidance others can use.', domain: 'notion.so' },
  { tool: 'ChatGPT / Claude / Copilot', workflow: 'AI assistance', description: 'Give the next action more than the latest message.', domain: 'openai.com' },
];

