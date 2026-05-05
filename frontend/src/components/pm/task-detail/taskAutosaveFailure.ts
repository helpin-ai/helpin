import type { UpdateTaskRequest } from '@/lib/pmTypes'

function stableStringify(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(',')}]`
  }

  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .sort(([left], [right]) => left.localeCompare(right))

    return `{${entries.map(([key, entry]) => `${JSON.stringify(key)}:${stableStringify(entry)}`).join(',')}}`
  }

  return JSON.stringify(value)
}

export function getTaskPatchSignature(patch: UpdateTaskRequest): string {
  return stableStringify(patch)
}

export function isBlockedTaskPatch(patch: UpdateTaskRequest, blockedSignature: string | null): boolean {
  return blockedSignature !== null && getTaskPatchSignature(patch) === blockedSignature
}
