import { AI_PRICING } from '@/generated/aiPricing';
import { SIGNUP_URL } from '../(site)/_components/ui';

type PlanKey = 'starter' | 'growth';
type Interval = 'monthly' | 'annual' | 'trial';

const allowance = (plan: PlanKey, interval: Interval) =>
  (AI_PRICING.plans.find(item => item.plan === plan && item.billing_interval === interval)?.allowance_microusd ?? 0) / 1_000_000;

export const AI_ALLOWANCE = {
  starter: { monthly: allowance('starter', 'monthly'), annual: allowance('starter', 'annual') },
  growth: { monthly: allowance('growth', 'monthly'), annual: allowance('growth', 'annual') },
  trial: allowance('growth', 'trial'),
};

export const PLANS = [
  {
    key: 'starter' as const,
    name: 'Starter',
    price: 99,
    annual: 79,
    description: 'Every module, hosted by us, with room to grow.',
    cta: 'Start free trial',
    href: `${SIGNUP_URL}?plan=starter`,
    popular: false,
    featuresHeading: 'Includes',
    features: [
      'Support, projects, CRM, meetings, and knowledge',
      '10 teams',
      '5,000 contacts',
      '500 documents',
      'Built-in AI agents',
      'GitHub integration',
      'Public help center on your domain',
    ],
  },
  {
    key: 'growth' as const,
    name: 'Growth',
    price: 299,
    annual: 239,
    description: 'For teams automating their process with custom agents.',
    cta: 'Start free trial',
    href: `${SIGNUP_URL}?plan=growth`,
    popular: true,
    featuresHeading: 'Everything in Starter, plus',
    features: [
      'Unlimited teams, contacts, and documents',
      'Custom agents',
      'Scheduled agent runs',
      'Automation flows',
      'AI conversation routing',
      'Round-robin assignment',
      'Multilingual help center',
      'No Helpin branding on the widget',
      'Priority support',
    ],
  },
];

export const COMPARISON_FEATURES = [
  { name: 'Hosting', category: true },
  { name: 'Hosting and updates', selfHosted: 'Your team', starter: 'Helpin', growth: 'Helpin' },
  { name: 'Your team', category: true },
  { name: 'Users', selfHosted: 'Unlimited', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Teams', selfHosted: 'Unlimited', starter: '10', growth: 'Unlimited' },
  { name: 'Mobile web app', selfHosted: true, starter: true, growth: true },

  { name: 'Planning and delivery', category: true },
  { name: 'Tasks & stories', selfHosted: 'Unlimited', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Epics', selfHosted: 'Unlimited', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Sprints', selfHosted: true, starter: true, growth: true },
  { name: 'Board, list, and roadmap views', selfHosted: true, starter: true, growth: true },
  { name: 'Custom fields', selfHosted: true, starter: true, growth: true },

  { name: 'Customer support', category: true },
  { name: 'Live chat widget', selfHosted: true, starter: true, growth: true },
  { name: 'Shared inbox', selfHosted: true, starter: true, growth: true },
  { name: 'Team inboxes', selfHosted: true, starter: true, growth: true },
  { name: 'Saved replies', selfHosted: true, starter: true, growth: true },
  { name: 'Email forwarding', selfHosted: true, starter: true, growth: true },
  { name: 'Round-robin assignment', selfHosted: true, starter: false, growth: true },
  { name: 'AI conversation routing', selfHosted: true, starter: false, growth: true },

  { name: 'Customer relationships', category: true },
  { name: 'Contacts', selfHosted: 'Unlimited', starter: '5,000', growth: 'Unlimited' },
  { name: 'Deals and pipelines', selfHosted: true, starter: true, growth: true },
  { name: 'Gmail sync', selfHosted: true, starter: true, growth: true },
  { name: 'Buyer signal detection', selfHosted: true, starter: true, growth: true },
  { name: 'Deal automation', selfHosted: true, starter: false, growth: true },

  { name: 'Docs and help center', category: true },
  { name: 'Documents', selfHosted: 'Unlimited', starter: '500', growth: 'Unlimited' },
  { name: 'Internal docs', selfHosted: true, starter: true, growth: true },
  { name: 'Public help center', selfHosted: true, starter: true, growth: true },
  { name: 'Custom domain', selfHosted: true, starter: true, growth: true },
  { name: 'Multilingual help center', selfHosted: true, starter: false, growth: true },
  { name: 'AI article translation', selfHosted: true, starter: false, growth: true },

  { name: 'Meetings', category: true },
  { name: 'Notes, summaries and action items', selfHosted: true, starter: true, growth: true },

  { name: 'AI agents', category: true },
  { name: 'Included AI usage per month', selfHosted: 'Your provider keys', starter: `$${AI_ALLOWANCE.starter.monthly}`, growth: `$${AI_ALLOWANCE.growth.monthly}` },
  { name: 'Your own AI provider keys', selfHosted: true, starter: false, growth: false },
  { name: 'Built-in agents', selfHosted: true, starter: true, growth: true },
  { name: 'Custom agents', selfHosted: true, starter: false, growth: true },
  { name: 'Agent scheduling', selfHosted: true, starter: false, growth: true },
  { name: 'Extra AI usage', selfHosted: 'Paid to your provider', starter: 'Metered, opt-in', growth: 'Metered, opt-in' },

  { name: 'Automation and integrations', category: true },
  { name: 'GitHub integration', selfHosted: true, starter: true, growth: true },
  { name: 'Import tools', selfHosted: true, starter: true, growth: true },
  { name: 'Module access controls', selfHosted: true, starter: true, growth: true },
  { name: 'Automation flows', selfHosted: true, starter: false, growth: true },

  { name: 'Branding', category: true },
  { name: 'Widget branding', selfHosted: 'Removed', starter: 'Helpin', growth: 'Removed' },

  { name: 'Help and support', category: true },
  { name: 'Support', selfHosted: 'Community', starter: 'Standard', growth: 'Priority' },
];

export const FAQS = [
  {
    q: 'What does “per workspace” mean?',
    a: 'Each workspace is its own subscription with its own billing. There is no per-seat charge, so adding teammates never changes the price.',
  },
  {
    q: 'Do we need a card for the trial?',
    a: `No. The 14-day trial runs on Growth and includes $${AI_ALLOWANCE.trial} of AI usage. When it ends, the workspace is locked until you choose Starter or Growth.`,
  },
  {
    q: 'What happens when our AI allowance runs out?',
    a: 'New AI work is declined until the allowance renews next month; work already running keeps the allowance it reserved. On an active paid plan, you can turn on metered overage in billing settings to keep going at the same rates. Unused allowance doesn’t carry over, including on annual plans.',
  },
  {
    q: 'What determines how much AI we use?',
    a: 'The model profile and the amount of work: input, cache, and output tokens, each at its profile’s rate. Billing settings show how much of the allowance you’ve used.',
  },
  {
    q: 'Can we bring our own AI provider keys?',
    a: 'On a self-hosted install, always: connect OpenAI, Anthropic, OpenRouter, or a compatible endpoint and pay your provider directly. On Cloud, your own keys are part of Enterprise; Starter and Growth use the included allowance.',
  },
  {
    q: 'Can we use Helpin without a Cloud plan?',
    a: 'Yes. The open-source Community edition (0.2 beta) is free to self-host, with every module and no plan limits: support, projects, CRM, meetings, docs, automation, and AI agents, including coding agents. Your team runs the infrastructure and pays for it, along with any AI providers you connect.',
  },
  {
    q: 'What’s the difference between open source and Enterprise?',
    a: 'Every product feature is open source under AGPL-3.0, including everything in Growth. The separate Enterprise license covers Helpin’s Cloud billing and hosted-AI metering code. Enterprise adds a commercial license for companies that can’t use AGPL, your own AI keys on Cloud, deployment help, and support terms.',
  },
  {
    q: 'Can we switch plans or billing period later?',
    a: 'Yes. The workspace’s billing owner can change the plan or billing period in billing settings. Check when the change takes effect and any price adjustment before you confirm.',
  },
];
