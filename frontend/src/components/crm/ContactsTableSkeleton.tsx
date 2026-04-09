import { Skeleton } from '@/components/ui/skeleton';
import {
  TABLE_CONTAINER,
  TABLE_HEADER,
  TABLE_HEADER_CELL,
  TABLE_ROW,
  TABLE_CELL,
  ROW_HEIGHT,
  CHECKBOX_COL_SIZE,
} from '@/lib/tableStyles';

const SKELETON_ROWS = 8;

const COLUMNS = [
  { id: 'select', width: CHECKBOX_COL_SIZE, type: 'checkbox' as const },
  { id: 'name', width: 260, type: 'avatar-text' as const },
  { id: 'email', width: 200, type: 'text' as const },
  { id: 'stage', width: 160, type: 'badge' as const },
  { id: 'status', width: 140, type: 'badge' as const },
  { id: 'owner', width: 180, type: 'avatar-text' as const },
  { id: 'created', width: 160, type: 'text' as const },
  { id: 'actions', width: 44, type: 'icon' as const },
];

function SkeletonCell({ type }: { type: (typeof COLUMNS)[number]['type'] }) {
  switch (type) {
    case 'checkbox':
      return <Skeleton className="h-4 w-4 rounded-sm" />;
    case 'avatar-text':
      return (
        <div className="flex items-center gap-2">
          <Skeleton className="h-6 w-6 shrink-0 rounded-full" />
          <Skeleton className="h-3 w-24 rounded" />
        </div>
      );
    case 'text':
      return <Skeleton className="h-3 w-28 rounded" />;
    case 'badge':
      return <Skeleton className="h-5 w-20 rounded-full" />;
    case 'icon':
      return <Skeleton className="h-4 w-4 rounded" />;
  }
}

export function ContactsTableSkeleton() {
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2">
      {/* Toolbar skeleton */}
      <div className="flex items-center gap-2 px-1">
        <Skeleton className="h-3 w-16 rounded" />
        <Skeleton className="h-7 w-[160px] rounded-md" />
        <Skeleton className="h-7 w-[90px] rounded-md" />
      </div>

      {/* Table skeleton */}
      <div className={TABLE_CONTAINER}>
        <div className="min-w-fit">
          {/* Header */}
          <div className={TABLE_HEADER}>
            <div className="flex items-center">
              {COLUMNS.map((col) => (
                <div
                  key={col.id}
                  className={TABLE_HEADER_CELL}
                  style={{ width: col.width }}
                >
                  <Skeleton className="h-3 w-12 rounded" />
                </div>
              ))}
            </div>
          </div>

          {/* Rows */}
          {Array.from({ length: SKELETON_ROWS }, (_, rowIdx) => (
            <div
              key={rowIdx}
              className={TABLE_ROW}
              style={{ height: ROW_HEIGHT }}
            >
              {COLUMNS.map((col) => (
                <div
                  key={col.id}
                  className={TABLE_CELL}
                  style={{ width: col.width }}
                >
                  <SkeletonCell type={col.type} />
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
