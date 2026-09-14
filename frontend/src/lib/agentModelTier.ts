import { AI_MODELS } from '@/generated/aiModels';
import type { AgentModelTier } from '@/lib/pmTypes';

export interface AgentModelTierOption {
  value: AgentModelTier;
  label: string;
  description: string;
}

const AGENT_MODEL_TIER_KEYS: AgentModelTier[] = ['small', 'medium', 'large', 'flagship'];

export const AGENT_MODEL_TIER_OPTIONS: AgentModelTierOption[] = AGENT_MODEL_TIER_KEYS.map((key) => {
  const tier = AI_MODELS.tiers.find((entry) => entry.key === key);
  if (!tier) {
    throw new Error(`AI model catalog is missing agent model tier ${key}`);
  }
  return {
    value: key,
    label: tier.label,
    description: tier.description,
  };
});

export function agentModelTierOption(value?: string | null): AgentModelTierOption | undefined {
  return AGENT_MODEL_TIER_OPTIONS.find((tier) => tier.value === value);
}

export function agentModelTierLabel(value?: string | null): string {
  return agentModelTierOption(value)?.label ?? 'Managed';
}
