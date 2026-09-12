import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { QuietDropdownRoot, QuietDropdownTrigger, QuietDropdownContent, QuietDropdownOptions, QuietDropdownGroup, QuietDropdownItem } from '@/components/design-system/quiet-dropdown';
import { QuietTextAction, QuietUnderlineInput } from '@/components/design-system/quiet';
import { useCreateCompany, useCreateContact, useCreateAssociation } from '@/hooks/queries';
import { crmCompanyService, crmContactService } from '@/lib/services/crmService';
import { unwrapRequired } from '@/lib/queryUtils';

export interface DealEntity { id: string; name: string; detail?: string }

export function DealEntityPicker({ workspaceId, kind, company, excludedIds = [], onSelect, onDraftChange, disabled = false }: {
  workspaceId: string; kind: 'company' | 'contact'; company?: DealEntity | null;
  onDraftChange?: (active: boolean) => void; excludedIds?: string[]; onSelect: (entity: DealEntity) => void; disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [draft, setDraft] = useState<string | null>(null);
  const [detail, setDetail] = useState('');
  const [draftCompany, setDraftCompany] = useState<DealEntity | null>(null);
  const [created, setCreated] = useState<DealEntity | null>(null);
  const [saving, setSaving] = useState(false);
  const createCompany = useCreateCompany(workspaceId);
  const createContact = useCreateContact(workspaceId);
  const createAssociation = useCreateAssociation(workspaceId);
  const search = useQuery({
    queryKey: ['deal-entity-picker', workspaceId, kind, query.trim(), company?.id],
    enabled: open && !!workspaceId,
    queryFn: async () => {
      if (kind === 'company') {
        const result = unwrapRequired(await crmCompanyService.list(workspaceId, { search: query.trim(), per_page: 20 }), 'Company search');
        return { suggested: [] as DealEntity[], results: result.data.map(c => ({ id: c.id, name: c.name, detail: c.domain })) };
      }
      const [all, suggested] = await Promise.all([
        crmContactService.list(workspaceId, { search: query.trim(), per_page: 20 }).then(r => unwrapRequired(r, 'Contact search')),
        company ? crmCompanyService.listContacts(workspaceId, company.id, { search: query.trim(), per_page: 20 }).then(r => unwrapRequired(r, 'Company contacts')) : Promise.resolve(null),
      ]);
      const convert = (c: { id: string; first_name: string; last_name?: string; email?: string }) => ({ id: c.id, name: [c.first_name, c.last_name].filter(Boolean).join(' ') || c.email || 'Unnamed contact', detail: c.email });
      const suggestions = suggested?.data.map(convert) ?? [];
      return { suggested: suggestions, results: all.data.filter(c => !suggestions.some(s => s.id === c.id)).map(convert) };
    },
  });
  const select = (entity: DealEntity) => { onSelect(entity); setOpen(false); setQuery(''); };
  const startCreate = () => { onDraftChange?.(true); setDraft(query.trim()); setDraftCompany(company ?? null); setDetail(''); setCreated(null); setOpen(false); };
  const save = async () => {
    if (!draft?.trim() || saving) return;
    setSaving(true);
    try {
      let entity = created;
      if (!entity) {
        if (kind === 'company') {
          const result = await createCompany.mutateAsync({ workspace_id: workspaceId, name: draft.trim(), domain: detail.trim() || undefined });
          entity = { id: result.id, name: result.name };
        } else {
          const [first, ...last] = draft.trim().split(/\s+/);
          const result = await createContact.mutateAsync({ workspace_id: workspaceId, first_name: first, last_name: last.join(' ') || undefined, email: detail.trim() || undefined });
          entity = { id: result.id, name: [result.first_name, result.last_name].filter(Boolean).join(' ') };
        }
        // Retain the created record if association saving fails, so retry never creates a duplicate.
        setCreated(entity);
      }
      if (kind === 'contact' && draftCompany) {
        await createAssociation.mutateAsync({ workspace_id: workspaceId, from_object_type: 'contact', from_object_id: entity.id, to_object_type: 'company', to_object_id: draftCompany.id, association_label: 'primary' });
      }
      select(entity); setDraft(null); onDraftChange?.(false);
    } catch (error) { toast.error(error instanceof Error ? error.message : `Could not add ${kind}`); }
    finally { setSaving(false); }
  };
  if (draft !== null) return <div className="space-y-2 rounded-md border border-quiet-divider-strong p-3">
    <QuietUnderlineInput aria-label={`New ${kind} name`} value={draft} disabled={saving || !!created} onChange={e => setDraft(e.target.value)} />
    <QuietUnderlineInput aria-label={kind === 'contact' ? 'Contact email' : 'Company domain'} placeholder={kind === 'contact' ? 'Email (optional)' : 'Domain (optional)'} value={detail} disabled={saving || !!created} onChange={e => setDetail(e.target.value)} />
    {kind === 'contact' && <div className="flex items-center gap-2 text-sm"><span className="text-quiet-muted">Company</span>{draftCompany ? <><span className="flex-1 truncate">{draftCompany.name}</span><QuietTextAction type="button" disabled={saving} onClick={() => setDraftCompany(null)}>Change</QuietTextAction></> : <DealEntityPicker workspaceId={workspaceId} kind="company" onSelect={setDraftCompany} disabled={saving} />}</div>}
    <div className="flex justify-end gap-3"><QuietTextAction type="button" disabled={saving} onClick={() => { if (created) select(created); setDraft(null); onDraftChange?.(false); }}>{created ? 'Keep contact without company' : 'Cancel'}</QuietTextAction><QuietTextAction type="button" disabled={saving || !draft.trim()} onClick={() => void save()}>{saving ? 'Saving…' : created ? 'Retry linking' : `Add ${kind}`}</QuietTextAction></div>
  </div>;
  return <QuietDropdownRoot open={open} onOpenChange={setOpen}>
    <QuietDropdownTrigger asChild><button type="button" disabled={disabled} className="w-full border-b border-quiet-field py-2 text-left text-sm text-quiet-muted hover:text-quiet-text-primary">{kind === 'company' ? 'Select company' : 'Add contact'}</button></QuietDropdownTrigger>
    <QuietDropdownContent className="w-80">
      <QuietDropdownOptions searchMode="always" searchLabel={`Search ${kind === 'company' ? 'companies' : 'contacts'}`} searchPlaceholder={`Search ${kind === 'company' ? 'companies' : 'contacts'}…`} query={query} onQueryChange={setQuery} shouldFilter={false}>
        {search.isFetching ? <div className="p-3 text-sm text-quiet-muted" role="status">Searching…</div> : search.isError ? <div className="p-3 text-sm" role="alert">Could not load results. <button type="button" onClick={() => void search.refetch()}>Retry</button></div> : <>
          {!!search.data?.suggested.length && <QuietDropdownGroup heading={`At ${company?.name}`}>{search.data.suggested.filter(e => !excludedIds.includes(e.id)).map(e => <QuietDropdownItem key={e.id} value={e.id} onSelect={() => select(e)}><span className="min-w-0"><span className="block truncate">{e.name}</span>{e.detail && <span className="block truncate text-xs text-quiet-muted">{e.detail}</span>}</span></QuietDropdownItem>)}</QuietDropdownGroup>}
          <QuietDropdownGroup>{search.data?.results.filter(e => !excludedIds.includes(e.id)).map(e => <QuietDropdownItem key={e.id} value={e.id} onSelect={() => select(e)}><span className="min-w-0"><span className="block truncate">{e.name}</span>{e.detail && <span className="block truncate text-xs text-quiet-muted">{e.detail}</span>}</span></QuietDropdownItem>)}</QuietDropdownGroup>
          {query.trim() && ![...(search.data?.suggested ?? []), ...(search.data?.results ?? [])].some(e => e.name.toLocaleLowerCase() === query.trim().toLocaleLowerCase()) && <QuietDropdownGroup><QuietDropdownItem value="__create__" onSelect={startCreate}>Add “{query.trim()}” as new {kind}</QuietDropdownItem></QuietDropdownGroup>}
        </>}
      </QuietDropdownOptions>
    </QuietDropdownContent>
  </QuietDropdownRoot>;
}
