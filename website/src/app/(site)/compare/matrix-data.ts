// The /compare hub matrix: Helpin beside every tool we compare, on the same rows.
// Facts match the individual comparison pages and their linked primary sources.
import type { Status } from './compare-data.ts';

export type MatrixCell = { status?: Status; text?: string };
export type MatrixRow = { label: string; helpin: MatrixCell; tools: Record<string, MatrixCell> };

const c = (status?: Status, text?: string): MatrixCell => ({ status, text });
const yes = c('yes');
const no = c('no');

export const MATRIX_TOOLS = ['intercom', 'zendesk', 'help-scout', 'chatwoot', 'chatbase', 'crisp', 'linear', 'jira', 'plane'] as const;

export const MATRIX: { group: string; rows: MatrixRow[] }[] = [
  {
    group: 'AI agents',
    rows: [
      { label: 'AI agent answers customers', helpin: yes, tools: { chatbase: yes, crisp: yes, intercom: yes, zendesk: yes, 'help-scout': yes, chatwoot: yes, linear: no , jira: c('partial', 'Service Collection'), plane: no } },
      { label: 'Code delivery from a task', helpin: c('yes', 'Built-in planning, coding and review'), tools: { chatbase: c(undefined, "Separate project workflow"), crisp: c(undefined, "Separate project workflow"), intercom: c(undefined, 'Separate project workflow'), zendesk: c(undefined, 'Separate project workflow'), 'help-scout': c(undefined, 'Separate project workflow'), chatwoot: c(undefined, 'External agents through CLI'), linear: c('yes', 'Coding sessions and connected agents'), jira: c('yes', 'Jira and connected agents'), plane: c('yes', 'Cursor integration') } },
      { label: 'AI billing', helpin: c(undefined, 'Included allowance; optional extra usage'), tools: { chatbase: c(undefined, "Included message credits; paid extras"), crisp: c(undefined, "Included Hugo credits; optional extra usage"), intercom: c(undefined, '$0.99 per Fin outcome'), zendesk: c(undefined, 'Resolution allowance; paid extra usage'), 'help-scout': c(undefined, '$0.75 per AI resolution'), chatwoot: c(undefined, 'Included credits; paid extra usage'), linear: c(undefined, 'Prepaid coding credits'), jira: c(undefined, 'Included Rovo credits'), plane: c(undefined, 'Personal and agent allowances') } },
    ],
  },
  {
    group: 'Support',
    rows: [
      { label: 'Shared inbox for chat and email', helpin: yes, tools: { chatbase: c("yes", "Helpdesk on Standard and above"), crisp: c("yes", "Email on paid plans"), intercom: yes, zendesk: yes, 'help-scout': yes, chatwoot: yes, linear: no , jira: c('partial', 'Service Collection'), plane: no } },
      { label: 'Phone, WhatsApp and social channels', helpin: no, tools: { chatbase: c("yes", "Voice and connected channels"), crisp: c("yes", "Phone through integrations"), intercom: yes, zendesk: yes, 'help-scout': c('partial', 'Phone via integrations'), chatwoot: yes, linear: no , jira: c('partial', 'Service Collection'), plane: no } },
      { label: 'Public help center', helpin: yes, tools: { chatbase: c("partial", "Agent Page uses existing docs"), crisp: c("yes", "Essentials and Plus"), intercom: yes, zendesk: yes, 'help-scout': yes, chatwoot: yes, linear: no , jira: c('partial', 'Service Collection and Confluence'), plane: c('partial', 'Published pages') } },
      { label: 'Native mobile SDKs', helpin: no, tools: { chatbase: c("yes", "Android and iOS"), crisp: c("yes", "Android and iOS"), intercom: yes, zendesk: yes, 'help-scout': yes, chatwoot: yes, linear: no , jira: c('partial', 'Marketplace apps'), plane: no } },
      { label: 'Company single sign-on (SAML)', helpin: no, tools: { chatbase: c(undefined, "Enterprise SSO; confirm SAML"), crisp: c(undefined, "Confirm with Crisp"), intercom: c('yes', 'Expert plan'), zendesk: yes, 'help-scout': c('yes', 'Plan-dependent'), chatwoot: c('yes', 'Enterprise plan'), linear: c('yes', 'Enterprise plan'), jira: c('yes', 'Guard or Enterprise'), plane: c('yes', 'Paid plans') } },
      { label: 'Support response-time policies (SLAs)', helpin: no, tools: { chatbase: c(undefined, "Confirm ticket-policy requirements"), crisp: c(undefined, "Confirm ticket-policy requirements"), intercom: c('yes', 'Expert plan'), zendesk: yes, 'help-scout': c('yes', 'Plan-dependent limits'), chatwoot: c('yes', 'Enterprise plan'), linear: no, jira: c('partial', 'Service Collection'), plane: no } },
    ],
  },
  {
    group: 'Beyond support',
    rows: [
      { label: 'Roadmaps and sprints', helpin: yes, tools: { chatbase: c("partial", "Separate project tools"), crisp: c("partial", "Separate project tools"), intercom: c('partial', 'Integrations'), zendesk: c('partial', 'Integrations'), 'help-scout': c('partial', 'Integrations'), chatwoot: c('partial', 'Integrations'), linear: yes , jira: yes, plane: yes } },
      { label: 'CRM with deals', helpin: c('yes', 'Deals and AI follow-ups'), tools: { chatbase: c("partial", "CRM integrations"), crisp: c("partial", "CRM integrations"), intercom: c('partial', 'CRM integrations'), zendesk: c('partial', 'Separate CRM; Sell retires 2027'), 'help-scout': c('partial', 'CRM integrations'), chatwoot: c('no', 'Contacts and companies'), linear: c('no', 'Customer profiles and requests'), jira: c('partial', 'CRM integrations'), plane: c('no', 'Customer profiles and requests') } },
      { label: 'Meeting notes', helpin: yes, tools: { chatbase: c("partial", "Separate meeting tools"), crisp: c("partial", "Separate meeting tools"), intercom: no, zendesk: no, 'help-scout': no, chatwoot: no, linear: c('partial', 'Gong on Enterprise') , jira: c('partial', 'Loom, sold separately'), plane: c('partial', 'Granola connector') } },
      { label: 'MCP connections for AI tools', helpin: c('yes', 'Server and external tools'), tools: { chatbase: c("yes", "Manage agents, sources and tickets"), crisp: c("yes", "External tools and knowledge search"), intercom: c('yes', 'Server and external tools'), zendesk: c('yes', 'Connect external tools to AI actions'), 'help-scout': c('yes', 'Read-only server'), chatwoot: c(undefined, 'Community-built servers'), linear: c('yes', 'MCP server'), jira: c('yes', 'Rovo MCP server'), plane: c('yes', 'Server and connectors') } },
    ],
  },
  {
    group: 'Pricing and hosting',
    rows: [
      { label: 'Price model', helpin: c(undefined, 'Per workspace'), tools: { chatbase: c(undefined, "Workspace plans with member limits"), crisp: c(undefined, "Per workspace; seats included by plan"), intercom: c(undefined, 'Per seat'), zendesk: c(undefined, 'Per agent'), 'help-scout': c(undefined, 'Per user'), chatwoot: c(undefined, 'Per agent'), linear: c(undefined, 'Per user') , jira: c(undefined, 'Per user'), plane: c(undefined, 'Per seat') } },
      { label: 'Open source and self-hosting', helpin: yes, tools: { chatbase: no, crisp: no, intercom: no, zendesk: no, 'help-scout': no, chatwoot: c('yes', 'Some features paid'), linear: no , jira: c('no', 'Data Center closing'), plane: c('yes', 'Some features paid') } },
      { label: 'Free trial', helpin: c(undefined, '14 days, no card'), tools: { chatbase: c(undefined, "7-day trial; free plan"), crisp: c(undefined, "14-day trial; free plan"), intercom: c(undefined, '14 days, no card'), zendesk: c(undefined, '14 days, no card'), 'help-scout': c(undefined, '15 days, no card'), chatwoot: c(undefined, '15 days'), linear: c(undefined, 'Free plan') , jira: c(undefined, 'Free plan'), plane: c(undefined, '14 days of Business') } },
    ],
  },
];
