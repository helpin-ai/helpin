import { createFileRoute } from '@tanstack/react-router'
import Workspaces from '@/pages/Workspaces'
import { parseWorkspaceOnboardingStep, type WorkspaceOnboardingStep } from '@/lib/workspaceOnboardingMode'

type WorkspacesSearch = {
  create?: boolean
  step?: WorkspaceOnboardingStep
}

export const Route = createFileRoute('/_authenticated/workspaces')({
  component: Workspaces,
  validateSearch: (search: Record<string, unknown>): WorkspacesSearch => ({
    create: search.create === true || search.create === 'true' || undefined,
    step: parseWorkspaceOnboardingStep(search.step),
  }),
})
