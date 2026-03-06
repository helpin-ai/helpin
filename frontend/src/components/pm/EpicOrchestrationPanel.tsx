import { useCallback, useEffect, useState } from 'react';
import { Bot, Check, Loader2, Sparkles, Trash2, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Textarea } from '@/components/ui/textarea';
import { Separator } from '@/components/ui/separator';
import { agentService } from '@/lib/services/agentService';
import type { Agent, OrchestrationProposal, ProposedStory } from '@/lib/pmTypes';

interface Props {
  epicId: string;
  workspaceId: string;
  orchestratorAgentId?: string;
  onStoriesCreated?: () => void;
}

export function EpicOrchestrationPanel({ epicId, workspaceId, orchestratorAgentId, onStoriesCreated }: Props) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState(orchestratorAgentId ?? '');
  const [proposal, setProposal] = useState<OrchestrationProposal | null>(null);
  const [editedStories, setEditedStories] = useState<ProposedStory[]>([]);
  const [additionalContext, setAdditionalContext] = useState('');
  const [orchestrating, setOrchestrating] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [assigning, setAssigning] = useState(false);

  useEffect(() => {
    agentService.list(workspaceId).then((res) => {
      const data = Array.isArray(res) ? res : (res as any)?.data ?? [];
      setAgents(data.filter((a: Agent) => a.agent_kind === 'llm'));
    });
  }, [workspaceId]);

  useEffect(() => {
    setSelectedAgentId(orchestratorAgentId ?? '');
  }, [orchestratorAgentId]);

  const handleAssign = async () => {
    if (!selectedAgentId) return;
    setAssigning(true);
    try {
      await agentService.assignOrchestrator(workspaceId, epicId, selectedAgentId);
    } catch (err) {
      console.error('Failed to assign orchestrator:', err);
    } finally {
      setAssigning(false);
    }
  };

  const handleOrchestrate = async () => {
    setOrchestrating(true);
    setProposal(null);
    try {
      const result = await agentService.orchestrateEpic(workspaceId, epicId, additionalContext);
      setProposal(result.data);
      setEditedStories(result.data?.proposed_stories || []);
    } catch (err: any) {
      console.error('Orchestration failed:', err);
    } finally {
      setOrchestrating(false);
    }
  };

  const handleConfirm = async () => {
    if (!editedStories.length) return;
    setConfirming(true);
    try {
      await agentService.confirmOrchestration(workspaceId, epicId, editedStories);
      setProposal(null);
      setEditedStories([]);
      onStoriesCreated?.();
    } catch (err) {
      console.error('Failed to confirm orchestration:', err);
    } finally {
      setConfirming(false);
    }
  };

  const removeStory = useCallback((index: number) => {
    setEditedStories((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const updateStory = useCallback((index: number, field: keyof ProposedStory, value: string | number) => {
    setEditedStories((prev) =>
      prev.map((s, i) => (i === index ? { ...s, [field]: value } : s))
    );
  }, []);

  return (
    <div className="mt-6">
      <Separator className="mb-6" />
      <div className="flex items-center gap-2 mb-4">
        <Sparkles className="h-4 w-4 text-purple-500" />
        <h3 className="text-sm font-semibold">Epic Orchestration</h3>
      </div>

      {/* Agent assignment */}
      {!orchestratorAgentId && (
        <div className="mb-4 rounded-md border border-border/60 bg-muted/20 p-3">
          <p className="text-xs text-muted-foreground mb-2">Assign an orchestrator agent to decompose this epic into stories.</p>
          <div className="flex items-center gap-2">
            <select
              value={selectedAgentId}
              onChange={(e) => setSelectedAgentId(e.target.value)}
              className="h-8 rounded-md border border-border bg-background px-2 text-xs flex-1"
            >
              <option value="">Select agent...</option>
              {agents.map((a) => (
                <option key={a.id} value={a.id}>{a.name} ({a.role || 'no role'})</option>
              ))}
            </select>
            <Button size="sm" variant="outline" onClick={handleAssign} disabled={!selectedAgentId || assigning}>
              {assigning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Bot className="h-3.5 w-3.5" />}
              Assign
            </Button>
          </div>
        </div>
      )}

      {/* Orchestrate button */}
      {(orchestratorAgentId || selectedAgentId) && !proposal && (
        <div className="space-y-3">
          <Textarea
            value={additionalContext}
            onChange={(e) => setAdditionalContext(e.target.value)}
            placeholder="Additional context or instructions for the orchestrator (optional)..."
            className="text-xs min-h-[60px]"
          />
          <Button
            onClick={handleOrchestrate}
            disabled={orchestrating}
            className="gap-1.5"
            variant="default"
            size="sm"
          >
            {orchestrating ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
            {orchestrating ? 'Orchestrating...' : 'Propose Stories'}
          </Button>
        </div>
      )}

      {/* Proposal review */}
      {proposal && editedStories.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <p className="text-xs text-muted-foreground">
              {editedStories.length} stories proposed · {proposal.tokens_used.toLocaleString()} tokens
            </p>
            <div className="flex gap-2">
              <Button size="sm" variant="ghost" onClick={() => { setProposal(null); setEditedStories([]); }}>
                <X className="h-3.5 w-3.5 mr-1" /> Discard
              </Button>
              <Button size="sm" onClick={handleConfirm} disabled={confirming} className="gap-1.5">
                {confirming ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Check className="h-3.5 w-3.5" />}
                Create {editedStories.length} Stories
              </Button>
            </div>
          </div>

          {proposal.summary && (
            <p className="text-xs text-muted-foreground bg-muted/30 rounded p-2">{proposal.summary}</p>
          )}

          <div className="space-y-2">
            {editedStories.map((story, idx) => (
              <div key={idx} className="rounded-md border border-border/60 p-3 space-y-2">
                <div className="flex items-start justify-between gap-2">
                  <input
                    value={story.name}
                    onChange={(e) => updateStory(idx, 'name', e.target.value)}
                    className="flex-1 bg-transparent text-sm font-medium outline-none"
                  />
                  <div className="flex items-center gap-1 shrink-0">
                    <Badge variant="secondary" className="text-[10px]">{story.story_type}</Badge>
                    {story.estimate && (
                      <Badge variant="outline" className="text-[10px]">{story.estimate}pt</Badge>
                    )}
                    <Button variant="ghost" size="icon" className="h-6 w-6" onClick={() => removeStory(idx)}>
                      <Trash2 className="h-3 w-3 text-muted-foreground" />
                    </Button>
                  </div>
                </div>
                <textarea
                  value={story.description}
                  onChange={(e) => updateStory(idx, 'description', e.target.value)}
                  className="w-full bg-transparent text-xs text-muted-foreground outline-none resize-none min-h-[40px]"
                />
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
