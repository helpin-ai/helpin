import { useCallback, useEffect, useState } from 'react';
import {
  ArrowRight,
  Bot,
  ChevronDown,
  ChevronRight,
  ClipboardCheck,
  Copy,
  Flag,
  Loader2,
  MessageSquare,
  Play,
  Plus,
  Save,
  Trash2,
  Wrench,
} from 'lucide-react';

import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  useFlowDBTemplates,
  useFlowDBTemplate,
  useFlowTemplates,
  useCreateFlowTemplate,
  useDeleteFlowTemplate,
  useDuplicateFlowTemplate,
  useDuplicateFlowTemplateFromSlug,
  useCreateFlowTemplateNode,
  useUpdateFlowTemplateNode,
  useDeleteFlowTemplateNode,
} from '@/hooks/queries/useFlow';
import { FlowPipeline } from '@/components/pm/FlowPipeline';
import { StartFlowDialog } from '@/components/pm/StartFlowDialog';

import type {
  Agent,
  FlowTemplate,
  FlowTemplateNode,
  FlowNodeType,
  FlowNodeSpec,
  FlowSpec,
  CreateFlowTemplateRequest,
  CreateFlowTemplateNodeRequest,
  UpdateFlowTemplateNodeRequest,
} from '@/lib/pmTypes';
import { agentService } from '@/lib/services/agentService';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const NODE_TYPE_OPTIONS: { value: FlowNodeType; label: string; icon: typeof Wrench }[] = [
  { value: 'system_action', label: 'System Action', icon: Wrench },
  { value: 'interactive_agent', label: 'Interactive Agent', icon: MessageSquare },
  { value: 'agent_task', label: 'Agent Task', icon: Bot },
  { value: 'approval_gate', label: 'Approval Gate', icon: ClipboardCheck },
  { value: 'terminal', label: 'Terminal', icon: Flag },
];

const TARGET_TYPE_OPTIONS = [
  { value: 'epic', label: 'Epic' },
  { value: 'story', label: 'Story' },
  { value: 'crm_deal', label: 'CRM Deal' },
];

const NODE_ICONS: Record<FlowNodeType, typeof Wrench> = {
  system_action: Wrench,
  interactive_agent: MessageSquare,
  agent_task: Bot,
  approval_gate: ClipboardCheck,
  terminal: Flag,
};

const KNOWN_ACTIONS = ['approve', 'request_changes', 'reject', 'finalize'];

const NONE_VALUE = '__none__';

// ---------------------------------------------------------------------------
// FlowTemplateEditorPage
// ---------------------------------------------------------------------------

export function FlowTemplateEditorPage() {
  const ws = useWorkspaceStore((s) => s.currentWorkspace);
  const wsId = ws?.id ?? '';

  const { data: templates, isLoading } = useFlowDBTemplates(wsId);
  const { data: flowSpecs } = useFlowTemplates(wsId);
  const [selectedTemplateId, setSelectedTemplateId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [startDialogOpen, setStartDialogOpen] = useState(false);
  const [preferredTemplateSlug, setPreferredTemplateSlug] = useState<string | null>(null);

  const [agents, setAgents] = useState<Agent[]>([]);
  const duplicateMutation = useDuplicateFlowTemplate(wsId);
  const duplicateFromSlugMutation = useDuplicateFlowTemplateFromSlug(wsId);

  useEffect(() => {
    if (!wsId) return;
    agentService.list(wsId).then((res) => {
      if (res.data) setAgents(res.data);
    });
  }, [wsId]);

  const handleStartFlow = useCallback((templateSlug: string) => {
    setPreferredTemplateSlug(templateSlug);
    setStartDialogOpen(true);
  }, []);

  const handleDuplicate = useCallback(async (templateId: string) => {
    const result = await duplicateMutation.mutateAsync(templateId);
    setSelectedTemplateId(result.template.id);
  }, [duplicateMutation]);

  const handleDuplicateFromSlug = useCallback(async (templateSlug: string) => {
    const result = await duplicateFromSlugMutation.mutateAsync(templateSlug);
    setSelectedTemplateId(result.template.id);
  }, [duplicateFromSlugMutation]);

  // System flows that don't have a corresponding DB template
  const dbTemplateSlugs = new Set(templates?.map((t) => t.template_slug) ?? []);
  const systemOnlySpecs = (flowSpecs ?? []).filter(
    (spec) => !dbTemplateSlugs.has(spec.template_id),
  );

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Flow Templates</h2>
          <p className="text-sm text-muted-foreground">
            Define reusable execution flows for agents and automations.
          </p>
        </div>
        <Button size="sm" className="gap-1.5" onClick={() => setShowCreate(true)}>
          <Plus className="h-3.5 w-3.5" />
          New Template
        </Button>
      </div>

      {isLoading && <p className="text-sm text-muted-foreground">Loading templates...</p>}

      {systemOnlySpecs.length > 0 && (
        <>
          <h3 className="text-sm font-medium text-muted-foreground">System Flows</h3>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {systemOnlySpecs.map((spec) => (
              <SystemFlowCard
                key={spec.template_id}
                spec={spec}
                onStartFlow={() => handleStartFlow(spec.template_id)}
                onDuplicate={() => handleDuplicateFromSlug(spec.template_id)}
              />
            ))}
          </div>
        </>
      )}

      {templates && templates.length > 0 && (
        <>
          <h3 className="text-sm font-medium text-muted-foreground">Custom & Builtin Templates</h3>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {templates.map((t) => (
              <TemplateCard
                key={t.id}
                template={t}
                onSelect={() => setSelectedTemplateId(t.id)}
                onStartFlow={() => handleStartFlow(t.template_slug)}
                onDuplicate={() => handleDuplicate(t.id)}
              />
            ))}
          </div>
        </>
      )}

      {!isLoading && templates?.length === 0 && systemOnlySpecs.length === 0 && (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            No flow templates found.
          </CardContent>
        </Card>
      )}

      {showCreate && (
        <CreateTemplateDialog
          wsId={wsId}
          open={showCreate}
          onOpenChange={setShowCreate}
        />
      )}

      {selectedTemplateId && (
        <TemplateDetailDialog
          wsId={wsId}
          templateId={selectedTemplateId}
          open={!!selectedTemplateId}
          onOpenChange={(open) => { if (!open) setSelectedTemplateId(null); }}
          onDuplicate={handleDuplicate}
          agents={agents}
        />
      )}

      {flowSpecs && (
        <StartFlowDialog
          open={startDialogOpen}
          onOpenChange={(open) => {
            setStartDialogOpen(open);
            if (!open) setPreferredTemplateSlug(null);
          }}
          workspaceId={wsId}
          templates={flowSpecs}
          preferredTemplateId={preferredTemplateSlug}
        />
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// TemplateCard
// ---------------------------------------------------------------------------

function TemplateCard({ template, onSelect, onStartFlow, onDuplicate }: { template: FlowTemplate; onSelect: () => void; onStartFlow: () => void; onDuplicate: () => void }) {
  const pipelineNodes = (template.nodes ?? [])
    .sort((a, b) => a.position - b.position)
    .map((n) => ({ id: n.node_slug, type: n.node_type, label: n.label }) as FlowNodeSpec);

  return (
    <Card className="cursor-pointer hover:border-primary/40 transition-colors" onClick={onSelect}>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold truncate">{template.name}</h3>
          <div className="flex items-center gap-1.5">
            {template.is_builtin && (
              <Badge variant="secondary" className="text-[10px]">Builtin</Badge>
            )}
            <Badge variant="outline" className="text-[10px]">v{template.version}</Badge>
          </div>
        </div>
        {template.description && (
          <p className="text-[12px] text-muted-foreground line-clamp-2">{template.description}</p>
        )}
      </CardHeader>
      <CardContent className="pt-0">
        <div className="flex items-center gap-2 text-[12px] text-muted-foreground">
          <Badge variant="outline" className="text-[10px]">{template.target_type}</Badge>
          <span>{template.nodes?.length ?? 0} nodes</span>
          <span>· starts at {template.initial_node_slug}</span>
        </div>

        {pipelineNodes.length > 0 && (
          <div className="mt-3 rounded-md border border-border/60 bg-muted/20 p-2.5">
            <FlowPipeline nodes={pipelineNodes} compact />
          </div>
        )}

        <div className="flex items-center gap-2 mt-3">
          <Button
            size="sm"
            className="gap-1.5"
            onClick={(e) => {
              e.stopPropagation();
              onStartFlow();
            }}
          >
            <Play className="h-3.5 w-3.5" />
            Start Flow
          </Button>
          {template.is_builtin && !template.workspace_id && (
            <Button
              size="sm"
              variant="outline"
              className="gap-1.5"
              onClick={(e) => {
                e.stopPropagation();
                onDuplicate();
              }}
            >
              <Copy className="h-3.5 w-3.5" />
              Duplicate
            </Button>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// SystemFlowCard — for hardcoded system templates not in the DB
// ---------------------------------------------------------------------------

function SystemFlowCard({ spec, onStartFlow, onDuplicate }: { spec: FlowSpec; onStartFlow: () => void; onDuplicate: () => void }) {
  return (
    <Card className="hover:border-primary/40 transition-colors">
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold truncate">{spec.name ?? spec.template_id}</h3>
          <div className="flex items-center gap-1.5">
            <Badge variant="default" className="text-[10px]">System</Badge>
            <Badge variant="outline" className="text-[10px]">v{spec.template_version}</Badge>
          </div>
        </div>
        {spec.description && (
          <p className="text-[12px] text-muted-foreground line-clamp-2">{spec.description}</p>
        )}
      </CardHeader>
      <CardContent className="pt-0">
        <div className="flex items-center gap-2 text-[12px] text-muted-foreground">
          <Badge variant="outline" className="text-[10px]">{spec.target_type}</Badge>
          <span>{spec.nodes?.length ?? 0} nodes</span>
        </div>

        {spec.nodes && spec.nodes.length > 0 && (
          <div className="mt-3 rounded-md border border-border/60 bg-muted/20 p-2.5">
            <FlowPipeline nodes={spec.nodes} compact />
          </div>
        )}

        <div className="flex items-center gap-2 mt-3">
          <Button
            size="sm"
            className="gap-1.5"
            onClick={onStartFlow}
          >
            <Play className="h-3.5 w-3.5" />
            Start Flow
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5"
            onClick={onDuplicate}
          >
            <Copy className="h-3.5 w-3.5" />
            Duplicate to Edit
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

// ---------------------------------------------------------------------------
// CreateTemplateDialog
// ---------------------------------------------------------------------------

function CreateTemplateDialog({
  wsId,
  open,
  onOpenChange,
}: {
  wsId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');
  const [targetType, setTargetType] = useState('story');

  const createMutation = useCreateFlowTemplate(wsId);

  const handleCreate = async () => {
    if (!name.trim() || !slug.trim()) return;

    const req: CreateFlowTemplateRequest = {
      name: name.trim(),
      description: description.trim() || undefined,
      template_slug: slug.trim(),
      target_type: targetType,
      initial_node_slug: 'start',
      nodes: [
        {
          node_slug: 'start',
          label: 'Start',
          node_type: 'agent_task',
          position: 0,
          next_node_slug: 'done',
        },
        {
          node_slug: 'done',
          label: 'Complete',
          node_type: 'terminal',
          position: 1,
        },
      ],
    };

    await createMutation.mutateAsync(req);
    onOpenChange(false);
    setName('');
    setSlug('');
    setDescription('');
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create Flow Template</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Engineer Story" />
          </div>
          <div>
            <Label>Slug</Label>
            <Input
              value={slug}
              onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9._-]/g, ''))}
              placeholder="pm.engineer_story"
            />
            <p className="text-[11px] text-muted-foreground mt-1">Unique identifier, e.g. pm.my_flow</p>
          </div>
          <div>
            <Label>Target Type</Label>
            <Select value={targetType} onValueChange={setTargetType}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                {TARGET_TYPE_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div>
            <Label>Description</Label>
            <Textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={2} placeholder="Optional description..." />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={handleCreate} disabled={!name.trim() || !slug.trim() || createMutation.isPending}>
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// TemplateDetailDialog -- View and edit template nodes
// ---------------------------------------------------------------------------

function TemplateDetailDialog({
  wsId,
  templateId,
  open,
  onOpenChange,
  onDuplicate,
  agents,
}: {
  wsId: string;
  templateId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDuplicate: (templateId: string) => Promise<void>;
  agents: Agent[];
}) {
  const { data, isLoading } = useFlowDBTemplate(wsId, templateId);
  const deleteMutation = useDeleteFlowTemplate(wsId);
  const createNodeMutation = useCreateFlowTemplateNode(wsId, templateId);
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());
  const [showAddNode, setShowAddNode] = useState(false);

  const toggleNode = useCallback((nodeId: string) => {
    setExpandedNodes((prev) => {
      const next = new Set(prev);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });
  }, []);

  if (isLoading || !data) {
    return (
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className="sm:max-w-2xl">
          <p className="text-sm text-muted-foreground py-8 text-center">Loading template...</p>
        </DialogContent>
      </Dialog>
    );
  }

  const { template, nodes } = data;
  const isEditable = !template.is_builtin || !!template.workspace_id;
  const sortedNodes = [...nodes].sort((a, b) => a.position - b.position);

  const handleDelete = async () => {
    if (!confirm('Archive this template? It will no longer be available for new flows.')) return;
    await deleteMutation.mutateAsync(templateId);
    onOpenChange(false);
  };

  const handleAddNode = async (newNode: CreateFlowTemplateNodeRequest) => {
    await createNodeMutation.mutateAsync(newNode);
    setShowAddNode(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {template.name}
            <Badge variant="outline" className="text-[10px]">v{template.version}</Badge>
            {template.is_builtin && <Badge variant="secondary" className="text-[10px]">Builtin</Badge>}
          </DialogTitle>
          {template.description && (
            <p className="text-sm text-muted-foreground">{template.description}</p>
          )}
        </DialogHeader>

        <div className="space-y-1 text-sm">
          <div className="flex items-center gap-4">
            <span className="text-muted-foreground">Target:</span>
            <Badge variant="outline">{template.target_type}</Badge>
            <span className="text-muted-foreground ml-4">Initial node:</span>
            <Badge variant="outline">{template.initial_node_slug}</Badge>
          </div>
        </div>

        <div className="mt-4 space-y-2">
          <div className="flex items-center justify-between">
            <h4 className="text-sm font-semibold">Nodes ({sortedNodes.length})</h4>
            {isEditable && (
              <Button
                size="sm"
                variant="outline"
                className="gap-1.5 h-7 text-xs"
                onClick={() => setShowAddNode(true)}
              >
                <Plus className="h-3 w-3" />
                Add Node
              </Button>
            )}
          </div>

          <div className="space-y-2">
            {sortedNodes.map((node) => (
              <EditableNodeCard
                key={node.id}
                node={node}
                allNodes={sortedNodes}
                isInitial={node.node_slug === template.initial_node_slug}
                isEditable={isEditable}
                isExpanded={expandedNodes.has(node.id)}
                onToggle={() => toggleNode(node.id)}
                wsId={wsId}
                templateId={templateId}
                agents={agents}
              />
            ))}
          </div>

          {showAddNode && (
            <AddNodeForm
              existingNodes={sortedNodes}
              onAdd={handleAddNode}
              onCancel={() => setShowAddNode(false)}
              isPending={createNodeMutation.isPending}
            />
          )}
        </div>

        {isEditable ? (
          <DialogFooter className="mt-4">
            <Button variant="destructive" size="sm" onClick={handleDelete} disabled={deleteMutation.isPending}>
              <Trash2 className="h-3.5 w-3.5 mr-1" />
              Archive
            </Button>
          </DialogFooter>
        ) : (
          <DialogFooter className="mt-4">
            <p className="text-xs text-muted-foreground mr-auto">
              This template is read-only. Duplicate it to create an editable copy.
            </p>
            <Button
              size="sm"
              className="gap-1.5"
              onClick={() => onDuplicate(templateId)}
            >
              <Copy className="h-3.5 w-3.5" />
              Duplicate to Edit
            </Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// EditableNodeCard -- Collapsible node with inline editing form
// ---------------------------------------------------------------------------

function EditableNodeCard({
  node,
  allNodes,
  isInitial,
  isEditable,
  isExpanded,
  onToggle,
  wsId,
  templateId,
  agents,
}: {
  node: FlowTemplateNode;
  allNodes: FlowTemplateNode[];
  isInitial: boolean;
  isEditable: boolean;
  isExpanded: boolean;
  onToggle: () => void;
  wsId: string;
  templateId: string;
  agents: Agent[];
}) {
  const Icon = NODE_ICONS[node.node_type] ?? Wrench;
  const typeOption = NODE_TYPE_OPTIONS.find((o) => o.value === node.node_type);
  const updateMutation = useUpdateFlowTemplateNode(wsId, templateId);
  const deleteMutation = useDeleteFlowTemplateNode(wsId, templateId);

  // Local form state, initialized from the node
  const [form, setForm] = useState<UpdateFlowTemplateNodeRequest>(() => ({
    label: node.label,
    node_type: node.node_type,
    position: node.position,
    next_node_slug: node.next_node_slug ?? '',
    loopback_node_slug: node.loopback_node_slug ?? '',
    agent_input_key: node.agent_input_key ?? '',
    default_agent_id: node.default_agent_id ?? '',
    system_prompt: node.system_prompt ?? '',
    allowed_tools: node.allowed_tools ?? [],
    output_tag: node.output_tag ?? '',
    actions: node.actions ?? [],
    retryable: node.retryable ?? false,
    command_name: node.command_name ?? '',
    approve_command_name: node.approve_command_name ?? '',
    feedback_from_node: node.feedback_from_node ?? '',
    additional_context_key: node.additional_context_key ?? '',
  }));

  const setField = <K extends keyof UpdateFlowTemplateNodeRequest>(key: K, value: UpdateFlowTemplateNodeRequest[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const otherNodeSlugs = allNodes.filter((n) => n.id !== node.id).map((n) => n.node_slug);
  const allNodeSlugs = allNodes.map((n) => n.node_slug);

  const nodeType = form.node_type ?? node.node_type;
  const showSystemPrompt = nodeType === 'interactive_agent' || nodeType === 'agent_task';
  const showAgentInputKey = nodeType === 'interactive_agent' || nodeType === 'agent_task';
  const showActions = nodeType === 'approval_gate' || nodeType === 'interactive_agent';
  const showLoopback = nodeType === 'approval_gate';
  const showRetryable = nodeType === 'system_action';
  const showCommandName = nodeType === 'system_action';

  const handleSave = async () => {
    const body: UpdateFlowTemplateNodeRequest = { ...form };
    // Clear empty strings to undefined/empty for optional fields
    if (!body.next_node_slug) body.next_node_slug = undefined;
    if (!body.loopback_node_slug) body.loopback_node_slug = undefined;
    if (!body.agent_input_key) body.agent_input_key = undefined;
    if (!body.default_agent_id) body.default_agent_id = undefined;
    if (!body.system_prompt) body.system_prompt = undefined;
    if (!body.output_tag) body.output_tag = undefined;
    if (!body.command_name) body.command_name = undefined;
    if (!body.approve_command_name) body.approve_command_name = undefined;
    if (!body.feedback_from_node) body.feedback_from_node = undefined;
    if (!body.additional_context_key) body.additional_context_key = undefined;
    await updateMutation.mutateAsync({ nodeId: node.id, body });
  };

  const handleDeleteNode = async () => {
    if (!confirm(`Remove node "${node.label}" (${node.node_slug})? This cannot be undone.`)) return;
    await deleteMutation.mutateAsync(node.id);
  };

  const toggleAction = (action: string) => {
    const current = form.actions ?? [];
    if (current.includes(action)) {
      setField('actions', current.filter((a) => a !== action));
    } else {
      setField('actions', [...current, action]);
    }
  };

  return (
    <div className="rounded-lg border bg-background">
      {/* Collapsed header */}
      <button
        type="button"
        className="flex items-center gap-3 w-full p-3 text-left hover:bg-muted/30 transition-colors rounded-lg"
        onClick={onToggle}
      >
        <div className="h-7 w-7 rounded-full bg-muted flex items-center justify-center shrink-0">
          <Icon className="h-3.5 w-3.5" />
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">{node.label}</span>
            <Badge variant="outline" className="text-[10px]">{node.node_slug}</Badge>
            {isInitial && <Badge variant="secondary" className="text-[10px]">Initial</Badge>}
          </div>
          <div className="flex items-center gap-2 text-[11px] text-muted-foreground flex-wrap">
            <span>{typeOption?.label ?? node.node_type}</span>
            {node.next_node_slug && (
              <>
                <ArrowRight className="h-2.5 w-2.5" />
                <span>{node.next_node_slug}</span>
              </>
            )}
            {node.loopback_node_slug && (
              <span className="text-amber-600">(loopback: {node.loopback_node_slug})</span>
            )}
            {node.command_name && <Badge variant="outline" className="text-[10px]">{node.command_name}</Badge>}
            {node.retryable && <Badge variant="outline" className="text-[10px]">Retryable</Badge>}
          </div>
        </div>

        {isExpanded ? (
          <ChevronDown className="h-4 w-4 text-muted-foreground shrink-0" />
        ) : (
          <ChevronRight className="h-4 w-4 text-muted-foreground shrink-0" />
        )}
      </button>

      {/* Expanded edit form */}
      {isExpanded && (
        <div className="border-t px-3 pb-3 pt-3 space-y-4">
          {!isEditable && (
            <p className="text-xs text-muted-foreground italic">
              This template is read-only. Fork it to make changes.
            </p>
          )}

          {/* Row 1: Label + Node Type */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="text-xs">Label</Label>
              <Input
                value={form.label ?? ''}
                onChange={(e) => setField('label', e.target.value)}
                disabled={!isEditable}
                className="h-8 text-sm"
              />
            </div>
            <div>
              <Label className="text-xs">Node Type</Label>
              <Select
                value={form.node_type}
                onValueChange={(v) => setField('node_type', v as FlowNodeType)}
                disabled={!isEditable}
              >
                <SelectTrigger className="h-8 text-sm">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {NODE_TYPE_OPTIONS.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Row 2: Next Node + Output Tag */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="text-xs">Next Node</Label>
              <Select
                value={form.next_node_slug || NONE_VALUE}
                onValueChange={(v) => setField('next_node_slug', v === NONE_VALUE ? '' : v)}
                disabled={!isEditable}
              >
                <SelectTrigger className="h-8 text-sm">
                  <SelectValue placeholder="None" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE_VALUE}>None</SelectItem>
                  {otherNodeSlugs.map((slug) => (
                    <SelectItem key={slug} value={slug}>{slug}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="text-xs">Output Tag</Label>
              <Input
                value={form.output_tag ?? ''}
                onChange={(e) => setField('output_tag', e.target.value)}
                disabled={!isEditable}
                className="h-8 text-sm"
                placeholder="e.g. planning_output"
              />
            </div>
          </div>

          {/* Conditional: Loopback Node (approval_gate only) */}
          {showLoopback && (
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label className="text-xs">Loopback Node</Label>
                <Select
                  value={form.loopback_node_slug || NONE_VALUE}
                  onValueChange={(v) => setField('loopback_node_slug', v === NONE_VALUE ? '' : v)}
                  disabled={!isEditable}
                >
                  <SelectTrigger className="h-8 text-sm">
                    <SelectValue placeholder="None" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE_VALUE}>None</SelectItem>
                    {otherNodeSlugs.map((slug) => (
                      <SelectItem key={slug} value={slug}>{slug}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs">Approve Command Name</Label>
                <Input
                  value={form.approve_command_name ?? ''}
                  onChange={(e) => setField('approve_command_name', e.target.value)}
                  disabled={!isEditable}
                  className="h-8 text-sm"
                  placeholder="e.g. approve_plan"
                />
              </div>
            </div>
          )}

          {/* Conditional: Default Agent + Agent Input Key (agent types) */}
          {showAgentInputKey && (
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label className="text-xs">Default Agent</Label>
                <Select
                  value={form.default_agent_id || NONE_VALUE}
                  onValueChange={(v) => setField('default_agent_id', v === NONE_VALUE ? '' : v)}
                  disabled={!isEditable}
                >
                  <SelectTrigger className="h-8 text-sm">
                    <SelectValue placeholder="Select an agent..." />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE_VALUE}>None (selected at flow start)</SelectItem>
                    {agents.map((a) => (
                      <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[10px] text-muted-foreground mt-0.5">Agent used by this node. Can be overridden when starting the flow.</p>
              </div>
              <div>
                <Label className="text-xs">Agent Input Key</Label>
                <Input
                  value={form.agent_input_key ?? ''}
                  onChange={(e) => setField('agent_input_key', e.target.value)}
                  disabled={!isEditable}
                  className="h-8 text-sm"
                  placeholder="agent_id"
                />
                <p className="text-[10px] text-muted-foreground mt-0.5">Override key in flow input (default: agent_id)</p>
              </div>
            </div>
          )}

          {/* Conditional: System Prompt (agent types) */}
          {showSystemPrompt && (
            <div>
              <Label className="text-xs">System Prompt</Label>
              <Textarea
                value={form.system_prompt ?? ''}
                onChange={(e) => setField('system_prompt', e.target.value)}
                disabled={!isEditable}
                className="text-sm font-mono min-h-[80px]"
                rows={3}
                placeholder="Custom system prompt for this agent node..."
              />
            </div>
          )}

          {/* Allowed Tools */}
          <div>
            <Label className="text-xs">Allowed Tools</Label>
            <Input
              value={(form.allowed_tools ?? []).join(', ')}
              onChange={(e) => {
                const raw = e.target.value;
                setField('allowed_tools', raw ? raw.split(',').map((s) => s.trim()).filter(Boolean) : []);
              }}
              disabled={!isEditable}
              className="h-8 text-sm"
              placeholder="Comma-separated tool names..."
            />
            <p className="text-[10px] text-muted-foreground mt-0.5">Separate multiple tools with commas</p>
          </div>

          {/* Conditional: Actions (approval_gate, interactive_agent) */}
          {showActions && (
            <div>
              <Label className="text-xs">Actions</Label>
              <div className="flex flex-wrap gap-2 mt-1.5">
                {KNOWN_ACTIONS.map((action) => (
                  <label
                    key={action}
                    className="flex items-center gap-1.5 text-xs cursor-pointer"
                  >
                    <Checkbox
                      checked={(form.actions ?? []).includes(action)}
                      onCheckedChange={() => toggleAction(action)}
                      disabled={!isEditable}
                    />
                    <span>{action}</span>
                  </label>
                ))}
              </div>
            </div>
          )}

          {/* Conditional: Command Name + Retryable (system_action) */}
          {showCommandName && (
            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label className="text-xs">Command Name</Label>
                <Input
                  value={form.command_name ?? ''}
                  onChange={(e) => setField('command_name', e.target.value)}
                  disabled={!isEditable}
                  className="h-8 text-sm"
                  placeholder="e.g. generate_plan"
                />
              </div>
              {showRetryable && (
                <div className="flex items-end pb-1">
                  <label className="flex items-center gap-2 cursor-pointer text-sm">
                    <Checkbox
                      checked={form.retryable ?? false}
                      onCheckedChange={(checked) => setField('retryable', checked === true)}
                      disabled={!isEditable}
                    />
                    <span>Retryable</span>
                  </label>
                </div>
              )}
            </div>
          )}

          {/* Feedback from node + Additional context key */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label className="text-xs">Feedback From Node</Label>
              <Select
                value={form.feedback_from_node || NONE_VALUE}
                onValueChange={(v) => setField('feedback_from_node', v === NONE_VALUE ? '' : v)}
                disabled={!isEditable}
              >
                <SelectTrigger className="h-8 text-sm">
                  <SelectValue placeholder="None" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE_VALUE}>None</SelectItem>
                  {allNodeSlugs.map((slug) => (
                    <SelectItem key={slug} value={slug}>{slug}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="text-xs">Additional Context Key</Label>
              <Input
                value={form.additional_context_key ?? ''}
                onChange={(e) => setField('additional_context_key', e.target.value)}
                disabled={!isEditable}
                className="h-8 text-sm"
                placeholder="e.g. epic_spec"
              />
            </div>
          </div>

          {/* Save and Delete buttons */}
          {isEditable && (
            <div className="flex items-center justify-between pt-2 border-t">
              <Button
                variant="destructive"
                size="sm"
                className="gap-1 h-7 text-xs"
                onClick={handleDeleteNode}
                disabled={deleteMutation.isPending}
              >
                {deleteMutation.isPending ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <Trash2 className="h-3 w-3" />
                )}
                Remove Node
              </Button>
              <Button
                size="sm"
                className="gap-1 h-7 text-xs"
                onClick={handleSave}
                disabled={updateMutation.isPending}
              >
                {updateMutation.isPending ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <Save className="h-3 w-3" />
                )}
                Save
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// AddNodeForm -- Inline form for creating a new node
// ---------------------------------------------------------------------------

function AddNodeForm({
  existingNodes,
  onAdd,
  onCancel,
  isPending,
}: {
  existingNodes: FlowTemplateNode[];
  onAdd: (node: CreateFlowTemplateNodeRequest) => void;
  onCancel: () => void;
  isPending: boolean;
}) {
  const nextPosition = existingNodes.length > 0
    ? Math.max(...existingNodes.map((n) => n.position)) + 1
    : 0;

  const [slug, setSlug] = useState('');
  const [label, setLabel] = useState('');
  const [nodeType, setNodeType] = useState<FlowNodeType>('agent_task');

  const handleSubmit = () => {
    if (!slug.trim() || !label.trim()) return;
    onAdd({
      node_slug: slug.trim(),
      label: label.trim(),
      node_type: nodeType,
      position: nextPosition,
    });
  };

  return (
    <div className="rounded-lg border border-dashed border-primary/40 bg-muted/20 p-3 space-y-3">
      <h5 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">New Node</h5>
      <div className="grid grid-cols-3 gap-3">
        <div>
          <Label className="text-xs">Slug</Label>
          <Input
            value={slug}
            onChange={(e) => setSlug(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, ''))}
            className="h-8 text-sm"
            placeholder="e.g. review"
            autoFocus
          />
        </div>
        <div>
          <Label className="text-xs">Label</Label>
          <Input
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            className="h-8 text-sm"
            placeholder="e.g. Review Step"
          />
        </div>
        <div>
          <Label className="text-xs">Type</Label>
          <Select value={nodeType} onValueChange={(v) => setNodeType(v as FlowNodeType)}>
            <SelectTrigger className="h-8 text-sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {NODE_TYPE_OPTIONS.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="flex items-center gap-2 justify-end">
        <Button variant="ghost" size="sm" className="h-7 text-xs" onClick={onCancel}>
          Cancel
        </Button>
        <Button
          size="sm"
          className="gap-1 h-7 text-xs"
          onClick={handleSubmit}
          disabled={!slug.trim() || !label.trim() || isPending}
        >
          {isPending ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            <Plus className="h-3 w-3" />
          )}
          Add
        </Button>
      </div>
    </div>
  );
}
