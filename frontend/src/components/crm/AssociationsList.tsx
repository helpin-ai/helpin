import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Building2, DollarSign, Users, X, Link2, FileText } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useDeleteAssociation } from '@/hooks/queries/useCRM';
import type { CRMAssociation, CRMObjectType } from '@/lib/crmTypes';

interface AssociationsListProps {
  workspaceId: string;
  slug: string;
  associations: CRMAssociation[];
  currentObjectType: CRMObjectType;
  currentObjectId: string;
  onAssociationRemoved?: () => void;
}

const typeIcons: Partial<Record<CRMObjectType, typeof Users>> = {
  contact: Users,
  company: Building2,
  deal: DollarSign,
};

const typeColors: Partial<Record<CRMObjectType, string>> = {
  contact: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
  company: 'bg-purple-100 text-purple-700 dark:bg-purple-900 dark:text-purple-300',
  deal: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
};

export function AssociationsList({
  workspaceId,
  slug,
  associations,
  currentObjectType,
  currentObjectId,
  onAssociationRemoved,
}: AssociationsListProps) {
  const navigate = useNavigate();
  const deleteAssociation = useDeleteAssociation(workspaceId);
  const [removeId, setRemoveId] = useState<string | null>(null);

  const getAssociatedType = (assoc: CRMAssociation): CRMObjectType => {
    return assoc.from_object_type === currentObjectType && assoc.from_object_id === currentObjectId
      ? assoc.to_object_type
      : assoc.from_object_type;
  };

  const getAssociatedId = (assoc: CRMAssociation): string => {
    return assoc.from_object_type === currentObjectType && assoc.from_object_id === currentObjectId
      ? assoc.to_object_id
      : assoc.from_object_id;
  };

  const handleNavigate = (type: CRMObjectType, id: string) => {
    const routes: Record<CRMObjectType, { to: string; params: Record<string, string> }> = {
      contact: { to: '/w/$slug/crm/contacts/$contactId', params: { slug, contactId: id } },
      company: { to: '/w/$slug/crm/companies/$companyId', params: { slug, companyId: id } },
      deal: { to: '/w/$slug/crm/deals/$dealId', params: { slug, dealId: id } },
    };
    navigate(routes[type] as any);
  };

  const handleRemove = async () => {
    if (!removeId) return;
    try {
      await deleteAssociation.mutateAsync(removeId);
      toast.success('Association removed');
      setRemoveId(null);
      onAssociationRemoved?.();
    } catch {
      toast.error('Failed to remove association');
    }
  };

  if (associations.length === 0) {
    return (
      <div className="py-4 text-center">
        <Link2 className="mx-auto h-8 w-8 text-muted-foreground/40" />
        <p className="mt-2 text-sm text-muted-foreground">No associations</p>
      </div>
    );
  }

  return (
    <div className="space-y-1.5">
      {associations.map((assoc) => {
        const type = getAssociatedType(assoc);
        const id = getAssociatedId(assoc);
        const Icon = typeIcons[type] ?? FileText;
        return (
          <div
            key={assoc.id}
            className="group flex items-center gap-2 rounded-md border px-2.5 py-1.5 transition-colors hover:bg-accent/50"
          >
            <button
              type="button"
              className="flex min-w-0 flex-1 cursor-pointer items-center gap-2 text-left"
              onClick={() => handleNavigate(type, id)}
            >
              <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
              <Badge variant="outline" className={`px-1.5 py-0 text-[10px] ${typeColors[type] ?? 'bg-gray-100 text-gray-700 dark:bg-gray-900 dark:text-gray-300'}`}>
                {type}
              </Badge>
              {assoc.association_label && (
                <span className="truncate text-xs text-muted-foreground">{assoc.association_label}</span>
              )}
            </button>
            <Button
              variant="ghost"
              size="icon"
              className="h-5 w-5 shrink-0 opacity-0 transition-opacity group-hover:opacity-100"
              onClick={(e) => {
                e.stopPropagation();
                setRemoveId(assoc.id);
              }}
            >
              <X className="h-3 w-3" />
            </Button>
          </div>
        );
      })}

      <ConfirmDialog
        open={!!removeId}
        onOpenChange={(open) => !open && setRemoveId(null)}
        title="Remove association"
        description="Are you sure you want to remove this association?"
        confirmLabel="Remove"
        variant="destructive"
        onConfirm={handleRemove}
      />
    </div>
  );
}
