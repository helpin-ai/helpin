import { useEffect, useMemo, useState } from 'react';
import { Loader2, Play } from 'lucide-react';

import { useEpics, useStories, useDeals } from '@/hooks/queries';
import {
  useFlowDBTemplates,
  useFlowDBTemplate,
  useStartFlowRun,
} from '@/hooks/queries/useFlow';
import { agentService } from '@/lib/services/agentService';
import { extractAgentInputKeys, templateLabel } from '@/components/pm/flowConstants';
import type {
  FlowSpec,
  FlowTemplate,
  Agent,
  EpicWithStats,
  StartFlowRunRequest,
  Story,
} from '@/lib/pmTypes';
import type { CRMDeal } from '@/lib/crmTypes';

import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';

export function StartFlowDialog({
  open,
  onOpenChange,
  workspaceId,
  templates,
  preferredTemplateId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  templates: FlowSpec[];
  preferredTemplateId?: string | null;
}) {
  const [templateId, setTemplateId] = useState(preferredTemplateId ?? templates[0]?.template_id ?? '');
  const [targetEpicId, setTargetEpicId] = useState('');
  const [targetStoryId, setTargetStoryId] = useState('');
  const [targetDealId, setTargetDealId] = useState('');
  const [flowInput, setFlowInput] = useState<Record<string, unknown>>({});
  const [context, setContext] = useState('');
  const [agents, setAgents] = useState<Agent[]>([]);

  const selectedTemplate = useMemo(
    () => templates.find((template) => template.template_id === templateId) ?? templates[0],
    [templateId, templates],
  );
  const selectedTargetType = selectedTemplate?.target_type ?? 'epic';

  const { data: dbTemplates } = useFlowDBTemplates(open ? workspaceId : '');
  const selectedDbTemplate = useMemo(
    () => dbTemplates?.find((t: FlowTemplate) => t.template_slug === templateId) ?? null,
    [dbTemplates, templateId],
  );
  const { data: dbTemplateView } = useFlowDBTemplate(
    open ? workspaceId : '',
    selectedDbTemplate?.id,
  );

  const agentInputKeys = useMemo(
    () => extractAgentInputKeys(templateId, dbTemplateView?.nodes),
    [templateId, dbTemplateView?.nodes],
  );

  const epicsWorkspaceId = open && selectedTargetType === 'epic' ? workspaceId : '';
  const storiesWorkspaceId = open && selectedTargetType === 'story' ? workspaceId : '';
  const dealsWorkspaceId = open && selectedTargetType === 'crm_deal' ? workspaceId : '';

  const { data: epics } = useEpics(epicsWorkspaceId);
  const { data: storiesPage } = useStories(storiesWorkspaceId, { per_page: 100, archived: false });
  const { data: dealsPage } = useDeals(dealsWorkspaceId, { per_page: 100 });
  const startMutation = useStartFlowRun(workspaceId);

  useEffect(() => {
    if (!open || !workspaceId) return;
    agentService.list(workspaceId).then((res) => {
      if (res.data) setAgents(res.data);
    });
  }, [open, workspaceId]);

  useEffect(() => {
    if (!open) return;
    setTemplateId(preferredTemplateId ?? templates[0]?.template_id ?? '');
    setFlowInput({});
  }, [open, preferredTemplateId, templates]);

  useEffect(() => {
    setFlowInput({});
  }, [templateId]);

  const stories = storiesPage?.data ?? [];
  const deals = dealsPage?.data ?? [];

  const setInputField = (key: string, value: unknown) => {
    setFlowInput((prev) => ({ ...prev, [key]: value }));
  };

  const handleStart = async () => {
    if (!selectedTemplate) return;
    let targetId = '';

    if (selectedTargetType === 'epic') {
      if (!targetEpicId) return;
      targetId = targetEpicId;
    } else if (selectedTargetType === 'story') {
      if (!targetStoryId) return;
      targetId = targetStoryId;
    } else if (selectedTargetType === 'crm_deal') {
      if (!targetDealId) return;
      targetId = targetDealId;
    }

    const input: Record<string, unknown> = {};
    for (const keyConfig of agentInputKeys) {
      const value = flowInput[keyConfig.key];
      if (keyConfig.required && !value) return;
      if (value) input[keyConfig.key] = value;
    }
    if (context.trim()) input.additional_context = context.trim();

    const req: StartFlowRunRequest = {
      template_id: selectedTemplate.template_id,
      target_type: selectedTargetType,
      target_id: targetId,
      input,
    };
    await startMutation.mutateAsync(req);
    onOpenChange(false);
    setTargetEpicId('');
    setTargetStoryId('');
    setTargetDealId('');
    setFlowInput({});
    setContext('');
  };

  const hasRequiredAgents = agentInputKeys
    .filter((k) => k.required)
    .every((k) => !!flowInput[k.key]);

  const hasRequiredTarget =
    (selectedTargetType === 'epic' && !!targetEpicId) ||
    (selectedTargetType === 'story' && !!targetStoryId) ||
    (selectedTargetType === 'crm_deal' && !!targetDealId);

  const isStartDisabled =
    startMutation.isPending || !selectedTemplate || !hasRequiredTarget || !hasRequiredAgents;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Start Flow</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          {templates.length > 1 && (
            <div className="space-y-1.5">
              <Label>Template</Label>
              <Select value={templateId} onValueChange={setTemplateId}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {templates.map((t) => (
                    <SelectItem key={t.template_id} value={t.template_id}>
                      {templateLabel(t.template_id, t.name)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {selectedTargetType === 'epic' && (
            <div className="space-y-1.5">
              <Label>Target Epic</Label>
              <Select value={targetEpicId} onValueChange={setTargetEpicId}>
                <SelectTrigger><SelectValue placeholder="Select an epic..." /></SelectTrigger>
                <SelectContent>
                  {(epics ?? []).map((epic: EpicWithStats) => (
                    <SelectItem key={epic.epic.id} value={epic.epic.id}>{epic.epic.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {selectedTargetType === 'story' && (
            <div className="space-y-1.5">
              <Label>Target Story</Label>
              <Select value={targetStoryId} onValueChange={setTargetStoryId}>
                <SelectTrigger><SelectValue placeholder="Select a story..." /></SelectTrigger>
                <SelectContent>
                  {stories.map((story: Story) => (
                    <SelectItem key={story.id} value={story.id}>
                      {story.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {selectedTargetType === 'crm_deal' && (
            <div className="space-y-1.5">
              <Label>Target Deal</Label>
              <Select value={targetDealId} onValueChange={setTargetDealId}>
                <SelectTrigger><SelectValue placeholder="Select a deal..." /></SelectTrigger>
                <SelectContent>
                  {deals.map((deal: CRMDeal) => (
                    <SelectItem key={deal.id} value={deal.id}>
                      {deal.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {/* Dynamic agent pickers */}
          {agentInputKeys.map((keyConfig) => {
            const filteredAgents = keyConfig.agentClassFilter
              ? agents.filter((a) => a.agent_class === keyConfig.agentClassFilter)
              : agents;
            const currentValue = (flowInput[keyConfig.key] as string) || '';

            return (
              <div key={keyConfig.key} className="space-y-1.5">
                <Label>
                  {keyConfig.label}
                  {!keyConfig.required && (
                    <span className="text-muted-foreground font-normal"> (optional)</span>
                  )}
                </Label>
                {keyConfig.required ? (
                  <Select value={currentValue} onValueChange={(v) => setInputField(keyConfig.key, v)}>
                    <SelectTrigger><SelectValue placeholder="Select an agent..." /></SelectTrigger>
                    <SelectContent>
                      {filteredAgents.map((a) => (
                        <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                ) : (
                  <Select
                    value={currentValue || '_none'}
                    onValueChange={(v) => setInputField(keyConfig.key, v === '_none' ? '' : v)}
                  >
                    <SelectTrigger><SelectValue /></SelectTrigger>
                    <SelectContent>
                      <SelectItem value="_none">{keyConfig.optionalHint ?? 'None'}</SelectItem>
                      {filteredAgents.map((a) => (
                        <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              </div>
            );
          })}

          <div className="space-y-1.5">
            <Label>Additional Context <span className="text-muted-foreground font-normal">(optional)</span></Label>
            <Textarea
              value={context}
              onChange={(e) => setContext(e.target.value)}
              placeholder="Any extra instructions for the planning agents..."
              rows={3}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            onClick={handleStart}
            disabled={isStartDisabled}
          >
            {startMutation.isPending ? (
              <><Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> Starting...</>
            ) : (
              <><Play className="mr-1.5 h-4 w-4" /> Start</>
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
