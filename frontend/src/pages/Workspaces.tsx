import { useEffect, useState, type FormEvent } from 'react';
import { useSearch } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useOrganizationStore } from '@/stores/organizationStore';
import { workspacesService } from '@/lib/services/workspacesService';
import { organizationsService } from '@/lib/services/organizationsService';
import { generateWorkspaceSlug } from '@/lib/slugUtils';
import { WorkspaceSelector } from '@/components/workspace/WorkspaceSelector';
import type { OrganizationWithRole } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Building2, Plus } from 'lucide-react';
import { toast } from 'sonner';

function OrgFormFields({
  name, slug, onNameChange, onSlugChange, nameId, slugId,
}: {
  name: string; slug: string;
  onNameChange: (val: string) => void; onSlugChange: (val: string) => void;
  nameId: string; slugId: string;
}) {
  return (
    <>
      <div className="space-y-2">
        <Label htmlFor={nameId}>Organization Name</Label>
        <Input id={nameId} placeholder="Acme Inc." value={name} onChange={e => onNameChange(e.target.value)} required />
      </div>
      <div className="space-y-2">
        <Label htmlFor={slugId}>Slug</Label>
        <Input id={slugId} placeholder="acme-inc" value={slug} onChange={e => onSlugChange(e.target.value)} required />
        <p className="text-xs text-muted-foreground">A URL-friendly identifier for your organization</p>
      </div>
    </>
  );
}

export default function Workspaces() {
  useTitle('Workspaces');
  const { workspaces, loading, loadWorkspaces } = useWorkspaceStore();
  const { organizations, currentOrganization, loading: orgsLoading, loadOrganizations, setCurrentOrganization } = useOrganizationStore();
  const { create } = useSearch({ from: '/_authenticated/workspaces' });
  const [dialogOpen, setDialogOpen] = useState(false);
  const [orgDialogOpen, setOrgDialogOpen] = useState(false);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [creating, setCreating] = useState(false);

  // Org creation state
  const [orgName, setOrgName] = useState('');
  const [orgSlug, setOrgSlug] = useState('');
  const [creatingOrg, setCreatingOrg] = useState(false);

  useEffect(() => {
    loadOrganizations();
  }, [loadOrganizations]);

  useEffect(() => {
    if (currentOrganization) {
      loadWorkspaces(currentOrganization.id);
    }
  }, [currentOrganization, loadWorkspaces]);

  // Auto-open create dialog when navigated with ?create=true
  useEffect(() => {
    if (create && currentOrganization && !orgsLoading) {
      setDialogOpen(true);
    }
  }, [create, currentOrganization, orgsLoading]);

  const handleNameChange = (val: string) => {
    setName(val);
    setSlug(generateWorkspaceSlug(val));
  };

  const handleOrgNameChange = (val: string) => {
    setOrgName(val);
    setOrgSlug(generateWorkspaceSlug(val));
  };

  const handleCreateOrg = async (e: FormEvent) => {
    e.preventDefault();
    setCreatingOrg(true);
    const { data, error } = await organizationsService.create({ name: orgName, slug: orgSlug });
    setCreatingOrg(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Organization created');
      setOrgDialogOpen(false);
      setOrgName('');
      setOrgSlug('');
      await loadOrganizations();
      if (data) setCurrentOrganization(data);
    }
  };

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    if (!currentOrganization) {
      toast.error('Please select an organization first');
      return;
    }
    setCreating(true);
    const { error } = await workspacesService.create({
      name,
      slug,
      organization_id: currentOrganization.id,
      description: description || undefined,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    });
    setCreating(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Workspace created');
      setDialogOpen(false);
      setName('');
      setSlug('');
      setDescription('');
      loadWorkspaces(currentOrganization.id);
    }
  };

  const handleOrgSwitch = (org: OrganizationWithRole) => {
    setCurrentOrganization(org);
  };

  const isLoading = loading || orgsLoading;

  // Show create org screen if user has no organizations
  if (!orgsLoading && organizations.length === 0) {
    return (
      <div className="min-h-screen bg-background">
        <div className="max-w-md mx-auto px-4 py-24">
          <div className="text-center mb-8">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-primary/10 mb-4">
              <Building2 className="h-7 w-7 text-primary" />
            </div>
            <h1 className="text-2xl font-bold">Create your Organization</h1>
            <p className="text-muted-foreground mt-2">
              Organizations group your workspaces and team members together.
            </p>
          </div>
          <form onSubmit={handleCreateOrg} className="space-y-4">
            <OrgFormFields
              name={orgName} slug={orgSlug}
              onNameChange={handleOrgNameChange} onSlugChange={setOrgSlug}
              nameId="org-name" slugId="org-slug"
            />
            <Button type="submit" className="w-full" disabled={creatingOrg}>
              {creatingOrg ? 'Creating...' : 'Create Organization'}
            </Button>
          </form>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-4xl mx-auto px-4 py-12">
        {/* Organization switcher */}
        {organizations.length > 0 && (
          <div className="flex items-center gap-3 mb-6">
            <Building2 className="h-5 w-5 text-muted-foreground" />
            {organizations.length === 1 ? (
              <span className="text-sm font-medium">{currentOrganization?.name}</span>
            ) : (
              <Select
                value={currentOrganization?.id ?? ''}
                onValueChange={(id) => {
                  const org = organizations.find(o => o.id === id);
                  if (org) handleOrgSwitch(org);
                }}
              >
                <SelectTrigger className="w-[240px] h-8">
                  <SelectValue placeholder="Select organization" />
                </SelectTrigger>
                <SelectContent>
                  {organizations.map(org => (
                    <SelectItem key={org.id} value={org.id}>{org.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            <Dialog open={orgDialogOpen} onOpenChange={setOrgDialogOpen}>
              <DialogTrigger asChild>
                <Button variant="outline" size="sm">
                  <Plus className="h-3.5 w-3.5 mr-1" />
                  New Org
                </Button>
              </DialogTrigger>
              <DialogContent>
                <form onSubmit={handleCreateOrg}>
                  <DialogHeader>
                    <DialogTitle>Create Organization</DialogTitle>
                    <DialogDescription>Create a new organization to group workspaces.</DialogDescription>
                  </DialogHeader>
                  <div className="space-y-4 py-4">
                    <OrgFormFields
                      name={orgName} slug={orgSlug}
                      onNameChange={handleOrgNameChange} onSlugChange={setOrgSlug}
                      nameId="new-org-name" slugId="new-org-slug"
                    />
                  </div>
                  <DialogFooter>
                    <Button type="button" variant="outline" onClick={() => setOrgDialogOpen(false)}>Cancel</Button>
                    <Button type="submit" disabled={creatingOrg}>{creatingOrg ? 'Creating...' : 'Create'}</Button>
                  </DialogFooter>
                </form>
              </DialogContent>
            </Dialog>
          </div>
        )}

        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold">Workspaces</h1>
            <p className="text-muted-foreground mt-1">Select a workspace or create a new one</p>
          </div>
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button disabled={!currentOrganization}>
                <Plus className="h-4 w-4 mr-2" />
                Create Workspace
              </Button>
            </DialogTrigger>
            <DialogContent>
              <form onSubmit={handleCreate}>
                <DialogHeader>
                  <DialogTitle>Create Workspace</DialogTitle>
                  <DialogDescription>Set up a new workspace in {currentOrganization?.name}.</DialogDescription>
                </DialogHeader>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label htmlFor="ws-name">Name</Label>
                    <Input id="ws-name" placeholder="Acme Corporation" value={name} onChange={e => handleNameChange(e.target.value)} required />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="ws-slug">Slug</Label>
                    <Input id="ws-slug" placeholder="acme-corporation" value={slug} onChange={e => setSlug(e.target.value)} required />
                    <p className="text-xs text-muted-foreground">Used in the workspace URL: /w/{slug || '...'}</p>
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="ws-desc">Description (optional)</Label>
                    <Textarea id="ws-desc" placeholder="A brief description of this workspace" value={description} onChange={e => setDescription(e.target.value)} />
                  </div>
                </div>
                <DialogFooter>
                  <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
                  <Button type="submit" disabled={creating}>
                    {creating ? 'Creating...' : 'Create'}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>

        {isLoading ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {[1, 2, 3].map(i => (
              <Skeleton key={i} className="h-24 rounded-lg" />
            ))}
          </div>
        ) : workspaces.length === 0 ? (
          <div className="text-center py-16">
            <p className="text-muted-foreground mb-4">No workspaces in this organization yet.</p>
            <Button onClick={() => setDialogOpen(true)} disabled={!currentOrganization}>
              <Plus className="h-4 w-4 mr-2" />
              Create your first workspace
            </Button>
          </div>
        ) : (
          <WorkspaceSelector workspaces={workspaces} />
        )}
      </div>
    </div>
  );
}
