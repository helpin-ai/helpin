import { Link } from '@tanstack/react-router';
import { QuietMetaLine, QuietPropertyRow, QuietStatusText } from '@/components/design-system/quiet';
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { useExternalAgents } from '@/hooks/queries/useExternalAgents';
import { externalAgentProtocolLabel } from '@/lib/externalAgents';
import type { ExternalAgentStatus } from '@/lib/externalAgentTypes';
import type { Agent, AgentTriggerMode } from '@/lib/pmTypes';

const TRIGGER_MODE_COPY: Record<AgentTriggerMode, string> = {
  auto_on_assignment: 'Starts when a task is assigned to it',
  manual: 'Starts only when someone runs it',
  auto_on_event: 'Starts from automation events',
};

const STATUS_COPY: Record<ExternalAgentStatus, { label: string; tone: 'positive' | 'neutral' | 'blocker' }> = {
  active: { label: 'Active', tone: 'positive' },
  disabled: { label: 'Disabled', tone: 'neutral' },
  error: { label: 'Error', tone: 'blocker' },
};

/**
 * Read-only view of an external (A2A) agent in the agent editor's place. Its
 * connection, token, and team access are managed in Settings → External agents;
 * it has no model, prompt, tools, or skills to edit in Helpin.
 */
export function ExternalAgentSummarySheet({
  agent,
  workspaceId,
  workspaceSlug,
  onOpenChange,
}: {
  agent: Agent | null;
  workspaceId: string;
  workspaceSlug?: string;
  onOpenChange: (open: boolean) => void;
}) {
  const externalAgentsQuery = useExternalAgents(agent ? workspaceId : '');
  const external = agent ? externalAgentsQuery.data?.items.find((item) => item.agent_id === agent.id) : undefined;
  const status = external ? STATUS_COPY[external.status] ?? STATUS_COPY.error : null;
  const description = external?.description || agent?.role || '';

  return (
    <Sheet open={Boolean(agent)} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full gap-0 p-0 data-[side=right]:sm:max-w-lg" data-external-agent-summary>
        {agent ? (
          <>
            <SheetHeader className="border-b border-quiet-divider-strong py-4 pl-6 pr-14">
              <div className="flex items-center gap-3">
                <AgentAvatar agent={agent} className="h-10 w-10 shrink-0 rounded-none border-0 bg-transparent shadow-none" genericBare />
                <div className="min-w-0 flex-1">
                  <SheetTitle className="truncate text-[20px] font-semibold tracking-[-0.018em]">{agent.name}</SheetTitle>
                  <SheetDescription className="mt-0.5 text-[12.5px] text-quiet-text-tertiary">External agent (A2A)</SheetDescription>
                </div>
              </div>
            </SheetHeader>
            <div className="space-y-4 overflow-y-auto px-6 py-5">
              {description ? (
                <p className="max-w-[680px] text-sm leading-[1.6] text-quiet-text-secondary [text-wrap:pretty]">{description}</p>
              ) : null}
              <div className="-mx-4 border-y border-quiet-divider-light">
                <QuietPropertyRow label="Runs on" value="An external agent, over the A2A protocol" />
                <QuietPropertyRow label="Starts" value={TRIGGER_MODE_COPY[agent.trigger_mode] ?? agent.trigger_mode} />
                {external ? (
                  <QuietPropertyRow
                    label="Connection"
                    value={(
                      <QuietMetaLine
                        className="text-sm"
                        items={[
                          status ? <QuietStatusText key="status" tone={status.tone}>{status.label}</QuietStatusText> : null,
                          external.provider_name || null,
                          externalAgentProtocolLabel(external),
                        ]}
                      />
                    )}
                  />
                ) : null}
              </div>
              {external?.status === 'error' && external.last_error ? (
                <p className="text-[12.5px] leading-5 text-quiet-accent">{external.last_error}</p>
              ) : null}
              <p className="text-[12.5px] leading-5 text-quiet-text-tertiary">
                Its connection, access token, and which teams can assign it tasks are managed in Settings.
                Model, instructions, tools, and skills belong to the external agent itself.
              </p>
              {workspaceSlug ? (
                <Link
                  to="/w/$slug/settings/external-agents"
                  params={{ slug: workspaceSlug }}
                  className="inline-flex text-[12.5px] text-quiet-text-secondary underline underline-offset-4 hover:text-quiet-text-primary"
                  onClick={() => onOpenChange(false)}
                >
                  Manage in Settings → External agents
                </Link>
              ) : null}
            </div>
          </>
        ) : null}
      </SheetContent>
    </Sheet>
  );
}
