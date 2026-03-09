import { useState } from 'react';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useSequences, useCreateSequence } from '@/hooks/queries/useCRM';
import { SequenceDetailView } from '@/components/crm/SequenceDetail';
import { useTitle } from '@/hooks/useTitle';

export function SequencesPage() {
  useTitle('Sequences');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const { data } = useSequences(wsId);
  const createSequence = useCreateSequence(wsId);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const sequences = data?.data ?? [];

  const handleCreate = () => {
    if (!name.trim()) return;
    createSequence.mutate(
      { workspace_id: wsId, name, description, steps: {}, status: 'draft' },
      {
        onSuccess: () => {
          setShowCreate(false);
          setName('');
          setDescription('');
        },
      }
    );
  };

  if (selectedId) {
    return (
      <div className="mx-auto max-w-5xl px-4 md:px-6">
        <SequenceDetailView
          workspaceId={wsId}
          sequenceId={selectedId}
          onBack={() => setSelectedId(null)}
        />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 md:px-6">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Sequences</h1>
          <p className="text-sm text-muted-foreground">Automate multi-step outbound workflows</p>
        </div>
        <Dialog open={showCreate} onOpenChange={setShowCreate}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="mr-1 h-4 w-4" />
              New Sequence
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create Sequence</DialogTitle>
            </DialogHeader>
            <div className="space-y-4">
              <Input placeholder="Sequence name" value={name} onChange={(e) => setName(e.target.value)} />
              <Textarea placeholder="Description (optional)" value={description} onChange={(e) => setDescription(e.target.value)} rows={3} />
              <Button onClick={handleCreate} disabled={!name.trim() || createSequence.isPending} className="w-full">
                Create
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>

      {sequences.length === 0 ? (
        <div className="py-12 text-center text-sm text-muted-foreground">
          No sequences yet. Create your first outbound sequence to get started.
        </div>
      ) : (
        <div className="space-y-3">
          {sequences.map((seq) => (
            <Card
              key={seq.id}
              className="cursor-pointer transition-colors hover:bg-accent/50"
              onClick={() => setSelectedId(seq.id)}
            >
              <CardContent className="flex items-center justify-between p-4">
                <div>
                  <p className="font-medium">{seq.name}</p>
                  {seq.description && (
                    <p className="text-xs text-muted-foreground">{seq.description}</p>
                  )}
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-xs text-muted-foreground">{seq.enrollment_count} enrolled</span>
                  <Badge variant={seq.status === 'active' ? 'default' : seq.status === 'draft' ? 'outline' : 'secondary'}>
                    {seq.status}
                  </Badge>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
