// The /compare hub matrix: Helpin beside every tool we compare, on the same rows.
// Facts match each comparison page (compare-data.ts). Rows where Helpin falls short stay in.
import type { Status } from './compare-data.ts';

export type MatrixCell = { status?: Status; text?: string };
export type MatrixRow = { label: string; helpin: MatrixCell; tools: Record<string, MatrixCell> };

const c = (status?: Status, text?: string): MatrixCell => ({ status, text });
const yes = c('yes');
const no = c('no');

export const MATRIX_TOOLS = ['intercom', 'zendesk', 'help-scout', 'chatwoot', 'linear'] as const;

export const MATRIX: { group: string; rows: MatrixRow[] }[] = [
  {
    group: 'AI agents',
    rows: [
      { label: 'AI agent answers customers', helpin: yes, tools: { intercom: yes, zendesk: yes, 'help-scout': yes, chatwoot: yes, linear: no } },
      { label: 'Coding agents open pull requests', helpin: yes, tools: { intercom: no, zendesk: no, 'help-scout': no, chatwoot: no, linear: yes } },
      { label: 'AI usage included in the plan', helpin: yes, tools: { intercom: c('no', '$0.99 per outcome'), zendesk: c('no', 'Per resolution'), 'help-scout': c('no', '$0.75 per resolution'), chatwoot: c('partial', 'Credits, then metered'), linear: c('partial', 'Credits for coding') } },
    ],
  },
  {
    group: 'Support',
    rows: [
      { label: 'Shared inbox for chat and email', helpin: yes, tools: { intercom: yes, zendesk: yes, 'help-scout': yes, chatwoot: yes, linear: no } },
      { label: 'Phone, WhatsApp and social channels', helpin: no, tools: { intercom: yes, zendesk: yes, 'help-scout': c('partial', 'Phone via integrations'), chatwoot: yes, linear: no } },
      { label: 'Public help center', helpin: yes, tools: { intercom: yes, zendesk: yes, 'help-scout': yes, chatwoot: yes, linear: no } },
      { label: 'Native mobile SDKs', helpin: no, tools: { intercom: yes, zendesk: yes, 'help-scout': yes, chatwoot: yes, linear: no } },
      { label: 'SSO and SLA policies', helpin: no, tools: { intercom: c('partial', 'Expert plan'), zendesk: yes, 'help-scout': c('partial', 'Limited by plan'), chatwoot: c('partial', 'Enterprise plan'), linear: c('partial', 'SSO on Enterprise') } },
    ],
  },
  {
    group: 'Beyond support',
    rows: [
      { label: 'Roadmaps and sprints', helpin: yes, tools: { intercom: c('partial', 'Integrations'), zendesk: c('partial', 'Integrations'), 'help-scout': c('partial', 'Integrations'), chatwoot: c('partial', 'Integrations'), linear: yes } },
      { label: 'CRM with deals', helpin: yes, tools: { intercom: c('partial', 'Integrations'), zendesk: c('partial', 'Sell retires 2027'), 'help-scout': c('partial', 'Integrations'), chatwoot: c('partial', 'No deals'), linear: no } },
      { label: 'Meeting notes', helpin: yes, tools: { intercom: no, zendesk: no, 'help-scout': no, chatwoot: no, linear: c('partial', 'Gong on Enterprise') } },
      { label: 'MCP server', helpin: c('yes', 'Beta'), tools: { intercom: yes, zendesk: c('partial', 'Announced'), 'help-scout': c('partial', 'Read-only'), chatwoot: c('no', 'Community-built'), linear: yes } },
    ],
  },
  {
    group: 'Pricing and hosting',
    rows: [
      { label: 'Price model', helpin: c(undefined, 'Per workspace'), tools: { intercom: c(undefined, 'Per seat'), zendesk: c(undefined, 'Per agent'), 'help-scout': c(undefined, 'Per user'), chatwoot: c(undefined, 'Per agent'), linear: c(undefined, 'Per user') } },
      { label: 'Open source and self-hosting', helpin: yes, tools: { intercom: no, zendesk: no, 'help-scout': no, chatwoot: c('yes', 'Some features paid'), linear: no } },
      { label: 'Free trial', helpin: c(undefined, '14 days, no card'), tools: { intercom: c(undefined, '14 days, no card'), zendesk: c(undefined, '14 days, no card'), 'help-scout': c(undefined, '15 days, no card'), chatwoot: c(undefined, '15 days'), linear: c(undefined, 'Free plan') } },
    ],
  },
];
