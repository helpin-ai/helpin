import { TeamLabel } from '@/components/workspace/TeamLabel';
import { useState, type FormEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { ManagedWorkspaceModule, MemberWithUser, WorkspaceModuleGrant, WorkspaceTeam } from '@/lib/types';
import { workspacesService } from '@/lib/services/workspacesService';
import { settingsService } from '@/lib/services/settingsService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { MEMBER_MODULES } from './memberAccess';
import { workspaceRoleLabel } from './roleScopePresentation';

type Props = {
  workspaceId: string;
  member: MemberWithUser;
  teams: WorkspaceTeam[];
  initialTeamIds: string[];
  grants: WorkspaceModuleGrant[] | null;
  canEditRole: boolean;
  canEditTeams: boolean;
  isSelf?: boolean;
  canEditModules: boolean;
  roleOptions: Array<{ value: string; label: string }>;
  onClose: () => void;
  onApplied: (role: string) => void;
};

export function EditMemberDialog({ workspaceId, member, teams, initialTeamIds, grants, canEditRole, canEditTeams, isSelf = false, canEditModules, roleOptions, onClose, onApplied }: Props) {
  const queryClient = useQueryClient();
  const [role, setRole] = useState(member.role);
  const [savedRole, setSavedRole] = useState(member.role);
  const [teamIds, setTeamIds] = useState(initialTeamIds);
  const [savedTeamIds, setSavedTeamIds] = useState(initialTeamIds);
  const [moduleAccessAvailable] = useState(grants !== null);
  const [savedGrants, setSavedGrants] = useState(grants ?? []);
  const [directModules, setDirectModules] = useState(() => (grants ?? [])
    .filter(grant => grant.subject_type === 'workspace_member' && grant.subject_id === member.id)
    .map(grant => grant.module));
  const [teamSearch, setTeamSearch] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const workspaceWide = role === 'owner' || role === 'admin';
  const savedDirect = savedGrants.filter(grant => grant.subject_type === 'workspace_member' && grant.subject_id === member.id);
  const teamsChanged = teamIds.length !== savedTeamIds.length || teamIds.some(id => !savedTeamIds.includes(id));
  const modulesChanged = directModules.length !== savedDirect.length || directModules.some(module => !savedDirect.some(grant => grant.module === module));
  const changed = role !== savedRole || (canEditTeams && !workspaceWide && teamsChanged) || (canEditModules && !workspaceWide && modulesChanged);

  const save = async (event: FormEvent) => {
    event.preventDefault();
    if (saving || !changed) return;
    setSaving(true);
    setError('');
    let appliedRole = savedRole;
    try {
      if (canEditRole && role !== savedRole) {
        unwrap(await workspacesService.updateMemberRole(workspaceId, member.id, { role: role as 'owner' | 'admin' | 'member' | 'viewer' }));
        appliedRole = role;
        setSavedRole(role);
      }
      if (canEditTeams && !workspaceWide) {
        for (const teamId of teamIds.filter(id => !savedTeamIds.includes(id))) {
          unwrap(await settingsService.addTeamMember(workspaceId, teamId, { user_id: member.user_id, role: 'member' }));
          setSavedTeamIds(current => [...current, teamId]);
        }
        for (const teamId of savedTeamIds.filter(id => !teamIds.includes(id))) {
          unwrap(await settingsService.removeTeamMember(workspaceId, teamId, member.user_id));
          setSavedTeamIds(current => current.filter(id => id !== teamId));
        }
      }
      if (canEditModules && moduleAccessAvailable && !workspaceWide) {
        for (const module of directModules.filter(value => !savedDirect.some(grant => grant.module === value))) {
          const grant = unwrap(await settingsService.createModuleGrant(workspaceId, { module: module as ManagedWorkspaceModule, subject_type: 'workspace_member', subject_id: member.id }));
          setSavedGrants(current => [...current, grant]);
        }
        for (const grant of savedDirect.filter(value => !directModules.includes(value.module))) {
          unwrap(await settingsService.deleteModuleGrant(workspaceId, grant.id));
          setSavedGrants(current => current.filter(value => value.id !== grant.id));
        }
      }
      toast.success('Member updated');
      onClose();
    } catch (cause) {
      setError(`${cause instanceof Error ? cause.message : 'Could not save changes.'} Any changes already saved are retained. You can retry the remaining changes.`);
    } finally {
      setSaving(false);
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.moduleAccess(workspaceId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces.access(workspaceId) });
      onApplied(appliedRole);
    }
  };

  return (
    <>
      <Dialog open onOpenChange={open => { if (!open && !saving) onClose(); }}>
        <DialogContent className="flex max-h-[90dvh] flex-col overflow-hidden p-0 sm:max-w-xl" onInteractOutside={event => event.preventDefault()}>
          <form onSubmit={save} className="flex min-h-0 flex-1 flex-col">
            <DialogHeader className="shrink-0 px-6 pb-4 pt-6">
              <DialogTitle>Edit member</DialogTitle>
              <DialogDescription>Manage this member’s role and access to your workspace.</DialogDescription>
            </DialogHeader>
            <div className="min-h-0 flex-1 space-y-6 overflow-y-auto px-6 pb-6">
            <div className="flex items-center gap-3 rounded-xl bg-muted/40 p-3">
              <UserAvatar name={member.full_name || member.email} avatarUrl={member.avatar_url} avatarStyle={member.avatar_style} avatarSeed={member.avatar_seed} avatarBackgroundMode={member.avatar_background_mode} avatarBackgroundColor={member.avatar_background_color} className="h-10 w-10" />
              <div className="min-w-0">
                <p className="truncate text-sm font-medium">{member.full_name || member.email}</p>
                <p className="truncate text-xs text-muted-foreground">{member.email}</p>
              </div>
            </div>
            <section className="space-y-2">
              <Label htmlFor="member-role">Workspace role</Label>
              {canEditRole ? (
                <Select value={role} onValueChange={setRole} disabled={saving}>
                  <SelectTrigger id="member-role" className="w-full"><SelectValue /></SelectTrigger>
                  <SelectContent>{roleOptions.map(option => <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>)}</SelectContent>
                </Select>
              ) : <p className="text-sm">{workspaceRoleLabel(role)}</p>}
              <p className="text-xs text-muted-foreground">{workspaceWide ? 'Owners and admins have access to all teams and modules.' : 'The workspace role sets their level of permission. Module access is managed below.'}</p>
            </section>
            <section className="space-y-3" aria-labelledby="member-teams-label">
              <div className="flex items-center justify-between">
                <h3 id="member-teams-label" className="text-sm font-medium">Teams</h3>
                <span className="text-xs text-muted-foreground">{workspaceWide ? 'All teams' : `${teamIds.length} selected`}</span>
              </div>
              {workspaceWide ? <p className="text-sm text-muted-foreground">Team access is included with this role.</p> : (
                <div className="rounded-lg border">
                  {teams.length > 5 && <div className="border-b p-2"><Input aria-label="Search teams" placeholder="Search teams…" value={teamSearch} onChange={event => setTeamSearch(event.target.value)} className="h-8" /></div>}
                  <div className="max-h-40 overflow-y-auto p-1">
                    {teams.filter(team => team.name.toLowerCase().includes(teamSearch.trim().toLowerCase())).map(team => (
                      <label key={team.id} className="flex cursor-pointer items-center gap-3 rounded-md px-3 py-2 text-sm hover:bg-muted/50 has-disabled:cursor-default">
                        <Checkbox checked={teamIds.includes(team.id)} disabled={!canEditTeams || saving || (isSelf && savedTeamIds.includes(team.id))} onCheckedChange={checked => setTeamIds(current => checked ? [...current, team.id] : current.filter(id => id !== team.id))} />
                        <TeamLabel team={team} />
                      </label>
                    ))}
                    {!teams.some(team => team.name.toLowerCase().includes(teamSearch.trim().toLowerCase())) && <p className="p-3 text-sm text-muted-foreground">{teams.length ? 'No teams match your search.' : 'No teams in this workspace yet.'}</p>}
                  </div>
                </div>
              )}
              {!workspaceWide && isSelf && <p className="text-xs text-muted-foreground">Ask another admin to remove you from a team you already belong to.</p>}
              {!workspaceWide && !canEditTeams && <p className="text-xs text-muted-foreground">You don’t have permission to change team membership.</p>}
            </section>
            <section className="space-y-3" aria-labelledby="member-modules-label">
              <h3 id="member-modules-label" className="text-sm font-medium">Module access</h3>
              <div className="divide-y rounded-lg border">
                {MEMBER_MODULES.map(({ module, label }) => {
                  const universal = module === 'pm' || module === 'docs';
                  const inherited = teams.filter(team => teamIds.includes(team.id) && savedGrants.some(grant => grant.module === module && grant.subject_type === 'team' && grant.subject_id === team.id));
                  const included = universal || workspaceWide;
                  const checked = included || inherited.length > 0 || directModules.includes(module);
                  const description = universal ? 'Available to every member' : workspaceWide ? 'Included with workspace role' : inherited.length ? `Via ${inherited.map(team => team.name).join(', ')}` : directModules.includes(module) ? 'Granted directly' : 'No access';
                  return (
                    <label key={module} className="flex items-center gap-3 px-3 py-2.5">
                      <Checkbox checked={checked} disabled={included || inherited.length > 0 || !canEditModules || !moduleAccessAvailable || saving} onCheckedChange={value => setDirectModules(current => value ? [...current, module] : current.filter(item => item !== module))} />
                      <span className="min-w-0 flex-1"><span className="block text-sm font-medium">{label}</span><span className="block text-xs text-muted-foreground">{!moduleAccessAvailable && !included ? 'Access details unavailable' : description}</span></span>
                      {(included || inherited.length > 0) && <Badge variant="secondary" className="shrink-0 text-[10px] font-normal">{inherited.length && !included ? 'Team access' : 'Included'}</Badge>}
                    </label>
                  );
                })}
              </div>
              <p className="text-xs text-muted-foreground">{canEditModules ? 'Team access follows membership. Remove a team above to remove access inherited from it.' : 'You don’t have permission to change module access.'}</p>
            </section>
            </div>
            {error && <p role="alert" className="mx-6 mb-4 rounded-lg bg-destructive/10 p-3 text-sm text-destructive">{error}</p>}
            <DialogFooter className="shrink-0 border-t bg-muted/20 px-6 py-4">
              <Button type="button" variant="outline" onClick={onClose} disabled={saving}>Cancel</Button>
              <Button type="submit" disabled={!changed || saving}>{saving ? 'Saving…' : 'Save changes'}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
