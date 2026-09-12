import { useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { Dialog, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { QuietRelationshipDialogContent, QuietRelationshipResults, QuietSearchInput, QuietTextAction, quietRelationshipResultRowClassName } from '@/components/design-system/quiet';
import { Building03Icon, UserIcon } from '@/lib/icons';
import { useCreateAssociation, useDeleteAssociation, useSetDealCustomer } from '@/hooks/queries';
import { crmSearchService } from '@/lib/services/crmService';
import type { CRMAssociationEnriched, CRMObjectType, CRMSearchResult } from '@/lib/crmTypes';
import { cn } from '@/lib/utils';

type LinkedAssociation = CRMAssociationEnriched & { linkedType: CRMObjectType; linkedId: string };

function linkedAssociation(association: CRMAssociationEnriched, dealId: string): LinkedAssociation {
  const dealIsFrom = association.from_object_type === 'deal' && association.from_object_id === dealId;
  return {
    ...association,
    linkedType: dealIsFrom ? association.to_object_type : association.from_object_type,
    linkedId: dealIsFrom ? association.to_object_id : association.from_object_id,
  };
}

export function DealRelationships({
  workspaceId,
  dealId,
  associations,
  onChanged,
  onNavigate,
}: {
  workspaceId: string;
  dealId: string;
  associations: CRMAssociationEnriched[];
  onChanged: () => void;
  onNavigate: (type: 'contact' | 'company', id: string) => void;
}) {
  const setCustomer = useSetDealCustomer(workspaceId);
  const createAssociation = useCreateAssociation(workspaceId);
  const deleteAssociation = useDeleteAssociation(workspaceId);
  const [picker, setPicker] = useState<'customer' | 'participant' | null>(null);
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<CRMSearchResult[]>([]);
  const [searching, setSearching] = useState(false);

  const linked = useMemo(() => associations.map((association) => linkedAssociation(association, dealId)), [associations, dealId]);
  const customer = linked.find((association) => association.association_label === 'deal_customer');
  const people = linked.filter((association) => association.linkedType === 'contact');
  const legacyCompanies = linked.filter((association) => association.linkedType === 'company' && association.association_label !== 'deal_customer');

  useEffect(() => {
    if (!picker || query.trim().length < 2) {
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      setSearching(true);
      const response = await crmSearchService.search(workspaceId, query.trim());
      if (cancelled) return;
      setResults((response.data ?? []).filter((result) => picker === 'participant' ? result.type === 'contact' : result.type === 'contact' || result.type === 'company'));
      setSearching(false);
    }, 220);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [picker, query, workspaceId]);

  const closePicker = () => {
    setPicker(null);
    setQuery('');
    setResults([]);
  };

  const choose = async (result: CRMSearchResult) => {
    try {
      if (picker === 'customer' && (result.type === 'contact' || result.type === 'company')) {
        await setCustomer.mutateAsync({
          id: dealId,
          workspace_id: workspaceId,
          ...(result.type === 'company' ? { company_id: result.id } : { contact_id: result.id }),
        });
        toast.success('Deal customer updated');
      } else if (picker === 'participant' && result.type === 'contact') {
        await createAssociation.mutateAsync({ workspace_id: workspaceId, from_object_type: 'deal', from_object_id: dealId, to_object_type: 'contact', to_object_id: result.id });
        toast.success('Participant added');
      }
      closePicker();
      onChanged();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not update deal relationships');
    }
  };

  const remove = async (association: LinkedAssociation) => {
    try {
      await deleteAssociation.mutateAsync(association.id);
      toast.success(association.association_label === 'deal_primary_contact' ? 'Primary contact removed' : 'Participant removed');
      onChanged();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Could not remove relationship');
    }
  };

  return (
    <>
		<section>
			<div className="mb-2 flex min-h-7 items-center gap-2">
				<h2 className="text-xs font-semibold uppercase tracking-wide text-foreground/70">Customer</h2>
				<QuietTextAction className="ml-auto text-xs" onClick={() => setPicker('customer')}>{customer ? 'Change' : 'Choose'}</QuietTextAction>
			</div>
        {customer ? (
			<button type="button" className="flex w-full items-center gap-2 rounded px-1.5 py-1.5 text-left hover:bg-accent" onClick={() => onNavigate(customer.linkedType as 'contact' | 'company', customer.linkedId)}>
				{customer.linkedType === 'company' ? <Building03Icon className="h-3.5 w-3.5 text-muted-foreground" /> : <UserIcon className="h-3.5 w-3.5 text-muted-foreground" />}
				<span className="min-w-0 flex-1 truncate text-sm font-medium text-foreground/90">{customer.linked_object_name || 'Customer'}</span>
				<span className="text-[10px] text-muted-foreground">{customer.linked_object_display_id}</span>
          </button>
        ) : (
			<div className="px-1.5 py-1.5">
				<p className="text-xs font-medium">Customer needed</p>
				<p className="mt-1 text-xs leading-5 text-muted-foreground">Choose the company or independent contact this opportunity belongs to.</p>
				<QuietTextAction className="mt-1.5 text-xs" onClick={() => setPicker('customer')}>Choose customer</QuietTextAction>
			</div>
        )}
		</section>

		<section className="-mx-4 mt-4 border-t border-border/60 px-4 pt-4">
			<div className="mb-2 flex min-h-7 items-center gap-2">
				<h2 className="text-xs font-semibold uppercase tracking-wide text-foreground/70">People <span className="font-normal text-muted-foreground">{people.length}</span></h2>
				<QuietTextAction className="ml-auto text-xs" onClick={() => setPicker('participant')}>Add</QuietTextAction>
			</div>
        {people.length ? people.map((person) => (
			<div key={person.id} className="group flex items-center gap-2 border-b border-border/40 px-1.5 py-1.5 last:border-b-0 hover:bg-accent">
            <button type="button" className="min-w-0 flex-1 text-left" onClick={() => onNavigate('contact', person.linkedId)}>
				<span className="block truncate text-sm font-medium text-foreground/90">{person.linked_object_name || 'Contact'}</span>
				<span className="text-[10px] text-muted-foreground">{person.association_label === 'deal_primary_contact' ? 'Primary contact' : person.association_label === 'deal_customer' ? 'Customer' : 'Participant'}</span>
            </button>
            {customer?.linkedType === 'company' && person.association_label !== 'deal_primary_contact' && <QuietTextAction type="button" className="text-xs" disabled={setCustomer.isPending} onClick={async () => {
              try { await setCustomer.mutateAsync({ id: dealId, workspace_id: workspaceId, company_id: customer.linkedId, contact_id: person.linkedId }); onChanged(); }
              catch (error) { toast.error(error instanceof Error ? error.message : 'Could not change primary contact'); }
            }}>Make primary</QuietTextAction>}
			{person.association_label !== 'deal_customer' ? <QuietTextAction className="text-xs opacity-0 group-hover:opacity-100 focus-visible:opacity-100" onClick={() => void remove(person)}>Remove</QuietTextAction> : null}
          </div>
		)) : <p className="px-1.5 py-1.5 text-xs leading-5 text-muted-foreground">No people linked. A company-backed deal does not require a contact.</p>}
		</section>

      {legacyCompanies.length ? (
		<section className="-mx-4 mt-4 border-t border-border/60 px-4 pt-4">
			<h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-foreground/70">Other companies <span className="font-normal text-muted-foreground">{legacyCompanies.length}</span></h2>
			<p className="mb-2 text-[10px] leading-4 text-muted-foreground">Legacy related companies are not deal customers.</p>
          {legacyCompanies.map((company) => (
			<div key={company.id} className="group flex items-center gap-2 border-b border-border/40 px-1.5 py-1.5 last:border-b-0 hover:bg-accent">
				<button type="button" onClick={() => onNavigate('company', company.linkedId)} className="min-w-0 flex-1 truncate text-left text-sm font-medium text-foreground/90">{company.linked_object_name || 'Company'}</button>
				<QuietTextAction className="text-xs opacity-0 group-hover:opacity-100 focus-visible:opacity-100" onClick={() => void remove(company)}>Remove</QuietTextAction>
            </div>
          ))}
		</section>
      ) : null}

      <Dialog open={!!picker} onOpenChange={(open) => { if (!open) closePicker(); }}>
        <QuietRelationshipDialogContent>
          <DialogHeader>
            <DialogTitle>{picker === 'customer' ? 'Choose deal customer' : 'Add participant'}</DialogTitle>
            <DialogDescription>{picker === 'customer' ? 'Changing the customer preserves other people as participants.' : 'Participants can contribute without changing the deal customer.'}</DialogDescription>
          </DialogHeader>
          <QuietSearchInput
            autoFocus
            value={query}
            onChange={(event) => {
              const value = event.target.value;
              setQuery(value);
              if (value.trim().length < 2) {
                setResults([]);
                setSearching(false);
              }
            }}
            placeholder={picker === 'customer' ? 'Search contacts or companies' : 'Search contacts'}
          />
          <QuietRelationshipResults className="max-h-72 overflow-y-auto border-b border-quiet-divider-strong">
            {searching ? <p className="py-4 text-sm text-quiet-muted">Searching…</p> : null}
            {!searching && query.trim().length >= 2 && results.length === 0 ? <p className="py-4 text-sm text-quiet-muted">No matching records</p> : null}
            {results.map((result) => (
              <button key={`${result.type}:${result.id}`} type="button" className={cn(quietRelationshipResultRowClassName, 'flex w-full items-center gap-2 border-t border-quiet-divider-light py-2.5 text-left hover:bg-quiet-row-hover')} onClick={() => void choose(result)}>
                {result.type === 'company' ? <Building03Icon className="h-4 w-4 shrink-0 text-quiet-muted" /> : <UserIcon className="h-4 w-4 shrink-0 text-quiet-muted" />}
                <span className="min-w-0 flex-1"><span className="block truncate text-sm font-medium text-quiet-text-primary">{result.name}</span><span className="block truncate text-[11.5px] text-quiet-muted">{result.detail}</span></span>
              </button>
            ))}
          </QuietRelationshipResults>
        </QuietRelationshipDialogContent>
      </Dialog>
    </>
  );
}
