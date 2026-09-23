import type { WorkspaceModule } from './types';

/** Record types the sample data loader creates; keys of SampleDataStatus.counts. */
export type SampleDataEntity =
  | 'support_conversation'
  | 'pm_task'
  | 'pm_epic'
  | 'workspace_team'
  | 'docs_document'
  | 'docs_space'
  | 'crm_deal'
  | 'crm_contact'
  | 'crm_company'
  | 'crm_pipeline';

/** Response of GET/POST/DELETE /workspaces/{id}/sample-data. */
export interface SampleDataStatus {
  loaded: boolean;
  loaded_at?: string;
  counts: Partial<Record<SampleDataEntity, number>>;
  /** Modules sample data can be loaded for on this server, for this user. */
  modules: WorkspaceModule[];
  /** Sample containers kept by a removal because people added their own records to them. */
  retained?: Partial<Record<SampleDataEntity, number>>;
}
