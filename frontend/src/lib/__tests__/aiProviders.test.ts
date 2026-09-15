import { expect, it } from 'vitest'
import {
  catalogLabel,
  connectionStatusInfo,
  connectionStatusMeta,
  modelCatalogFor,
  providerLabel,
  providerShortLabel,
  providerControls,
} from '@/lib/aiProviders'
import { AI_MODELS } from '@/generated/aiModels'

it('names every supported provider and falls back to the raw identifier', () => {
  expect(providerLabel('openai_chatgpt')).toBe('ChatGPT subscription')
  expect(providerLabel('openai_compatible')).toBe('Compatible endpoint')
  expect(providerShortLabel('openrouter')).toBe('OpenRouter')
  expect(providerLabel('some_future_provider')).toBe('some_future_provider')
  expect(providerShortLabel('some_future_provider')).toBe('some_future_provider')
})

it('groups suggested models by catalog tier and hides disabled entries', () => {
  const groups = modelCatalogFor('openai')
  expect(groups.length).toBeGreaterThan(0)
  const order = AI_MODELS.tiers.map((tier) => tier.key).filter((key) => groups.some((group) => group.key === key))
  expect(groups.map((group) => group.key)).toEqual(order)
  const suggested = groups.flatMap((group) => group.models.map((model) => model.selectionModel))
  const disabled = AI_MODELS.models.filter((model) => model.provider === 'openai' && !model.enabled)
  for (const model of disabled) expect(suggested).not.toContain(model.selection_model)
})

it('reuses the OpenAI catalog for ChatGPT and leaves compatible endpoints free text', () => {
  expect(modelCatalogFor('openai_chatgpt')).toEqual(modelCatalogFor('openai'))
  expect(modelCatalogFor('openai_compatible')).toEqual([])
})

it('resolves catalog display names only for known identifiers', () => {
  const known = AI_MODELS.models.find((model) => model.provider === 'openai' && model.enabled)!
  expect(catalogLabel('openai', known.selection_model)).toBe(known.label)
  expect(catalogLabel('openai_compatible', 'my-local-model')).toBeUndefined()
})

it('describes every connection status and tolerates unknown ones', () => {
  expect(Object.keys(connectionStatusMeta).sort()).toEqual([
    'connected',
    'disconnected',
    'pending',
    'reauthorization_required',
  ])
  expect(connectionStatusInfo('reauthorization_required')).toEqual({ label: 'Reconnect required', tone: 'attention' })
  expect(connectionStatusInfo('brand_new_status').label).toBe('brand new status')
})

it('exposes model controls only where the provider supports them', () => {
  expect(providerControls('openai').serviceTier).toBe(true)
  expect(providerControls('anthropic').reasoningEffort).toBe(false)
  expect(providerControls('openrouter').openrouterQuantizations).toBe(true)
  expect(providerControls('openai_compatible')).toEqual({
    reasoningEffort: false,
    serviceTier: false,
    openrouterQuantizations: false,
  })
})
