// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { buildGitBranch, TaskSidebarIdRow } from '../TaskSidebarIdRow'

;(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true

describe('buildGitBranch', () => {
  it('generates feature branch with slugified task name', () => {
    expect(buildGitBranch('ST-123', 'Add user auth')).toBe('feature/st-123-add-user-auth')
  })

  it('falls back to id-only when no name', () => {
    expect(buildGitBranch('ST-42')).toBe('feature/st-42')
    expect(buildGitBranch('ST-42', '')).toBe('feature/st-42')
    expect(buildGitBranch('ST-42', '  ')).toBe('feature/st-42')
  })

  it('strips special characters and trims dashes', () => {
    expect(buildGitBranch('PM-7', '  Fix: login (URGENT)!! ')).toBe('feature/pm-7-fix-login-urgent')
  })

  it('truncates long names to 48 chars', () => {
    const longName = 'a'.repeat(100)
    const branch = buildGitBranch('ST-1', longName)
    // "feature/st-1-" = 13 chars + 48 chars slug
    expect(branch.length).toBeLessThanOrEqual(13 + 48)
  })
})

describe('TaskSidebarIdRow', () => {
  const originalClipboard = navigator.clipboard
  const originalExecCommand = document.execCommand
  const originalSecureContext = window.isSecureContext

  afterEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: originalClipboard,
    })
    document.execCommand = originalExecCommand
    Object.defineProperty(window, 'isSecureContext', {
      configurable: true,
      value: originalSecureContext,
    })
  })

  function setupClipboard() {
    const writeText = vi.fn().mockResolvedValue(undefined)
    const execCommand = vi.fn(() => true)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    document.execCommand = execCommand
    Object.defineProperty(window, 'isSecureContext', {
      configurable: true,
      value: true,
    })
    return { writeText, execCommand }
  }

  it('renders the Task ID label and copies the display ID', async () => {
    const { execCommand } = setupClipboard()

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<TaskSidebarIdRow displayId="ST-123" />)
    })

    expect(container.textContent).toContain('Task ID:')
    expect(container.textContent).toContain('ST-123')

    const buttons = container.querySelectorAll('button')
    expect(buttons.length).toBe(2) // copy ID + copy branch

    // Click the copy ID button (first button)
    await act(async () => {
      buttons[0]?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(execCommand).toHaveBeenCalledWith('copy')

    act(() => {
      root.unmount()
    })
    container.remove()
  })

  it('copies git branch name when git branch button clicked', async () => {
    const { execCommand } = setupClipboard()

    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    act(() => {
      root.render(<TaskSidebarIdRow displayId="ST-123" taskName="Fix login bug" />)
    })

    const buttons = container.querySelectorAll('button')
    // Click the git branch button (second button)
    await act(async () => {
      buttons[1]?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })

    expect(execCommand).toHaveBeenCalledWith('copy')

    act(() => {
      root.unmount()
    })
    container.remove()
  })
})
