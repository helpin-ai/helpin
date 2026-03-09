import { Badge } from '@/components/ui/badge';
import { formatDistanceToNow } from 'date-fns';
import { useSequenceEnrollments } from '@/hooks/queries/useCRM';
import type { CRMSequenceEnrollment, CRMEnrollmentStatus } from '@/lib/crmTypes';

const statusVariant: Record<CRMEnrollmentStatus, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  active: 'default',
  completed: 'secondary',
  paused: 'outline',
  bounced: 'destructive',
  unsubscribed: 'destructive',
  exited: 'secondary',
};

interface SequenceEnrollmentsProps {
  workspaceId: string;
  sequenceId: string;
}

export function SequenceEnrollments({ workspaceId, sequenceId }: SequenceEnrollmentsProps) {
  const { data } = useSequenceEnrollments(workspaceId, sequenceId);
  const enrollments = (data?.data ?? []) as CRMSequenceEnrollment[];

  if (enrollments.length === 0) {
    return (
      <div className="py-8 text-center text-sm text-muted-foreground">
        No enrollments yet. Enroll contacts to start the sequence.
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left">
            <th className="pb-2 font-medium">Contact ID</th>
            <th className="pb-2 font-medium">Step</th>
            <th className="pb-2 font-medium">Status</th>
            <th className="pb-2 font-medium">Enrolled</th>
          </tr>
        </thead>
        <tbody>
          {enrollments.map((enrollment) => (
            <tr key={enrollment.id} className="border-b">
              <td className="py-2 font-mono text-xs">{enrollment.contact_id.slice(0, 8)}...</td>
              <td className="py-2">{enrollment.current_step}</td>
              <td className="py-2">
                <Badge variant={statusVariant[enrollment.status] ?? 'outline'} className="text-xs">
                  {enrollment.status}
                </Badge>
              </td>
              <td className="py-2 text-xs text-muted-foreground">
                {formatDistanceToNow(new Date(enrollment.enrolled_at), { addSuffix: true })}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
