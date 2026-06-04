export interface User {
  id: string
  email: string
  full_name: string
  avatar_url?: string
  default_workspace_id?: string
  is_platform_admin?: boolean
  mfa_satisfied_in_token?: boolean
  created_at: string
  updated_at: string
}

export interface AuthResponse {
  user: User
  access_token: string
  refresh_token: string
}

export interface SigninResponse {
  user?: User
  access_token?: string
  refresh_token?: string
  requires_2fa?: boolean
  two_fa_token?: string
}

export interface PasskeyOptionsResponse {
  challenge: string
  options: Record<string, unknown>
}

export type PasskeyAuthenticationResponse = SigninResponse;

export interface Workspace {
  id: string
  name: string
  slug: string
  owner_id: string
  organization_id?: string
  description?: string
  logo_url?: string
  timezone: string
  created_at: string
  updated_at: string
}
