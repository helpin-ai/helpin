import { TasksPage } from '@/pages/pm/Tasks';

interface TaskRouteFallbackBackgroundProps {
  teamId?: string;
}

export function TaskRouteFallbackBackground({
  teamId,
}: TaskRouteFallbackBackgroundProps) {
  return (
    <div className="h-full overflow-hidden">
      <TasksPage teamId={teamId} />
    </div>
  );
}
