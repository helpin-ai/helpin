export interface ShortcutVariableContext {
  customer?: {
    fullName?: string | null;
    email?: string | null;
  };
  agent?: {
    fullName?: string | null;
    email?: string | null;
  };
  workspaceName?: string | null;
  conversationSubject?: string | null;
}

export const SHORTCUT_VARIABLES = [
  { label: '{{customer.first_name}}', token: '{{customer.first_name | fallback: "there"}}' },
  { label: '{{customer.full_name}}', token: '{{customer.full_name | fallback: "there"}}' },
  { label: '{{customer.email}}', token: '{{customer.email}}' },
  { label: '{{agent.first_name}}', token: '{{agent.first_name}}' },
  { label: '{{agent.full_name}}', token: '{{agent.full_name}}' },
] as const;

function firstName(value?: string | null) {
  return (value ?? '').trim().split(/\s+/)[0] ?? '';
}

function lookupVariable(name: string, context: ShortcutVariableContext) {
  switch (name) {
    case 'customer.first_name':
    case 'first_name':
      return firstName(context.customer?.fullName);
    case 'customer.full_name':
    case 'full_name':
      return context.customer?.fullName?.trim() ?? '';
    case 'customer.email':
    case 'email':
      return context.customer?.email?.trim() ?? '';
    case 'agent.first_name':
    case 'agent_first_name':
      return firstName(context.agent?.fullName);
    case 'agent.full_name':
    case 'agent_full_name':
      return context.agent?.fullName?.trim() ?? '';
    case 'workspace_name':
      return context.workspaceName?.trim() ?? '';
    case 'conversation_subject':
      return context.conversationSubject?.trim() ?? '';
    default:
      return '';
  }
}

function parseDefault(expression: string) {
  const match = expression.match(/\|\s*(?:fallback|default)\s*:\s*(?:"([^"]*)"|'([^']*)'|([^|]+))\s*$/);
  if (!match) return { variable: expression.trim(), fallback: '' };
  const variable = expression.slice(0, match.index).trim();
  const fallback = (match[1] ?? match[2] ?? match[3] ?? '').trim();
  return { variable, fallback };
}

export function resolveShortcutVariables(content: string, context: ShortcutVariableContext) {
  return content.replace(/\{\{\s*([^}]+?)\s*\}\}/g, (_token, expression: string) => {
    const { variable, fallback } = parseDefault(expression);
    return lookupVariable(variable, context) || fallback;
  });
}
