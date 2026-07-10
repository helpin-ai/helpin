import { trackAnalyticsEvent } from './analytics';
import type { SetupGoalKey } from './setupTypes';

export type WorkspaceOnboardingUseCase =
  | 'product_engineering'
  | 'team_project_management'
  | 'customer_support'
  | 'help_center_docs'
  | 'internal_docs'
  | 'sales_crm';

export type WorkspaceOnboardingUseCaseOption = {
  value: WorkspaceOnboardingUseCase;
  label: string;
  description: string;
  replaces: string;
};

export const ONBOARDING_USE_CASE_OPTIONS: WorkspaceOnboardingUseCaseOption[] = [
  {
    value: 'product_engineering',
    label: 'Product & engineering work',
    description: 'AI agents help plan work, draft specs, use repo context, and keep product execution moving.',
    replaces: 'Replaces Jira, Linear, Shortcut',
  },
  {
    value: 'team_project_management',
    label: 'Team/project management',
    description: 'AI agents help organize tasks, summarize progress, automate follow-ups, and support team workflows.',
    replaces: 'Replaces Asana, Monday, Trello',
  },
  {
    value: 'customer_support',
    label: 'Customer support',
    description: 'AI agents help answer customers, draft replies, find gaps, and use your knowledge sources.',
    replaces: 'Replaces Intercom, Zendesk, Freshdesk',
  },
  {
    value: 'help_center_docs',
    label: 'Help center docs',
    description: 'AI agents help create, update, and improve public-facing help articles for customers.',
    replaces: 'Replaces Zendesk Guide, Help Scout Docs',
  },
  {
    value: 'internal_docs',
    label: 'Internal docs',
    description: 'AI agents help maintain company, product, process, and team knowledge for internal use.',
    replaces: 'Replaces Notion, Confluence, Google Docs',
  },
  {
    value: 'sales_crm',
    label: 'Sales/CRM',
    description: 'AI agents help manage follow-ups, summarize customer context, and support sales workflows.',
    replaces: 'Replaces HubSpot, Pipedrive, Salesforce',
  },
];

const setupGoalByUseCase: Record<WorkspaceOnboardingUseCase, SetupGoalKey> = {
  product_engineering: 'product_delivery',
  team_project_management: 'product_delivery',
  customer_support: 'customer_support',
  help_center_docs: 'help_center_docs',
  internal_docs: 'internal_docs',
  sales_crm: 'sales_crm',
};

export function mapOnboardingUseCasesToSetupGoals(useCases: WorkspaceOnboardingUseCase[]): SetupGoalKey[] {
  return [...new Set(useCases.map((useCase) => setupGoalByUseCase[useCase]))];
}

type TrackingTarget = {
  location?: { hostname?: string };
  usermaven?:
    | ((command: 'track', eventName: string, properties?: Record<string, unknown>) => void)
    | {
      track?: (eventName: string, properties?: Record<string, unknown>) => void;
    };
};

export function trackWorkspaceOnboardingUseCases(
  useCases: WorkspaceOnboardingUseCase[],
  target: TrackingTarget = globalThis as TrackingTarget,
) {
  const properties = {
    use_cases: useCases,
    use_case_count: useCases.length,
  };
  const hostname = target.location?.hostname;
  if (!target.usermaven) {
    trackAnalyticsEvent('workspace_onboarding_use_cases_selected', properties, { hostname });
    return;
  }
  if (hostname !== 'app.helpin.ai') return;

  const tracker = target.usermaven;
  if (typeof tracker === 'function') {
    tracker('track', 'workspace_onboarding_use_cases_selected', properties);
    return;
  }
  if (typeof tracker?.track === 'function') {
    tracker.track('workspace_onboarding_use_cases_selected', properties);
  }
}
