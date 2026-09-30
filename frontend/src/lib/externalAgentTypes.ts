export type ExternalAgentStatus = 'active' | 'disabled' | 'error';

type ExternalAgentSkill = {
  id: string;
  name: string;
  description?: string;
};

type ExternalAgentCapabilities = {
  streaming?: boolean;
  push_notifications?: boolean;
};

/** The A2A Agent Card fields Helpin shows before and after an agent is added. */
export type AgentCardSummary = {
  name: string;
  description: string;
  card_url: string;
  interface_url: string;
  protocol_binding: string;
  protocol_version: string;
  provider_name: string;
  version: string;
  skills: ExternalAgentSkill[];
  capabilities: ExternalAgentCapabilities;
};

export type ExternalAgent = AgentCardSummary & {
  id: string;
  workspace_id: string;
  /** The linked Helpin agent that tasks are assigned to. */
  agent_id: string;
  status: ExternalAgentStatus;
  last_checked_at: string | null;
  last_error: string;
  token_hint: string;
  allowed_team_ids: string[];
  created_at: string;
  updated_at: string;
};

export type PreviewExternalAgentRequest = {
  card_url: string;
  token: string;
};

export type CreateExternalAgentRequest = PreviewExternalAgentRequest & {
  allowed_team_ids?: string[];
};

export type UpdateExternalAgentRequest = {
  name?: string;
  token?: string;
  allowed_team_ids?: string[];
  status?: Exclude<ExternalAgentStatus, 'error'>;
};

export type ExternalAgentList = {
  /** False when the server has no external-agent encryption key (HTTP 503). */
  configured: boolean;
  items: ExternalAgent[];
};
