import { formatDistanceToNow } from 'date-fns';
import { QuietMetaLine } from '@/components/design-system/quiet';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useWorkspaceCapabilities } from '@/hooks/queries/useCapabilities';
import type { Capability } from '@/lib/capabilityTypes';
import type { SetupGoalKey } from '@/lib/setupTypes';
import { cn } from '@/lib/utils';
import { CapabilityActionView } from './CapabilityActions';
import { capabilityStatus, capabilityTitle, goalsNeeding, groupCapabilities } from './capabilityPresentation';
import { SetupAIStep } from './SetupAIStep';
import { SetupEmailStep } from './SetupEmailStep';
import { SetupGitHubStep } from './SetupGitHubStep';
import { SetupWidgetStep } from './SetupWidgetStep';

export type SetupConnectionsSectionProps = {
  workspaceId: string;
  slug: string;
  goals: SetupGoalKey[];
  /** Workspace settings access (`workspace.update`): runs tests and adds connections. */
  canManage: boolean;
  /** Only the owner can create the instance GitHub App. */
  isOwner: boolean;
};

/**
 * Setup guide section that reports what this server and workspace can do,
 * ordered by the chosen goals, with the step that fixes each gap inline.
 */
export function SetupConnectionsSection({ workspaceId, slug, goals, canManage, isOwner }: SetupConnectionsSectionProps) {
  const capabilities = useWorkspaceCapabilities(workspaceId);
  const headingId = 'setup-connections-heading';

  let body;
  if (capabilities.isLoading) {
    body = (
      <div className="space-y-4 py-5" aria-busy="true" aria-label="Checking connections">
        <Skeleton className="h-5 w-64" />
        <Skeleton className="h-5 w-80 max-w-full" />
        <Skeleton className="h-5 w-72" />
      </div>
    );
  } else if (capabilities.isError || !capabilities.data) {
    body = (
      <div className="py-5">
        <p className="text-sm font-medium text-quiet-text-primary">We couldn’t check this workspace’s connections.</p>
        <p className="mt-1 text-sm text-quiet-text-tertiary">The rest of the guide still works. Try the check again.</p>
        <Button className="mt-3" variant="outline" size="sm" onClick={() => void capabilities.refetch()}>Check again</Button>
      </div>
    );
  } else if (capabilities.data.capabilities.length === 0) {
    body = <p className="py-5 text-sm text-quiet-text-tertiary">This server doesn’t report any connections to check.</p>;
  } else {
    const groups = groupCapabilities(capabilities.data.capabilities, goals);
    const rowProps = { workspaceId, slug, goals, canManage, isOwner };
    body = (
      <>
        {groups.blocking.length > 0 && (
          <div className="relative mt-5 py-1 pl-4" role="note">
            <span aria-hidden="true" className="absolute inset-y-0 left-0 w-[3px] bg-quiet-accent" />
            <p className="text-sm font-semibold text-quiet-text-primary">
              {groups.blocking.length === 1 ? 'A required service needs attention' : `${groups.blocking.length} required services need attention`}
            </p>
            <p className="mt-0.5 text-[12.5px] text-quiet-text-tertiary">Uploads and background jobs depend on these. Whoever runs this server can fix them.</p>
          </div>
        )}
        <CapabilityList items={[...groups.blocking, ...groups.relevant]} {...rowProps} />
        {groups.other.length > 0 && (
          <CapabilityGroup title="Other connections" items={groups.other} {...rowProps} />
        )}
        {groups.unavailable.length > 0 && (
          <CapabilityGroup title="Not available on this server" items={groups.unavailable} muted {...rowProps} />
        )}
      </>
    );
  }

  return (
    <section id="setup-connections" aria-labelledby={headingId} className="scroll-mt-6">
      <div className="flex flex-col gap-1 border-b border-quiet-divider-strong pb-5">
        <h2 id={headingId} className="text-xl font-semibold">Connections</h2>
        <p className="max-w-2xl text-sm leading-6 text-muted-foreground">
          What this server and workspace can do right now, with what your goals need first. Checks refresh when you come back to this page.
        </p>
      </div>
      {body}
    </section>
  );
}

type RowContext = SetupConnectionsSectionProps;

function CapabilityGroup({ title, items, muted, ...context }: RowContext & { title: string; items: Capability[]; muted?: boolean }) {
  return (
    <div className="mt-8">
      <h3 className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">{title}</h3>
      <CapabilityList items={items} muted={muted} {...context} />
    </div>
  );
}

function CapabilityList({ items, muted, ...context }: RowContext & { items: Capability[]; muted?: boolean }) {
  if (items.length === 0) return null;
  return (
    <ul className="divide-y divide-quiet-divider-light">
      {items.map((capability) => <CapabilityRow key={capability.key} capability={capability} muted={muted} {...context} />)}
    </ul>
  );
}

function CapabilityRow({ capability, muted, workspaceId, slug, goals, canManage, isOwner }: RowContext & { capability: Capability; muted?: boolean }) {
  const status = capabilityStatus(capability.status);
  const StatusIcon = status.icon;
  const title = capabilityTitle(capability.key);
  const neededFor = goalsNeeding(capability.key, goals);
  const blocking = capability.required && capability.status !== 'ready';
  const checked = checkedLabel(capability.checked_at);

  return (
    <li
      className={cn('flex gap-4 py-5 sm:gap-6', muted && 'opacity-70')}
      data-capability={capability.key}
      data-status={capability.status}
    >
      <div className="flex w-8 shrink-0 justify-center pt-0.5" aria-hidden="true">
        <StatusIcon className={cn('h-5 w-5', status.className)} />
      </div>
      <div className="min-w-0 flex-1 space-y-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <p id={`capability-${capability.key}`} className={cn('text-sm font-medium leading-5', muted ? 'text-quiet-text-tertiary' : 'text-quiet-text-primary')}>{title}</p>
            <span className={cn('text-[11.5px] font-semibold uppercase tracking-[0.03em]', status.className)}>{status.label}</span>
            {blocking && <span className="text-[11.5px] font-semibold uppercase tracking-[0.03em] text-quiet-accent">Required</span>}
          </div>
          <p className="mt-1 max-w-2xl text-[12.5px] leading-5 text-quiet-text-tertiary">{capability.detail}</p>
          {(checked || neededFor.length > 0) && (
            <QuietMetaLine
              className="mt-1"
              items={[
                checked,
                neededFor.length > 0 ? `Needed for ${neededFor.join(', ')}` : null,
              ]}
            />
          )}
        </div>
        <CapabilityStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} isOwner={isOwner} />
      </div>
    </li>
  );
}

function CapabilityStep({ capability, workspaceId, slug, canManage, isOwner }: { capability: Capability; workspaceId: string; slug: string; canManage: boolean; isOwner: boolean }) {
  switch (capability.key) {
    case 'ai_chat':
      return <SetupAIStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} />;
    case 'email_outbound':
      return <SetupEmailStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} />;
    case 'support_widget':
      return <SetupWidgetStep capability={capability} slug={slug} canManage={canManage} />;
    case 'github':
      return <SetupGitHubStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} isOwner={isOwner} />;
    default:
      return <CapabilityActionView capability={capability} slug={slug} canManage={canManage} />;
  }
}

function checkedLabel(checkedAt?: string) {
  if (!checkedAt) return null;
  const date = new Date(checkedAt);
  if (Number.isNaN(date.getTime())) return null;
  return <time dateTime={checkedAt} title={date.toLocaleString()}>Checked {formatDistanceToNow(date, { addSuffix: true })}</time>;
}
