import { memo, useCallback, useMemo, useState } from 'react'
import { toast } from 'sonner'
import {
  ArrowDown01Icon,
  Briefcase01Icon,
  HeadphonesIcon,
  InformationCircleIcon,
  LockIcon,
  PlusSignIcon,
  Shield01Icon,
  Tick01Icon,
  UserGroupIcon,
} from '@/lib/icons'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import { CompactChip } from '@/components/ui/compact-chip'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Skeleton } from '@/components/ui/skeleton'
import {
  useCreateModuleGrant,
  useDeleteModuleGrant,
  useWorkspaceModuleAccess,
} from '@/hooks/queries/useSettings'
import type {
  WorkspaceModuleGrant,
  WorkspacePerson,
  WorkspaceTeam,
} from '@/lib/types'
import { cn } from '@/lib/utils'

type ModuleKey = 'crm' | 'support'

const MODULE_META: Record<
  ModuleKey,
  {
    title: string
    description: string
    icon: React.FC<{ className?: string }>
    color: string
    bgColor: string
  }
> = {
  crm: {
    title: 'CRM',
    description:
      'Control which teams and members can access the CRM module. Owners and admins always retain full access.',
    icon: Briefcase01Icon,
    color: 'text-indigo-600 dark:text-indigo-400',
    bgColor: 'bg-indigo-50 dark:bg-indigo-950/40',
  },
  support: {
    title: 'Support',
    description:
      'Control which teams and members can access Support. Inbox and mailbox rules still apply after access is granted.',
    icon: HeadphonesIcon,
    color: 'text-emerald-600 dark:text-emerald-400',
    bgColor: 'bg-emerald-50 dark:bg-emerald-950/40',
  },
}

const TEAM_TYPE_LABELS: Record<string, string> = {
  engineering: 'Engineering',
  product: 'Product',
  design: 'Design',
  support: 'Support',
  marketing: 'Marketing',
  sales: 'Sales',
  hr: 'HR',
  operations: 'Operations',
  custom: 'Custom',
}

const MODULES: ModuleKey[] = ['crm', 'support']

type ModuleAccessTabProps = {
  workspaceId: string
  teams: WorkspaceTeam[]
  people: WorkspacePerson[]
  editable: boolean
}

export function ModuleAccessTab({
  workspaceId,
  teams,
  people,
  editable,
}: ModuleAccessTabProps) {
  const { data, isLoading } = useWorkspaceModuleAccess(workspaceId)

  const activePeople = useMemo(
    () => people.filter((person) => person.status === 'active'),
    [people],
  )

  const grants = useMemo(() => data?.grants ?? [], [data?.grants])

  if (!editable) {
    return (
      <Card className="border-dashed">
        <CardHeader className="text-center py-12">
          <div className="mx-auto mb-3 flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <LockIcon className="h-6 w-6 text-muted-foreground" />
          </div>
          <CardTitle className="text-base">
            Access management is restricted
          </CardTitle>
          <CardDescription className="max-w-sm mx-auto">
            You need the module access management permission to edit CRM and
            Support grants. Contact a workspace owner or admin for access.
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-16 w-full rounded-lg" />
        <Skeleton className="h-64 w-full rounded-lg" />
        <Skeleton className="h-64 w-full rounded-lg" />
      </div>
    )
  }

  return (
    <div className="space-y-5">
      {/* Info banner */}
      <div className="flex items-start gap-2.5 rounded-lg border border-border/50 bg-muted/25 px-3.5 py-2.5">
        <InformationCircleIcon className="mt-px h-4 w-4 shrink-0 text-muted-foreground/70" />
        <p className="text-[13px] text-muted-foreground leading-relaxed">
          Owners and admins always retain full access to all modules. For
          everyone else, grant access by team first, then add individual
          exceptions as needed.
        </p>
      </div>

      {MODULES.map((module) => (
        <ModuleCard
          key={module}
          module={module}
          workspaceId={workspaceId}
          teams={teams}
          activePeople={activePeople}
          grants={grants}
        />
      ))}
    </div>
  )
}

/* ------------------------------------------------------------------ */
/* Module card — isolated state per module                             */
/* ------------------------------------------------------------------ */

type ModuleCardProps = {
  module: ModuleKey
  workspaceId: string
  teams: WorkspaceTeam[]
  activePeople: WorkspacePerson[]
  grants: WorkspaceModuleGrant[]
}

const ModuleCard = memo(function ModuleCard({
  module,
  workspaceId,
  teams,
  activePeople,
  grants,
}: ModuleCardProps) {
  const createGrant = useCreateModuleGrant(workspaceId)
  const deleteGrant = useDeleteModuleGrant(workspaceId)
  const [selectedTeamIds, setSelectedTeamIds] = useState<string[]>([])
  const [selectedMemberIds, setSelectedMemberIds] = useState<string[]>([])

  const meta = MODULE_META[module]
  const Icon = meta.icon

  // Derived data — memoized
  const moduleGrants = useMemo(
    () => grants.filter((g) => g.module === module),
    [grants, module],
  )
  const teamGrants = useMemo(
    () => moduleGrants.filter((g) => g.subject_type === 'team'),
    [moduleGrants],
  )
  const memberGrants = useMemo(
    () => moduleGrants.filter((g) => g.subject_type === 'workspace_member'),
    [moduleGrants],
  )

  const teamMap = useMemo(() => {
    const map = new Map<string, WorkspaceTeam>()
    for (const team of teams) map.set(team.id, team)
    return map
  }, [teams])

  const personMap = useMemo(() => {
    const map = new Map<string, WorkspacePerson>()
    for (const person of activePeople) map.set(person.id, person)
    return map
  }, [activePeople])

  const availableTeamItems = useMemo(() => {
    const grantedIds = new Set(teamGrants.map((g) => g.subject_id))
    return teams
      .filter((t) => !grantedIds.has(t.id))
      .map((t) => ({
        id: t.id,
        label: t.name,
        detail: t.team_type
          ? (TEAM_TYPE_LABELS[t.team_type] ?? t.team_type)
          : undefined,
      }))
  }, [teams, teamGrants])

  const availableMemberItems = useMemo(() => {
    const grantedIds = new Set(memberGrants.map((g) => g.subject_id))
    return activePeople
      .filter((p) => !grantedIds.has(p.id))
      .map((p) => ({ id: p.id, label: p.name, detail: p.email }))
  }, [activePeople, memberGrants])

  // Stable callbacks
  const handleDeleteGrant = useCallback(
    async (grantId: string) => {
      try {
        await deleteGrant.mutateAsync(grantId)
        toast.success('Access revoked')
      } catch (error) {
        toast.error(
          error instanceof Error
            ? error.message
            : 'Failed to remove module grant',
        )
      }
    },
    [deleteGrant],
  )

  const handleAddTeams = useCallback(async () => {
    if (selectedTeamIds.length === 0) return
    try {
      await Promise.all(
        selectedTeamIds.map((id) =>
          createGrant.mutateAsync({
            module,
            subject_type: 'team',
            subject_id: id,
          }),
        ),
      )
      toast.success(
        selectedTeamIds.length === 1
          ? `${meta.title} team access granted`
          : `${selectedTeamIds.length} teams granted ${meta.title} access`,
      )
      setSelectedTeamIds([])
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : 'Failed to update module access',
      )
    }
  }, [selectedTeamIds, createGrant, module, meta.title])

  const handleAddMembers = useCallback(async () => {
    if (selectedMemberIds.length === 0) return
    try {
      await Promise.all(
        selectedMemberIds.map((id) =>
          createGrant.mutateAsync({
            module,
            subject_type: 'workspace_member',
            subject_id: id,
          }),
        ),
      )
      toast.success(
        selectedMemberIds.length === 1
          ? `${meta.title} member access granted`
          : `${selectedMemberIds.length} members granted ${meta.title} access`,
      )
      setSelectedMemberIds([])
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : 'Failed to update module access',
      )
    }
  }, [selectedMemberIds, createGrant, module, meta.title])

  const totalGrants = teamGrants.length + memberGrants.length

  return (
    <Card>
      <CardHeader>
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-start gap-3">
            <div
              className={cn(
                'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
                meta.bgColor,
              )}
            >
              <Icon className={cn('h-4 w-4', meta.color)} />
            </div>
            <div className="space-y-0.5">
              <CardTitle className="text-sm">{meta.title}</CardTitle>
              <CardDescription className="text-[13px] leading-snug">
                {meta.description}
              </CardDescription>
            </div>
          </div>
          {totalGrants > 0 && (
            <Badge
              variant="secondary"
              className="shrink-0 tabular-nums text-[11px]"
            >
              {totalGrants} {totalGrants === 1 ? 'grant' : 'grants'}
            </Badge>
          )}
        </div>
      </CardHeader>

      <CardContent className="pt-0">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          {/* Team access column */}
          <div className="space-y-2.5 rounded-lg border border-border/50 bg-muted/15 p-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1.5">
                <UserGroupIcon className="h-3.5 w-3.5 text-muted-foreground" />
                <h3 className="text-[13px] font-medium">Teams</h3>
              </div>
              {teamGrants.length > 0 && (
                <span className="text-[11px] text-muted-foreground tabular-nums">
                  {teamGrants.length}
                </span>
              )}
            </div>

            {teamGrants.length > 0 ? (
              <div className="flex flex-wrap gap-1.5">
                {teamGrants.map((grant) => {
                  const team = teamMap.get(grant.subject_id)
                  return (
                    <CompactChip
                      key={grant.id}
                      title={team?.name ?? 'Unknown team'}
                      displayId={
                        team?.team_type
                          ? (TEAM_TYPE_LABELS[team.team_type] ??
                              team.team_type)
                          : undefined
                      }
                      onRemove={() => handleDeleteGrant(grant.id)}
                    />
                  )
                })}
              </div>
            ) : (
              <EmptyGrants
                icon={UserGroupIcon}
                title="No team access"
                description="Grant a team access to give all its members entry to this module."
              />
            )}

            {availableTeamItems.length > 0 ? (
              <MultiSelectAdd
                items={availableTeamItems}
                selectedIds={selectedTeamIds}
                onSelectedChange={setSelectedTeamIds}
                onAdd={handleAddTeams}
                isPending={createGrant.isPending}
                placeholder="Search teams..."
                triggerLabel="Select teams"
                emptyLabel="No teams available"
              />
            ) : teamGrants.length > 0 ? (
              <AllGrantedNote label="All teams have been added" />
            ) : null}
          </div>

          {/* Direct member access column */}
          <div className="space-y-2.5 rounded-lg border border-border/50 bg-muted/15 p-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-1.5">
                <Shield01Icon className="h-3.5 w-3.5 text-muted-foreground" />
                <h3 className="text-[13px] font-medium">Members</h3>
              </div>
              {memberGrants.length > 0 && (
                <span className="text-[11px] text-muted-foreground tabular-nums">
                  {memberGrants.length}
                </span>
              )}
            </div>

            {memberGrants.length > 0 ? (
              <div className="flex flex-wrap gap-1.5">
                {memberGrants.map((grant) => {
                  const person = personMap.get(grant.subject_id)
                  return (
                    <CompactChip
                      key={grant.id}
                      title={person?.name ?? 'Unknown member'}
                      displayId={person?.email}
                      onRemove={() => handleDeleteGrant(grant.id)}
                    />
                  )
                })}
              </div>
            ) : (
              <EmptyGrants
                icon={Shield01Icon}
                title="No direct access"
                description="Add individual members for one-off exceptions like founders or cross-functional roles."
              />
            )}

            {availableMemberItems.length > 0 ? (
              <MultiSelectAdd
                items={availableMemberItems}
                selectedIds={selectedMemberIds}
                onSelectedChange={setSelectedMemberIds}
                onAdd={handleAddMembers}
                isPending={createGrant.isPending}
                placeholder="Search members..."
                triggerLabel="Select members"
                emptyLabel="No members available"
              />
            ) : memberGrants.length > 0 ? (
              <AllGrantedNote label="All members have been added" />
            ) : null}
          </div>
        </div>
      </CardContent>
    </Card>
  )
})

/* ------------------------------------------------------------------ */
/* Shared sub-components                                               */
/* ------------------------------------------------------------------ */

type SelectableItem = {
  id: string
  label: string
  detail?: string
}

const MultiSelectAdd = memo(function MultiSelectAdd({
  items,
  selectedIds,
  onSelectedChange,
  onAdd,
  isPending,
  placeholder,
  triggerLabel,
  emptyLabel,
}: {
  items: SelectableItem[]
  selectedIds: string[]
  onSelectedChange: (ids: string[]) => void
  onAdd: () => void
  isPending: boolean
  placeholder: string
  triggerLabel: string
  emptyLabel: string
}) {
  const [open, setOpen] = useState(false)
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds])

  const handleToggle = useCallback(
    (id: string) => {
      onSelectedChange(
        selectedSet.has(id)
          ? selectedIds.filter((v) => v !== id)
          : [...selectedIds, id],
      )
    },
    [selectedIds, selectedSet, onSelectedChange],
  )

  const handleAdd = useCallback(() => {
    onAdd()
    setOpen(false)
  }, [onAdd])

  return (
    <div className="flex items-center gap-1.5 pt-0.5">
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="inline-flex h-7 max-w-[240px] items-center gap-1 rounded-md border border-input bg-background px-2.5 text-xs text-muted-foreground shadow-xs hover:bg-accent hover:text-accent-foreground transition-colors"
          >
            <span className="truncate">
              {selectedIds.length > 0
                ? `${selectedIds.length} selected`
                : triggerLabel}
            </span>
            <ArrowDown01Icon className="ml-auto h-3 w-3 shrink-0 opacity-50" />
          </button>
        </PopoverTrigger>
        <PopoverContent className="w-[240px] p-0" align="start">
          <Command>
            <CommandInput
              placeholder={placeholder}
              className="h-7 text-xs"
            />
            <CommandList>
              <CommandEmpty className="py-2.5 text-center text-xs text-muted-foreground">
                {emptyLabel}
              </CommandEmpty>
              <CommandGroup>
                {items.map((item) => {
                  const isSelected = selectedSet.has(item.id)
                  return (
                    <CommandItem
                      key={item.id}
                      value={`${item.label} ${item.detail ?? ''}`}
                      onSelect={() => handleToggle(item.id)}
                      className="flex items-center gap-2 text-xs py-1"
                      data-checked={isSelected}
                    >
                      <Checkbox
                        checked={isSelected}
                        className="pointer-events-none size-3.5"
                        tabIndex={-1}
                      />
                      <span className="min-w-0 flex-1 truncate">
                        {item.label}
                      </span>
                      {item.detail && (
                        <span className="shrink-0 text-[10px] text-muted-foreground">
                          {item.detail}
                        </span>
                      )}
                    </CommandItem>
                  )
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="h-7 gap-1 px-2.5 text-xs"
        disabled={selectedIds.length === 0 || isPending}
        onClick={handleAdd}
      >
        <PlusSignIcon className="h-3 w-3" />
        Add{selectedIds.length > 1 ? ` (${selectedIds.length})` : ''}
      </Button>
    </div>
  )
})

function EmptyGrants({
  icon: Icon,
  title,
  description,
}: {
  icon: React.FC<{ className?: string }>
  title: string
  description: string
}) {
  return (
    <div className="flex flex-col items-center gap-1.5 rounded-md border border-dashed py-5 px-3 text-center">
      <div className="flex h-7 w-7 items-center justify-center rounded-full bg-muted">
        <Icon className="h-3.5 w-3.5 text-muted-foreground/60" />
      </div>
      <p className="text-xs font-medium text-muted-foreground">{title}</p>
      <p className="text-[11px] text-muted-foreground/70 leading-snug max-w-[200px]">
        {description}
      </p>
    </div>
  )
}

function AllGrantedNote({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-1.5 pt-0.5">
      <Tick01Icon className="h-3 w-3 text-emerald-500" />
      <p className="text-[11px] text-muted-foreground">{label}</p>
    </div>
  )
}
