import { Mail, Trash2, Plus } from 'lucide-react';
import { toast } from 'sonner';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { ConfirmDialog } from '@/components/pm/ConfirmDialog';
import { useEmailAccounts, useDeleteEmailAccount } from '@/hooks/queries/useCRM';
import type { CRMEmailAccount } from '@/lib/crmTypes';
import { useState } from 'react';

interface EmailAccountConnectProps {
  workspaceId: string;
  memberId: string;
}

export function EmailAccountConnect({ workspaceId }: EmailAccountConnectProps) {
  const { data: accounts = [] } = useEmailAccounts(workspaceId);
  const deleteAccount = useDeleteEmailAccount(workspaceId);
  const [deleteId, setDeleteId] = useState<string | null>(null);

  const handleConnect = (provider: 'gmail' | 'microsoft') => {
    toast.info(`${provider === 'gmail' ? 'Gmail' : 'Microsoft'} integration coming soon`);
  };

  const handleDelete = async () => {
    if (!deleteId) return;
    try {
      await deleteAccount.mutateAsync(deleteId);
      toast.success('Account removed');
      setDeleteId(null);
    } catch {
      toast.error('Failed to remove account');
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Email Accounts</CardTitle>
        <CardDescription>Connect email accounts to sync conversations with contacts.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {accounts.length === 0 && (
          <div className="py-4 text-center">
            <Mail className="mx-auto h-8 w-8 text-muted-foreground/40" />
            <p className="mt-2 text-sm text-muted-foreground">No email accounts connected</p>
            <p className="text-xs text-muted-foreground/70">Connect your email to sync conversations</p>
          </div>
        )}

        {(accounts as CRMEmailAccount[]).map((account) => (
          <div key={account.id} className="flex items-center justify-between rounded-md border p-3">
            <div className="flex items-center gap-3">
              <Mail className="h-4 w-4 text-muted-foreground" />
              <div>
                <p className="text-sm font-medium">{account.email_address}</p>
                <div className="flex items-center gap-2">
                  <Badge variant="outline" className="text-xs">{account.provider}</Badge>
                  <Badge variant={account.is_active ? 'default' : 'secondary'} className="text-xs">
                    {account.is_active ? 'Active' : 'Inactive'}
                  </Badge>
                </div>
              </div>
            </div>
            <Button variant="ghost" size="icon" onClick={() => setDeleteId(account.id)}>
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        ))}

        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => handleConnect('gmail')}>
            <Plus className="mr-1 h-3 w-3" /> Gmail
            <Badge variant="secondary" className="ml-1.5 text-[10px]">Soon</Badge>
          </Button>
          <Button variant="outline" size="sm" onClick={() => handleConnect('microsoft')}>
            <Plus className="mr-1 h-3 w-3" /> Microsoft
            <Badge variant="secondary" className="ml-1.5 text-[10px]">Soon</Badge>
          </Button>
        </div>

        <ConfirmDialog
          open={!!deleteId}
          onOpenChange={(open) => !open && setDeleteId(null)}
          title="Remove email account"
          description="Are you sure you want to disconnect this email account?"
          confirmLabel="Remove"
          variant="destructive"
          onConfirm={handleDelete}
        />
      </CardContent>
    </Card>
  );
}
