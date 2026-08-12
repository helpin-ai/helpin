import { fireEvent, render, screen } from '@testing-library/react'
import { createElement } from 'react'
import { EmptyWorkspacesState, resolveWorkspaceRedirect } from '../workspaces-screen'
import type { Workspace } from '@mobile/lib/types'

function ws(slug: string): Workspace {
  return { id: slug, name: slug, slug }
}

test('a single workspace redirects to it regardless of stored slug', () => {
  expect(resolveWorkspaceRedirect([ws('acme')], null)).toBe('acme')
  expect(resolveWorkspaceRedirect([ws('acme')], 'someone-else')).toBe('acme')
})

test('multiple workspaces with a matching stored slug redirect to that one', () => {
  const workspaces = [ws('acme'), ws('beta')]
  expect(resolveWorkspaceRedirect(workspaces, 'beta')).toBe('beta')
})

test('multiple workspaces with no stored slug match requires a manual pick', () => {
  const workspaces = [ws('acme'), ws('beta')]
  expect(resolveWorkspaceRedirect(workspaces, null)).toBeNull()
  expect(resolveWorkspaceRedirect(workspaces, 'gamma')).toBeNull()
})

test('zero workspaces never redirects', () => {
  expect(resolveWorkspaceRedirect([], null)).toBeNull()
  expect(resolveWorkspaceRedirect([], 'acme')).toBeNull()
})

test('the empty workspace state lets the user sign out with confirmation', () => {
  vi.useFakeTimers()
  const onSignOut = vi.fn()
  render(createElement(EmptyWorkspacesState, { onSignOut }))

  expect(screen.getByText('No workspaces yet')).toBeTruthy()
  const button = screen.getByRole('button', { name: 'Sign out' })
  fireEvent.click(button)
  expect(onSignOut).not.toHaveBeenCalled()
  expect(screen.getByRole('button', { name: 'Tap again to confirm' })).toBeTruthy()
  fireEvent.click(screen.getByRole('button', { name: 'Tap again to confirm' }))
  expect(onSignOut).toHaveBeenCalledOnce()
  vi.useRealTimers()
})
