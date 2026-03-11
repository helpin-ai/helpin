import { useState, type FormEvent } from 'react';
import { settingsService } from '@/lib/services/settingsService';
import type { WorkspacePerson } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { Pencil, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { LINEAR_CARD_CLASS } from './settingsConstants';

export function PeopleTab({ workspaceId, people, editable, onRefresh }: {
  workspaceId: string;
  people: WorkspacePerson[];
  editable: boolean;
  onRefresh: () => void;
}) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editPerson, setEditPerson] = useState<WorkspacePerson | null>(null);
  const [role, setRole] = useState<WorkspacePerson['role']>('employee');
  const [jobRole, setJobRole] = useState('');
  const [salary, setSalary] = useState('0');
  const [saving, setSaving] = useState(false);
  const [deletePersonConfirm, setDeletePersonConfirm] = useState<string | null>(null);

  const openEdit = (p: WorkspacePerson) => {
    setEditPerson(p);
    setRole(p.role); setJobRole(p.job_role); setSalary(String(p.base_salary));
    setDialogOpen(true);
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!editPerson) return;
    setSaving(true);
    const { error } = await settingsService.updatePerson(workspaceId, editPerson.id, {
      role, job_role: jobRole, base_salary: Number(salary),
    });
    if (error) toast.error(error);
    else { toast.success('Person updated'); setDialogOpen(false); onRefresh(); }
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    const { error } = await settingsService.deletePerson(workspaceId, id);
    if (error) toast.error(error);
    else { toast.success('Person removed'); onRefresh(); }
  };

  return (
    <Card className={LINEAR_CARD_CLASS}>
      <CardHeader>
        <div className="flex items-center gap-2">
          <CardTitle className="text-base">People</CardTitle>
          <Badge variant="outline" className="text-xs font-normal">{people.length}</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {people.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-6">No people yet. Invite members from the Members tab to get started.</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Job Role</TableHead>
                  <TableHead>Status</TableHead>
                  {editable && <TableHead className="w-24">Actions</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {people.map(p => (
                  <TableRow key={p.id}>
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell className="text-muted-foreground">{p.email}</TableCell>
                    <TableCell><Badge variant="outline" className="text-xs">{p.role}</Badge></TableCell>
                    <TableCell>{p.job_role || '—'}</TableCell>
                    <TableCell>
                      <Badge variant={p.status === 'active' ? 'default' : 'secondary'} className="text-xs">
                        {p.status}
                      </Badge>
                    </TableCell>
                    {editable && (
                      <TableCell>
                        <div className="flex gap-1">
                          <Button size="icon" variant="ghost" onClick={() => openEdit(p)}>
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                          <Button size="icon" variant="ghost" className="text-destructive hover:text-destructive" onClick={() => setDeletePersonConfirm(p.id)}>
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        </div>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent>
          <form onSubmit={handleSubmit}>
            <DialogHeader>
              <DialogTitle>Edit Person</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label>Name</Label>
                <Input value={editPerson?.name ?? ''} disabled className="bg-muted" />
              </div>
              <div className="space-y-2">
                <Label>Email</Label>
                <Input value={editPerson?.email ?? ''} disabled className="bg-muted" />
              </div>
              <div className="space-y-2">
                <Label>Role</Label>
                <Select value={role} onValueChange={v => setRole(v as WorkspacePerson['role'])}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="executive">Executive</SelectItem>
                    <SelectItem value="manager">Manager</SelectItem>
                    <SelectItem value="employee">Employee</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>Job Role</Label>
                <Input placeholder="e.g. Frontend Developer" value={jobRole} onChange={e => setJobRole(e.target.value)} />
              </div>
              <div className="space-y-2">
                <Label>Base Salary</Label>
                <Input type="number" value={salary} onChange={e => setSalary(e.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
              <Button type="submit" disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        open={deletePersonConfirm !== null}
        onOpenChange={(open) => { if (!open) setDeletePersonConfirm(null); }}
        title="Delete person"
        description="This will permanently remove this person from the workspace. This action cannot be undone."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => { if (deletePersonConfirm) handleDelete(deletePersonConfirm); setDeletePersonConfirm(null); }}
      />
    </Card>
  );
}
