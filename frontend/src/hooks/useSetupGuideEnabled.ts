import { isSetupSuccessEnabled } from '@/lib/featureFlags';
import { useAuthStore } from '@/stores/authStore';

/** Whether the workspace Setup guide is available, as reported by the API. */
export function useSetupGuideEnabled(): boolean {
  return useAuthStore((state) => isSetupSuccessEnabled(state.configuration));
}
