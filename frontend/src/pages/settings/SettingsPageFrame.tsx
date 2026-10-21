import type { ReactNode } from 'react';
import { QuietPageHeader } from '@/components/design-system/quiet';
import { Skeleton } from '@/components/ui/skeleton';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { SETTINGS_ROUTE_SECTIONS, type SettingsRouteSection } from '@/lib/settingsSections';
import type { WorkspaceSettings } from '@/lib/types';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export type SettingsPageContext = {
  workspaceId: string;
  currentWorkspaceName: string;
  currentWorkspaceSlug: string;
  currentWorkspaceWebsiteUrl?: string;
  organizationId?: string;
  settings: WorkspaceSettings;
  access: ReturnType<typeof useWorkspaceAccess>['data'];
  permissions: ReturnType<typeof usePermissions>;
};

type SettingsPageFrameProps = {
  section: SettingsRouteSection;
  /** When true, the section title + description header is hidden.
   *  Useful when the child component renders its own header
   *  (e.g. TeamsTab showing a specific team's name). */
  hideHeader?: boolean;
  descriptionSuffix?: string;
  headerAction?: (context: SettingsPageContext) => ReactNode;
  children: (context: SettingsPageContext) => ReactNode;
};

export function SettingsPageFrame({ section, hideHeader, descriptionSuffix, headerAction, children }: SettingsPageFrameProps) {
  const sectionLabel = SETTINGS_ROUTE_SECTIONS.find((s) => s.id === section)?.label;
  useTitle(sectionLabel ? `${sectionLabel} Settings` : 'Settings');

  const { currentWorkspace } = useWorkspaceStore();
  const workspaceId = currentWorkspace?.id ?? '';
  const organizationId = currentWorkspace?.organization_id;
  const currentWorkspaceName = currentWorkspace?.name ?? '';
  const currentWorkspaceSlug = currentWorkspace?.slug ?? '';
  const { data: settings, isLoading } = useWorkspaceSettings(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const sectionMeta = SETTINGS_ROUTE_SECTIONS.find((candidate) => candidate.id === section);

  const description = [sectionMeta?.description, descriptionSuffix].filter(Boolean).join(' ') || undefined;

  const pageHeader = sectionMeta && !hideHeader ? (
    <QuietPageHeader
      title={sectionMeta.label}
      description={description}
    />
  ) : null;

  if (isLoading) {
    return (
      <div className="space-y-6">
        {pageHeader ?? <Skeleton className="h-6 w-48" />}
        <Skeleton className="h-96" />
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="space-y-4">
        {pageHeader}
        <div className="py-12 text-center">
          <p className="text-muted-foreground">Could not load workspace settings.</p>
        </div>
      </div>
    );
  }

  const resolvedWorkspaceId = workspaceId || settings.settings.workspace_id;
  if (!resolvedWorkspaceId) {
    return (
      <div className="space-y-4">
        {pageHeader}
        <div className="py-12 text-center">
          <p className="text-muted-foreground">Could not determine workspace for settings.</p>
        </div>
      </div>
    );
  }

  const context: SettingsPageContext = {
    workspaceId: resolvedWorkspaceId,
    currentWorkspaceName,
    currentWorkspaceSlug,
    currentWorkspaceWebsiteUrl: currentWorkspace?.website_url,
    organizationId,
    settings,
    access,
    permissions,
  };

  return (
    <div className="space-y-4">
      {sectionMeta && !hideHeader ? (
        <QuietPageHeader
          title={sectionMeta.label}
          description={description}
          actions={headerAction?.(context)}
        />
      ) : null}
      {children(context)}
    </div>
  );
}
