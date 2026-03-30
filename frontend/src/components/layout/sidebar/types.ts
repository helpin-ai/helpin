import type { LucideIcon } from 'lucide-react';

export type NavItem = {
  link: string;
  label: string;
  icon: LucideIcon;
};

export type NavGroup = {
  label: string;
  items: NavItem[];
};

export type RailId = 'projects' | 'support' | 'crm' | 'agents' | 'docs' | 'settings';

export type RailItem = {
  id: RailId;
  label: string;
  icon: LucideIcon;
  defaultLink: string;
  badge?: number;
  indicator?: boolean;
};
