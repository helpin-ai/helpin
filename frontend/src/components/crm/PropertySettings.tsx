import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Plus, Pencil, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { useCRMProperties, useCreateCRMProperty, useUpdateCRMProperty, useDeleteCRMProperty } from '@/hooks/queries/useCRM';
import type { CRMObjectType, CRMFieldType } from '@/lib/crmTypes';

const FIELD_TYPES: { value: CRMFieldType; label: string }[] = [
  { value: 'text', label: 'Text' },
  { value: 'number', label: 'Number' },
  { value: 'date', label: 'Date' },
  { value: 'select', label: 'Select' },
  { value: 'multiselect', label: 'Multi-select' },
  { value: 'boolean', label: 'Boolean' },
  { value: 'url', label: 'URL' },
  { value: 'email', label: 'Email' },
  { value: 'phone', label: 'Phone' },
  { value: 'currency', label: 'Currency' },
];

interface PropertySettingsProps {
  workspaceId: string;
  objectType: CRMObjectType;
}

export function PropertySettings({ workspaceId, objectType }: PropertySettingsProps) {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  const { data: properties = [] } = useCRMProperties(workspaceId, objectType);
  const createProperty = useCreateCRMProperty(workspaceId);
  const updateProperty = useUpdateCRMProperty(workspaceId);
  const deleteProperty = useDeleteCRMProperty(workspaceId);

  const [formData, setFormData] = useState({
    internal_name: '',
    label: '',
    field_type: 'text' as CRMFieldType,
    is_required: false,
    options_text: '',
  });

  const resetForm = () => {
    setFormData({ internal_name: '', label: '', field_type: 'text', is_required: false, options_text: '' });
    setEditingId(null);
  };

  const handleCreate = async () => {
    try {
      const options: Record<string, unknown> = {};
      if (['select', 'multiselect'].includes(formData.field_type) && formData.options_text) {
        options.choices = formData.options_text.split(',').map((s) => s.trim()).filter(Boolean);
      }

      await createProperty.mutateAsync({
        workspace_id: workspaceId,
        object_type: objectType,
        internal_name: formData.internal_name,
        label: formData.label,
        field_type: formData.field_type,
        is_required: formData.is_required,
        options,
      });
      toast.success('Property created');
      setIsDialogOpen(false);
      resetForm();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create property');
    }
  };

  const handleUpdate = async () => {
    if (!editingId) return;
    try {
      const options: Record<string, unknown> = {};
      if (['select', 'multiselect'].includes(formData.field_type) && formData.options_text) {
        options.choices = formData.options_text.split(',').map((s) => s.trim()).filter(Boolean);
      }

      await updateProperty.mutateAsync({
        id: editingId,
        label: formData.label,
        field_type: formData.field_type,
        is_required: formData.is_required,
        options,
      });
      toast.success('Property updated');
      setIsDialogOpen(false);
      resetForm();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to update property');
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteProperty.mutateAsync(id);
      toast.success('Property deleted');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete property');
    }
  };

  const startEdit = (prop: typeof properties[number]) => {
    const choices = (prop.options as Record<string, unknown>)?.choices;
    setFormData({
      internal_name: prop.internal_name,
      label: prop.label,
      field_type: prop.field_type,
      is_required: prop.is_required,
      options_text: Array.isArray(choices) ? choices.join(', ') : '',
    });
    setEditingId(prop.id);
    setIsDialogOpen(true);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base">Custom Properties</CardTitle>
        <Dialog open={isDialogOpen} onOpenChange={(open) => { setIsDialogOpen(open); if (!open) resetForm(); }}>
          <DialogTrigger asChild>
            <Button size="sm" variant="outline" onClick={() => { resetForm(); setIsDialogOpen(true); }}>
              <Plus className="h-4 w-4 mr-1" /> Add Property
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{editingId ? 'Edit Property' : 'Create Property'}</DialogTitle>
            </DialogHeader>
            <div className="space-y-4 pt-2">
              {!editingId && (
                <div className="space-y-1.5">
                  <Label>Internal Name</Label>
                  <Input
                    placeholder="e.g. lead_source"
                    value={formData.internal_name}
                    onChange={(e) => setFormData({ ...formData, internal_name: e.target.value })}
                  />
                  <p className="text-xs text-muted-foreground">Lowercase, alphanumeric with underscores</p>
                </div>
              )}
              <div className="space-y-1.5">
                <Label>Label</Label>
                <Input
                  placeholder="Display name"
                  value={formData.label}
                  onChange={(e) => setFormData({ ...formData, label: e.target.value })}
                />
              </div>
              <div className="space-y-1.5">
                <Label>Field Type</Label>
                <Select value={formData.field_type} onValueChange={(v) => setFormData({ ...formData, field_type: v as CRMFieldType })}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FIELD_TYPES.map((ft) => (
                      <SelectItem key={ft.value} value={ft.value}>{ft.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {['select', 'multiselect'].includes(formData.field_type) && (
                <div className="space-y-1.5">
                  <Label>Options (comma-separated)</Label>
                  <Input
                    placeholder="Option 1, Option 2, Option 3"
                    value={formData.options_text}
                    onChange={(e) => setFormData({ ...formData, options_text: e.target.value })}
                  />
                </div>
              )}
              <div className="flex items-center gap-2">
                <Switch
                  checked={formData.is_required}
                  onCheckedChange={(checked) => setFormData({ ...formData, is_required: checked })}
                />
                <Label>Required</Label>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <Button variant="outline" onClick={() => { setIsDialogOpen(false); resetForm(); }}>Cancel</Button>
                <Button onClick={editingId ? handleUpdate : handleCreate}>
                  {editingId ? 'Save' : 'Create'}
                </Button>
              </div>
            </div>
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent>
        {properties.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-4">No custom properties defined yet.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Label</TableHead>
                <TableHead>Internal Name</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Required</TableHead>
                <TableHead className="w-[80px]" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {properties.map((prop) => (
                <TableRow key={prop.id}>
                  <TableCell className="font-medium">{prop.label}</TableCell>
                  <TableCell>
                    <code className="text-xs bg-muted px-1 py-0.5 rounded">{prop.internal_name}</code>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className="text-xs">{prop.field_type}</Badge>
                  </TableCell>
                  <TableCell>{prop.is_required ? 'Yes' : 'No'}</TableCell>
                  <TableCell>
                    {!prop.is_system && (
                      <div className="flex gap-1">
                        <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => startEdit(prop)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive" onClick={() => handleDelete(prop.id)}>
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
