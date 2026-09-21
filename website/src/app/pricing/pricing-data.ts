import { Users, Building2 } from 'lucide-react';
import { SIGNUP_URL } from '../new/_components/ui';

export const PLANS = [
  {
    name: 'Starter',
    price: 99,
    annual: 79,
    description: 'For teams bringing their customer work together.',
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
    description: 'For teams that need more capacity and agent controls.',
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
      'Custom AI agents',
      'Scheduled agents',
      'Advanced automation flows',
      'AI conversation routing',
      'Remove Helpin branding',
      'Priority support',
    ],
  },
];

export const COMPARISON_FEATURES = [
  { name: 'Users & Access', category: true },
  { name: 'Users', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Teams', starter: '10', growth: 'Unlimited' },
  { name: 'Mobile access', starter: true, growth: true },

  { name: 'Projects', category: true },
  { name: 'Tasks & stories', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Epics', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Sprints', starter: true, growth: true },
  { name: 'Board, list, and roadmap views', starter: true, growth: true },
  { name: 'Custom fields', starter: true, growth: true },

  { name: 'Support', category: true },
  { name: 'Live chat widget', starter: true, growth: true },
  { name: 'Shared inbox', starter: true, growth: true },
  { name: 'Team inboxes', starter: true, growth: true },
  { name: 'Saved replies', starter: true, growth: true },
  { name: 'Email forwarding', starter: true, growth: true },
  { name: 'Round-robin assignment', starter: false, growth: true },
  { name: 'SLA management', starter: false, growth: true },
  { name: 'AI conversation routing', starter: false, growth: true },

  { name: 'CRM', category: true },
  { name: 'Contacts', starter: '5,000', growth: 'Unlimited' },
  { name: 'Deals and pipelines', starter: true, growth: true },
  { name: 'Gmail sync', starter: true, growth: true },
  { name: 'Buyer signal detection', starter: true, growth: true },
  { name: 'Deal automation', starter: false, growth: true },

  { name: 'Knowledge', category: true },
  { name: 'Documents', starter: '500', growth: 'Unlimited' },
  { name: 'Internal docs', starter: true, growth: true },
  { name: 'Public help center', starter: true, growth: true },
  { name: 'Custom domain', starter: true, growth: true },
  { name: 'AI article translation', starter: false, growth: true },

  { name: 'Meetings', category: true },
  { name: 'Notes, summaries and action items', starter: true, growth: true },

  { name: 'AI Agents', category: true },
  { name: 'Included AI usage', starter: 'Standard monthly allowance', growth: '3× Starter allowance' },
  { name: 'Built-in agents', starter: true, growth: true },
  { name: 'Custom agents', starter: false, growth: true },
  { name: 'Agent scheduling', starter: false, growth: true },
  { name: 'Extra AI usage', starter: 'Exact metered usage', growth: 'Exact metered usage' },

  { name: 'Platform', category: true },
  { name: 'GitHub integration', starter: true, growth: true },
  { name: 'Import tools', starter: true, growth: true },
  { name: 'Module access controls', starter: true, growth: true },
  { name: 'Automation flows', starter: false, growth: true },

  { name: 'Branding', category: true },
  { name: 'Widget branding', starter: 'Helpin', growth: 'Removed' },

  { name: 'Customer service', category: true },
  { name: 'Standard support', starter: true, growth: true },
  { name: 'Priority support', starter: false, growth: true },
];

export const FAQS = [
  { q: 'Can we use Helpin without a Cloud subscription?', a: 'Yes. Self-host the open-source product on your own infrastructure. You cover hosting and provider costs; Enterprise features are licensed separately.' },
  { q: 'What is the difference between open source and Enterprise?', a: 'The product modules are open source. Code in ee/ directories, including subscription billing, managed AI routes and commercial policies, uses a separate Enterprise license.' },
  {
    q: 'Do I need to buy modules separately?',
    a: 'No. Both Cloud plans include support, projects, CRM, meetings and knowledge. Capacity, automation and agent controls vary by plan.',
  },
  {
    q: 'What happens if I use all of my included AI usage?',
    a: 'Paid workspaces can allow extra AI usage. Only exact usage beyond the allowance is settled each month, before applicable taxes — there are no prepaid blocks.',
  },
  {
    q: 'How is AI usage measured?',
    a: 'Usage reflects the model size and the amount of AI work completed. Routine tasks use the allowance more slowly than demanding planning, coding, and review work. Settings shows consumption as a simple percentage.',
  },
  {
    q: 'Can I switch plans anytime?',
    a: 'You can change Cloud plans in billing settings. Review the effective date and any billing adjustment before confirming.',
  },
  {
    q: 'Do you offer annual billing?',
    a: 'Yes. Starter is $99/month or $948/year. Growth is $299/month or $2,868/year.',
  },
  {
    q: 'Is a credit card required for the trial?',
    a: 'No. New workspaces start on a no-card 14-day Growth trial. Add billing only when you are ready to keep using the workspace.',
  },
  {
    q: 'What AI models does Helpin support?',
    a: 'Helpin supports models from multiple providers. Cloud includes managed AI access; self-hosted installations use the providers you configure.',
  },
  {
    q: 'How does pricing compare to my current tools?',
    a: 'Compare the tools and workflows your team actually needs. Helpin combines customer work in one product; your savings depend on the subscriptions you replace and the integrations you keep.',
  },
  {
    q: 'Does unused AI usage roll over?',
    a: 'No. Cloud AI allowances reset each month on your renewal date, including on annual subscriptions.',
  },
  {
    q: 'How does per-workspace pricing work?',
    a: 'Each Cloud workspace gets its own plan and billing. Create a workspace, start with a 14-day Growth trial, then choose Starter or Growth. If you do not upgrade, workspace access is limited until you choose a paid plan.',
  },
  {
    q: 'Are seats really unlimited on paid plans?',
    a: 'Yes. Starter and Growth both include unlimited seats. We do not charge per seat — your whole team gets access.',
  },
  {
    q: 'Is my data safe?',
    a: 'Access follows workspace and resource permissions. Review our privacy policy and deployment documentation, or self-host to operate the data and infrastructure yourself.',
  },
];

export const WORKFLOW_COMPARISON = [
  { tool: 'Jira / Linear / ClickUp', workflow: 'Project planning', domain: 'linear.app' },
  { tool: 'Intercom / Zendesk / Freshdesk', workflow: 'Customer support', domain: 'intercom.com' },
  { tool: 'HubSpot / Salesforce / Pipedrive', workflow: 'Customer relationships', domain: 'hubspot.com' },
  { tool: 'Notion / Confluence / Slite', workflow: 'Knowledge and docs', domain: 'notion.so' },
  { tool: 'ChatGPT / Claude / Copilot', workflow: 'AI assistance', domain: 'openai.com' },
];

