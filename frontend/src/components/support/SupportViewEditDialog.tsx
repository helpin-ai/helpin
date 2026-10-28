import { useState } from 'react';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { SupportViewFilters } from './SupportViewFilters';
import { buildSupportInboxViewFilters, parseSupportInboxViewFilters } from '@/lib/supportInboxFilters';
import type { SupportInboxView, SupportInboxScope, SupportTag } from '@/lib/pmTypes';

export interface SupportViewEditValues {
  name: string;
  is_shared?: boolean;
  filters: Record<string, string>;
}

// Mount per edit session so every draft starts from the selected saved view.
export function SupportViewEditDialog({ view, canManageSettings, tags, mailboxes, slaEnabled, isPending, onSave, onClose }: {
  view: SupportInboxView;
  canManageSettings: boolean;
  tags: SupportTag[];
  mailboxes: Pick<SupportInboxScope, 'id' | 'name'>[];
  slaEnabled: boolean;
  isPending: boolean;
  onSave: (values: SupportViewEditValues) => void;
  onClose: () => void;
}) {
  const [name, setName] = useState(view.name);
  const [shared, setShared] = useState(view.is_shared);
  const [draft, setDraft] = useState(() => parseSupportInboxViewFilters(view.filters, 'inbox'));
  const save = () => {
    if (!name.trim() || isPending) return;
    // Replace editable keys, retaining other saved criteria and future fields.
    const filters = { ...view.filters };
    for (const key of ['nav_filter', 'states', 'mailbox_id', 'search', 'assignment', 'mailbox_ids', 'tag_ids', 'ai', 'system_tags', 'sort', 'sla', 'priority']) delete filters[key];
    Object.assign(filters, buildSupportInboxViewFilters(draft));
    onSave({ name: name.trim(), ...(canManageSettings ? { is_shared: shared } : {}), filters });
  };
  return (
    <Dialog open onOpenChange={(open) => { if (!open && !isPending) onClose(); }}>
      <DialogContent aria-describedby={undefined} className="sm:max-w-2xl max-h-[90dvh] overflow-hidden flex flex-col">
        <DialogHeader><DialogTitle>Edit view</DialogTitle></DialogHeader>
        <div className="space-y-4 overflow-y-auto min-h-0 pr-1">
          <div className="space-y-2">
            <Label htmlFor="support-custom-view-name">Name</Label>
            <Input id="support-custom-view-name" value={name} onChange={(event) => setName(event.target.value)} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); save(); } }} />
          </div>
          {canManageSettings && <div className="flex items-center justify-between rounded-md border px-3 py-2">
            <Label htmlFor="support-custom-view-shared" className="text-sm font-medium">Shared with workspace</Label>
            <Switch id="support-custom-view-shared" checked={shared} onCheckedChange={setShared} />
          </div>}
          <div className="space-y-4 border-t pt-4">
            <div className="text-sm font-medium">Filters</div>
            <SupportViewFilters
              conversationListFilters={draft.listFilters}
              navFilter={draft.navFilter}
              selectedMailboxId={draft.selectedMailboxId}
              setConversationListFilter={(key, value) => setDraft(current => ({ ...current, listFilters: { ...current.listFilters, [key]: value } }))}
              setConversationMailboxFilters={(ids) => setDraft(current => ({ ...current, selectedMailboxId: 'all', listFilters: { ...current.listFilters, mailboxIds: ids } }))}
              supportTags={tags} teamInboxFilterOptions={mailboxes} slaEnabled={slaEnabled}
            />
          </div>
        </div>
        <DialogFooter className="shrink-0 border-t pt-4">
          <Button variant="outline" disabled={isPending} onClick={onClose}>Cancel</Button>
          <Button disabled={!name.trim() || isPending} onClick={save}>{isPending ? 'Saving…' : 'Save'}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
