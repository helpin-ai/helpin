import { FunctionComponent } from 'preact';

interface IconProps {
  size?: number;
  color?: string;
  strokeWidth?: number;
  class?: string;
}

// Helper to create consistent icon components from lucide SVG paths.
// All icons use stroke-based rendering (lucide style): 24x24 viewBox,
// stroke="currentColor", fill="none", strokeLinecap/Join="round".
function createIcon(paths: string[], displayName: string): FunctionComponent<IconProps> {
  const Icon: FunctionComponent<IconProps> = ({
    size = 24,
    color = 'currentColor',
    strokeWidth = 2,
    class: className,
  }) => (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke={color}
      stroke-width={strokeWidth}
      stroke-linecap="round"
      stroke-linejoin="round"
      class={className}
    >
      {paths.map((d, i) => (
        <path key={i} d={d} />
      ))}
    </svg>
  );
  Icon.displayName = displayName;
  return Icon;
}

// ── Navigation ────────────────────────────────────────────────
export const HomeIcon = createIcon(
  ['M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8', 'M3 10a2 2 0 0 1 .709-1.528l7-5.999a2 2 0 0 1 2.582 0l7 5.999A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z'],
  'HomeIcon',
);

export const MessageSquareIcon = createIcon(
  ['M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z'],
  'MessageSquareIcon',
);

export const CircleHelpIcon = createIcon(
  ['M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z', 'M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3', 'M12 17h.01'],
  'CircleHelpIcon',
);

export const HelpCircleIcon = CircleHelpIcon;

// ── Actions ───────────────────────────────────────────────────
export const SendIcon = createIcon(
  ['m22 2-7 20-4-9-9-4z', 'M22 2 11 13'],
  'SendIcon',
);

export const PaperclipIcon = createIcon(
  ['m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48'],
  'PaperclipIcon',
);

export const SmileIcon = createIcon(
  ['M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2 2 6.477 2 12s4.477 10 10 10z', 'M8 14s1.5 2 4 2 4-2 4-2', 'M9 9h.01', 'M15 9h.01'],
  'SmileIcon',
);

// ── Chrome ────────────────────────────────────────────────────
export const XIcon = createIcon(
  ['M18 6 6 18', 'M6 6l12 12'],
  'XIcon',
);

export const ChevronDownIcon = createIcon(
  ['m6 9 6 6 6-6'],
  'ChevronDownIcon',
);

export const ChevronLeftIcon = createIcon(
  ['m15 18-6-6 6-6'],
  'ChevronLeftIcon',
);

export const ChevronRightIcon = createIcon(
  ['m9 18 6-6-6-6'],
  'ChevronRightIcon',
);

export const MoreVerticalIcon: FunctionComponent<IconProps> = ({
  size = 24,
  color = 'currentColor',
  strokeWidth = 2,
  class: className,
}) => (
  <svg
    xmlns="http://www.w3.org/2000/svg"
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    stroke={color}
    stroke-width={strokeWidth}
    stroke-linecap="round"
    stroke-linejoin="round"
    class={className}
  >
    <circle cx="12" cy="12" r="1" />
    <circle cx="12" cy="5" r="1" />
    <circle cx="12" cy="19" r="1" />
  </svg>
);
MoreVerticalIcon.displayName = 'MoreVerticalIcon';

export const ExternalLinkIcon = createIcon(
  ['M15 3h6v6', 'M10 14 21 3', 'M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6'],
  'ExternalLinkIcon',
);

// ── Launcher icons ────────────────────────────────────────────
export const MessageCircleIcon = createIcon(
  ['M7.9 20A9 9 0 1 0 4 16.1L2 22z'],
  'MessageCircleIcon',
);

export const LifeBuoyIcon = createIcon(
  [
    'M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20z',
    'M12 16a4 4 0 1 0 0-8 4 4 0 0 0 0 8z',
    'M4.93 4.93 9.17 9.17',
    'M14.83 14.83 19.07 19.07',
    'M14.83 9.17 19.07 4.93',
    'M4.93 19.07 9.17 14.83',
  ],
  'LifeBuoyIcon',
);

// ── Misc ──────────────────────────────────────────────────────
export const SearchIcon = createIcon(
  ['M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16z', 'm21 21-4.3-4.3'],
  'SearchIcon',
);

export const ArrowRightIcon = createIcon(
  ['M5 12h14', 'm12 5 7 7-7 7'],
  'ArrowRightIcon',
);

export const StarIcon = createIcon(
  ['M11.525 2.295a.53.53 0 0 1 .95 0l2.31 4.679a.53.53 0 0 0 .4.29l5.16.756a.53.53 0 0 1 .294.904l-3.733 3.638a.53.53 0 0 0-.152.469l.882 5.14a.53.53 0 0 1-.77.56l-4.614-2.426a.53.53 0 0 0-.494 0L6.18 18.73a.53.53 0 0 1-.77-.56l.882-5.14a.53.53 0 0 0-.152-.469L2.407 8.924a.53.53 0 0 1 .294-.906l5.16-.754a.53.53 0 0 0 .4-.29z'],
  'StarIcon',
);

export const ThumbsUpIcon = createIcon(
  ['M7 10v12', 'M15 5.88 14 10h5.83a2 2 0 0 1 1.92 2.56l-2.33 8A2 2 0 0 1 17.5 22H4a2 2 0 0 1-2-2v-8a2 2 0 0 1 2-2h2.76a2 2 0 0 0 1.79-1.11L12 2a3.13 3.13 0 0 1 3 3.88z'],
  'ThumbsUpIcon',
);

export const ThumbsDownIcon = createIcon(
  ['M17 14V2', 'M9 18.12 10 14H4.17a2 2 0 0 1-1.92-2.56l2.33-8A2 2 0 0 1 6.5 2H20a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2h-2.76a2 2 0 0 0-1.79 1.11L12 22a3.13 3.13 0 0 1-3-3.88z'],
  'ThumbsDownIcon',
);

export const MailIcon = createIcon(
  ['M22 6a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2z', 'm22 6-10 7L2 6'],
  'MailIcon',
);

export const PhoneIcon = createIcon(
  ['M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z'],
  'PhoneIcon',
);

export const FileTextIcon = createIcon(
  ['M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7z', 'M14 2v4a2 2 0 0 0 2 2h4', 'M10 9H8', 'M16 13H8', 'M16 17H8'],
  'FileTextIcon',
);
