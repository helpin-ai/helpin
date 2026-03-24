export interface User {
  id: string
  email: string
  full_name: string
  avatar_url?: string
  default_workspace_id?: string
  created_at: string
  updated_at: string
}

export interface AuthResponse {
  user: User
  access_token: string
  refresh_token: string
}

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
