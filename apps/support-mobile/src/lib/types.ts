export interface User {
  id: string
  email: string
  full_name: string
  avatar_url?: string
  avatar_style?: string
  avatar_seed?: string
  avatar_background_mode?: string
  avatar_background_color?: string
  default_workspace_id?: string
  two_fa_enabled?: boolean
  mfa_satisfied_in_token?: boolean
  email_verified?: boolean
  email_verified_at?: string
  created_at: string
  updated_at: string
}

export interface AuthResponse {
  user: User
  access_token: string
  refresh_token: string
}

export interface WorkspaceAccess {
  workspace_id: string
  membership: {
    id: string
    user_id: string
    role: 'owner' | 'admin' | 'member' | 'viewer'
    status: string
  }
  permissions: string[]
  team_memberships: Array<{ team_id: string; role: string }>
  modules: string[]
}

export interface Workspace {
  id: string
  name: string
  slug: string
  logo_url?: string
  role?: string
}
