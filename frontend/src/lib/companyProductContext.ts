export const companyProductContextGenerateHelper =
  'Uses your workspace website to draft a compact context profile. Review before saving.';

export function companyProductContextGenerateLabel(context: string | undefined | null): string {
  return context?.trim() ? 'Regenerate with AI' : 'Generate with AI';
}

export function companyProductContextGenerateDisabledReason(websiteUrl: string | undefined | null): string | null {
  return websiteUrl?.trim() ? null : 'Add a website URL in General settings to generate context.';
}

export function shouldConfirmCompanyProductContextReplacement(current: string, saved: string): boolean {
  const trimmedCurrent = current.trim();
  return trimmedCurrent !== '' && trimmedCurrent !== saved.trim();
}
