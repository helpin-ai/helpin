import { useEffect, useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useQuarterStore } from '@/stores/quarterStore';
import { useSessionStore } from '@/stores/sessionStore';
import { sprintsService } from '@/lib/services/sprintsService';
import type { Sprint } from '@/lib/types';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Calendar, Lock, Unlock } from 'lucide-react';
import { toast } from 'sonner';
import dayjs from 'dayjs';

const statusVariant = (status: Sprint['status']): 'default' | 'secondary' | 'outline' | 'destructive' => {
  switch (status) {
    case 'active': return 'default';
    case 'completed': return 'secondary';
    case 'locked': return 'destructive';
    default: return 'outline';
  }
};

export default function Sprints() {
  const navigate = useNavigate();
  const { currentWorkspace } = useWorkspaceStore();
  const { currentQuarter } = useQuarterStore();
  const { isAdmin } = useSessionStore();
  const [sprints, setSprints] = useState<Sprint[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    const q = useQuarterStore.getState().currentQuarter;
    if (!q?.id) {
      setSprints([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      const { data } = await sprintsService.list(q.id);
      if (data) setSprints(data.sort((a, b) => a.sprint_number - b.sprint_number));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentQuarter?.id]);

  const handleLock = async (sprintId: string) => {
    const { error } = await sprintsService.lock(sprintId);
    if (error) toast.error(error);
    else { toast.success('Sprint locked'); load(); }
  };

  const handleUnlock = async (sprintId: string) => {
    const { error } = await sprintsService.unlock(sprintId);
    if (error) toast.error(error);
    else { toast.success('Sprint unlocked'); load(); }
  };

  if (loading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-10 w-48" />
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[1, 2, 3, 4, 5, 6].map(i => <Skeleton key={i} className="h-40" />)}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">Sprints</h1>
        <p className="text-muted-foreground">{currentQuarter?.name} &middot; {sprints.length} sprints</p>
      </div>

      {sprints.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center">
            <p className="text-muted-foreground">No sprints found for this quarter.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {sprints.map(sprint => (
            <Card
              key={sprint.id}
              className="cursor-pointer hover:border-primary/50 transition-colors"
              onClick={() => navigate({ to: `/w/${currentWorkspace?.slug}/sprints/${sprint.id}` })}
            >
              <CardHeader className="pb-3">
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base flex items-center gap-2">
                    <Calendar className="h-4 w-4 text-muted-foreground" />
                    Sprint {sprint.sprint_number}
                  </CardTitle>
                  <Badge variant={statusVariant(sprint.status)} className="text-xs">
                    {sprint.status}
                  </Badge>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="text-sm text-muted-foreground">
                  {dayjs(sprint.start_date).format('MMM D')} &ndash; {dayjs(sprint.end_date).format('MMM D, YYYY')}
                </div>
                {sprint.locked_at && (
                  <p className="text-xs text-muted-foreground flex items-center gap-1">
                    <Lock className="h-3 w-3" />
                    Locked {dayjs(sprint.locked_at).format('MMM D')}
                  </p>
                )}
                {isAdmin() && (
                  <div className="flex gap-2 pt-1" onClick={e => e.stopPropagation()}>
                    {sprint.status !== 'locked' ? (
                      <Button size="sm" variant="outline" onClick={() => handleLock(sprint.id)}>
                        <Lock className="h-3 w-3 mr-1" />
                        Lock
                      </Button>
                    ) : (
                      <Button size="sm" variant="outline" onClick={() => handleUnlock(sprint.id)}>
                        <Unlock className="h-3 w-3 mr-1" />
                        Unlock
                      </Button>
                    )}
                  </div>
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
