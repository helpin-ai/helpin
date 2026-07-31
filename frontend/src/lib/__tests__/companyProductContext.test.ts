import { describe, expect, it } from 'vitest';

import {
  companyProductContextGenerateDisabledReason,
  companyProductContextGenerateHelper,
  companyProductContextGenerateLabel,
  shouldConfirmCompanyProductContextReplacement,
} from '../companyProductContext';

describe('company product context presentation', () => {
  it('labels generation based on whether context already exists', () => {
    expect(companyProductContextGenerateLabel('')).toBe('Generate with AI');
    expect(companyProductContextGenerateLabel('Acme helps support teams.')).toBe('Regenerate with AI');
  });

  it('requires a website before AI generation', () => {
    expect(companyProductContextGenerateDisabledReason('')).toBe('Add a website URL in General settings to generate context.');
    expect(companyProductContextGenerateDisabledReason('https://acme.com')).toBeNull();
    expect(companyProductContextGenerateHelper).toContain('Review before saving');
  });

  it('asks before replacing unsaved editor content', () => {
    expect(shouldConfirmCompanyProductContextReplacement('Draft changes', 'Saved context')).toBe(true);
    expect(shouldConfirmCompanyProductContextReplacement('Saved context', 'Saved context')).toBe(false);
    expect(shouldConfirmCompanyProductContextReplacement('', 'Saved context')).toBe(false);
  });
});
