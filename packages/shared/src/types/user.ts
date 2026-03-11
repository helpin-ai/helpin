export interface User {
  id: string;
  email: string;
  name: string;
  avatarUrl?: string;
  isOnline: boolean;
  lastSeenAt: string;
  createdAt: string;
}
