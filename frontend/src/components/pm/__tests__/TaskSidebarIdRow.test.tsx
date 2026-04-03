// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { buildGitBranch } from '../TaskSidebarIdRow'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('buildGitBranch', () => {
  it('uses task type as branch prefix', () => {
    expect(buildGitBranch('ST-123', 'Add user auth', 'feature')).toBe('feature/st-123-add-user-auth')
    expect(buildGitBranch('ST-123', 'Login crash', 'bug')).toBe('fix/st-123-login-crash')
    expect(buildGitBranch('ST-123', 'Update deps', 'chore')).toBe('chore/st-123-update-deps')
  })

  it('defaults to feature/ when no task type', () => {
    expect(buildGitBranch('ST-123', 'Add user auth')).toBe('feature/st-123-add-user-auth')
  })

  it('falls back to id-only when no name', () => {
    expect(buildGitBranch('ST-42')).toBe('feature/st-42')
    expect(buildGitBranch('ST-42', '')).toBe('feature/st-42')
    expect(buildGitBranch('ST-42', '  ')).toBe('feature/st-42')
  })

  it('strips special characters and trims dashes', () => {
    expect(buildGitBranch('PM-7', '  Fix: login (URGENT)!! ', 'bug')).toBe('fix/pm-7-fix-login-urgent')
  })

  it('truncates long names to 48 chars', () => {
    const longName = 'a'.repeat(100)
    const branch = buildGitBranch('ST-1', longName, 'feature')
    // prefix "feature/st-1-" = 13 chars + 48 chars slug max
    expect(branch.length).toBeLessThanOrEqual(13 + 48)
  })
})
