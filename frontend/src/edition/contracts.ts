export type UpgradeRequiredKind =
  | "custom_agents"
  | "automation_flows"
  | "agent_scheduling"
  | "ai_usage"
  | "workspace_locked"
  | "contacts_limit"
  | "documents_limit"
  | "generic";

export interface UpgradeRequiredReason {
  kind: UpgradeRequiredKind;
  title: string;
  message: string;
  primaryBenefit: string;
}
