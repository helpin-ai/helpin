import { createFileRoute } from '@tanstack/react-router'
import { OnboardingFlow } from '@/components/onboarding/OnboardingFlow'
import {
  parseWorkspaceOnboardingSlug,
  parseWorkspaceOnboardingStep,
  type WorkspaceOnboardingStep,
} from '@/lib/workspaceOnboardingMode'

type OnboardingSearch = {
  step?: WorkspaceOnboardingStep
  /** Slug of the workspace created earlier in the flow. */
  workspace?: string
}

export const Route = createFileRoute('/_authenticated/onboarding')({
  component: OnboardingRoute,
  validateSearch: (search: Record<string, unknown>): OnboardingSearch => ({
    step: parseWorkspaceOnboardingStep(search.step),
    workspace: parseWorkspaceOnboardingSlug(search.workspace),
  }),
})

function OnboardingRoute() {
  const { step, workspace } = Route.useSearch()
  return <OnboardingFlow step={step} workspaceSlug={workspace} />
}
