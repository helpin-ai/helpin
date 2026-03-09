import { useState } from 'react';
import { Plus, Mail, Clock, CheckSquare, Trash2, GripVertical } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

interface SequenceStep {
  step_number: number;
  step_type: 'email' | 'delay' | 'task';
  delay_days: number;
  template_subject: string;
  template_body: string;
}

interface SequenceBuilderProps {
  steps: SequenceStep[];
  onChange: (steps: SequenceStep[]) => void;
  readOnly?: boolean;
}

const stepIcons = {
  email: Mail,
  delay: Clock,
  task: CheckSquare,
};

export function SequenceBuilder({ steps, onChange, readOnly }: SequenceBuilderProps) {
  const [editingStep, setEditingStep] = useState<number | null>(null);

  const addStep = (type: 'email' | 'delay' | 'task') => {
    const newStep: SequenceStep = {
      step_number: steps.length + 1,
      step_type: type,
      delay_days: type === 'delay' ? 1 : 0,
      template_subject: '',
      template_body: '',
    };
    onChange([...steps, newStep]);
    if (type !== 'delay') {
      setEditingStep(steps.length);
    }
  };

  const removeStep = (index: number) => {
    const newSteps = steps.filter((_, i) => i !== index).map((s, i) => ({ ...s, step_number: i + 1 }));
    onChange(newSteps);
    setEditingStep(null);
  };

  const updateStep = (index: number, updates: Partial<SequenceStep>) => {
    const newSteps = [...steps];
    newSteps[index] = { ...newSteps[index], ...updates };
    onChange(newSteps);
  };

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">Sequence Steps</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {steps.map((step, index) => {
          const Icon = stepIcons[step.step_type];
          const isEditing = editingStep === index;

          return (
            <div key={index} className="rounded-md border p-3">
              <div className="flex items-center gap-2">
                <GripVertical className="h-4 w-4 text-muted-foreground" />
                <span className="flex h-5 w-5 items-center justify-center rounded-full bg-primary text-xs text-primary-foreground">
                  {step.step_number}
                </span>
                <Icon className="h-4 w-4 text-muted-foreground" />
                <span className="flex-1 text-sm font-medium capitalize">{step.step_type}</span>
                {!readOnly && (
                  <div className="flex gap-1">
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6"
                      onClick={() => setEditingStep(isEditing ? null : index)}
                    >
                      <Mail className="h-3 w-3" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-6 w-6"
                      onClick={() => removeStep(index)}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                )}
              </div>

              {step.step_type === 'delay' && (
                <div className="mt-2 flex items-center gap-2">
                  <span className="text-xs text-muted-foreground">Wait</span>
                  <Input
                    type="number"
                    className="h-7 w-16 text-xs"
                    value={step.delay_days}
                    onChange={(e) => updateStep(index, { delay_days: Number(e.target.value) })}
                    disabled={readOnly}
                    min={1}
                  />
                  <span className="text-xs text-muted-foreground">days</span>
                </div>
              )}

              {isEditing && step.step_type !== 'delay' && (
                <div className="mt-2 space-y-2">
                  <Input
                    placeholder="Subject"
                    className="h-8 text-sm"
                    value={step.template_subject}
                    onChange={(e) => updateStep(index, { template_subject: e.target.value })}
                    disabled={readOnly}
                  />
                  <textarea
                    placeholder="Body content..."
                    className="w-full rounded-md border p-2 text-sm"
                    rows={3}
                    value={step.template_body}
                    onChange={(e) => updateStep(index, { template_body: e.target.value })}
                    disabled={readOnly}
                  />
                </div>
              )}
            </div>
          );
        })}

        {!readOnly && (
          <div className="flex gap-2 pt-2">
            <Button variant="outline" size="sm" onClick={() => addStep('email')}>
              <Plus className="mr-1 h-3 w-3" /> Email
            </Button>
            <Button variant="outline" size="sm" onClick={() => addStep('delay')}>
              <Plus className="mr-1 h-3 w-3" /> Delay
            </Button>
            <Button variant="outline" size="sm" onClick={() => addStep('task')}>
              <Plus className="mr-1 h-3 w-3" /> Task
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// Re-export step type selector for external use
export function StepTypeSelector({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="h-8 w-28">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="email">Email</SelectItem>
        <SelectItem value="delay">Delay</SelectItem>
        <SelectItem value="task">Task</SelectItem>
      </SelectContent>
    </Select>
  );
}
