import { formatDistanceToNow } from 'date-fns';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useWorkspaceCapabilities } from '@/hooks/queries/useCapabilities';
import type { Capability } from '@/lib/capabilityTypes';
import { cn } from '@/lib/utils';
import { CapabilityActionView } from './CapabilityActions';
import { capabilityStatus, capabilityTitle, groupSystemCapabilities, isBlocking } from './capabilityPresentation';
import { SetupAIStep } from './SetupAIStep';
import { SetupEmailStep } from './SetupEmailStep';
import { SetupGitHubStep } from './SetupGitHubStep';

export type SystemStatusPanelProps = {
  workspaceId: string;
  slug: string;
  /** Workspace settings access (`workspace.update`): runs tests and adds connections. */
  canManage: boolean;
  /** Only the owner can create the instance GitHub App. */
  isOwner: boolean;
  /**
   * The page shows the application email settings card, which owns Save and
   * Send test email; the Application email row then shows status only.
   */
  emailSettingsOnPage?: boolean;
};

/**
 * Settings → System status: the services this Community server provides to
 * every workspace, with the step that fixes each gap inline.
 */
export function SystemStatusPanel({ workspaceId, slug, canManage, isOwner, emailSettingsOnPage }: SystemStatusPanelProps) {
  const capabilities = useWorkspaceCapabilities(workspaceId);

  if (capabilities.isLoading) {
    return (
      <div className="space-y-4 py-5" aria-busy="true" aria-label="Checking system status">
        <Skeleton className="h-5 w-64" />
        <Skeleton className="h-5 w-80 max-w-full" />
        <Skeleton className="h-5 w-72" />
      </div>
    );
  }
  if (capabilities.isError || !capabilities.data) {
    return (
      <div className="py-5">
        <p className="text-sm font-medium text-quiet-text-primary">We couldn’t check this server.</p>
        <p className="mt-1 text-sm text-quiet-text-tertiary">The check itself failed, not necessarily a service. Try again.</p>
        <Button className="mt-3" variant="outline" size="sm" onClick={() => void capabilities.refetch()}>Check again</Button>
      </div>
    );
  }
  if (capabilities.data.edition !== 'community') {
    return <p className="py-5 text-sm text-quiet-text-tertiary">Server services are managed by the platform on this edition.</p>;
  }

  const groups = groupSystemCapabilities(capabilities.data.capabilities);
  const rowProps = { workspaceId, slug, canManage, isOwner, emailSettingsOnPage };
  return (
    <div>
      {groups.blocking.length > 0 && (
        <div className="relative mb-2 py-1 pl-4" role="note">
          <span aria-hidden="true" className="absolute inset-y-0 left-0 w-[3px] bg-quiet-accent" />
          <p className="text-sm font-semibold text-quiet-text-primary">
            {groups.blocking.length === 1 ? 'A required service needs attention' : `${groups.blocking.length} required services need attention`}
          </p>
          <p className="mt-0.5 text-[12.5px] text-quiet-text-tertiary">Uploads and background jobs depend on these. Whoever runs this server can fix them.</p>
        </div>
      )}
      {groups.blocking.length + groups.services.length === 0 && groups.unavailable.length === 0 ? (
        <p className="py-5 text-sm text-quiet-text-tertiary">This server doesn’t report any services to check.</p>
      ) : (
        <CapabilityList items={[...groups.blocking, ...groups.services]} {...rowProps} />
      )}
      {groups.unavailable.length > 0 && (
        <div className="mt-8">
          <h3 className="text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">Not available on this server</h3>
          <CapabilityList items={groups.unavailable} muted {...rowProps} />
        </div>
      )}
      <p className="mt-6 border-t border-quiet-divider-light pt-4 text-[12.5px] text-quiet-text-tertiary">
        The same checks run from the terminal with <code className="font-mono text-[12px] text-quiet-text-secondary">helpin doctor</code>.
      </p>
    </div>
  );
}

type RowContext = SystemStatusPanelProps;

function CapabilityList({ items, muted, ...context }: RowContext & { items: Capability[]; muted?: boolean }) {
  if (items.length === 0) return null;
  return (
    <ul className="divide-y divide-quiet-divider-light">
      {items.map((capability) => <CapabilityRow key={capability.key} capability={capability} muted={muted} {...context} />)}
    </ul>
  );
}

function CapabilityRow({ capability, muted, workspaceId, slug, canManage, isOwner, emailSettingsOnPage }: RowContext & { capability: Capability; muted?: boolean }) {
  const status = capabilityStatus(capability.status);
  const StatusIcon = status.icon;
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
            <p id={`capability-${capability.key}`} className={cn('text-sm font-medium leading-5', muted ? 'text-quiet-text-tertiary' : 'text-quiet-text-primary')}>
              {capabilityTitle(capability.key)}
            </p>
            <span className={cn('text-[11.5px] font-semibold uppercase tracking-[0.03em]', status.className)}>{status.label}</span>
            {isBlocking(capability) && <span className="text-[11.5px] font-semibold uppercase tracking-[0.03em] text-quiet-accent">Required</span>}
          </div>
          <p className="mt-1 max-w-2xl text-[12.5px] leading-5 text-quiet-text-tertiary">{capability.detail}</p>
          {checked && <p className="mt-1 text-[11.5px] text-quiet-muted">{checked}</p>}
        </div>
        <CapabilityStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} isOwner={isOwner} emailSettingsOnPage={emailSettingsOnPage} />
      </div>
    </li>
  );
}

function CapabilityStep({ capability, workspaceId, slug, canManage, isOwner, emailSettingsOnPage }: { capability: Capability; workspaceId: string; slug: string; canManage: boolean; isOwner: boolean; emailSettingsOnPage?: boolean }) {
  switch (capability.key) {
    case 'ai_chat':
      return <SetupAIStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} />;
    case 'email_outbound':
      // The email settings card below owns Save and Send test email.
      if (emailSettingsOnPage) return null;
      return <SetupEmailStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} />;
    case 'github':
      // The row's detail already states why GitHub is blocked; don't repeat it.
      return <SetupGitHubStep capability={capability} workspaceId={workspaceId} slug={slug} canManage={canManage} isOwner={isOwner} returnTo="system_status" detailShown />;
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
