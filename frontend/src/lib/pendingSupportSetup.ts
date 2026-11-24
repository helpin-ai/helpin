import type { QuickstartAIMode, SupportQuickstartDetection } from './services/supportQuickstartService';

/**
 * What a visitor chose on /start before they had an account. It is kept in
 * the browser so setup can start right after signup, sign-in, or an email
 * confirmation in another tab.
 */
export type PendingSupportSetup = {
  detection: SupportQuickstartDetection;
  companyName: string;
  helpCenterUrl?: string;
  aiMode: QuickstartAIMode;
};

const STORAGE_KEY = 'helpin:pending-support-setup';
// An old choice is dropped rather than surprising someone weeks later.
const MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000;

export function savePendingSupportSetup(setup: PendingSupportSetup) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...setup, savedAt: Date.now() }));
  } catch {
    // Storage can be unavailable (private mode); the person can still set up from the form.
  }
}

export function readPendingSupportSetup(): PendingSupportSetup | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const value = JSON.parse(raw) as Partial<PendingSupportSetup> & { savedAt?: number };
    const fresh = typeof value.savedAt === 'number' && Date.now() - value.savedAt < MAX_AGE_MS;
    const valid = typeof value.detection?.website_url === 'string' && typeof value.companyName === 'string'
      && (value.aiMode === 'answer' || value.aiMode === 'draft');
    if (!fresh || !valid) {
      localStorage.removeItem(STORAGE_KEY);
      return null;
    }
    return { detection: value.detection!, companyName: value.companyName!, helpCenterUrl: value.helpCenterUrl, aiMode: value.aiMode! };
  } catch {
    return null;
  }
}

export function clearPendingSupportSetup() {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    // Nothing to clear when storage is unavailable.
  }
}
