import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { toast } from 'sonner';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCreateContact } from '@/hooks/queries';
import type { CRMContact, LifecycleStage, LeadStatus } from '@/lib/crmTypes';
import { entityCreatedToastIcons, showEntityCreatedToast } from '@/components/ui/entity-created-toast';

interface CreateContactDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialValues?: {
    first_name?: string;
    last_name?: string;
    email?: string;
    phone?: string;
    job_title?: string;
    lifecycle_stage?: LifecycleStage;
    lead_status?: LeadStatus;
    source?: string;
  };
  showCreatedToast?: boolean;
  onCreated?: (contact: CRMContact) => void | Promise<void>;
}

const lifecycleOptions: { value: LifecycleStage; label: string }[] = [
  { value: 'subscriber', label: 'Subscriber' },
  { value: 'lead', label: 'Lead' },
  { value: 'marketing_qualified', label: 'Marketing Qualified' },
  { value: 'sales_qualified', label: 'Sales Qualified' },
  { value: 'opportunity', label: 'Opportunity' },
  { value: 'customer', label: 'Customer' },
  { value: 'evangelist', label: 'Evangelist' },
];

const leadStatusOptions: { value: LeadStatus; label: string }[] = [
  { value: 'new', label: 'New' },
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'unqualified', label: 'Unqualified' },
];

const sourceOptions = ['web', 'support', 'referral', 'social', 'event', 'cold_outreach', 'other'];

export function CreateContactDialog({
  open,
  onOpenChange,
  initialValues,
  showCreatedToast = true,
  onCreated,
}: CreateContactDialogProps) {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const createContact = useCreateContact(wsId);

  const [firstName, setFirstName] = useState(initialValues?.first_name ?? '');
  const [lastName, setLastName] = useState(initialValues?.last_name ?? '');
  const [email, setEmail] = useState(initialValues?.email ?? '');
  const [phone, setPhone] = useState(initialValues?.phone ?? '');
  const [jobTitle, setJobTitle] = useState(initialValues?.job_title ?? '');
  const [lifecycleStage, setLifecycleStage] = useState<LifecycleStage>(initialValues?.lifecycle_stage ?? 'subscriber');
  const [leadStatus, setLeadStatus] = useState<LeadStatus>(initialValues?.lead_status ?? 'new');
  const [source, setSource] = useState(initialValues?.source ?? '');

  const resetForm = () => {
    setFirstName(initialValues?.first_name ?? '');
    setLastName(initialValues?.last_name ?? '');
    setEmail(initialValues?.email ?? '');
    setPhone(initialValues?.phone ?? '');
    setJobTitle(initialValues?.job_title ?? '');
    setLifecycleStage(initialValues?.lifecycle_stage ?? 'subscriber');
    setLeadStatus(initialValues?.lead_status ?? 'new');
    setSource(initialValues?.source ?? '');
  };

  useEffect(() => {
    if (open) resetForm();
  }, [
    open,
    initialValues?.first_name,
    initialValues?.last_name,
    initialValues?.email,
    initialValues?.phone,
    initialValues?.job_title,
    initialValues?.lifecycle_stage,
    initialValues?.lead_status,
    initialValues?.source,
  ]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!firstName.trim() || !email.trim()) return;

    try {
      const contact = await createContact.mutateAsync({
        workspace_id: wsId,
        first_name: firstName.trim(),
        last_name: lastName.trim() || undefined,
        email: email.trim(),
        phone: phone.trim() || undefined,
        job_title: jobTitle.trim() || undefined,
        lifecycle_stage: lifecycleStage,
        lead_status: leadStatus,
        source: source || undefined,
      });
      await onCreated?.(contact);
      if (showCreatedToast) {
        showEntityCreatedToast({
          entityLabel: 'Contact',
          title: [contact.first_name, contact.last_name].filter(Boolean).join(' '),
          tone: 'crm',
          icon: entityCreatedToastIcons.contact,
          onOpen: currentWorkspace?.slug
            ? () => navigate({
                to: '/w/$slug/crm/contacts/$contactId',
                params: { slug: currentWorkspace.slug, contactId: contact.id },
              })
            : undefined,
        });
      }
      onOpenChange(false);
      resetForm();
    } catch {
      toast.error(onCreated ? 'Failed to create or link contact' : 'Failed to create contact');
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Create Contact</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="firstName">First Name *</Label>
              <Input id="firstName" value={firstName} onChange={(e) => setFirstName(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="lastName">Last Name</Label>
              <Input id="lastName" value={lastName} onChange={(e) => setLastName(e.target.value)} />
            </div>
          </div>
          <div className="space-y-2">
            <Label htmlFor="email">Email *</Label>
            <Input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label htmlFor="phone">Phone</Label>
              <Input id="phone" type="tel" value={phone} onChange={(e) => setPhone(e.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="jobTitle">Job Title</Label>
              <Input id="jobTitle" value={jobTitle} onChange={(e) => setJobTitle(e.target.value)} />
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>Lifecycle Stage</Label>
              <Select value={lifecycleStage} onValueChange={(v) => setLifecycleStage(v as LifecycleStage)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {lifecycleOptions.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>Lead Status</Label>
              <Select value={leadStatus} onValueChange={(v) => setLeadStatus(v as LeadStatus)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {leadStatusOptions.map((opt) => (
                    <SelectItem key={opt.value} value={opt.value}>{opt.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>
          <div className="space-y-2">
            <Label>Source</Label>
            <Select value={source} onValueChange={setSource}>
              <SelectTrigger><SelectValue placeholder="Select source" /></SelectTrigger>
              <SelectContent>
                {sourceOptions.map((s) => (
                  <SelectItem key={s} value={s}>{s.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={createContact.isPending || !firstName.trim() || !email.trim()}>
              {createContact.isPending ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
