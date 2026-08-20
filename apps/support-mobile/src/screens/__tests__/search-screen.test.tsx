import { render, screen } from '@testing-library/react'
import { highlightSearchText } from '../search-screen'

vi.mock('@tanstack/react-router', () => ({ useParams: vi.fn(), useRouter: vi.fn() }))
vi.mock('@mobile/lib/services/workspaces-service', () => ({ workspacesService: {} }))
vi.mock('@mobile/lib/use-workspace-permissions', () => ({ useWorkspacePermissions: vi.fn() }))
vi.mock('@mobile/stores/workspace-store', () => ({ useWorkspaceStore: vi.fn() }))

test('renders safe backend highlight ranges without injecting markup', () => {
  render(<p>{highlightSearchText('Refund <script> request', [{ start: 0, end: 6 }])}</p>)
  expect(screen.getByText('Refund').tagName).toBe('MARK')
  expect(screen.getByText(/<script> request/)).toBeTruthy()
  expect(document.querySelector('script')).toBeNull()
})
