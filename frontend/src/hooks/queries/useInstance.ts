import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { instanceService } from '@/lib/services/instanceService';
import type { AppEmailSettingsUpdate, SignupMode } from '@/lib/instanceTypes';
import type { TestEmailResult } from '@/lib/capabilityTypes';
import { useAuthStore } from '@/stores/authStore';

/** Whether the signed-in account administers this self-hosted server. */
export function useIsServerAdmin() {
  return useAuthStore((state) => Boolean(state.user?.is_server_admin));
}

export function useSignupPolicy(enabled = true) {
  return useQuery({
    queryKey: queryKeys.instance.signupPolicy,
    queryFn: async () => unwrap(await instanceService.signupPolicy()),
    enabled,
  });
}

export function useUpdateSignupPolicy() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (data: { mode: SignupMode; allowed_domains: string[] }) => unwrap(await instanceService.updateSignupPolicy(data)),
    onSuccess: (policy) => {
      queryClient.setQueryData(queryKeys.instance.signupPolicy, policy);
      // The sign-in pages read the policy from the public auth configuration.
      useAuthStore.setState((state) => state.configuration
        ? { configuration: { ...state.configuration, signup_mode: policy.mode, signup_allowed_domains: policy.mode === 'domains' ? policy.allowed_domains : [] } }
        : {});
    },
  });
}

export function useServerAdmins(enabled = true) {
  return useQuery({
    queryKey: queryKeys.instance.admins,
    queryFn: async () => unwrap(await instanceService.admins()),
    enabled,
  });
}

export function useGrantServerAdmin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (email: string) => unwrap(await instanceService.grantAdmin(email)),
    onSuccess: (admins) => queryClient.setQueryData(queryKeys.instance.admins, admins),
  });
}

export function useRevokeServerAdmin() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (userId: string) => unwrap(await instanceService.revokeAdmin(userId)),
    onSuccess: (admins, userId) => {
      queryClient.setQueryData(queryKeys.instance.admins, admins);
      // Removing yourself ends access to these pages.
      const self = useAuthStore.getState().user;
      if (self?.id === userId) useAuthStore.setState({ user: { ...self, is_server_admin: false } });
    },
  });
}

export function useAppEmailSettings(enabled = true) {
  return useQuery({
    queryKey: queryKeys.instance.email,
    queryFn: async () => unwrap(await instanceService.emailSettings()),
    enabled,
  });
}

/** Keeps the email capability row and the auth configuration in step with saved settings. */
function useEmailSettingsSaved() {
  const queryClient = useQueryClient();
  return (configured: boolean) => {
    void queryClient.invalidateQueries({ queryKey: ['workspaces'], predicate: (query) => query.queryKey[2] === 'capabilities' });
    useAuthStore.setState((state) => state.configuration
      ? { configuration: { ...state.configuration, app_email_configured: configured } }
      : {});
  };
}

export function useSaveAppEmailSettings() {
  const queryClient = useQueryClient();
  const saved = useEmailSettingsSaved();
  return useMutation({
    mutationFn: async (data: AppEmailSettingsUpdate) => unwrap(await instanceService.updateEmailSettings(data)),
    onSuccess: (settings) => {
      queryClient.setQueryData(queryKeys.instance.email, settings);
      void queryClient.invalidateQueries({ queryKey: queryKeys.instance.signupPolicy });
      saved(settings.source !== 'none' && !settings.problem);
    },
  });
}

export function useClearAppEmailSettings() {
  const queryClient = useQueryClient();
  const saved = useEmailSettingsSaved();
  return useMutation({
    mutationFn: async () => unwrap(await instanceService.clearEmailSettings()),
    onSuccess: (settings) => {
      queryClient.setQueryData(queryKeys.instance.email, settings);
      void queryClient.invalidateQueries({ queryKey: queryKeys.instance.signupPolicy });
      saved(false);
    },
  });
}

/** Sends a test email to the signed-in admin; rate limits resolve with `ok: false`. */
export function useSendServerTestEmail() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (): Promise<TestEmailResult> => {
      const response = await instanceService.sendTestEmail();
      if (response.status === 429) {
        return { ok: false, rate_limited: true, error: response.error ?? 'Too many test emails. Wait a minute and try again.' };
      }
      if (response.status === 422) {
        return { ok: false, error: response.error ?? 'Your account has no email address.' };
      }
      return unwrap(response);
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ['workspaces'], predicate: (query) => query.queryKey[2] === 'capabilities' });
    },
  });
}
