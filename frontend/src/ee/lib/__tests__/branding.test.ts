import { describe, expect, it } from 'vitest';
import { canRemoveHelpinBranding } from '../branding';
describe('SaaS branding policy', () => {
 it('requires an unlocked Growth workspace', () => {
  expect(canRemoveHelpinBranding({ plan:'growth', locked:false })).toBe(true);
  expect(canRemoveHelpinBranding({ plan:'starter', locked:false })).toBe(false);
  expect(canRemoveHelpinBranding({ plan:'founder', locked:false })).toBe(false);
  expect(canRemoveHelpinBranding({ plan:'growth', locked:true })).toBe(false);
  expect(canRemoveHelpinBranding(null)).toBe(false);
 });
});
