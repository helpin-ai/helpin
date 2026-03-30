import { BarChart3, CalendarDays, CheckCircle2, Plus, Timer } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface SprintPlanningEmptyStateProps {
  canEdit: boolean;
  onCreateSprint: () => void;
}

export function SprintPlanningEmptyState({ canEdit, onCreateSprint }: SprintPlanningEmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center px-4 py-16">
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500/10 mb-5">
        <Timer className="h-7 w-7 text-emerald-500" />
      </div>
      <h3 className="text-lg font-semibold mb-1.5">Create your first sprint</h3>
      <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
        Sprints are time-boxed cycles that help your team plan, focus, and deliver work in a predictable rhythm.
      </p>
      {canEdit && (
        <Button className="gap-2 mb-8" onClick={onCreateSprint}>
          <Plus className="h-4 w-4" />
          Create Sprint
        </Button>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-4xl">
        {[
          { icon: CalendarDays, title: 'Set a cadence', desc: 'Define start and end dates for focused work cycles' },
          { icon: BarChart3, title: 'Track progress', desc: 'Monitor story and point completion in real time' },
          { icon: CheckCircle2, title: 'Ship consistently', desc: 'Build momentum with regular delivery milestones' },
        ].map((item) => (
          <div key={item.title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
            <item.icon className="h-5 w-5 text-muted-foreground mb-3" />
            <p className="text-sm font-medium mb-1">{item.title}</p>
            <p className="text-[13px] text-muted-foreground leading-relaxed">{item.desc}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
