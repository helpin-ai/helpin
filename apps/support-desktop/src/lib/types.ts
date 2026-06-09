export interface User {
  id: string
  email: string
  full_name: string
  default_workspace_id?: string | null
}

export interface Workspace {
  id: string
  name: string
  slug: string
}

export interface AuthResponse {
  access_token: string
  refresh_token: string
  user: User
}

export interface UnreadStats {
  total: number
  mine: number
  mentions: number
}
