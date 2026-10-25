import { describe, expect, it } from 'vitest'
import {
  describeReportTrend,
  formatReportDuration,
  formatReportPercent,
  formatReportRating,
  reportChannelLabel,
  supportReportParams,
  supportReportRange,
} from '../supportReportFormat'

describe('supportReportRange', () => {
  const now = new Date(2026, 9, 25, 15, 30)

  it('ends at the next local midnight and spans the preset', () => {
    const { from, to } = supportReportRange('7d', now)
    expect(to).toEqual(new Date(2026, 9, 26))
    expect(from).toEqual(new Date(2026, 9, 19))
  })

  it('starts this month on the first', () => {
    expect(supportReportRange('month', now).from).toEqual(new Date(2026, 9, 1))
  })

  it('builds API params with the inbox filter', () => {
    const params = supportReportParams('30d', ['mailbox-1'], now)
    expect(params.mailboxIds).toEqual(['mailbox-1'])
    expect(new Date(params.to).getTime() - new Date(params.from).getTime()).toBeGreaterThanOrEqual(29 * 86_400_000)
    expect(params.tz).toBeTruthy()
  })
})

describe('report formatting', () => {
  it('formats durations compactly', () => {
    expect(formatReportDuration(null)).toBe('—')
    expect(formatReportDuration(42)).toBe('42s')
    expect(formatReportDuration(600)).toBe('10m')
    expect(formatReportDuration(3900)).toBe('1h 5m')
    expect(formatReportDuration(7200)).toBe('2h')
    expect(formatReportDuration(7 * 86400)).toBe('7d')
    expect(formatReportDuration(90000)).toBe('1d 1h')
  })

  it('formats rates and ratings', () => {
    expect(formatReportPercent(0.4)).toBe('40%')
    expect(formatReportPercent(null)).toBe('—')
    expect(formatReportRating(4.25)).toBe('4.3')
    expect(formatReportRating(undefined)).toBe('—')
  })

  it('labels channels', () => {
    expect(reportChannelLabel('widget')).toBe('Chat')
    expect(reportChannelLabel('internal')).toBe('Started by team')
    expect(reportChannelLabel('sms')).toBe('Sms')
  })
})

describe('describeReportTrend', () => {
  it('reports volume changes without judging them', () => {
    expect(describeReportTrend(12, 10)).toEqual({ label: '+20%', tone: 'neutral' })
    expect(describeReportTrend(3, 0)).toEqual({ label: '+3', tone: 'neutral' })
  })

  it('marks faster responses as better', () => {
    expect(describeReportTrend(300, 600, { better: 'down', kind: 'duration' })).toEqual({ label: '−5m', tone: 'positive' })
    expect(describeReportTrend(900, 600, { better: 'down', kind: 'duration' })).toEqual({ label: '+5m', tone: 'negative' })
  })

  it('compares rates in points and ratings in tenths', () => {
    expect(describeReportTrend(0.55, 0.5, { better: 'up', kind: 'rate' })).toEqual({ label: '+5 pts', tone: 'positive' })
    expect(describeReportTrend(3.8, 4.2, { better: 'up', kind: 'rating' })).toEqual({ label: '−0.4', tone: 'negative' })
  })

  it('has nothing to compare without both periods', () => {
    expect(describeReportTrend(null, 5)).toBeNull()
    expect(describeReportTrend(5, 5)).toEqual({ label: 'No change', tone: 'neutral' })
    expect(describeReportTrend(0.501, 0.5, { better: 'up', kind: 'rate' })).toEqual({ label: 'No change', tone: 'neutral' })
  })
})
