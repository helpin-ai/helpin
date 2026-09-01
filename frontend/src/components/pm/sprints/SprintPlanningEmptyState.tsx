import { ChartColumnIcon, Calendar03Icon, CheckmarkCircle02Icon, Timer01Icon } from '@/lib/icons';

export function SprintPlanningEmptyState() {
  return (
    <div className="flex flex-col items-center justify-center px-4 py-16">
      <div className="flex h-14 w-14 items-center justify-center rounded-full bg-emerald-500/10 mb-5">
        <Timer01Icon className="h-7 w-7 text-emerald-500" />
      </div>
      <h3 className="text-lg font-semibold mb-1.5">Create your first sprint</h3>
      <p className="text-sm text-muted-foreground text-center max-w-md mb-6">
        Sprints are time-boxed cycles that help your team plan, focus, and deliver work in a predictable rhythm.
      </p>
      <div className="mt-8 grid grid-cols-1 sm:grid-cols-3 gap-4 w-full max-w-4xl">
        {[
          { icon: Calendar03Icon, title: 'Set a cadence', desc: 'Define start and end dates for focused work cycles' },
          { icon: ChartColumnIcon, title: 'Track progress', desc: 'Monitor task and point completion in real time' },
          { icon: CheckmarkCircle02Icon, title: 'Ship consistently', desc: 'Build momentum with regular delivery milestones' },
        ].map((item) => (
          <div key={item.title} className="flex flex-col items-center text-center rounded-lg border border-border/50 bg-muted/30 p-6">
            <item.icon className="h-5 w-5 text-muted-foreground mb-3" />
            <p className="text-sm font-medium mb-1">{item.title}</p>
            <p className="text-sm text-muted-foreground leading-relaxed">{item.desc}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
