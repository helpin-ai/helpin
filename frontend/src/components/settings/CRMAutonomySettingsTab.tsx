import { useEffect, useState } from 'react';
import { useAutonomySettings, useUpdateAutonomySettings } from '@/hooks/queries/useCRM';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { toast } from 'sonner';

export function CRMAutonomySettingsTab({ workspaceId }: { workspaceId: string }) {
  const { data: settings, isLoading } = useAutonomySettings(workspaceId);
  const updateSettings = useUpdateAutonomySettings(workspaceId);
  const [saving, setSaving] = useState(false);

  const [enabled, setEnabled] = useState(true);
  const [autoCreateDeals, setAutoCreateDeals] = useState(true);
  const [autoProgressDeals, setAutoProgressDeals] = useState(true);
  const [autoExecuteThreshold, setAutoExecuteThreshold] = useState(0.9);
  const [reviewThreshold, setReviewThreshold] = useState(0.7);

  useEffect(() => {
    if (settings) {
      setEnabled(settings.enabled);
      setAutoCreateDeals(settings.auto_create_deals);
      setAutoProgressDeals(settings.auto_progress_deals);
      setAutoExecuteThreshold(settings.auto_execute_threshold);
      setReviewThreshold(settings.review_threshold);
    }
  }, [settings]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await updateSettings.mutateAsync({
        enabled,
        auto_create_deals: autoCreateDeals,
        auto_progress_deals: autoProgressDeals,
        auto_execute_threshold: autoExecuteThreshold,
        review_threshold: reviewThreshold,
      });
      toast.success('Autonomy settings saved');
    } catch {
      toast.error('Failed to save settings');
    } finally {
      setSaving(false);
    }
  };

  if (isLoading) return <Skeleton className="h-64 w-full" />;

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Self-Driving CRM</CardTitle>
          <CardDescription>
            Configure how aggressively the system auto-creates and advances deals based on detected buyer signals.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Enable Automation</Label>
              <p className="text-xs text-muted-foreground">Master switch for all CRM automation</p>
            </div>
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </div>

          <Separator />

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Auto-Create Deals</Label>
              <p className="text-xs text-muted-foreground">Automatically create deals when buying intent is detected</p>
            </div>
            <Switch checked={autoCreateDeals} onCheckedChange={setAutoCreateDeals} disabled={!enabled} />
          </div>

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Auto-Progress Deals</Label>
              <p className="text-xs text-muted-foreground">Automatically advance deal stages based on signals</p>
            </div>
            <Switch checked={autoProgressDeals} onCheckedChange={setAutoProgressDeals} disabled={!enabled} />
          </div>

          <Separator />

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label className="text-sm font-medium">Auto-Execute Threshold</Label>
              <p className="text-xs text-muted-foreground">Signals above this confidence are executed without review</p>
              <Select
                value={String(autoExecuteThreshold)}
                onValueChange={(v) => {
                  const n = Number(v);
                  setAutoExecuteThreshold(n);
                  if (n < reviewThreshold) setReviewThreshold(n);
                }}
                disabled={!enabled}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {[50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100].map((p) => (
                    <SelectItem key={p} value={String(p / 100)}>{p}%</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label className="text-sm font-medium">Review Threshold</Label>
              <p className="text-xs text-muted-foreground">Signals between this and auto-execute create pending suggestions</p>
              <Select
                value={String(reviewThreshold)}
                onValueChange={(v) => {
                  const n = Number(v);
                  setReviewThreshold(n);
                  if (n > autoExecuteThreshold) setAutoExecuteThreshold(n);
                }}
                disabled={!enabled}
              >
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {[30, 35, 40, 45, 50, 55, 60, 65, 70, 75, 80, 85, 90, 95, 100].map((p) => (
                    <SelectItem key={p} value={String(p / 100)}>{p}%</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="rounded-md border p-3 text-xs text-muted-foreground">
            <p className="font-medium text-foreground">How it works:</p>
            <ul className="mt-1 list-inside list-disc space-y-0.5">
              <li>Confidence &ge; {(autoExecuteThreshold * 100).toFixed(0)}%: auto-executed (no review needed)</li>
              <li>Confidence {(reviewThreshold * 100).toFixed(0)}%&ndash;{(autoExecuteThreshold * 100).toFixed(0)}%: pending review in the Review feed</li>
              <li>Confidence &lt; {(reviewThreshold * 100).toFixed(0)}%: low-priority suggestion</li>
            </ul>
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button onClick={handleSave} disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </Button>
      </div>
    </div>
  );
}
