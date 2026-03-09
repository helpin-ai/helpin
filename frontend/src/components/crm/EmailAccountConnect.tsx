import { Mail, Trash2, Plus } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { useEmailAccounts, useCreateEmailAccount, useDeleteEmailAccount } from '@/hooks/queries/useCRM';
import type { CRMEmailAccount } from '@/lib/crmTypes';

interface EmailAccountConnectProps {
  workspaceId: string;
  memberId: string;
}

export function EmailAccountConnect({ workspaceId, memberId }: EmailAccountConnectProps) {
  const { data: accounts = [] } = useEmailAccounts(workspaceId);
  const createAccount = useCreateEmailAccount(workspaceId);
  const deleteAccount = useDeleteEmailAccount(workspaceId);

  const handleConnect = (provider: 'gmail' | 'microsoft') => {
    createAccount.mutate({
      workspace_id: workspaceId,
      member_id: memberId,
      provider,
      email_address: `${provider}@placeholder.com`,
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Email Accounts</CardTitle>
        <CardDescription>Connect email accounts to sync conversations with contacts.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {accounts.map((account: CRMEmailAccount) => (
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
            <Button
              variant="ghost"
              size="icon"
              onClick={() => deleteAccount.mutate(account.id)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        ))}
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={() => handleConnect('gmail')}>
            <Plus className="mr-1 h-3 w-3" /> Gmail
          </Button>
          <Button variant="outline" size="sm" onClick={() => handleConnect('microsoft')}>
            <Plus className="mr-1 h-3 w-3" /> Microsoft
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
