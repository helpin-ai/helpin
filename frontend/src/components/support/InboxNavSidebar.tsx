import { Bot, Inbox, Mail, UserX, Users, Circle, Clock, Pause, CheckCircle2, List, ArrowUpRight } from 'lucide-react';
import { Separator } from '@/components/ui/separator';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import { cn } from '@/lib/utils';

function NavItem({ icon: Icon, label, active, onClick }: {
  icon: React.ElementType;
  label: string;
  active: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        'flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors',
        active ? 'bg-primary/10 text-primary font-medium' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
      )}
    >
      <Icon className="h-4 w-4 shrink-0" />
      <span className="truncate">{label}</span>
    </button>
  );
}

interface InboxNavSidebarProps {
  teams?: Array<{ id: string; name: string }>;
}

export function InboxNavSidebar({ teams }: InboxNavSidebarProps) {
  const { navFilter, setNavFilter, statusFilter, setStatusFilter } = useSupportInboxStore();

  return (
    <div className="flex w-[170px] flex-col border-r bg-muted/30">
      <div className="flex-1 space-y-1 p-2 overflow-y-auto">
        <h3 className="mb-2 px-2.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Inbox
        </h3>
        <NavItem icon={Inbox} label="My Inbox" active={navFilter === 'my_inbox'} onClick={() => setNavFilter('my_inbox')} />
        <NavItem icon={Mail} label="All Conversations" active={navFilter === 'all'} onClick={() => setNavFilter('all')} />
        <NavItem icon={UserX} label="Unassigned" active={navFilter === 'unassigned'} onClick={() => setNavFilter('unassigned')} />

        <Separator className="my-2" />
        <h3 className="mb-1 px-2.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Status
        </h3>
        <NavItem icon={Circle} label="Open" active={statusFilter === 'open'} onClick={() => setStatusFilter('open')} />
        <NavItem icon={Clock} label="In Progress" active={statusFilter === 'in_progress'} onClick={() => setStatusFilter('in_progress')} />
        <NavItem icon={Pause} label="Waiting" active={statusFilter === 'waiting'} onClick={() => setStatusFilter('waiting')} />
        <NavItem icon={CheckCircle2} label="Resolved" active={statusFilter === 'resolved'} onClick={() => setStatusFilter('resolved')} />
        <NavItem icon={List} label="All" active={statusFilter === 'all'} onClick={() => setStatusFilter('all')} />

        <Separator className="my-2" />
        <h3 className="mb-1 px-2.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
          Helpin AI Agent
        </h3>
        <NavItem icon={Bot} label="All AI" active={navFilter === 'ai_all'} onClick={() => setNavFilter('ai_all')} />
        <NavItem icon={CheckCircle2} label="Resolved" active={navFilter === 'ai_resolved'} onClick={() => setNavFilter('ai_resolved')} />
        <NavItem icon={ArrowUpRight} label="Escalated" active={navFilter === 'ai_escalated'} onClick={() => setNavFilter('ai_escalated')} />
        <NavItem icon={Clock} label="Pending" active={navFilter === 'ai_pending'} onClick={() => setNavFilter('ai_pending')} />

        {teams && teams.length > 0 && (
          <>
            <Separator className="my-2" />
            <h3 className="mb-1 px-2.5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Teams
            </h3>
            {teams.map((team) => (
              <button
                key={team.id}
                type="button"
                className="flex w-full items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              >
                <Users className="h-4 w-4 shrink-0" />
                <span className="truncate">{team.name}</span>
              </button>
            ))}
          </>
        )}
      </div>
    </div>
  );
}
