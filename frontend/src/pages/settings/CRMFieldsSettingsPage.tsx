import { CustomFieldsSettings } from '@/components/settings/fields/CustomFieldsSettings'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useCompanyFilterOptions } from '@/hooks/useCompanyFilterOptions'
import { useContactFilterOptions } from '@/hooks/useContactFilterOptions'
import { useState } from 'react'
import { SettingsPageFrame } from './SettingsPageFrame'

type CRMFieldObject = 'crm_contact' | 'crm_company'

function CRMFieldsSettings({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const [objectType, setObjectType] = useState<CRMFieldObject>('crm_contact')
  const contactOptions = useContactFilterOptions(workspaceId)
  const companyOptions = useCompanyFilterOptions(workspaceId)
  const discovered = objectType === 'crm_contact' ? contactOptions.data?.custom_properties : companyOptions.data?.custom_properties
  return (
    <div className="space-y-4">
      <Tabs value={objectType} onValueChange={(value) => setObjectType(value as CRMFieldObject)}>
        <TabsList variant="line" aria-label="Record type">
          <TabsTrigger value="crm_contact">Contacts</TabsTrigger>
          <TabsTrigger value="crm_company">Companies</TabsTrigger>
        </TabsList>
      </Tabs>
      <CustomFieldsSettings
        key={objectType}
        workspaceId={workspaceId}
        objectType={objectType}
        editable={editable}
        title={objectType === 'crm_contact' ? 'Contact fields' : 'Company fields'}
        description="Defined fields show first on records and in filters, with their label and type, and can be edited on the record."
        recordNoun={objectType === 'crm_contact' ? ['contact', 'contacts'] : ['company', 'companies']}
        discovered={discovered ?? []}
      />
    </div>
  )
}

export function CRMFieldsSettingsPage() {
  return (
    <SettingsPageFrame section="crm-fields">
      {({ workspaceId, permissions }) => <CRMFieldsSettings workspaceId={workspaceId} editable={permissions.has('crm.admin')} />}
    </SettingsPageFrame>
  )
}
