import type { GroupedAssociations } from '@/lib/pmTypes';

export function hasVisibleTaskAssociations(associations: GroupedAssociations | null | undefined) {
  if (!associations) return false;

  const relationships = associations.task_relationships;
  const hasRelationships = relationships
    ? Object.values(relationships).some((group) => group.length > 0)
    : false;

  return hasRelationships || associations.docs.length > 0;
}
