import { resolveWorkspaceRedirect } from '../workspaces-screen'
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
