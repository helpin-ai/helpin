// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/lib/services/gitService', () => ({
  gitService: {
    getGitHubAppStatus: vi.fn(),
    createGitHubAppManifest: vi.fn(),
  },
}))

import { gitService } from '@/lib/services/gitService'
import { queryKeys } from '@/lib/queryKeys'
import type { GitHubAppStatus } from '@/lib/pmTypes'
import { GitHubAppCreateButton, submitGitHubAppManifest } from '../GitHubAppCreateButton'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

let root: Root | null = null
let container: HTMLDivElement | null = null

const missingApp: GitHubAppStatus = {
  configured: false,
  source: 'none',
  slug: '',
  install_url: '',
  webhook_configured: false,
  manifest_available: true,
}

async function renderButton(status: GitHubAppStatus, role: string) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
  queryClient.setQueryData(queryKeys.git.githubAppStatus('ws-1'), status)
  queryClient.setQueryData(queryKeys.workspaces.access('ws-1'), { membership: { role } })
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => {
    root!.render(
      <QueryClientProvider client={queryClient}>
        <GitHubAppCreateButton workspaceId="ws-1" organization="acme" />
      </QueryClientProvider>,
    )
  })
  return container
}

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  root = null
  container = null
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

describe('GitHubAppCreateButton', () => {
  it('posts the manifest to GitHub in a form for owners when no app exists', async () => {
    const submit = vi.spyOn(HTMLFormElement.prototype, 'submit').mockImplementation(() => {})
    vi.mocked(gitService.createGitHubAppManifest).mockResolvedValue({
      data: { manifest: { name: 'Helpin' }, post_url: 'https://github.com/organizations/acme/settings/apps/new?state=s', state: 's' },
      error: null,
      status: 200,
    } as Awaited<ReturnType<typeof gitService.createGitHubAppManifest>>)

    const view = await renderButton(missingApp, 'owner')
    const button = view.querySelector('button')
    expect(button?.textContent).toContain('Create GitHub App')

    await act(async () => {
      button!.click()
    })

    expect(gitService.createGitHubAppManifest).toHaveBeenCalledWith('ws-1', 'acme')
    expect(submit).toHaveBeenCalledTimes(1)
    const form = document.querySelector('form')
    expect(form?.getAttribute('method')).toBe('post')
    expect(form?.getAttribute('action')).toBe('https://github.com/organizations/acme/settings/apps/new?state=s')
    const input = form?.querySelector<HTMLInputElement>('input[name="manifest"]')
    expect(JSON.parse(input?.value ?? '{}')).toEqual({ name: 'Helpin' })
  })

  it('renders nothing for non-owners or when an app is configured', async () => {
    expect((await renderButton(missingApp, 'admin')).querySelector('button')).toBeNull()
    act(() => root?.unmount())
    expect((await renderButton({ ...missingApp, configured: true, source: 'env' }, 'owner')).querySelector('button')).toBeNull()
    act(() => root?.unmount())
    expect((await renderButton({ ...missingApp, manifest_available: false }, 'owner')).querySelector('button')).toBeNull()
  })

  it('builds a hidden manifest form', () => {
    const submit = vi.spyOn(HTMLFormElement.prototype, 'submit').mockImplementation(() => {})
    submitGitHubAppManifest('https://github.com/settings/apps/new?state=x', { url: 'https://helpin.example.com' })
    const input = document.querySelector<HTMLInputElement>('form input[name="manifest"]')
    expect(input?.type).toBe('hidden')
    expect(input?.value).toBe('{"url":"https://helpin.example.com"}')
    expect(submit).toHaveBeenCalled()
  })
})
