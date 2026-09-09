import type { IconComponent } from '@/lib/icons';

export type NavItem = {
  link: string;
  label: string;
  icon: IconComponent;
  badge?: number;
  children?: NavSubItem[];
  separatorBefore?: boolean;
};

export type NavSubItem = {
  link: string;
  label: string;
};

export type NavGroup = {
  label: string;
  items: NavItem[];
};

export type RailId = 'projects' | 'support' | 'crm' | 'automation' | 'docs' | 'settings' | 'setup';

export type RailItem = {
  id: RailId;
  label: string;
  icon: IconComponent;
  defaultLink: string;
  badge?: number;
  indicator?: boolean;
  progressPercent?: number;
  separatorBefore?: boolean;
};
