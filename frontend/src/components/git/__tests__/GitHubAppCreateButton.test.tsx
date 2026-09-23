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
import type { GitHubAppStatus, GitHubReturnTo } from '@/lib/pmTypes'
import { GitHubAppCreateButton } from '../GitHubAppCreateButton'
import { GITHUB_APP_OWNER_HINT, submitGitHubAppManifest } from '../githubApp'

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

async function renderButton(status: GitHubAppStatus, role: string, props: { organization?: string; returnTo?: GitHubReturnTo } = {}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false, staleTime: Infinity } } })
  queryClient.setQueryData(queryKeys.git.githubAppStatus('ws-1'), status)
  queryClient.setQueryData(queryKeys.workspaces.access('ws-1'), { membership: { role } })
  container = document.createElement('div')
  document.body.appendChild(container)
  root = createRoot(container)
  await act(async () => {
    root!.render(
      <QueryClientProvider client={queryClient}>
        <GitHubAppCreateButton workspaceId="ws-1" {...props} />
      </QueryClientProvider>,
    )
  })
  return container
}

function buttonNamed(view: ParentNode, name: string) {
  return Array.from(view.querySelectorAll('button')).find((button) => button.textContent?.trim() === name)
}

async function click(element: HTMLElement | undefined) {
  if (!element) throw new Error('element not found')
  await act(async () => {
    element.click()
  })
}

async function typeInto(input: HTMLInputElement, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

function mockManifest(postURL = 'https://github.com/organizations/acme/settings/apps/new?state=s') {
  vi.mocked(gitService.createGitHubAppManifest).mockResolvedValue({
    data: { manifest: { name: 'Helpin' }, post_url: postURL, state: 's' },
    error: null,
    status: 200,
  } as Awaited<ReturnType<typeof gitService.createGitHubAppManifest>>)
}

afterEach(() => {
  act(() => root?.unmount())
  container?.remove()
  root = null
  container = null
  document.body.innerHTML = ''
  vi.restoreAllMocks()
  vi.mocked(gitService.createGitHubAppManifest).mockReset()
})

describe('GitHubAppCreateButton', () => {
  it('asks who owns the App, defaulting to an organization, before going to GitHub', async () => {
    const submit = vi.spyOn(HTMLFormElement.prototype, 'submit').mockImplementation(() => {})
    mockManifest()
    const view = await renderButton(missingApp, 'owner')

    const trigger = buttonNamed(view, 'Create GitHub App')
    expect(trigger?.getAttribute('aria-expanded')).toBe('false')
    await click(trigger)
    expect(trigger?.getAttribute('aria-expanded')).toBe('true')
    expect(view.textContent).toContain('Who should own the App?')
    expect(view.textContent).toContain(GITHUB_APP_OWNER_HINT)
    const organization = view.querySelector<HTMLInputElement>('input[type="radio"][value="organization"]')
    expect(organization?.checked).toBe(true)

    await typeInto(view.querySelector<HTMLInputElement>('input[placeholder="acme-inc"]')!, 'acme')
    await click(buttonNamed(view, 'Continue to GitHub'))

    expect(gitService.createGitHubAppManifest).toHaveBeenCalledWith('ws-1', { organization: 'acme', return_to: 'settings' })
    expect(submit).toHaveBeenCalledTimes(1)
    const form = document.querySelector<HTMLFormElement>('form[method="post"]')
    expect(form?.getAttribute('action')).toBe('https://github.com/organizations/acme/settings/apps/new?state=s')
    const input = form?.querySelector<HTMLInputElement>('input[name="manifest"]')
    expect(JSON.parse(input?.value ?? '{}')).toEqual({ name: 'Helpin' })
  })

  it('validates the organization login before contacting the server', async () => {
    const view = await renderButton(missingApp, 'owner')
    await click(buttonNamed(view, 'Create GitHub App'))

    await click(buttonNamed(view, 'Continue to GitHub'))
    expect(view.querySelector('[role="alert"]')?.textContent).toContain('Enter the GitHub organization login')

    const login = view.querySelector<HTMLInputElement>('input[placeholder="acme-inc"]')!
    await typeInto(login, '-acme/inc')
    await click(buttonNamed(view, 'Continue to GitHub'))
    expect(view.querySelector('[role="alert"]')?.textContent).toContain('letters, numbers and hyphens')
    expect(login.getAttribute('aria-invalid')).toBe('true')
    expect(gitService.createGitHubAppManifest).not.toHaveBeenCalled()
  })

  it('sends no organization for a personal App and carries return_to', async () => {
    vi.spyOn(HTMLFormElement.prototype, 'submit').mockImplementation(() => {})
    mockManifest('https://github.com/settings/apps/new?state=s')
    const view = await renderButton(missingApp, 'owner', { organization: 'prefilled', returnTo: 'setup' })
    await click(buttonNamed(view, 'Create GitHub App'))
    expect(view.querySelector<HTMLInputElement>('input[placeholder="acme-inc"]')?.value).toBe('prefilled')

    await click(view.querySelector<HTMLInputElement>('input[type="radio"][value="personal"]')!)
    expect(view.querySelector('input[placeholder="acme-inc"]')).toBeNull()
    await click(buttonNamed(view, 'Continue to GitHub'))

    expect(gitService.createGitHubAppManifest).toHaveBeenCalledWith('ws-1', { organization: undefined, return_to: 'setup' })
  })

  it('explains that GitHub cannot reach the server instead of offering creation', async () => {
    const reason = 'GitHub must reach this server to deliver events. Set APP_BASE_URL to a public https address (currently http://localhost:8080).'
    const view = await renderButton({ ...missingApp, manifest_available: false, manifest_blocked_reason: reason }, 'owner')
    expect(buttonNamed(view, 'Create GitHub App')).toBeUndefined()
    expect(view.textContent).toBe(reason)
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
