import { WorkflowManager, type WorkspaceEventRuleTemplate } from '@/components/settings';
import { Card, CardContent } from '@/components/ui/card';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { SettingsPageFrame } from './SettingsPageFrame';

type WorkflowTemplateHint = {
  title: string;
  description: string;
};

interface WorkflowsSettingsPageProps {
  search: {
    workflow?: string;
    team?: string;
    template?: string;
    template_title?: string;
    template_description?: string;
    show_trigger?: string;
    show_trigger_title?: string;
    create_event_rule?: boolean;
    trigger_type?: string;
    agent_id?: string;
    repo_full_name?: string;
    branch?: string;
    base_branch?: string;
    tag_name?: string;
    conclusion?: string;
    target_mode?: 'event' | 'task' | 'epic' | 'repository';
    target_id?: string;
  };
}

const WORKSPACE_EVENT_TRIGGER_TYPES = new Set([
  'github.push',
  'github.pull_request_opened',
  'github.pull_request_merged',
  'github.pull_request_closed',
  'github.pull_request_review_requested',
  'github.release_published',
  'github.check_suite_completed',
]);

function buildEventRuleTemplate(search: WorkflowsSettingsPageProps['search']): WorkspaceEventRuleTemplate | undefined {
  if (!search.trigger_type || !WORKSPACE_EVENT_TRIGGER_TYPES.has(search.trigger_type)) {
    return undefined;
  }

  return {
    triggerType: search.trigger_type,
    agentId: search.agent_id,
    repoFullName: search.repo_full_name,
    branch: search.branch,
    baseBranch: search.base_branch,
    tagName: search.tag_name,
    conclusion: search.conclusion,
    targetMode: search.target_mode,
    targetId: search.target_id,
  };
}

function buildTemplateHint(search: WorkflowsSettingsPageProps['search']): WorkflowTemplateHint | undefined {
  if (!search.template || !search.template_title || !search.template_description) {
    return undefined;
  }
  return {
    title: search.template_title,
    description: search.template_description,
  };
}

export function WorkflowsSettingsPage({ search }: WorkflowsSettingsPageProps) {
  const { currentWorkspace } = useWorkspaceStore();
  const eventRuleTemplate = buildEventRuleTemplate(search);
  const templateHint = buildTemplateHint(search);
  const triggerFilterLabel = search.show_trigger_title || search.show_trigger;
  const clearFilterHref = currentWorkspace?.slug
    ? `/w/${currentWorkspace.slug}/settings/workflows`
    : '/settings/workflows';

  return (
    <SettingsPageFrame section="workflows">
      {({ workspaceId, settings, permissions }) => (
        <div className="space-y-4">
          {triggerFilterLabel && (
            <Card>
              <CardContent className="flex flex-wrap items-center justify-between gap-3 px-5 py-4">
                <div className="space-y-1">
                  <p className="text-sm font-medium">Showing rules using {triggerFilterLabel}</p>
                  <p className="text-sm text-muted-foreground">
                    Review the existing bindings for this trigger across workspace event rules and workflow pipeline rules.
                  </p>
                </div>
                <a href={clearFilterHref} className="text-sm text-muted-foreground hover:text-foreground">
                  Clear filter
                </a>
              </CardContent>
            </Card>
          )}

          {templateHint && (
            <Card>
              <CardContent className="space-y-1 px-5 py-4">
                <p className="text-sm font-medium">{templateHint.title}</p>
                <p className="text-sm text-muted-foreground">{templateHint.description}</p>
              </CardContent>
            </Card>
          )}

          <WorkflowManager
            workspaceId={workspaceId}
            teams={settings.teams}
            editable={permissions.canManageSettings}
            initialWorkflowId={search.workflow}
            initialTeamId={search.team}
            highlightTriggerType={search.show_trigger}
            highlightTriggerLabel={search.show_trigger_title}
            initialEventRuleTemplate={eventRuleTemplate}
          />
        </div>
      )}
    </SettingsPageFrame>
  );
}
