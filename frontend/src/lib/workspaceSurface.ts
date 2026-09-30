import type { WorkspaceModule } from './types';

// Deployment and workspace access come from /me, including for workspace owners.
export function workspaceSurface(path: string): WorkspaceModule | null {
  const [, , , surface, page] = path.split('/');
  if (surface === 'settings') {
    if (page?.startsWith('crm-')) return 'crm';
    if (['workflows', 'delivery', 'labels', 'task-templates', 'recurring-tasks'].includes(page)) return 'pm';
    if (page === 'automations') return 'automation';
    if (['support-ai-assistant', 'support-translation', 'chat-general', 'inboxes-routing'].includes(page)) return 'support';
    if (['helpcenter', 'redirects', 'knowledge'].includes(page)) return 'docs';
    if (['ai', 'ai-connections', 'external-mcp', 'external-agents'].includes(page)) return 'agents';
  }
  if (surface === 'automation') return ['agents', 'tools', 'skills'].includes(page) ? 'agents' : 'automation';
  if (surface === 'pm' && page === 'agents') return 'agents';
  if (['pm', 'docs', 'crm', 'support'].includes(surface)) return surface as WorkspaceModule;
  return null;
}

export function workspaceHome(slug: string, modules: readonly WorkspaceModule[]): string | null {
  const routes: [WorkspaceModule, string][] = [['pm', 'pm/my-work'], ['support', 'support'], ['docs', 'docs'], ['agents', 'automation/agents'], ['crm', 'crm/overview'], ['automation', 'automation/flows']];
  const destination = routes.find(([module]) => modules.includes(module));
  return destination ? `/w/${encodeURIComponent(slug)}/${destination[1]}` : null;
}

export function filterWorkspaceNav<T extends { link: string }>(items: readonly T[], modules: readonly WorkspaceModule[]): T[] {
  return items.filter(item => { const module = workspaceSurface(item.link); return module === null || modules.includes(module); });
}
