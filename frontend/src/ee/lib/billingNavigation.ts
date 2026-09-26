export type BillingSettingsSearch = {
  choose_plan?: boolean | string;
};

export const BILLING_CHOOSE_PLAN_SEARCH = { choose_plan: true } as const;
export const BILLING_OVERVIEW_SEARCH = { choose_plan: undefined } as const;

export function shouldOpenBillingPlanChooser(search: BillingSettingsSearch | null | undefined): boolean {
  return search?.choose_plan === true || search?.choose_plan === 'true';
}
