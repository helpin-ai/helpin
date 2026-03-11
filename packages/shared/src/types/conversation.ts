export interface Conversation {
  id: string;
  workspaceId: string;
  title?: string;
  status: 'open' | 'pending' | 'resolved' | 'closed';
  priority: 'low' | 'medium' | 'high' | 'urgent';
  assignedTo?: string;
  customerName?: string;
  customerEmail?: string;
  createdAt: string;
  updatedAt: string;
  lastMessageAt: string;
}
