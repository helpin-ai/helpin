import { useState } from 'react';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { ListManager } from '@/components/crm/ListManager';
import { CRMImportWizard } from '@/components/crm/CRMImportWizard';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useTitle } from '@/hooks/useTitle';

export function ListsPage() {
  useTitle('Lists & Import');
  const { currentWorkspace } = useWorkspaceStore();
  const wsId = currentWorkspace?.id ?? '';
  const [activeTab, setActiveTab] = useState('lists');

  return (
    <div className="flex h-full flex-col px-4 md:px-6">
      <div className="mb-4">
        <h1 className="text-xl font-semibold">Lists & Import</h1>
      </div>
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="lists">Lists</TabsTrigger>
          <TabsTrigger value="import">Import</TabsTrigger>
        </TabsList>
        <TabsContent value="lists" className="mt-4">
          <ListManager workspaceId={wsId} />
        </TabsContent>
        <TabsContent value="import" className="mt-4">
          <CRMImportWizard workspaceId={wsId} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
