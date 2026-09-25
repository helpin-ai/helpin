// @vitest-environment jsdom
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
