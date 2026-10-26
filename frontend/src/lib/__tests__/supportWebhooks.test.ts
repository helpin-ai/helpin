import { describe, expect, it } from 'vitest'
import { describeWebhookEvents, webhookEventLabel, webhookURLProblem } from '../supportWebhooks'

describe('support webhooks', () => {
  it('summarises subscriptions', () => {
    expect(describeWebhookEvents([])).toBe('All events')
    expect(describeWebhookEvents(['sla.breached'])).toBe('SLA missed')
    expect(describeWebhookEvents(['sla.breached', 'csat.submitted', 'message.created'])).toBe('3 events')
    expect(webhookEventLabel('ping')).toBe('Test event')
  })

  it('checks URLs before saving', () => {
    expect(webhookURLProblem('')).toMatch(/Enter the URL/)
    expect(webhookURLProblem('hooks.example.com')).toMatch(/full URL/)
    expect(webhookURLProblem('ftp://hooks.example.com')).toMatch(/https/)
    expect(webhookURLProblem('https://user:pw@hooks.example.com')).toMatch(/user name/)
    expect(webhookURLProblem('https://hooks.example.com/helpin?token=1')).toBeNull()
  })
})
