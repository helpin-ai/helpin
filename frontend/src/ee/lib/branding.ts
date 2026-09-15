export function canRemoveHelpinBranding(
  billing: { plan?: string; locked?: boolean } | null | undefined,
): boolean {
  return billing?.plan === "growth" && !billing.locked;
}

export function brandingDescription(
  billing: { plan?: string; locked?: boolean } | null | undefined,
): string {
  return canRemoveHelpinBranding(billing)
    ? "Display branding in the widget footer."
    : billing?.plan === "founder"
      ? "Workspaces on the Founder plan keep Helpin branding visible."
      : "Upgrade to the Growth plan to hide Helpin branding.";
}
