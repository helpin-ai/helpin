import { createFileRoute } from '@tanstack/react-router'
import Workspaces from '@/pages/Workspaces'

type WorkspacesSearch = {
  create?: boolean
}

export const Route = createFileRoute('/_authenticated/workspaces')({
  component: Workspaces,
  validateSearch: (search: Record<string, unknown>): WorkspacesSearch => ({
    create: search.create === true || search.create === 'true' || undefined,
  }),
})
