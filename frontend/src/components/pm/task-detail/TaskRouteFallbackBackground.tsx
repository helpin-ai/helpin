import { TasksPage } from '@/pages/pm/Tasks';

interface TaskRouteFallbackBackgroundProps {
  teamId?: string;
}

export function TaskRouteFallbackBackground({
  teamId,
}: TaskRouteFallbackBackgroundProps) {
  return (
    <div className="h-full overflow-hidden pt-4 md:pt-6">
      <TasksPage teamId={teamId} />
    </div>
  );
}
