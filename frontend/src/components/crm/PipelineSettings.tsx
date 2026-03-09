import { useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { usePipelines, useCreatePipeline, useDeletePipeline } from '@/hooks/queries';

export function PipelineSettings() {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const { data: pipelines, isLoading } = usePipelines(wsId);
  const createPipeline = useCreatePipeline(wsId);
  const deletePipeline = useDeletePipeline(wsId);

  const [newPipelineName, setNewPipelineName] = useState('');

  const handleCreatePipeline = async () => {
    if (!newPipelineName.trim()) return;
    try {
      await createPipeline.mutateAsync({
        workspace_id: wsId,
        name: newPipelineName.trim(),
        stages: [
          { name: 'Qualification', stage_type: 'open', position: 0, probability: 10 },
          { name: 'Proposal', stage_type: 'open', position: 1, probability: 30 },
          { name: 'Negotiation', stage_type: 'open', position: 2, probability: 60 },
          { name: 'Closed Won', stage_type: 'won', position: 3, probability: 100 },
          { name: 'Closed Lost', stage_type: 'lost', position: 4, probability: 0 },
        ],
      });
      toast.success('Pipeline created');
      setNewPipelineName('');
    } catch {
      toast.error('Failed to create pipeline');
    }
  };

  if (isLoading) {
    return <div className="p-4 text-muted-foreground">Loading pipelines...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-2">
        <Input
          placeholder="New pipeline name"
          value={newPipelineName}
          onChange={(e) => setNewPipelineName(e.target.value)}
          className="max-w-xs"
        />
        <Button size="sm" onClick={handleCreatePipeline} disabled={!newPipelineName.trim()}>
          <Plus className="mr-1 h-4 w-4" />
          Add Pipeline
        </Button>
      </div>

      {(pipelines ?? []).map((pipeline) => (
        <Card key={pipeline.id}>
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-base">{pipeline.name}</CardTitle>
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive"
              onClick={async () => {
                try {
                  await deletePipeline.mutateAsync(pipeline.id);
                  toast.success('Pipeline deleted');
                } catch {
                  toast.error('Failed to delete pipeline');
                }
              }}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {(pipeline.stages ?? []).map((stage, idx) => (
                <div key={stage.id} className="flex items-center gap-2 text-sm">
                  <span className="w-6 text-muted-foreground">{idx + 1}.</span>
                  <span className="flex-1">{stage.name}</span>
                  <span className="text-xs text-muted-foreground capitalize">{stage.stage_type}</span>
                  <span className="text-xs text-muted-foreground">{stage.probability}%</span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      ))}

      {(!pipelines || pipelines.length === 0) && (
        <p className="text-sm text-muted-foreground">No pipelines configured. Create one to start managing deals.</p>
      )}
    </div>
  );
}
