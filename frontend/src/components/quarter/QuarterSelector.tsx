import { useRewardQuarterStore } from '@/stores/quarterStore';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';

export function QuarterSelector() {
  const { quarters, currentQuarter, setCurrentQuarter } = useRewardQuarterStore();

  if (quarters.length === 0) return null;

  const statusColor = (status: string) => {
    switch (status) {
      case 'active': return 'default';
      case 'completed': return 'secondary';
      case 'draft': return 'outline';
      default: return 'secondary';
    }
  };

  return (
    <Select
      value={currentQuarter?.id || ''}
      onValueChange={(val) => {
        const q = quarters.find(q => q.id === val);
        if (q) setCurrentQuarter(q);
      }}
    >
      <SelectTrigger className="w-[180px]">
        <SelectValue placeholder="Select quarter" />
      </SelectTrigger>
      <SelectContent>
        {quarters.map(q => (
          <SelectItem key={q.id} value={q.id}>
            <div className="flex items-center gap-2">
              {q.name}
              <Badge variant={statusColor(q.status)} className="text-xs">{q.status}</Badge>
            </div>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
