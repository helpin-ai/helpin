import { useEffect, useState, type FormEvent } from 'react';
import { useTitle } from '@/hooks/useTitle';
import { useOrganizationStore } from '@/stores/organizationStore';
import { organizationsService } from '@/lib/services/organizationsService';
import type { MemberWithUser } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { cn } from '@/lib/utils';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { Building2, Trash2, Users } from 'lucide-react';
import { toast } from 'sonner';

const ROLE_COLORS: Record<string, string> = {
  owner: 'bg-amber-100 text-amber-800',
  admin: 'bg-blue-100 text-blue-800',
  member: 'bg-gray-100 text-gray-700',
};

export default function AccountSettings() {
  useTitle('Account Settings');
  const { currentOrganization, setCurrentOrganization } = useOrganizationStore();
  const [name, setName] = useState('');
  const [saving, setSaving] = useState(false);
  const [members, setMembers] = useState<MemberWithUser[]>([]);
  const [loadingMembers, setLoadingMembers] = useState(true);

  const orgId = currentOrganization?.id;
  const isAdminOrOwner = currentOrganization?.role === 'owner' || currentOrganization?.role === 'admin';

  useEffect(() => {
    if (currentOrganization) {
      setName(currentOrganization.name);
    }
  }, [currentOrganization]);

  useEffect(() => {
    if (!orgId) return;
    setLoadingMembers(true);
    organizationsService.listMembers(orgId).then(({ data }) => {
      setMembers(data ?? []);
      setLoadingMembers(false);
    });
  }, [orgId]);

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    if (!orgId) return;
    setSaving(true);
    const { data, error } = await organizationsService.update(orgId, { name });
    setSaving(false);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Organization updated');
      if (data && currentOrganization) {
        setCurrentOrganization({ ...currentOrganization, name: data.name, slug: data.slug });
      }
    }
  };

  const handleRoleChange = async (userId: string, newRole: string) => {
    if (!orgId) return;
    const { error } = await organizationsService.updateMember(orgId, userId, { role: newRole });
    if (error) {
      toast.error(error);
    } else {
      toast.success('Role updated');
      setMembers((prev) => prev.map((m) => (m.user_id === userId ? { ...m, role: newRole } : m)));
    }
  };

  const handleRemoveMember = async (userId: string, memberName: string) => {
    if (!orgId) return;
    if (!confirm(`Remove ${memberName} from this organization?`)) return;
    const { error } = await organizationsService.removeMember(orgId, userId);
    if (error) {
      toast.error(error);
    } else {
      toast.success('Member removed');
      setMembers((prev) => prev.filter((m) => m.user_id !== userId));
    }
  };

  if (!currentOrganization) {
    return (
      <div className="flex items-center justify-center py-16">
        <p className="text-muted-foreground">No organization selected</p>
      </div>
    );
  }

  return (
    <div className="space-y-6 max-w-3xl">
      <div>
        <h2 className="text-xl font-semibold">Account Settings</h2>
      </div>

      {/* Organization Name */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Building2 className="h-4 w-4" />
            Organization
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSave} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="org-name">Name</Label>
              <Input
                id="org-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={!isAdminOrOwner}
                required
              />
            </div>
            {isAdminOrOwner && (
              <Button type="submit" disabled={saving || name === currentOrganization.name}>
                {saving ? 'Saving...' : 'Save Changes'}
              </Button>
            )}
          </form>
        </CardContent>
      </Card>

      {/* Members */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Users className="h-4 w-4" />
            Members
          </CardTitle>
        </CardHeader>
        <CardContent>
          {loadingMembers ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Member</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  {isAdminOrOwner && <TableHead className="w-10" />}
                </TableRow>
              </TableHeader>
              <TableBody>
                {members.map((member) => (
                  <TableRow key={member.id}>
                    <TableCell>
                      <div className="flex items-center gap-2">
                        <UserAvatar name={member.full_name} className="h-7 w-7" />
                        <span className="text-sm font-medium">{member.full_name}</span>
                      </div>
                    </TableCell>
                    <TableCell className="text-sm text-muted-foreground">{member.email}</TableCell>
                    <TableCell>
                      {isAdminOrOwner && member.role !== 'owner' ? (
                        <Select
                          value={member.role}
                          onValueChange={(val) => handleRoleChange(member.user_id, val)}
                        >
                          <SelectTrigger className="h-7 w-24 text-xs">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="admin">Admin</SelectItem>
                            <SelectItem value="member">Member</SelectItem>
                          </SelectContent>
                        </Select>
                      ) : (
                        <Badge variant="secondary" className={cn('text-xs', ROLE_COLORS[member.role])}>
                          {member.role}
                        </Badge>
                      )}
                    </TableCell>
                    {isAdminOrOwner && (
                      <TableCell>
                        {member.role !== 'owner' && (
                          <Button
                            variant="ghost"
                            size="sm"
                            className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                            onClick={() => handleRemoveMember(member.user_id, member.full_name)}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        )}
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
