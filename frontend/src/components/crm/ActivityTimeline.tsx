import { format } from 'date-fns';
import { Mail, MessageSquare, Phone, Calendar, CheckSquare } from 'lucide-react';
import type { CRMActivity, CRMActivityType } from '@/lib/crmTypes';

interface ActivityTimelineProps {
  activities: CRMActivity[];
}

const activityIcons: Record<CRMActivityType, typeof Mail> = {
  note: MessageSquare,
  call: Phone,
  meeting: Calendar,
  email: Mail,
  task: CheckSquare,
};

export function ActivityTimeline({ activities }: ActivityTimelineProps) {
  if (activities.length === 0) {
    return <p className="text-sm text-muted-foreground">No activities yet</p>;
  }

  return (
    <div className="space-y-4">
      {activities.map((activity) => {
        const Icon = activityIcons[activity.activity_type] ?? MessageSquare;
        return (
          <div key={activity.id} className="flex gap-3">
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-muted">
              <Icon className="h-4 w-4 text-muted-foreground" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium capitalize">{activity.activity_type}</span>
                <span className="text-xs text-muted-foreground">
                  {format(new Date(activity.occurred_at), 'MMM d, yyyy h:mm a')}
                </span>
              </div>
              {activity.subject && (
                <p className="mt-0.5 text-sm">{activity.subject}</p>
              )}
              {activity.body && (
                <p className="mt-1 text-sm text-muted-foreground line-clamp-2">{activity.body}</p>
              )}
            </div>
          </div>
        );
      })}
    </div>
  );
}
