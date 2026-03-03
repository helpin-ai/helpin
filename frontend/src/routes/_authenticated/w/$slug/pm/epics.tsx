import { createFileRoute } from '@tanstack/react-router';
import { EpicsPage } from '@/pages/pm/Epics';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/epics')({
  component: EpicsPage,
});
