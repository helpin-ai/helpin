import { useState } from 'react';
import { ArrowLeft, Play, Pause, Edit } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useSequence, useUpdateSequence } from '@/hooks/queries/useCRM';
import { SequenceBuilder } from './SequenceBuilder';
import { SequenceEnrollments } from './SequenceEnrollments';

interface SequenceDetailProps {
  workspaceId: string;
  sequenceId: string;
  onBack: () => void;
}

export function SequenceDetailView({ workspaceId, sequenceId, onBack }: SequenceDetailProps) {
  const { data: sequence } = useSequence(workspaceId, sequenceId);
  const updateSequence = useUpdateSequence(workspaceId);
  const [steps, setSteps] = useState<Array<{ step_number: number; step_type: 'email' | 'delay' | 'task'; delay_days: number; template_subject: string; template_body: string }>>([]);

  if (!sequence) return null;

  const isActive = sequence.status === 'active';
  const isDraft = sequence.status === 'draft';

  const toggleStatus = () => {
    updateSequence.mutate({
      id: sequenceId,
      status: isActive ? 'paused' : 'active',
    });
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={onBack}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h2 className="text-lg font-semibold">{sequence.name}</h2>
            {sequence.description && (
              <p className="text-sm text-muted-foreground">{sequence.description}</p>
            )}
          </div>
          <Badge variant={isActive ? 'default' : isDraft ? 'outline' : 'secondary'}>
            {sequence.status}
          </Badge>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={toggleStatus}>
            {isActive ? <Pause className="mr-1 h-3 w-3" /> : <Play className="mr-1 h-3 w-3" />}
            {isActive ? 'Pause' : 'Activate'}
          </Button>
        </div>
      </div>

      <div className="flex items-center gap-4 text-sm text-muted-foreground">
        <span>{sequence.enrollment_count} enrolled</span>
      </div>

      <Tabs defaultValue="steps">
        <TabsList>
          <TabsTrigger value="steps">Steps</TabsTrigger>
          <TabsTrigger value="enrollments">Enrollments</TabsTrigger>
        </TabsList>
        <TabsContent value="steps" className="mt-4">
          <SequenceBuilder
            steps={steps.length > 0 ? steps : []}
            onChange={setSteps}
            readOnly={isActive}
          />
        </TabsContent>
        <TabsContent value="enrollments" className="mt-4">
          <SequenceEnrollments workspaceId={workspaceId} sequenceId={sequenceId} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
