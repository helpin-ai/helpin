import { useEffect, useState, type FormEvent } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useTitle } from '@/hooks/useTitle';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { workspacesService } from '@/lib/services/workspacesService';
import { generateWorkspaceSlug } from '@/lib/slugUtils';
import { WorkspaceSelector } from '@/components/workspace/WorkspaceSelector';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Skeleton } from '@/components/ui/skeleton';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Plus } from 'lucide-react';
import { toast } from 'sonner';

export default function Workspaces() {
  useTitle('Workspaces');
  const { workspaces, loading, loadWorkspaces } = useWorkspaceStore();
  const navigate = useNavigate();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    loadWorkspaces();
  }, [loadWorkspaces]);

  // Auto-redirect to first workspace if user has exactly one.
  useEffect(() => {
    if (!loading && workspaces.length === 1) {
      navigate({ to: '/w/$slug/dashboard', params: { slug: workspaces[0].slug } });
    }
  }, [loading, workspaces, navigate]);

  const handleNameChange = (val: string) => {
    setName(val);
    setSlug(generateWorkspaceSlug(val));
  };

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    setCreating(true);
    const { error } = await workspacesService.create({ name, slug, description: description || undefined });
    setCreating(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Workspace created');
      setDialogOpen(false);
      setName('');
      setSlug('');
      setDescription('');
      loadWorkspaces();
    }
  };

  return (
    <div className="min-h-screen bg-background">
      <div className="max-w-4xl mx-auto px-4 py-12">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold">Workspaces</h1>
            <p className="text-muted-foreground mt-1">Select a workspace or create a new one</p>
          </div>
          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogTrigger asChild>
              <Button>
                <Plus className="h-4 w-4 mr-2" />
                Create Workspace
              </Button>
            </DialogTrigger>
            <DialogContent>
              <form onSubmit={handleCreate}>
                <DialogHeader>
                  <DialogTitle>Create Workspace</DialogTitle>
                  <DialogDescription>Set up a new workspace for your organization.</DialogDescription>
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

        {loading ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {[1, 2, 3].map(i => (
              <Skeleton key={i} className="h-24 rounded-lg" />
            ))}
          </div>
        ) : workspaces.length === 0 ? (
          <div className="text-center py-16">
            <p className="text-muted-foreground mb-4">You don't have any workspaces yet.</p>
            <Button onClick={() => setDialogOpen(true)}>
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
