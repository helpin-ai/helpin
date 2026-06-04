import type { SVGProps } from 'react';
import { cn } from '@/lib/utils';
import type { Priority, Severity, StateType, TaskType } from '@/lib/pmTypes';

type PMIconProps = SVGProps<SVGSVGElement>;

export function ArrowUpDownIcon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M7 15L12 20L17 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M7 9L12 4L17 9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function ChevronUpIcon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M18 15L12 9L6 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function ChevronRightIcon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M9 18L15 12L9 6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function ChevronDownIcon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M6 9L12 15L18 9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function MoreVerticalIcon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <circle cx="12" cy="12" r="1" fill="currentColor" />
      <circle cx="12" cy="5" r="1" fill="currentColor" />
      <circle cx="12" cy="19" r="1" fill="currentColor" />
    </svg>
  );
}

export function Link01Icon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M10 13C11.8369 14.8369 14.7038 15.0782 16.8321 13.7237C17.0852 13.5626 17.3224 13.3716 17.54 13.154L20.54 10.154C22.4926 8.20138 22.4926 5.03553 20.54 3.08289C18.5874 1.13025 15.4216 1.13025 13.4689 3.08289L11.75 4.8" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M14 11C12.1631 9.16314 9.29622 8.92176 7.16795 10.2763C6.91484 10.4374 6.67759 10.6284 6.46 10.846L3.46 13.846C1.50736 15.7986 1.50736 18.9645 3.46 20.9171C5.41265 22.8698 8.57849 22.8698 10.5311 20.9171L12.24 19.21" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function LinkSquare01Icon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M21 13V19C21 20.1046 20.1046 21 19 21H5C3.89543 21 3 20.1046 3 19V5C3 3.89543 3.89543 3 5 3H11" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M21 3L12 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M15 3H21V9" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function ArchiveIcon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <rect x="2" y="3" width="20" height="5" rx="1" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M4 8V19C4 20.1046 4.89543 21 6 21H18C19.1046 21 20 20.1046 20 19V8" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M10 12H14" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function Tick01Icon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M5 12.5L9.5 17L19 7.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function CheckmarkCircle02Icon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <circle cx="12" cy="12" r="8.5" stroke="currentColor" strokeWidth="2" />
      <path d="M8 12.5L10.75 15.25L16 10" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function Calendar03Icon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <rect x="3" y="5" width="18" height="16" rx="2.5" stroke="currentColor" strokeWidth="2" />
      <path d="M8 3V7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M16 3V7" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M3 10H21" stroke="currentColor" strokeWidth="2" />
    </svg>
  );
}

export function StickyNote01Icon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M7 3.5H14.5L18.5 7.5V18.5C18.5 19.6046 17.6046 20.5 16.5 20.5H7C5.89543 20.5 5 19.6046 5 18.5V5.5C5 4.39543 5.89543 3.5 7 3.5Z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
      <path d="M14 3.5V8H18.5" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
    </svg>
  );
}

export function ChartColumnIcon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M4 19.5H20" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <rect x="6" y="11" width="3" height="6.5" rx="1" fill="currentColor" />
      <rect x="10.5" y="8" width="3" height="9.5" rx="1" fill="currentColor" />
      <rect x="15" y="5" width="3" height="12.5" rx="1" fill="currentColor" />
    </svg>
  );
}

export function UserAdd01Icon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <circle cx="10" cy="8" r="3" stroke="currentColor" strokeWidth="2" />
      <path d="M4.5 18C5.2 14.9 7.2 13.5 10 13.5C12.8 13.5 14.8 14.9 15.5 18" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M18 7V13" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M15 10H21" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

export function ArrowReloadHorizontalIcon({ className, ...props }: PMIconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M7 7H17L14.5 4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M17 17H7L9.5 19.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M17 7C19.2091 7 21 8.79086 21 11" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      <path d="M7 17C4.79086 17 3 15.2091 3 13" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    </svg>
  );
}

export function TaskTypeIcon({
  taskType,
  className,
  ...props
}: {
  taskType: TaskType;
} & PMIconProps) {
  if (taskType === 'feature') {
    return (
      <svg className={cn('text-[#ED8B38]', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
        <g transform="translate(12 12) scale(1.08) translate(-12 -12)">
          <path d="M12 4L14 10L20 12L14 14L12 20L10 14L4 12L10 10Z" fill="currentColor" />
          <path d="M20 3L20.5 4.5L22 5L20.5 5.5L20 7L19.5 5.5L18 5L19.5 4.5Z" fill="currentColor" />
          <path d="M4 17L4.5 18.5L6 19L4.5 19.5L4 21L3.5 19.5L2 19L3.5 18.5Z" fill="currentColor" />
        </g>
      </svg>
    );
  }
  if (taskType === 'bug') {
    return (
      <svg className={cn('text-[#E24A3B]', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
        <g transform="translate(12 12.5) scale(1.18) translate(-12 -12.5)">
          <ellipse cx="12" cy="14.5" rx="4.25" ry="4.5" fill="currentColor" />
          <path d="M10.75 10.5Q9.25 8.5 8 6.5" fill="none" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
          <path d="M13.25 10.5Q14.75 8.5 16 6.5" fill="none" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
          <circle cx="8" cy="6.5" r="0.9" fill="currentColor" />
          <circle cx="16" cy="6.5" r="0.9" fill="currentColor" />
          <line x1="8" y1="12.25" x2="5.5" y2="11.25" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
          <line x1="7.75" y1="14.5" x2="5" y2="14.5" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
          <line x1="8" y1="16.75" x2="5.5" y2="17.75" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
          <line x1="16" y1="12.25" x2="18.5" y2="11.25" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
          <line x1="16.25" y1="14.5" x2="19" y2="14.5" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
          <line x1="16" y1="16.75" x2="18.5" y2="17.75" stroke="currentColor" strokeWidth="1.1" strokeLinecap="round" />
        </g>
      </svg>
    );
  }
  return (
    <svg className={cn('text-[#3B82F6]', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <g transform="translate(12 12) scale(1.12) rotate(-40)">
        <path d="M1.8-2L1.8 9A1.8 1.8 0 0 1-1.8 9L-1.8-2Q-2.5-3-3.5-4L-3.5-10L-2.5-10L-1.25-7L1.25-7L2.5-10L3.5-10L3.5-4Q2.5-3 1.8-2Z" fill="currentColor" />
      </g>
    </svg>
  );
}

export function TaskTypeTileIcon({
  taskType,
  className,
  ...props
}: {
  taskType: TaskType;
} & PMIconProps) {
  if (taskType === 'feature') {
    return (
      <svg className={className} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
        <rect width="48" height="48" rx="11" fill="#ED8B38" />
        <path d="M24 8L28 20L40 24L28 28L24 40L20 28L8 24L20 20Z" fill="#F7F5F2" />
        <path d="M40 6L41 9L44 10L41 11L40 14L39 11L36 10L39 9Z" fill="#F7F5F2" />
        <path d="M8 34L9 37L12 38L9 39L8 42L7 39L4 38L7 37Z" fill="#F7F5F2" />
      </svg>
    );
  }
  if (taskType === 'bug') {
    return (
      <svg className={className} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
        <rect width="48" height="48" rx="11" fill="#E24A3B" />
        <ellipse cx="24" cy="29" rx="8.5" ry="9" fill="#F7F5F2" />
        <path d="M21.5 21Q18.5 17 16 13" fill="none" stroke="#F7F5F2" strokeWidth="1.5" strokeLinecap="round" />
        <path d="M26.5 21Q29.5 17 32 13" fill="none" stroke="#F7F5F2" strokeWidth="1.5" strokeLinecap="round" />
        <circle cx="16" cy="13" r="1.25" fill="#F7F5F2" />
        <circle cx="32" cy="13" r="1.25" fill="#F7F5F2" />
        <line x1="24" y1="21" x2="24" y2="37" stroke="#E24A3B" strokeWidth="1.1" strokeLinecap="round" />
        <circle cx="20.5" cy="25" r="1" fill="#E24A3B" />
        <circle cx="27.5" cy="25" r="1" fill="#E24A3B" />
        <circle cx="20.5" cy="32" r="1" fill="#E24A3B" />
        <circle cx="27.5" cy="32" r="1" fill="#E24A3B" />
        <line x1="16" y1="24" x2="11" y2="22" stroke="#F7F5F2" strokeWidth="1.5" strokeLinecap="round" />
        <line x1="15.5" y1="29" x2="10" y2="29" stroke="#F7F5F2" strokeWidth="1.5" strokeLinecap="round" />
        <line x1="16" y1="34" x2="11" y2="36" stroke="#F7F5F2" strokeWidth="1.5" strokeLinecap="round" />
        <line x1="32" y1="24" x2="37" y2="22" stroke="#F7F5F2" strokeWidth="1.5" strokeLinecap="round" />
        <line x1="32.5" y1="29" x2="38" y2="29" stroke="#F7F5F2" strokeWidth="1.5" strokeLinecap="round" />
        <line x1="32" y1="34" x2="37" y2="36" stroke="#F7F5F2" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    );
  }
  return (
    <svg className={className} viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <rect width="48" height="48" rx="11" fill="#3B82F6" />
      <g transform="translate(24 24) rotate(-40)">
        <path fill="#F7F5F2" d="M3.6-4L3.6 18A3.6 3.6 0 0 1-3.6 18L-3.6-4Q-5-6-7-8L-7-20L-5-20L-2.5-14L2.5-14L5-20L7-20L7-8Q5-6 3.6-4Z" />
      </g>
    </svg>
  );
}

export function PriorityIcon({
  priority,
  className,
  ...props
}: {
  priority: Priority;
} & PMIconProps) {
  switch (priority) {
    case 'urgent':
      return (
        <svg className={cn('text-red-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <path d="M8 16L12 11L16 16" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
          <path d="M8 11L12 6L16 11" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case 'high':
      return (
        <svg className={cn('text-orange-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <rect x="5" y="13" width="3" height="6" rx="1" fill="currentColor" />
          <rect x="10.5" y="10" width="3" height="9" rx="1" fill="currentColor" />
          <rect x="16" y="7" width="3" height="12" rx="1" fill="currentColor" />
        </svg>
      );
    case 'medium':
      return (
        <svg className={cn('text-amber-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <rect x="6.5" y="11" width="4" height="8" rx="1" fill="currentColor" />
          <rect x="13.5" y="8" width="4" height="11" rx="1" fill="currentColor" />
        </svg>
      );
    case 'low':
      return (
        <svg className={cn('text-sky-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <rect x="10" y="9" width="4" height="10" rx="1" fill="currentColor" />
        </svg>
      );
    default:
      return (
        <svg className={cn('text-zinc-400', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <circle cx="12" cy="12" r="7.5" stroke="currentColor" strokeWidth="2" />
          <path d="M9 9L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          <path d="M15 9L9 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
      );
  }
}

export function SeverityIcon({
  severity,
  className,
  ...props
}: {
  severity: Severity;
} & PMIconProps) {
  switch (severity) {
    case 'critical':
      return (
        <svg className={cn('text-red-600', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <path d="M9 3.5H15L20.5 9V15L15 20.5H9L3.5 15V9L9 3.5Z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
        </svg>
      );
    case 'major':
      return (
        <svg className={cn('text-orange-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <path d="M12 3.5L18 6V11.5C18 15.2 15.8 18.3 12 20.5C8.2 18.3 6 15.2 6 11.5V6L12 3.5Z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
        </svg>
      );
    case 'minor':
      return (
        <svg className={cn('text-amber-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <path d="M12 4.5L20 18.5H4L12 4.5Z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
          <path d="M12 9V13" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          <circle cx="12" cy="16" r="1" fill="currentColor" />
        </svg>
      );
    default:
      return (
        <svg className={cn('text-zinc-400', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <circle cx="12" cy="12" r="7.5" stroke="currentColor" strokeWidth="2" />
          <path d="M9 9L15 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          <path d="M15 9L9 15" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
      );
  }
}

export function StateTypeIcon({
  stateType,
  className,
  ...props
}: {
  stateType: StateType;
} & PMIconProps) {
  switch (stateType) {
    case 'backlog':
      return (
        <svg className={cn('text-zinc-400', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <circle cx="12" cy="12" r="7.5" stroke="currentColor" strokeWidth="2" strokeDasharray="3 3" />
        </svg>
      );
    case 'unstarted':
      return (
        <svg className={cn('text-zinc-400', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <path d="M7 12H17" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" />
        </svg>
      );
    case 'started':
      return (
        <svg className={cn('text-zinc-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <circle cx="12" cy="12" r="7" stroke="currentColor" strokeWidth="2" />
          <path d="M12 5A7 7 0 0 1 12 19Z" fill="currentColor" />
        </svg>
      );
    default:
      return <CheckmarkCircle02Icon className={cn('text-green-500', className)} {...props} />;
  }
}
