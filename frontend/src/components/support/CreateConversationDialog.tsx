import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { useCreateConversation } from '@/hooks/queries/useSupport';
import type { ConversationPriority } from '@/lib/pmTypes';

interface CreateConversationDialogProps {
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CreateConversationDialog({ workspaceId, open, onOpenChange }: CreateConversationDialogProps) {
  const [subject, setSubject] = useState('');
  const [priority, setPriority] = useState<ConversationPriority>('medium');
  const [customerName, setCustomerName] = useState('');
  const [customerEmail, setCustomerEmail] = useState('');

  const createMutation = useCreateConversation(workspaceId);

  const handleCreate = async () => {
    if (!subject.trim()) return;
    await createMutation.mutateAsync({
      subject: subject.trim(),
      priority,
      customer_name: customerName || undefined,
      customer_email: customerEmail || undefined,
    });
    setSubject('');
    setPriority('medium');
    setCustomerName('');
    setCustomerEmail('');
    onOpenChange(false);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create Conversation</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="conv-subject">Subject</Label>
            <Input
              id="conv-subject"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
              placeholder="Conversation subject"
            />
          </div>
          <div className="space-y-1.5">
            <Label>Priority</Label>
            <Select value={priority} onValueChange={(v) => setPriority(v as ConversationPriority)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="low">Low</SelectItem>
                <SelectItem value="medium">Medium</SelectItem>
                <SelectItem value="high">High</SelectItem>
                <SelectItem value="urgent">Urgent</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="conv-name">Customer Name</Label>
            <Input
              id="conv-name"
              value={customerName}
              onChange={(e) => setCustomerName(e.target.value)}
              placeholder="Customer name"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="conv-email">Customer Email</Label>
            <Input
              id="conv-email"
              type="email"
              value={customerEmail}
              onChange={(e) => setCustomerEmail(e.target.value)}
              placeholder="customer@example.com"
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" size="sm" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button
              size="sm"
              disabled={createMutation.isPending || !subject.trim()}
              onClick={handleCreate}
            >
              {createMutation.isPending ? 'Creating...' : 'Create'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
