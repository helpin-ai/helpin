import { api } from '../api';
import type { TestEmailResult } from '../capabilityTypes';
import type { AppEmailSettings, AppEmailSettingsUpdate, ServerAdmin, SignupMode, SignupPolicy } from '../instanceTypes';

/** Self-hosted server administration (Community, server admins only). */
export const instanceService = {
  signupPolicy: () => api.get<SignupPolicy>('/instance/signup-policy'),
  updateSignupPolicy: (data: { mode: SignupMode; allowed_domains: string[] }) =>
    api.put<SignupPolicy>('/instance/signup-policy', data),
  admins: () => api.get<ServerAdmin[]>('/instance/admins'),
  grantAdmin: (email: string) => api.post<ServerAdmin[]>('/instance/admins', { email }),
  revokeAdmin: (userId: string) => api.del<ServerAdmin[]>(`/instance/admins/${encodeURIComponent(userId)}`),
  emailSettings: () => api.get<AppEmailSettings>('/instance/email'),
  updateEmailSettings: (data: AppEmailSettingsUpdate) => api.put<AppEmailSettings>('/instance/email', data),
  clearEmailSettings: () => api.del<AppEmailSettings>('/instance/email'),
  sendTestEmail: () => api.post<TestEmailResult>('/instance/email/test'),
};
