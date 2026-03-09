import { useState } from 'react';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCreateDeal, usePipelines } from '@/hooks/queries';

interface CreateDealDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateDealDialog({ open, onOpenChange }: CreateDealDialogProps) {
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const createDeal = useCreateDeal(wsId);
  const { data: pipelines } = usePipelines(wsId);

  const [name, setName] = useState('');
  const [pipelineId, setPipelineId] = useState('');
  const [stageId, setStageId] = useState('');
  const [amount, setAmount] = useState('');

  const selectedPipeline = pipelines?.find((p) => p.id === pipelineId);
  const stages = selectedPipeline?.stages ?? [];

  // Auto-select first pipeline
  if (pipelines && pipelines.length > 0 && !pipelineId) {
    setPipelineId(pipelines[0].id);
    if (pipelines[0].stages && pipelines[0].stages.length > 0) {
      setStageId(pipelines[0].stages[0].id);
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !pipelineId || !stageId) return;

    try {
      await createDeal.mutateAsync({
        workspace_id: wsId,
        name: name.trim(),
        pipeline_id: pipelineId,
        stage_id: stageId,
        amount: amount ? parseFloat(amount) : undefined,
      });
      toast.success('Deal created');
      onOpenChange(false);
      setName('');
      setAmount('');
    } catch (err) {
      toast.error('Failed to create deal');
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Deal</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="dealName">Deal Name *</Label>
            <Input id="dealName" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          {pipelines && pipelines.length > 0 && (
            <div className="space-y-2">
              <Label>Pipeline</Label>
              <Select value={pipelineId} onValueChange={(v) => { setPipelineId(v); setStageId(''); }}>
                <SelectTrigger><SelectValue placeholder="Select pipeline" /></SelectTrigger>
                <SelectContent>
                  {pipelines.map((p) => (
                    <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          {stages.length > 0 && (
            <div className="space-y-2">
              <Label>Stage</Label>
              <Select value={stageId} onValueChange={setStageId}>
                <SelectTrigger><SelectValue placeholder="Select stage" /></SelectTrigger>
                <SelectContent>
                  {stages.map((s) => (
                    <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <div className="space-y-2">
            <Label htmlFor="dealAmount">Amount</Label>
            <Input id="dealAmount" type="number" step="0.01" value={amount} onChange={(e) => setAmount(e.target.value)} placeholder="0.00" />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={createDeal.isPending || !name.trim() || !pipelineId || !stageId}>
              {createDeal.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
