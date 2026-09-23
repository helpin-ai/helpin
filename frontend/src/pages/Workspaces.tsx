import { useEffect, useMemo } from 'react';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useAuthStore } from '@/stores/authStore';
import { useOrganizations, useWorkspaces } from '@/hooks/queries';
import { WorkspaceSelector } from '@/components/workspace/WorkspaceSelector';
import type { OrganizationWithRole } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PlusSignIcon, Logout01Icon } from '@/lib/icons';
import { UserAvatar } from '@/components/pm/UserAvatar';

/**
 * Lists the person's workspaces. Creating a workspace, including the first
 * one, happens in the onboarding flow at `/onboarding`.
 */
export default function Workspaces() {
  useTitle('Workspaces');
  const { data: organizations = [], isLoading: orgsLoading } = useOrganizations();
  const { data: allWorkspaces = [], isLoading: wsLoading, isSuccess: wsLoaded } = useWorkspaces();
  const navigate = useNavigate();
  const { signOut } = useAuthStore();
  const { create } = useSearch({ strict: false }) as { create?: boolean };
  const isLoading = wsLoading || orgsLoading;

  const openOnboarding = () => {
    void navigate({ to: '/onboarding', search: { step: 'workspace' } });
  };

  // `?create=true` links and people without a workspace continue in onboarding.
  const redirectToOnboarding = Boolean(create) || (wsLoaded && allWorkspaces.length === 0);
  useEffect(() => {
    if (redirectToOnboarding) {
      void navigate({ to: '/onboarding', search: { step: 'workspace' }, replace: true });
    }
  }, [navigate, redirectToOnboarding]);

  // Group workspaces by organization for display
  const workspacesByOrg = useMemo(() => {
    const groups: { org: OrganizationWithRole; workspaces: typeof allWorkspaces }[] = [];
    for (const org of organizations) {
      const orgWorkspaces = allWorkspaces.filter((ws) => ws.organization_id === org.id);
      if (orgWorkspaces.length > 0) {
        groups.push({ org, workspaces: orgWorkspaces });
      }
    }
    // Include workspaces with no matching org (edge case)
    const ungrouped = allWorkspaces.filter((ws) => !organizations.some((o) => o.id === ws.organization_id));
    if (ungrouped.length > 0) {
      groups.push({ org: { id: '', name: 'Other', slug: '', owner_id: '', created_at: '', updated_at: '', role: 'member' as const }, workspaces: ungrouped });
    }
    return groups;
  }, [allWorkspaces, organizations]);

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-6xl mx-auto px-4 py-12">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold">Workspaces</h1>
            <p className="text-muted-foreground mt-1">Select a workspace or create a new one</p>
          </div>
          <div className="flex flex-col-reverse items-stretch gap-2 sm:flex-row sm:items-center">
            <Button onClick={() => openOnboarding()}>
              <PlusSignIcon className="h-4 w-4 mr-2" />
              Create workspace
            </Button>
            <Button type="button" variant="outline" onClick={() => void signOut()}>
              <Logout01Icon className="h-4 w-4 mr-2" />
              Sign out
            </Button>
          </div>
        </div>

        {isLoading || redirectToOnboarding ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {[1, 2, 3].map(i => (
              <Skeleton key={i} className="h-24 rounded-lg" />
            ))}
          </div>
        ) : allWorkspaces.length === 0 ? (
          <p className="py-16 text-center text-muted-foreground">Workspaces couldn’t be loaded. Refresh the page to try again.</p>
        ) : organizations.length <= 1 ? (
          <WorkspaceSelector workspaces={allWorkspaces} />
        ) : (
          <div className="space-y-8">
            {workspacesByOrg.map(({ org, workspaces: orgWs }) => (
              <div key={org.id}>
                <div className="flex items-center gap-2 mb-4">
                  <UserAvatar name={org.name} avatarUrl={org.logo_url} className="h-5 w-5 rounded" fallbackClassName="text-[8px] rounded" />
                  <h2 className="text-sm font-medium text-muted-foreground">{org.name}</h2>
                </div>
                <WorkspaceSelector workspaces={orgWs} />
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
