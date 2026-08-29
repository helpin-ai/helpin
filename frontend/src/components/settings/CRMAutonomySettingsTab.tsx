import { useState } from 'react';
import { useActivateSignalRollout, useActivateSignalRule, useAutonomySettings, useSaveSignalRoutingPolicy, useSignalOperations, useUpdateAutonomySettings, useUpdateSignalRoutingSettings } from '@/hooks/queries/useCRM';
import { useWorkspaceMembers } from '@/hooks/queries';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { useConfirm } from '@/components/ui/confirm-dialog';

export function CRMAutonomySettingsTab({ workspaceId }: { workspaceId: string }) {
  const { data: settings, isLoading } = useAutonomySettings(workspaceId);
  const updateSettings = useUpdateAutonomySettings(workspaceId);
  const signalOperations = useSignalOperations(workspaceId);
  const activateRule = useActivateSignalRule(workspaceId);
  const saveRoutingPolicy = useSaveSignalRoutingPolicy(workspaceId);
	const updateRoutingSettings = useUpdateSignalRoutingSettings(workspaceId);
	const activateRollout = useActivateSignalRollout(workspaceId);
	const members = useWorkspaceMembers(workspaceId);
  const confirm = useConfirm();
  const [saving, setSaving] = useState(false);

  const [enabledOverride, setEnabled] = useState<boolean>();
  const [autoCreateDealsOverride, setAutoCreateDeals] = useState<boolean>();
  const [autoProgressDealsOverride, setAutoProgressDeals] = useState<boolean>();
  const [autoExecuteThresholdOverride, setAutoExecuteThreshold] = useState<number>();
  const [reviewThresholdOverride, setReviewThreshold] = useState<number>();
  const [minimumPriorityOverride, setMinimumPriority] = useState<number>();
  const [requiredTrustOverride, setRequiredTrust] = useState<string>();
	const [lanePriorityOverride, setLanePriority] = useState<number>();
	const [defaultOwnerOverride, setDefaultOwner] = useState<string>();
  const enabled = enabledOverride ?? settings?.enabled ?? true;
  const autoCreateDeals = autoCreateDealsOverride ?? settings?.auto_create_deals ?? true;
  const autoProgressDeals = autoProgressDealsOverride ?? settings?.auto_progress_deals ?? true;
  const autoExecuteThreshold = autoExecuteThresholdOverride ?? settings?.auto_execute_threshold ?? 0.9;
  const reviewThreshold = reviewThresholdOverride ?? settings?.review_threshold ?? 0.7;
  const minimumPriority = minimumPriorityOverride ?? signalOperations.data?.policy?.minimum_priority ?? 20;
  const requiredTrust = requiredTrustOverride ?? signalOperations.data?.policy?.required_trust ?? 'verified';
	const lanePriority = lanePriorityOverride ?? signalOperations.data?.routingSettings.minimum_lane_priority ?? 5;
	const defaultOwner = defaultOwnerOverride ?? signalOperations.data?.routingSettings.default_signal_owner_member_id ?? 'unassigned';

  const handleActivateRule = async (ruleKey: string, version: number) => {
    const accepted = await confirm({
      title: `Make ${ruleKey.replace(/_/g, ' ')} v${version} live?`,
      description: 'This promotes the exact rule version for this workspace. Only evidence that also passes identity, priority, deduplication, and routing policy can become actionable.',
      confirmText: 'Promote version',
    });
    if (!accepted) return;
    try {
      await activateRule.mutateAsync({ ruleKey, version });
      toast.success('Signal rule promoted');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Signal rule could not be promoted');
    }
  };

  const handleSaveRouting = async () => {
    try {
      await saveRoutingPolicy.mutateAsync({ minimum_priority: minimumPriority, required_trust: requiredTrust, route_to_owner: true, channels: ['feed'] });
      toast.success('Signal routing policy activated');
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Routing policy could not be saved');
    }
  };

	const handleSaveInboxSettings = async () => {
		try {
			await updateRoutingSettings.mutateAsync({
				minimum_lane_priority: lanePriority,
				...(defaultOwner === 'unassigned'
					? { clear_default_signal_owner: true }
					: { default_signal_owner_member_id: defaultOwner }),
			});
			toast.success('Signal inbox settings saved');
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Signal inbox settings could not be saved');
		}
	};

	const handleActivateRollout = async () => {
		const accepted = await confirm({
			title: 'Take the motion-aware signal spine live?',
			description: 'This enables delivery and customer-state processing. The cutover is allowed only while every shadow invariant passes.',
			confirmText: 'Take live',
		});
		if (!accepted) return;
		try {
			await activateRollout.mutateAsync();
			toast.success('Motion-aware signals are live');
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'The shadow gate did not pass');
		}
	};

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
          <CardTitle className="text-base">Deal recommendations</CardTitle>
          <CardDescription>
            Configure LLM-assisted deal creation and stage recommendations. Signal extraction confidence and recommendation confidence are evaluated separately.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Enable deal recommendations</Label>
              <p className="text-xs text-muted-foreground">Allow CRM activity to produce deal suggestions</p>
            </div>
            <Switch checked={enabled} onCheckedChange={setEnabled} />
          </div>

          <Separator />

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Create-deal recommendations</Label>
              <p className="text-xs text-muted-foreground">Suggest a deal when corroborated activity shows buying intent</p>
            </div>
            <Switch checked={autoCreateDeals} onCheckedChange={setAutoCreateDeals} disabled={!enabled} />
          </div>

          <div className="flex items-center justify-between">
            <div>
              <Label className="text-sm font-medium">Stage-change recommendations</Label>
              <p className="text-xs text-muted-foreground">Suggest the next stage when recent evidence shows real progress</p>
            </div>
            <Switch checked={autoProgressDeals} onCheckedChange={setAutoProgressDeals} disabled={!enabled} />
          </div>

          <Separator />

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label className="text-sm font-medium">Direct execution threshold</Label>
              <p className="text-xs text-muted-foreground">Recommendation confidence required after every supporting rule version passes activation policy</p>
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
              <Label className="text-sm font-medium">Review threshold</Label>
              <p className="text-xs text-muted-foreground">Recommendation confidence required to enter the human review queue</p>
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
              <li>Recommendation confidence &ge; {(autoExecuteThreshold * 100).toFixed(0)}%: direct execution only when every supporting signal is verified and its exact rule version is live</li>
              <li>Recommendation confidence {(reviewThreshold * 100).toFixed(0)}%&ndash;{(autoExecuteThreshold * 100).toFixed(0)}%: pending review</li>
              <li>Unversioned, shadow, or lower-trust evidence always requires review, regardless of confidence</li>
            </ul>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Signal activation and routing</CardTitle>
          <CardDescription>Rules start in shadow mode. Promote an exact version only after reviewing its evidence, then control which eligible signals enter the inbox.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
			<div className="rounded-md border bg-muted/20 p-3">
				<div className="flex flex-wrap items-center justify-between gap-3">
					<div>
						<div className="flex items-center gap-2">
							<p className="text-sm font-medium">Motion-aware signal rollout</p>
							<Badge variant={signalOperations.data?.rollout.mode === 'live' ? 'default' : 'secondary'}>
								{signalOperations.data?.rollout.mode === 'live' ? 'Live' : 'Shadow'}
							</Badge>
						</div>
						<p className="mt-1 text-xs text-muted-foreground">
							{signalOperations.data?.shadowGate.observation_count ?? 0} observations · {Math.round((signalOperations.data?.shadowGate.unmapped_observation_rate ?? 0) * 1000) / 10}% unmapped · {Math.round((signalOperations.data?.shadowGate.duplicate_fingerprint_rate ?? 0) * 1000) / 10}% duplicate fingerprints
						</p>
						{signalOperations.data?.rollout.mode !== 'live' ? (
							<p className="mt-1 text-xs text-muted-foreground">
								Admin preview: {signalOperations.data?.shadowPreview.total ?? 0} evidence-gated accounts across {signalOperations.data?.shadowPreview.lanes.length ?? 0} motion lanes. The regular inbox remains empty until cutover.
							</p>
						) : null}
					</div>
					{signalOperations.data?.rollout.mode !== 'live' ? (
						<Button size="sm" onClick={() => void handleActivateRollout()} disabled={!signalOperations.data?.shadowGate.eligible || activateRollout.isPending}>
							{activateRollout.isPending ? 'Taking live…' : 'Take live'}
						</Button>
					) : null}
				</div>
				{(signalOperations.data?.shadowGate.immutable_meaning_violations ?? 0) > 0 ? (
					<p className="mt-2 text-xs text-destructive">Cutover blocked: stored signal meaning changed across a rescore.</p>
				) : null}
			</div>

			<div className="grid gap-4 sm:grid-cols-2">
				<div className="space-y-2">
					<Label>Minimum lane priority</Label>
					<Select value={String(lanePriority)} onValueChange={(value) => setLanePriority(Number(value))}>
						<SelectTrigger><SelectValue /></SelectTrigger>
						<SelectContent>{[0, 5, 8, 10, 15, 20, 30].map((value) => <SelectItem key={value} value={String(value)}>{value}+</SelectItem>)}</SelectContent>
					</Select>
					<p className="text-xs text-muted-foreground">An account enters a lane only when evidence for that motion reaches this score.</p>
				</div>
				<div className="space-y-2">
					<Label>Default signal owner</Label>
					<Select value={defaultOwner} onValueChange={setDefaultOwner}>
						<SelectTrigger><SelectValue /></SelectTrigger>
						<SelectContent>
							<SelectItem value="unassigned">Unassigned but visible</SelectItem>
							{(members.data ?? []).map((member) => <SelectItem key={member.id} value={member.id}>{member.full_name || member.email || 'Unnamed member'}</SelectItem>)}
						</SelectContent>
					</Select>
					<p className="text-xs text-muted-foreground">Used after CS owner and company owner; workspace owner remains the final fallback.</p>
				</div>
			</div>
			<div className="flex justify-end">
				<Button variant="outline" size="sm" onClick={() => void handleSaveInboxSettings()} disabled={updateRoutingSettings.isPending}>{updateRoutingSettings.isPending ? 'Saving…' : 'Save inbox settings'}</Button>
			</div>

			<Separator />

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2">
              <Label>Minimum business priority</Label>
              <Select value={String(minimumPriority)} onValueChange={(value) => setMinimumPriority(Number(value))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {[10, 15, 20, 25, 30, 40, 50].map((value) => <SelectItem key={value} value={String(value)}>{value}+</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Required identity trust</Label>
              <Select value={requiredTrust} onValueChange={setRequiredTrust}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="verified">Verified only</SelectItem>
                  <SelectItem value="probabilistic">Probabilistic or verified</SelectItem>
                  <SelectItem value="untrusted">All identity levels</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border bg-muted/20 p-3">
            <div>
              <p className="text-sm font-medium">Active routing policy</p>
              <p className="text-xs text-muted-foreground">{signalOperations.data?.policy ? `Version ${signalOperations.data.policy.version} · owner feed routing` : 'No active policy. Signals remain context-only.'}</p>
            </div>
            <Button variant="outline" size="sm" onClick={() => void handleSaveRouting()} disabled={saveRoutingPolicy.isPending}>{saveRoutingPolicy.isPending ? 'Activating…' : 'Save and activate policy'}</Button>
          </div>

          <Separator />

          <div>
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-medium">Rule versions</p>
                <p className="text-xs text-muted-foreground">Precision is based on explicit review feedback, never extraction confidence.</p>
              </div>
              <Badge variant="outline">{signalOperations.data?.rules.length ?? 0} rules</Badge>
            </div>
            {signalOperations.isLoading ? <Skeleton className="mt-3 h-24 w-full" /> : (
              <div className="mt-3 divide-y rounded-md border">
                {(signalOperations.data?.rules ?? []).map((rule) => {
                  const samples = (signalOperations.data?.precision ?? []).filter((row) => row.rule_key === rule.rule_key && row.rule_version === rule.version);
                  const reviewed = samples.reduce((sum, row) => sum + row.reviewed_count, 0);
                  const valid = samples.reduce((sum, row) => sum + row.valid_count, 0);
                  const precision = reviewed > 0 ? Math.round((valid / reviewed) * 100) : null;
                  const live = rule.activation_eligible && !rule.shadow_mode;
                  const status = !rule.enabled ? 'Disabled' : !rule.activation_eligible ? 'Context only' : live ? 'Live' : 'Shadow';
                  return (
                    <div key={`${rule.rule_key}:${rule.version}`} className="flex flex-wrap items-center justify-between gap-3 px-3 py-3">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="text-sm font-medium">{rule.rule_key.replace(/_/g, ' ')}</p>
                          <Badge variant={live ? 'default' : 'secondary'}>{status}</Badge>
                          <span className="text-xs text-muted-foreground">v{rule.version}</span>
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">{reviewed > 0 ? `${precision}% precision · ${reviewed} reviewed · ${samples.reduce((sum, row) => sum + row.acted_count, 0)} acted on` : 'No reviewed sample yet'}</p>
						{(() => {
							const outcomes = (signalOperations.data?.outcomes ?? []).filter((row) => row.rule_key === rule.rule_key && row.rule_version === rule.version);
							const matured = outcomes.reduce((sum, row) => sum + row.matured_signals, 0);
							const matched = outcomes.reduce((sum, row) => sum + row.outcome_matched, 0);
							return matured > 0 ? <p className="mt-0.5 text-xs text-muted-foreground">{Math.round((matched / matured) * 100)}% subscription-outcome precision · {matured} matured signals</p> : null;
						})()}
                      </div>
                      {rule.enabled && rule.activation_eligible && !live ? <Button variant="outline" size="sm" className="h-8" onClick={() => void handleActivateRule(rule.rule_key, rule.version)} disabled={activateRule.isPending}>Promote v{rule.version}</Button> : null}
                    </div>
                  );
                })}
                {(signalOperations.data?.rules.length ?? 0) === 0 ? <p className="px-3 py-6 text-sm text-muted-foreground">No signal rule versions are available.</p> : null}
              </div>
            )}
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
