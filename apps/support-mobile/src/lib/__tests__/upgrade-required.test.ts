import { describe, expect, it } from 'vitest'

import { getUpgradeRequiredReason } from '@mobile/lib/upgrade-required'

describe('getUpgradeRequiredReason', () => {
  it('classifies exhausted AI usage', () => {
    expect(getUpgradeRequiredReason(new Error('AI usage exhausted'))).toMatchObject({
      kind: 'ai_usage',
      title: 'Upgrade to continue',
      primaryBenefit: 'Larger included AI usage allowance',
    })
  })

  it('classifies locked workspaces and Growth-only features', () => {
    expect(getUpgradeRequiredReason({ error: 'Workspace is locked' })?.kind).toBe('workspace_locked')
    expect(getUpgradeRequiredReason('Custom AI agents requires the Growth plan')?.kind).toBe('custom_agents')
  })

  it('returns null for normal validation and network errors', () => {
    expect(getUpgradeRequiredReason(new Error('Name is required'))).toBeNull()
    expect(getUpgradeRequiredReason(undefined)).toBeNull()
  })
})
