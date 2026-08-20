import type { SupportAgentRun } from '@helpin-ai/support-core'

export function orderAgentRuns(runs: SupportAgentRun[]): SupportAgentRun[] {
  return [...runs].sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at))
}

export function agentRunStatusLabel(value: string): string {
  return value.replaceAll('_', ' ').replace(/^./, (character) => character.toUpperCase())
}

export function agentRunSummary(run: SupportAgentRun): string | null {
  const output = run.output_summary
  for (const key of ['summary', 'message', 'result', 'content']) {
    const value = output?.[key]
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return run.error_message?.trim() || null
}

export function formatAgentRunTimestamp(value?: string): string | null {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return null
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}
