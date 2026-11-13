// @vitest-environment jsdom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const service = vi.hoisted(() => ({
  previewPublicHelpCenter: vi.fn(),
  startPublicHelpCenter: vi.fn(),
  getStatus: vi.fn(),
  cancel: vi.fn(),
  resumePublicHelpCenter: vi.fn(),
}))
const access = vi.hoisted(() => ({ permissions: ['docs.import'] as string[] }))
vi.mock('@/lib/services/docsImportService', () => ({ docsImportService: service, PUBLIC_HELP_CENTER_IMPORT_SOURCE: 'intercom_public' }))
vi.mock('@/hooks/queries/useSession', () => ({ useWorkspaceAccess: () => ({ data: { permissions: access.permissions }, isLoading: false }) }))
vi.mock('@/hooks/queries', () => ({
  useDocsSpaces: () => ({ data: [
    { id: 'space-hc', name: 'Existing help center', type: 'external_capable' },
    { id: 'space-internal', name: 'Team wiki', type: 'internal' },
  ] }),
}))
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: (selector: (state: { currentWorkspace: { slug: string } }) => unknown) => selector({ currentWorkspace: { slug: 'acme' } }),
}))
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

import { PublicHelpCenterImportWizard } from '../PublicHelpCenterImportWizard'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const preview = {
  platform: 'intercom',
  url: 'https://docs.acme.com/en/',
  name: 'Acme Help Center',
  locale: 'en',
  collections: 9,
  articles: 124,
  collections_merged: 1,
  top_collections: Array.from({ length: 8 }, (_, index) => ({ name: `Collection ${index + 1}`, articles: 10 + index })),
}

function job(overrides: Record<string, unknown> = {}) {
  return {
    id: 'job-1', space_id: 'space-new', status: 'running', source: 'intercom_public', total: 124, completed: 40, failed: 0,
    failures: [], redirect_map: null, summary: null, started_by: 'u', started_at: '', completed_at: null, created_at: '', ...overrides,
  }
}

// Lets pending requests resolve and their state updates render. A query's
// result takes more than one turn of the event loop to reach the screen.
async function flush() {
  for (let turn = 0; turn < 4; turn += 1) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0))
    })
  }
}

function setInput(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!
  setter.call(input, value)
  input.dispatchEvent(new Event('input', { bubbles: true }))
}

describe('PublicHelpCenterImportWizard', () => {
  let container: HTMLDivElement
  let unmount: () => void

  function render(props: { activeJobId?: string } = {}) {
    container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    act(() => root.render(<QueryClientProvider client={client}><PublicHelpCenterImportWizard workspaceId="ws" {...props} /></QueryClientProvider>))
    unmount = () => act(() => root.unmount())
  }

  const button = (label: string) => [...container.querySelectorAll('button')].find((element) => element.textContent?.trim() === label) as HTMLButtonElement | undefined
  const addressInput = () => container.querySelector('#public-help-center-address') as HTMLInputElement

  async function checkAddress(value: string) {
    act(() => setInput(addressInput(), value))
    await act(async () => {
      container.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    })
    await flush()
  }

  beforeEach(() => {
    access.permissions = ['docs.import']
    service.previewPublicHelpCenter.mockResolvedValue({ data: preview, error: null })
    service.startPublicHelpCenter.mockResolvedValue({ data: { job_id: 'job-1' }, error: null })
    service.getStatus.mockResolvedValue({ data: job(), error: null })
    service.cancel.mockResolvedValue({ data: {}, error: null })
    service.resumePublicHelpCenter.mockResolvedValue({ data: {}, error: null })
  })
  afterEach(() => {
    unmount()
    container.remove()
    vi.clearAllMocks()
  })

  it('previews an address and starts the import with the chosen options', async () => {
    render()
    expect(button('Check address')!.disabled).toBe(true)
    await checkAddress('  docs.acme.com ')

    expect(service.previewPublicHelpCenter).toHaveBeenCalledWith('ws', 'docs.acme.com')
    const found = container.querySelector('[data-testid="public-help-center-preview"]')!
    expect(found.textContent).toContain('Acme Help Center')
    expect(found.textContent).toContain('124 articles in 9 collections at docs.acme.com/en')
    expect(found.textContent).toContain('1 collection is nested deeper than Helpin supports; its articles move into the collection above.')
    // A long collection list starts collapsed.
    expect(found.querySelectorAll('li')).toHaveLength(6)
    act(() => button('Show all 8 top-level collections')!.click())
    expect(found.querySelectorAll('li')).toHaveLength(8)

    // The new space takes the help center's name, and can be renamed.
    const spaceName = container.querySelector('#public-help-center-space-name') as HTMLInputElement
    expect(spaceName.value).toBe('Acme Help Center')
    act(() => setInput(spaceName, 'Acme docs'))

    await act(async () => button('Import 124 articles')!.click())
    await flush()
    expect(service.startPublicHelpCenter).toHaveBeenCalledWith('ws', {
      url: 'https://docs.acme.com/en/', import_status: 'published', new_space_name: 'Acme docs',
    })
    expect(container.textContent).toContain('Importing Acme Help Center')
    expect(container.textContent).toContain('You can leave this page; the import keeps running.')
    expect(container.textContent).toContain('40 of 124 articles')
  })

  it('names a resumed import by its address until the report arrives', async () => {
    service.getStatus.mockResolvedValue({ data: job({ config: { source_url: 'https://docs.acme.com/en/' } }), error: null })
    render({ activeJobId: 'job-1' })
    await flush()
    expect(container.textContent).toContain('Importing docs.acme.com/en')
  })

  it('shows why an address was refused and keeps it for correction', async () => {
    service.previewPublicHelpCenter.mockResolvedValue({ data: null, error: 'This address is not an Intercom help center.' })
    render()
    await checkAddress('https://example.com')

    const alert = container.querySelector('[role="alert"]')!
    expect(alert.textContent).toBe('This address is not an Intercom help center.')
    expect(addressInput().value).toBe('https://example.com')
    expect(addressInput().getAttribute('aria-invalid')).toBe('true')
    expect(addressInput().getAttribute('aria-describedby')).toBe(alert.id)
    expect(document.activeElement).toBe(addressInput())
    expect(container.querySelector('[data-testid="public-help-center-preview"]')).toBeNull()

    // Typing clears the error.
    act(() => setInput(addressInput(), 'https://help.example.com'))
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })

  it('hides a preview once the address is edited', async () => {
    render()
    await checkAddress('docs.acme.com')
    expect(button('Check again')).toBeDefined()
    act(() => setInput(addressInput(), 'docs.other.com'))
    expect(container.querySelector('[data-testid="public-help-center-preview"]')).toBeNull()
    expect(button('Check address')).toBeDefined()
    expect(button('Import 124 articles')).toBeUndefined()
  })

  it('needs a name for a new space and reports a start failure in place', async () => {
    service.startPublicHelpCenter.mockResolvedValue({ data: null, error: 'An import into this space is already running.' })
    render()
    await checkAddress('docs.acme.com')
    act(() => setInput(container.querySelector('#public-help-center-space-name')!, '   '))
    expect(button('Import 124 articles')!.disabled).toBe(true)
    act(() => setInput(container.querySelector('#public-help-center-space-name')!, 'Docs'))
    await act(async () => button('Import 124 articles')!.click())
    await flush()
    expect(container.textContent).toContain('An import into this space is already running.')
    // Still on the form, with the preview intact.
    expect(container.querySelector('[data-testid="public-help-center-preview"]')).not.toBeNull()
  })

  it('explains the missing permission instead of showing a form', () => {
    access.permissions = []
    render()
    expect(container.textContent).toContain('You need permission to import Help Center articles')
    expect(container.querySelector('form')).toBeNull()
  })

  it('describes each stage of a running import', async () => {
    service.getStatus.mockResolvedValue({ data: job({ status: 'pending', total: 0, completed: 0 }), error: null })
    render({ activeJobId: 'job-1' })
    await flush()
    expect(container.querySelector('[data-testid="public-help-center-progress"]')!.textContent).toContain('Starting…')
    unmount()
    container.remove()

    service.getStatus.mockResolvedValue({ data: job({ total: 0, completed: 0 }), error: null })
    render({ activeJobId: 'job-1' })
    await flush()
    expect(container.textContent).toContain('Reading the help center…')
    unmount()
    container.remove()

    // Every article is in; the links and order are still being finished.
    service.getStatus.mockResolvedValue({ data: job({ completed: 123, failed: 1 }), error: null })
    render({ activeJobId: 'job-1' })
    await flush()
    expect(container.textContent).toContain('Updating links between articles and restoring their order…')
  })

  it('stops a running import', async () => {
    render({ activeJobId: 'job-1' })
    await flush()
    service.getStatus.mockResolvedValue({ data: job({ status: 'interrupted', completed: 40 }), error: null })
    await act(async () => button('Stop import')!.click())
    await flush()
    expect(service.cancel).toHaveBeenCalledWith('ws', 'job-1')
    expect(container.textContent).toContain('Stopped after 40 articles.')

    // Continuing picks the same job up again.
    service.getStatus.mockResolvedValue({ data: job({ status: 'running', completed: 41 }), error: null })
    await act(async () => button('Continue import')!.click())
    await flush()
    expect(service.resumePublicHelpCenter).toHaveBeenCalledWith('ws', 'job-1')
    expect(container.textContent).toContain('41 of 124 articles')
  })

  it('reports a finished import and everything that needs a look', async () => {
    service.getStatus.mockResolvedValue({
      data: job({
        status: 'done', completed: 122, failed: 2,
        failures: [{ article_id: '9', title: 'Billing FAQ', error: 'The article requires sign-in on the source help center.' }],
        summary: {
          source_name: 'Acme Help Center', source_url: 'https://docs.acme.com/en/', articles_published: 122, articles_drafted: 0,
          images_copied: 583, links_rewritten: 199, redirects_created: 150, articles_outside_collections: 3, collections_merged: 1,
          unresolved_links_total: 250, failed_images_total: 1,
          unresolved_links: [{ document_id: 'doc-1', article_title: 'Getting started', url: 'https://docs.acme.com/en/articles/404-gone' }],
          failed_images: [{ document_id: 'doc-2', article_title: 'Invoices', url: 'https://cdn.acme.com/a.png', reason: 'The image could not be downloaded.' }],
        },
      }),
      error: null,
    })
    render({ activeJobId: 'job-1' })
    await flush()

    const text = container.textContent!
    expect(text).toContain('Import of Acme Help Center')
    expect(text).toContain('122 articles imported, 2 failed.')
    expect(text).toContain('583 images copied.')
    expect(text).toContain('199 links between articles now point to the new addresses.')
    expect(text).toContain('3 articles that were in no collection went into “Other articles”.')
    expect(text).not.toContain('You can leave this page')

    const attention = container.querySelector('[data-testid="public-help-center-attention"]')!
    expect(attention.textContent).toContain('2 articles were not imported')
    expect(attention.textContent).toContain('Billing FAQ: The article requires sign-in')
    expect(attention.textContent).toContain('250 links still point at the original help center')
    expect(attention.textContent).toContain('Showing the first 1 of 250.')
    expect(attention.textContent).toContain('1 image was not copied')
    expect(attention.textContent).toContain('The article still shows it from the original help center.')
    expect(attention.textContent).toContain('Their targets were not found there')
    // Each item leads to the article that needs the fix.
    const articleLinks = [...attention.querySelectorAll('a')].map((link) => link.getAttribute('href'))
    expect(articleLinks).toContain('/w/acme/docs/documents/doc-1')
    expect(articleLinks).toContain('/w/acme/docs/documents/doc-2')
    const external = [...attention.querySelectorAll('a')].find((link) => link.getAttribute('href') === 'https://docs.acme.com/en/articles/404-gone')!
    expect(external.getAttribute('rel')).toBe('noopener noreferrer')
    expect(external.textContent).toBe('docs.acme.com/en/articles/404-gone')

    expect((container.querySelector('a[href="/w/acme/docs/spaces/space-new"]') as HTMLAnchorElement).textContent).toBe('Open the space')
    await act(async () => button('Retry failed articles')!.click())
    expect(service.resumePublicHelpCenter).toHaveBeenCalledWith('ws', 'job-1')
  })

  it('shows a clean result without a to-do list, and can start over', async () => {
    service.getStatus.mockResolvedValue({
      data: job({ status: 'done', completed: 12, total: 12, summary: { source_name: 'Acme Help Center', articles_published: 0, articles_drafted: 12, images_copied: 0, links_rewritten: 1 } }),
      error: null,
    })
    render({ activeJobId: 'job-1' })
    await flush()
    expect(container.textContent).toContain('12 articles imported.')
    expect(container.textContent).toContain('They are drafts; nothing is live yet.')
    expect(container.textContent).toContain('1 link between articles now points to the new addresses.')
    expect(container.querySelector('[data-testid="public-help-center-attention"]')).toBeNull()
    expect(button('Retry failed articles')).toBeUndefined()

    act(() => button('Import another help center')!.click())
    expect(addressInput().value).toBe('')
  })

  it('shows why an import failed outright and offers to try again', async () => {
    service.getStatus.mockResolvedValue({
      data: job({ status: 'failed', total: 0, completed: 0, error: 'The help center did not respond. Check the address and try again in a moment.' }),
      error: null,
    })
    render({ activeJobId: 'job-1' })
    await flush()
    expect(container.querySelector('[role="alert"]')!.textContent).toBe('The help center did not respond. Check the address and try again in a moment.')
    expect(button('Try again')).toBeDefined()
    expect(container.querySelector('a[href="/w/acme/docs/spaces/space-new"]')).toBeNull()
  })
})
