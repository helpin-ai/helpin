import { Cancel01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { useContactsSearchParams } from '@/hooks/useContactsSearchParams';
import type { AssignableMember } from '@/lib/types';

const LIFECYCLE_STAGES = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'Marketing Qualified' },
  { value: 'sales_qualified', label: 'Sales Qualified' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
];

const LEAD_STATUSES = [
  { value: 'new', label: 'New' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'unqualified', label: 'Unqualified' },
];

interface ContactsFilterBarProps {
  assignableMembers: AssignableMember[];
}

export function ContactsFilterBar({ assignableMembers }: ContactsFilterBarProps) {
  const { search: params, setParam, clearFilters, hasActiveFilters } =
    useContactsSearchParams();

  const activeFilterCount = [params.stage, params.status, params.owner].filter(Boolean).length;

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {/* Lifecycle Stage */}
      <Select
        size="sm"
        value={params.stage ?? '__all__'}
        onValueChange={(v) => setParam('stage', v === '__all__' ? undefined : v)}
      >
        <SelectTrigger className="h-7 w-[150px] text-xs">
          <SelectValue placeholder="Stage" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All stages</SelectItem>
          {LIFECYCLE_STAGES.map((s) => (
            <SelectItem key={s.value} value={s.value}>
              {s.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Lead Status */}
      <Select
        size="sm"
        value={params.status ?? '__all__'}
        onValueChange={(v) => setParam('status', v === '__all__' ? undefined : v)}
      >
        <SelectTrigger className="h-7 w-[130px] text-xs">
          <SelectValue placeholder="Status" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All statuses</SelectItem>
          {LEAD_STATUSES.map((s) => (
            <SelectItem key={s.value} value={s.value}>
              {s.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Owner */}
      <Select
        size="sm"
        value={params.owner ?? '__all__'}
        onValueChange={(v) => setParam('owner', v === '__all__' ? undefined : v)}
      >
        <SelectTrigger className="h-7 w-[150px] text-xs">
          <SelectValue placeholder="Owner" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="__all__">All owners</SelectItem>
          {assignableMembers.map((m) => (
            <SelectItem key={m.id} value={m.id}>
              <div className="flex items-center gap-1.5">
                <UserAvatar
                  name={m.display_name || m.email}
                  avatarUrl={m.avatar_url}
                  avatarStyle={m.avatar_style}
                  avatarSeed={m.avatar_seed}
                  avatarBackgroundMode={m.avatar_background_mode}
                  avatarBackgroundColor={m.avatar_background_color}
                  className="h-4 w-4 shrink-0"
                />
                <span className="truncate">{m.display_name || m.email}</span>
              </div>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {/* Active filter count + clear */}
      {hasActiveFilters && (
        <div className="flex items-center gap-1">
          {activeFilterCount > 0 && (
            <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
              {activeFilterCount}
            </Badge>
          )}
          <Button
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs text-muted-foreground"
            onClick={clearFilters}
          >
            <Cancel01Icon className="mr-1 h-3 w-3" />
            Clear
          </Button>
        </div>
      )}
    </div>
  );
}
