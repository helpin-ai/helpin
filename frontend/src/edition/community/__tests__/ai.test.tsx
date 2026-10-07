// @vitest-environment jsdom
import { sharedAIModelsPricing } from '@/components/agents/AIModelsPricingContext'
import { expect, it } from 'vitest'
import { aiUsagePricingText } from '../ai'

it('tells community users that Helpin adds no fee', () => {
  const text = aiUsagePricingText({ mode: 'community', funding_mode: 'customer_unbilled' })
  expect(text).toContain('billed by your provider account')
  expect(text).toContain('no token fee')
})

it('stays silent for a commercial snapshot the community build cannot price', () => {
  expect(aiUsagePricingText({ mode: 'ee', funding_mode: 'helpin_hosted' })).toBeNull()
  expect(aiUsagePricingText(undefined)).toBeNull()
})

it('only summarizes pricing when every listed connection has a known allowed policy', () => {
  const policy = { allowed: true, pricing: { mode: 'community' as const, funding_mode: 'customer_unbilled' } }
  expect(sharedAIModelsPricing([policy, policy])).toBe(aiUsagePricingText(policy.pricing))
  expect(sharedAIModelsPricing([policy, undefined])).toBeNull()
  expect(sharedAIModelsPricing([policy, { allowed: false }])).toBeNull()
  expect(sharedAIModelsPricing([])).toBeNull()
})
