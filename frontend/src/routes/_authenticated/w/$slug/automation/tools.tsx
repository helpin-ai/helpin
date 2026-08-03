import { createFileRoute } from '@tanstack/react-router';
import { AutomationToolsLayout } from '@/pages/automation/AutomationToolsLayout';

export const Route = createFileRoute('/_authenticated/w/$slug/automation/tools')({
  component: AutomationToolsLayout,
});
