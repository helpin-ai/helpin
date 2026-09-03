import { readFileSync } from 'node:fs'

import { describe, expect, it } from 'vitest'

const source = readFileSync(new URL('../KanbanBoard.tsx', import.meta.url), 'utf8')

describe('KanbanBoard loading sources', () => {
  it('uses shared query hooks for task reference data', () => {
    expect(source).toContain('useLabels(workspaceId)')
    expect(source).toContain('useEpics(workspaceId)')
    expect(source).toContain('useSprints(workspaceId)')
    expect(source).not.toContain('pmLabelService.list(')
    expect(source).not.toContain('pmEpicService.list(')
    expect(source).not.toContain('pmSprintService.list(')
  })

  it('uses workflows already loaded by the board store', () => {
    expect(source).toContain('usePMBoardStore((state) => state.workflows)')
    expect(source).not.toContain('useWorkflows(')
  })
})
