import { useState } from 'react';
import { settingsService } from '@/lib/services/settingsService';
import type { JobRoleCriteria } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function JobRolesTab({ workspaceId, criteria, editable, onRefresh }: {
  workspaceId: string;
  criteria: JobRoleCriteria[];
  editable: boolean;
  onRefresh: () => void;
}) {
  // Group criteria by job_role
  const grouped = criteria.reduce<Record<string, JobRoleCriteria[]>>((acc, c) => {
    if (!acc[c.job_role]) acc[c.job_role] = [];
    acc[c.job_role].push(c);
    return acc;
  }, {});
  const jobRoles = Object.keys(grouped).sort();
  const [deleteRoleConfirm, setDeleteRoleConfirm] = useState<string | null>(null);

  const handleDeleteRole = async (jobRole: string) => {
    const { error } = await settingsService.deleteJobRole(workspaceId, jobRole);
    if (error) toast.error(error);
    else { toast.success(`"${jobRole}" criteria deleted`); onRefresh(); }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <CardTitle className="text-base">Job Role Criteria</CardTitle>
      </CardHeader>
      <CardContent>
        {jobRoles.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No job role criteria configured.</p>
        ) : (
          <div className="space-y-4">
            {jobRoles.map(role => (
              <div key={role} className="border border-border rounded-xl p-4">
                <div className="flex items-center justify-between mb-3">
                  <h3 className="font-medium">{role}</h3>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="text-xs">
                      {grouped[role].length} criteria
                    </Badge>
                    {editable && (
                      <Button size="icon" variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setDeleteRoleConfirm(role)}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    )}
                  </div>
                </div>
                <div className="space-y-1">
                  {grouped[role].map(c => (
                    <div key={c.criteria_id} className="flex items-center justify-between text-sm py-1">
                      <div>
                        <span className="font-medium">{c.name}</span>
                        <span className="text-muted-foreground ml-2">- {c.question}</span>
                      </div>
                      <Badge variant={c.enabled ? 'default' : 'secondary'} className="text-xs">
                        {c.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>

      <ConfirmDialog
        open={deleteRoleConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeleteRoleConfirm(null); }}
        title="Delete job role criteria"
        description={`This will permanently delete all criteria for "${deleteRoleConfirm}". This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deleteRoleConfirm) handleDeleteRole(deleteRoleConfirm); setDeleteRoleConfirm(null); }}
      />
    </Card>
  );
}
