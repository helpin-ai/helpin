import { createFileRoute } from '@tanstack/react-router'
import Workspaces from '@/pages/Workspaces'
import { parseWorkspaceOnboardingStep, type WorkspaceOnboardingStep } from '@/lib/workspaceOnboardingMode'

type OnboardingSearch = {
  step?: WorkspaceOnboardingStep
}

export const Route = createFileRoute('/_authenticated/onboarding')({
  component: () => <Workspaces dedicatedOnboarding />,
  validateSearch: (search: Record<string, unknown>): OnboardingSearch => ({
    step: parseWorkspaceOnboardingStep(search.step),
  }),
})
