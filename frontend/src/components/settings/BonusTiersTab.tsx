import { useEffect, useState } from 'react';
import { settingsService } from '@/lib/services/settingsService';
import type { BonusTierConfig } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Pencil } from 'lucide-react';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function BonusTiersTab({ workspaceId, tiers, editable, onRefresh }: {
  workspaceId: string;
  tiers: BonusTierConfig[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [localTiers, setLocalTiers] = useState(tiers);
  const [saving, setSaving] = useState(false);

  useEffect(() => { setLocalTiers(tiers); }, [tiers]);

  const handleSave = async () => {
    setSaving(true);
    const { error } = await settingsService.updateBonusTiers(
      workspaceId,
      localTiers.map(t => ({
        tier: t.tier,
        min_score: t.min_score,
        max_score: t.max_score,
        salary_multiplier: t.salary_multiplier,
        description: t.description,
        editable: t.editable,
      })),
    );
    setSaving(false);
    if (error) toast.error(error);
    else { toast.success('Bonus tiers updated'); setEditing(false); onRefresh(); }
  };

  const updateTier = (idx: number, field: string, value: string) => {
    setLocalTiers(prev => prev.map((t, i) => i === idx ? { ...t, [field]: field === 'description' ? value : Number(value) } : t));
  };

  const tierVariant = (tier: string): 'default' | 'secondary' | 'destructive' => {
    switch (tier) {
      case 'A': return 'default';
      case 'B': return 'secondary';
      default: return 'destructive';
    }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center justify-between">
          <CardTitle className="text-base">Bonus Tiers</CardTitle>
          {editable && !editing && (
            <Button size="sm" variant="outline" onClick={() => setEditing(true)}>
              <Pencil className="h-3.5 w-3.5 mr-1" /> Edit
            </Button>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {localTiers.map((tier, idx) => (
            <div key={tier.tier} className="border border-border rounded-none p-4 space-y-3">
              <div className="flex items-center gap-2">
                <Badge variant={tierVariant(tier.tier)}>Tier {tier.tier}</Badge>
                {tier.description && <span className="text-sm text-muted-foreground">{tier.description}</span>}
              </div>
              {editing ? (
                <div className="grid grid-cols-3 gap-3">
                  <div className="space-y-1">
                    <Label className="text-xs">Min Score</Label>
                    <Input type="number" value={tier.min_score} onChange={e => updateTier(idx, 'min_score', e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Max Score</Label>
                    <Input type="number" value={tier.max_score} onChange={e => updateTier(idx, 'max_score', e.target.value)} />
                  </div>
                  <div className="space-y-1">
                    <Label className="text-xs">Multiplier</Label>
                    <Input type="number" step="0.1" value={tier.salary_multiplier} onChange={e => updateTier(idx, 'salary_multiplier', e.target.value)} />
                  </div>
                </div>
              ) : (
                <div className="flex gap-6 text-sm">
                  <span>Score: {tier.min_score} - {tier.max_score}</span>
                  <span>Multiplier: {tier.salary_multiplier}x</span>
                </div>
              )}
            </div>
          ))}
        </div>
        {editing && (
          <div className="flex gap-2 mt-4 justify-end">
            <Button variant="outline" onClick={() => { setEditing(false); setLocalTiers(tiers); }}>Cancel</Button>
            <Button onClick={handleSave} disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
