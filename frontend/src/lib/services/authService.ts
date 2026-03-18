import { api, API_BASE } from '../api';
import type { AuthResponse, User } from '../types';

export const authService = {
  signup: (email: string, password: string, fullName: string) =>
    api.post<AuthResponse>('/auth/signup', { email, password, full_name: fullName }),
  signin: (email: string, password: string) =>
    api.post<AuthResponse>('/auth/signin', { email, password }),
  me: () => api.get<User>('/auth/me'),
  updateProfile: (data: { full_name?: string; avatar_url?: string; default_workspace_id?: string }) =>
    api.put<User>('/auth/me', data),
  refresh: (refreshToken: string) =>
    api.post<AuthResponse>('/auth/refresh', { refresh_token: refreshToken }),
  changePassword: (currentPassword: string, newPassword: string) =>
    api.put<{ message: string }>('/auth/change-password', { current_password: currentPassword, new_password: newPassword }),
  forgotPassword: (email: string) =>
    api.post<{ message: string }>('/auth/forgot-password', { email }),
  resetPassword: (token: string, password: string) =>
    api.post<{ message: string }>('/auth/reset-password', { token, password }),
  verifyEmail: (token: string) =>
    api.post<{ message: string }>('/auth/verify-email', { token }),
  uploadAvatar: async (file: File): Promise<{ data: User | null; error: string | null }> => {
    const token = localStorage.getItem('access_token');
    const formData = new FormData();
    formData.append('avatar', file);
    try {
      const res = await fetch(`${API_BASE}/auth/me/avatar`, {
        method: 'POST',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
        body: formData,
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        return { data: null, error: err.error || res.statusText };
      }
      const data = await res.json();
      return { data, error: null };
    } catch (e) {
      return { data: null, error: e instanceof Error ? e.message : 'Upload failed' };
    }
  },
  deleteAvatar: () => api.del<User>('/auth/me/avatar'),
};
