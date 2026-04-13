import { useMemo } from 'react';
import { QueryBuilderPopover } from '@/components/ui/query-builder/QueryBuilderPopover';
import { useContactsSearchParams } from '@/hooks/useContactsSearchParams';
import { buildCRMContactQueryFields } from '@/lib/crmContactQueryBuilder';
import type { AssignableMember } from '@/lib/types';

interface ContactsFilterBarProps {
  assignableMembers: AssignableMember[];
}

export function ContactsFilterBar({ assignableMembers }: ContactsFilterBarProps) {
  const { filterGroup, setFilterGroup } = useContactsSearchParams();
  const fields = useMemo(
    () => buildCRMContactQueryFields(assignableMembers),
    [assignableMembers],
  );

  return (
    <QueryBuilderPopover
      fields={fields}
      value={filterGroup}
      onApply={setFilterGroup}
      triggerLabel="Filter"
    />
  );
}
