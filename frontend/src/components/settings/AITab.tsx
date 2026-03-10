import { useEffect, useState } from 'react';
import { settingsService } from '@/lib/services/settingsService';
import type { WorkspaceSettings } from '@/lib/types';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function AITab({ workspaceId, config, editable, onRefresh }: {
  workspaceId: string;
  config: WorkspaceSettings['settings'];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [planningMethodology, setPlanningMethodology] = useState(config.planning_methodology);
  const [planningWebSearchEnabled, setPlanningWebSearchEnabled] = useState(config.planning_web_search_enabled);
  const [planningWebSearchProvider, setPlanningWebSearchProvider] = useState(config.planning_web_search_provider);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setPlanningMethodology(config.planning_methodology);
    setPlanningWebSearchEnabled(config.planning_web_search_enabled);
    setPlanningWebSearchProvider(config.planning_web_search_provider);
  }, [config]);

  const handleSave = async () => {
    setSaving(true);
    const { error } = await settingsService.updateSystem(workspaceId, {
      planning_methodology: planningMethodology,
      planning_web_search_enabled: planningWebSearchEnabled,
      planning_web_search_provider: planningWebSearchProvider,
    });
    setSaving(false);
    if (error) toast.error(error);
    else { toast.success('AI settings updated'); onRefresh(); }
  };

  return (
    <div className="space-y-6">
      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">Planning Methodology</CardTitle>
          <CardDescription>
            This controls the hidden prompt pack used for epic `draft_spec` and `plan_stories` runs. It does not change story execution or support prompts yet.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="space-y-2">
            <Label>Workspace Planning Mode</Label>
            <Select
              value={planningMethodology}
              onValueChange={(value) => setPlanningMethodology(value as 'structured_v1' | 'basic_v1')}
              disabled={!editable}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="structured_v1">Structured v1</SelectItem>
                <SelectItem value="basic_v1">Basic v1</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              `Structured v1` uses a stronger internal planning methodology inspired by staged PM and architecture roles. `Basic v1` keeps planning simpler for fallback and comparison.
            </p>
          </div>

          {editable && (
            <div className="flex justify-end">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : 'Save Settings'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      <Card className={LINEAR_CARD_CLASS}>
        <CardHeader>
          <CardTitle className="text-base">External Research</CardTitle>
          <CardDescription>
            Product planners can use controlled web research during `draft_spec` to gather market context, standards, and external evidence. Sources are cited back into the saved PRD.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between gap-4 rounded-md border p-4">
            <div className="space-y-1">
              <Label className="text-sm font-medium">Enable planning web research</Label>
              <p className="text-xs text-muted-foreground">
                This only affects `product_planner` during the PRD drafting stage. Story planning remains repo/spec-driven.
              </p>
            </div>
            <Switch
              checked={planningWebSearchEnabled}
              onCheckedChange={setPlanningWebSearchEnabled}
              disabled={!editable}
            />
          </div>

          <div className="space-y-2">
            <Label>Research Provider</Label>
            <Select
              value={planningWebSearchProvider}
              onValueChange={(value) => setPlanningWebSearchProvider(value as 'brave')}
              disabled={!editable || !planningWebSearchEnabled}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="brave">Brave Search</SelectItem>
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              Brave is the first supported provider. The API server will reject enabling research if Brave credentials are not configured.
            </p>
          </div>

          {editable && (
            <div className="flex justify-end">
              <Button onClick={handleSave} disabled={saving}>
                {saving ? 'Saving...' : 'Save Settings'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
