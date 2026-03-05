import { useNavigate } from '@tanstack/react-router';
import type { Workspace } from '@/lib/types';
import { Card, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Building2 } from 'lucide-react';

interface WorkspaceSelectorProps {
  workspaces: Workspace[];
}

export function WorkspaceSelector({ workspaces }: WorkspaceSelectorProps) {
  const navigate = useNavigate();

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {workspaces.map(ws => (
        <Card
          key={ws.id}
          className="cursor-pointer hover:border-primary/50 transition-colors"
          onClick={() => navigate({ to: `/w/${ws.slug}/pm/stories` })}
        >
          <CardHeader>
            <div className="flex items-center gap-3">
              <div className="h-10 w-10 rounded-lg bg-primary/10 flex items-center justify-center">
                <Building2 className="h-5 w-5 text-primary" />
              </div>
              <div>
                <CardTitle className="text-base">{ws.name}</CardTitle>
                <CardDescription className="text-xs">{ws.slug}</CardDescription>
              </div>
            </div>
          </CardHeader>
        </Card>
      ))}
    </div>
  );
}
