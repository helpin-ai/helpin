import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Badge } from '@/components/ui/badge';
import { Plus, Trash2, Filter, List, Users } from 'lucide-react';
import { toast } from 'sonner';
import {
  useCRMLists,
  useCreateCRMList,
  useUpdateCRMList,
  useDeleteCRMList,
  useCRMListMembers,
  useAddCRMListMember,
  useRemoveCRMListMember,
} from '@/hooks/queries/useCRM';
import type { CRMObjectType, CRMListType, CRMList } from '@/lib/crmTypes';

interface ListManagerProps {
  workspaceId: string;
}

export function ListManager({ workspaceId }: ListManagerProps) {
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [selectedList, setSelectedList] = useState<CRMList | null>(null);
  const [objectTypeFilter, setObjectTypeFilter] = useState<string>('all');
  const [searchFilter, setSearchFilter] = useState('');

  const { data: listsData } = useCRMLists(workspaceId, {
    object_type: objectTypeFilter === 'all' ? undefined : objectTypeFilter || undefined,
    search: searchFilter || undefined,
  });

  const lists = listsData?.data ?? [];

  return (
    <div className="space-y-6">
      {/* Filters + Create */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-2 flex-1">
          <Input
            placeholder="Search lists..."
            value={searchFilter}
            onChange={(e) => setSearchFilter(e.target.value)}
            className="max-w-xs"
          />
          <Select value={objectTypeFilter} onValueChange={setObjectTypeFilter}>
            <SelectTrigger className="w-[150px]">
              <SelectValue placeholder="All types" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All types</SelectItem>
              <SelectItem value="contact">Contacts</SelectItem>
              <SelectItem value="company">Companies</SelectItem>
              <SelectItem value="deal">Deals</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Dialog open={isCreateOpen} onOpenChange={setIsCreateOpen}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="h-4 w-4 mr-1" /> Create List
            </Button>
          </DialogTrigger>
          <DialogContent>
            <CreateListDialog workspaceId={workspaceId} onClose={() => setIsCreateOpen(false)} />
          </DialogContent>
        </Dialog>
      </div>

      {/* Lists Table */}
      <Card>
        <CardContent className="p-0">
          {lists.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              <List className="h-8 w-8 mx-auto mb-2 opacity-50" />
              <p>No lists yet. Create one to organize your CRM records.</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead>Object</TableHead>
                  <TableHead>Members</TableHead>
                  <TableHead className="w-[80px]" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {lists.map((list) => (
                  <ListRow
                    key={list.id}
                    list={list}
                    workspaceId={workspaceId}
                    onSelect={() => setSelectedList(list)}
                  />
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* Selected list member management */}
      {selectedList && (
        <ListMemberPanel
          workspaceId={workspaceId}
          list={selectedList}
          onClose={() => setSelectedList(null)}
        />
      )}
    </div>
  );
}

function ListRow({
  list,
  workspaceId,
  onSelect,
}: {
  list: CRMList;
  workspaceId: string;
  onSelect: () => void;
}) {
  const deleteList = useDeleteCRMList(workspaceId);

  const handleDelete = async (e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await deleteList.mutateAsync(list.id);
      toast.success('List deleted');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to delete list');
    }
  };

  return (
    <TableRow className="cursor-pointer" onClick={onSelect}>
      <TableCell className="font-medium">{list.name}</TableCell>
      <TableCell>
        <Badge variant={list.list_type === 'smart' ? 'default' : 'outline'} className="text-xs">
          {list.list_type === 'smart' ? (
            <><Filter className="h-3 w-3 mr-1" /> Smart</>
          ) : (
            <><Users className="h-3 w-3 mr-1" /> Static</>
          )}
        </Badge>
      </TableCell>
      <TableCell className="capitalize">{list.object_type}</TableCell>
      <TableCell>{list.member_count}</TableCell>
      <TableCell>
        <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive" onClick={handleDelete}>
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </TableCell>
    </TableRow>
  );
}

function CreateListDialog({
  workspaceId,
  onClose,
}: {
  workspaceId: string;
  onClose: () => void;
}) {
  const [name, setName] = useState('');
  const [listType, setListType] = useState<CRMListType>('static');
  const [objectType, setObjectType] = useState<CRMObjectType>('contact');
  const [filterField, setFilterField] = useState('');
  const [filterValue, setFilterValue] = useState('');

  const createList = useCreateCRMList(workspaceId);

  const handleSubmit = async () => {
    try {
      const filterCriteria: Record<string, unknown> = {};
      if (listType === 'smart' && filterField && filterValue) {
        filterCriteria[filterField] = filterValue;
      }

      await createList.mutateAsync({
        workspace_id: workspaceId,
        name,
        list_type: listType,
        object_type: objectType,
        filter_criteria: filterCriteria,
      });
      toast.success('List created');
      onClose();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create list');
    }
  };

  return (
    <>
      <DialogHeader>
        <DialogTitle>Create List</DialogTitle>
      </DialogHeader>
      <div className="space-y-4 pt-2">
        <div className="space-y-1.5">
          <Label>Name</Label>
          <Input placeholder="My list" value={name} onChange={(e) => setName(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label>List Type</Label>
          <Select value={listType} onValueChange={(v) => setListType(v as CRMListType)}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="static">Static - manually add members</SelectItem>
              <SelectItem value="smart">Smart - auto-populate by filters</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-1.5">
          <Label>Object Type</Label>
          <Select value={objectType} onValueChange={(v) => setObjectType(v as CRMObjectType)}>
            <SelectTrigger><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="contact">Contacts</SelectItem>
              <SelectItem value="company">Companies</SelectItem>
              <SelectItem value="deal">Deals</SelectItem>
            </SelectContent>
          </Select>
        </div>
        {listType === 'smart' && (
          <Card className="bg-muted/50">
            <CardHeader className="pb-2">
              <CardTitle className="text-sm">Filter Criteria</CardTitle>
              <CardDescription className="text-xs">Objects matching these criteria are auto-included</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <div className="flex gap-2">
                <Input
                  placeholder="Field name (e.g. lifecycle_stage)"
                  value={filterField}
                  onChange={(e) => setFilterField(e.target.value)}
                  className="flex-1"
                />
                <Input
                  placeholder="Value"
                  value={filterValue}
                  onChange={(e) => setFilterValue(e.target.value)}
                  className="flex-1"
                />
              </div>
            </CardContent>
          </Card>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={!name.trim()}>Create</Button>
        </div>
      </div>
    </>
  );
}

function ListMemberPanel({
  workspaceId,
  list,
  onClose,
}: {
  workspaceId: string;
  list: CRMList;
  onClose: () => void;
}) {
  const [addObjectId, setAddObjectId] = useState('');
  const { data: membersData } = useCRMListMembers(workspaceId, list.id);
  const addMember = useAddCRMListMember(workspaceId, list.id);
  const removeMember = useRemoveCRMListMember(workspaceId, list.id);

  const members = membersData?.data ?? [];

  const handleAdd = async () => {
    if (!addObjectId.trim()) return;
    try {
      await addMember.mutateAsync(addObjectId.trim());
      setAddObjectId('');
      toast.success('Member added');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to add member');
    }
  };

  const handleRemove = async (objectId: string) => {
    try {
      await removeMember.mutateAsync(objectId);
      toast.success('Member removed');
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to remove member');
    }
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div>
          <CardTitle className="text-base">{list.name} - Members</CardTitle>
          <CardDescription>{members.length} member(s)</CardDescription>
        </div>
        <Button variant="outline" size="sm" onClick={onClose}>Close</Button>
      </CardHeader>
      <CardContent>
        {list.list_type === 'static' && (
          <div className="flex gap-2 mb-4">
            <Input
              placeholder="Enter object ID to add..."
              value={addObjectId}
              onChange={(e) => setAddObjectId(e.target.value)}
              className="flex-1"
            />
            <Button onClick={handleAdd} disabled={!addObjectId.trim()}>Add</Button>
          </div>
        )}
        {list.list_type === 'smart' && (
          <p className="text-sm text-muted-foreground mb-4">
            Smart list members are calculated automatically from filter criteria.
          </p>
        )}
        {members.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-4">No members in this list.</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Object ID</TableHead>
                <TableHead>Added</TableHead>
                {list.list_type === 'static' && <TableHead className="w-[60px]" />}
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((member) => (
                <TableRow key={member.id}>
                  <TableCell className="font-mono text-xs">{member.object_id}</TableCell>
                  <TableCell className="text-sm text-muted-foreground">
                    {new Date(member.created_at).toLocaleDateString()}
                  </TableCell>
                  {list.list_type === 'static' && (
                    <TableCell>
                      <Button
                        size="icon"
                        variant="ghost"
                        className="h-7 w-7 text-destructive"
                        onClick={() => handleRemove(member.object_id)}
                      >
                        <Trash2 className="h-3.5 w-3.5" />
                      </Button>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
