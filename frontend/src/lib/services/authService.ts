import { api } from '../api';
import type { AuthResponse, User } from '../types';

export const authService = {
  signup: (email: string, password: string, fullName: string) =>
    api.post<AuthResponse>('/auth/signup', { email, password, full_name: fullName }),
  signin: (email: string, password: string) =>
    api.post<AuthResponse>('/auth/signin', { email, password }),
  me: () => api.get<User>('/auth/me'),
  updateProfile: (data: { full_name?: string; avatar_url?: string }) =>
    api.patch<User>('/auth/me', data),
  refresh: (refreshToken: string) =>
    api.post<AuthResponse>('/auth/refresh', { refresh_token: refreshToken }),
};
