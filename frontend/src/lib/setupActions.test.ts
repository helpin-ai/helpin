import { describe, expect, it } from 'vitest';
import { resolveSetupAction } from './setupActions';

describe('resolveSetupAction', () => {
  it('resolves every setup action to an existing product route', () => {
    expect({
      workspace_context: resolveSetupAction('workspace_context', 'acme'),
      workspace_teams: resolveSetupAction('workspace_teams', 'acme'),
      workspace_members: resolveSetupAction('workspace_members', 'acme'),
      pm_create_task: resolveSetupAction('pm_create_task', 'acme'),
      pm_tasks: resolveSetupAction('pm_tasks', 'acme'),
	  pm_sprints: resolveSetupAction('pm_sprints', 'acme'),
      git_settings: resolveSetupAction('git_settings', 'acme'),
      support_email_inbox: resolveSetupAction('support_email_inbox', 'acme'),
      support_live_chat: resolveSetupAction('support_live_chat', 'acme'),
      support_help_docs: resolveSetupAction('support_help_docs', 'acme'),
      support_brand_knowledge: resolveSetupAction('support_brand_knowledge', 'acme'),
      support_team_inboxes: resolveSetupAction('support_team_inboxes', 'acme'),
      support_routing: resolveSetupAction('support_routing', 'acme'),
      support_inbox: resolveSetupAction('support_inbox', 'acme'),
      support_ai: resolveSetupAction('support_ai', 'acme'),
	  support_coverage: resolveSetupAction('support_coverage', 'acme'),
      automation_flows: resolveSetupAction('automation_flows', 'acme'),
      automation_agents: resolveSetupAction('automation_agents', 'acme'),
    }).toEqual({
      workspace_context: '/w/acme/settings/general',
      workspace_teams: '/w/acme/settings/teams',
      workspace_members: '/w/acme/settings/members',
      pm_create_task: '/w/acme/pm/tasks',
      pm_tasks: '/w/acme/pm/tasks',
	  pm_sprints: '/w/acme/pm/sprints',
      git_settings: '/w/acme/settings/repositories',
      support_email_inbox: '/w/acme/settings/inboxes-routing?tab=email',
      support_live_chat: '/w/acme/settings/chat-general',
      support_help_docs: '/w/acme/docs',
      support_brand_knowledge: '/w/acme/settings/knowledge',
      support_team_inboxes: '/w/acme/settings/inboxes-routing?tab=inboxes',
      support_routing: '/w/acme/settings/inboxes-routing?tab=routing',
      support_inbox: '/w/acme/support',
      support_ai: '/w/acme/settings/support-ai-assistant',
	  support_coverage: '/w/acme/support/coverage',
      automation_flows: '/w/acme/automation/flows',
      automation_agents: '/w/acme/automation/agents',
    });
  });

  it('does not invent a destination for an unknown action', () => {
    expect(resolveSetupAction('not_available', 'acme')).toBeUndefined();
  });
});
