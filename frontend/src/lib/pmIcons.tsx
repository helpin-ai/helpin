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
      <svg className={cn('text-amber-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
        <path d="M12 4L13.8 9.2L19 11L13.8 12.8L12 18L10.2 12.8L5 11L10.2 9.2L12 4Z" fill="currentColor" />
      </svg>
    );
  }
  if (taskType === 'bug') {
    return (
      <svg className={cn('text-red-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
        <ellipse cx="12" cy="13" rx="4.5" ry="5.5" stroke="currentColor" strokeWidth="2" />
        <path d="M12 7V4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        <path d="M9 5.5L12 7L15 5.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
        <path d="M7 11L4.5 9.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        <path d="M17 11L19.5 9.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        <path d="M7 15H4.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        <path d="M17 15H19.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      </svg>
    );
  }
  return (
    <svg className={cn('text-indigo-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
      <path d="M14.5 6.5L17.5 3.5L20.5 6.5L17 10L14.5 6.5Z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
      <path d="M13.5 7.5L6 15L4 20L9 18L16.5 10.5" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
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
        <svg className={cn('text-amber-500', className)} viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
          <circle cx="12" cy="12" r="5" fill="currentColor" />
        </svg>
      );
    default:
      return <CheckmarkCircle02Icon className={cn('text-green-500', className)} {...props} />;
  }
}
