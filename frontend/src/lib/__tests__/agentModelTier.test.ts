import { describe, expect, it } from 'vitest';
import { AI_PRICING } from '@/generated/aiPricing';
import { AGENT_MODEL_TIER_OPTIONS, agentModelTierLabel } from '../agentModelTier';

describe('agent model tiers', () => {
  it('uses labels and descriptions from the generated pricing catalog', () => {
    expect(AGENT_MODEL_TIER_OPTIONS).toEqual(AI_PRICING.tiers.map((tier) => ({
      value: tier.key,
      label: tier.label,
      description: tier.description,
    })));
  });

  it('does not expose an exact provider or model', () => {
    expect(AGENT_MODEL_TIER_OPTIONS).toHaveLength(4);
    expect(JSON.stringify(AGENT_MODEL_TIER_OPTIONS)).not.toMatch(/openai|anthropic|openrouter|gpt|claude|deepseek/i);
    expect(agentModelTierLabel('large')).toBe('Large');
    expect(agentModelTierLabel('unknown')).toBe('Managed');
  });
});
