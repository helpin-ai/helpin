import type { CRMCompany, CRMContact, CRMDeal } from '../crmTypes';
import type { CRMRecordTargetType } from '../agentCRMTargets';
import { unwrapRequired } from '../queryUtils';
import { crmCompanyService, crmContactService, crmDealService } from './crmService';

export interface CRMAgentRecordOption {
  id: string;
  name: string;
  detail: string;
}

function contactOption(contact: CRMContact): CRMAgentRecordOption {
  return {
    id: contact.id,
    name: [contact.first_name, contact.last_name].filter(Boolean).join(' ').trim() || contact.email || 'Unnamed contact',
    detail: contact.email || contact.display_id,
  };
}

function companyOption(company: CRMCompany): CRMAgentRecordOption {
  return { id: company.id, name: company.name, detail: company.domain || company.display_id };
}

function dealOption(deal: CRMDeal): CRMAgentRecordOption {
  return { id: deal.id, name: deal.name, detail: [deal.display_id, deal.pipeline?.name, deal.stage?.name].filter(Boolean).join(' · ') };
}

// Search each record type on the server so a page of other CRM records cannot hide matches.
export async function listCRMAgentRecords(workspaceId: string, type: CRMRecordTargetType, search: string) {
  const filters = { search, page: 1, per_page: 25 };
  switch (type) {
    case 'crm_contact': {
      const page = unwrapRequired(await crmContactService.list(workspaceId, filters), 'Contacts');
      return { items: (page.data ?? []).map(contactOption), total: page.total };
    }
    case 'crm_company': {
      const page = unwrapRequired(await crmCompanyService.list(workspaceId, filters), 'Companies');
      return { items: (page.data ?? []).map(companyOption), total: page.total };
    }
    case 'crm_deal': {
      const page = unwrapRequired(await crmDealService.list(workspaceId, filters), 'Deals');
      return { items: (page.data ?? []).map(dealOption), total: page.total };
    }
  }
}

export async function getCRMAgentRecord(workspaceId: string, type: CRMRecordTargetType, id: string) {
  switch (type) {
    case 'crm_contact': return contactOption(unwrapRequired(await crmContactService.get(workspaceId, id), 'Contact'));
    case 'crm_company': return companyOption(unwrapRequired(await crmCompanyService.get(workspaceId, id), 'Company'));
    case 'crm_deal': return dealOption(unwrapRequired(await crmDealService.get(workspaceId, id), 'Deal'));
  }
}
