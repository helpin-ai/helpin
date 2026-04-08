import type { ReactNode } from 'react';
import { Skeleton } from '@/components/ui/skeleton';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useWorkspaceSettings } from '@/hooks/queries/useSettings';
import { SETTINGS_ROUTE_SECTIONS, type SettingsSection } from '@/lib/settingsSections';
import type { WorkspaceSettings } from '@/lib/types';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export type SettingsPageContext = {
  workspaceId: string;
  currentWorkspaceName: string;
  organizationId?: string;
  settings: WorkspaceSettings;
  access: ReturnType<typeof useWorkspaceAccess>['data'];
  permissions: ReturnType<typeof usePermissions>;
};

type SettingsPageFrameProps = {
  section: SettingsSection;
  children: (context: SettingsPageContext) => ReactNode;
};

export function SettingsPageFrame({ section, children }: SettingsPageFrameProps) {
  const sectionLabel = SETTINGS_ROUTE_SECTIONS.find((s) => s.id === section)?.label;
  useTitle(sectionLabel ? `${sectionLabel} Settings` : 'Settings');

  const { currentWorkspace } = useWorkspaceStore();
  const workspaceId = currentWorkspace?.id ?? '';
  const organizationId = currentWorkspace?.organization_id;
  const currentWorkspaceName = currentWorkspace?.name ?? '';
  const { data: settings, isLoading } = useWorkspaceSettings(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);
  const sectionMeta = SETTINGS_ROUTE_SECTIONS.find((candidate) => candidate.id === section);

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <Skeleton className="h-96" />
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not load workspace settings.</p>
      </div>
    );
  }

  const resolvedWorkspaceId = workspaceId || settings.settings.workspace_id;
  if (!resolvedWorkspaceId) {
    return (
      <div className="text-center py-12">
        <p className="text-muted-foreground">Could not determine workspace for settings.</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {sectionMeta && (
        <div className="mb-2">
          <h2 className="text-xl font-semibold">{sectionMeta.label}</h2>
          {sectionMeta.description && (
            <p className="text-sm text-muted-foreground mt-1">{sectionMeta.description}</p>
          )}
        </div>
      )}
      {children({
        workspaceId: resolvedWorkspaceId,
        currentWorkspaceName,
        organizationId,
        settings,
        access,
        permissions,
      })}
    </div>
  );
}
