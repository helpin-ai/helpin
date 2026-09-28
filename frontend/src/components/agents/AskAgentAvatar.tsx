import { useId, type CSSProperties, type HTMLAttributes } from 'react';

import { cn } from '@/lib/utils';
import { ASK_AGENT_BODY_PATH } from '@helpin-ai/shared';

export type AskAgentAvatarState = 'idle' | 'thinking' | 'speaking' | 'error';
export type AskAgentPlateStyle = 'solid' | 'feather';

interface AskAgentAvatarProps extends Omit<HTMLAttributes<HTMLSpanElement>, 'color'> {
  state?: AskAgentAvatarState;
  size?: number | string;
  plate?: boolean;
  plateStyle?: AskAgentPlateStyle;
  feather?: number;
  plateGradient?: readonly [string, string];
  plateColor?: string;
  radius?: number;
  color?: string;
  ink?: string;
  label?: string;
  decorative?: boolean;
}

export { ASK_AGENT_BODY_PATH } from '@helpin-ai/shared';

const STATE_LABELS: Record<AskAgentAvatarState, string> = {
  idle: 'Ask Agent ready',
  thinking: 'Ask Agent is thinking',
  speaking: 'Ask Agent is replying',
  error: 'Ask Agent hit a problem',
};

type AvatarStyle = CSSProperties & {
  '--aa-color': string;
  '--aa-plate': string;
  '--aa-ink': string;
};

/** The product-owned Ask Agent mark. Saved and specialist agents use AgentAvatar instead. */
export function AskAgentAvatar({
  state = 'idle',
  size,
  plate = true,
  plateStyle = 'feather',
  feather = 9,
  plateGradient,
  plateColor = '#1E1C1A',
  radius = 6,
  color = '#F7F5F2',
  ink,
  label,
  decorative = true,
  className,
  style,
  ...rest
}: AskAgentAvatarProps) {
  const uid = useId().replaceAll(':', '');
  const soft = plate && plateStyle === 'feather';
  const gradient = plateGradient ?? (soft ? ['#4A433D', '#0B0A09'] as const : null);
  const dimension = typeof size === 'number' ? `${size}px` : size;
  const avatarStyle: AvatarStyle = {
    width: dimension,
    height: dimension,
    '--aa-color': color,
    '--aa-plate': plateColor,
    '--aa-ink': ink ?? plateColor,
    ...style,
  };

  return (
    <span
      className={cn('ask-agent-avatar', !size && 'h-10 w-10', className)}
      data-state={state}
      data-plate-style={plate ? plateStyle : 'none'}
      style={avatarStyle}
      aria-hidden={decorative || undefined}
      role={decorative ? undefined : 'img'}
      aria-label={decorative ? undefined : label ?? STATE_LABELS[state]}
      {...rest}
    >
      <svg
        viewBox={soft ? '-2.5 -2.5 105 105' : '0 0 100 100'}
        aria-hidden="true"
        focusable="false"
      >
        <defs>
          {soft ? (
            <filter id={`ask-agent-feather-${uid}`} x="-50%" y="-50%" width="200%" height="200%">
              <feGaussianBlur stdDeviation={Math.min(12, Math.max(2, feather))} />
            </filter>
          ) : null}
          {gradient ? (
            <linearGradient id={`ask-agent-gradient-${uid}`} x1="0" y1="0" x2="1" y2="1">
              <stop offset="0" stopColor={gradient[0]} />
              <stop offset="1" stopColor={gradient[1]} />
            </linearGradient>
          ) : null}
        </defs>

        {plate ? (
          <rect
            className={gradient ? undefined : 'ask-agent-plate'}
            x="0"
            y="0"
            width="100"
            height="100"
            rx={soft ? 16 : Math.min(50, Math.max(0, radius))}
            ry={soft ? 16 : Math.min(50, Math.max(0, radius))}
            fill={gradient ? `url(#ask-agent-gradient-${uid})` : undefined}
            filter={soft ? `url(#ask-agent-feather-${uid})` : undefined}
          />
        ) : null}

        <g className="ask-agent-body-group">
          <path className="ask-agent-body" d={ASK_AGENT_BODY_PATH} />
        </g>
        <g className="ask-agent-face">
          <g className="ask-agent-eyes">
            <g className="ask-agent-eye-l">
              <g className="ask-agent-eye-open">
                <rect className="ask-agent-ink" x="35.03" y="32.62" width="8.40" height="12" rx="4.20" />
                <circle className="ask-agent-spark" cx="39.82" cy="35.85" r="1.58" />
              </g>
              <path className="ask-agent-stroke ask-agent-eye-closed" d="M35.57,39.40 Q39.16,35.89 42.75,39.40" strokeWidth="2.23" />
            </g>
            <g className="ask-agent-eye-r">
              <g className="ask-agent-eye-open">
                <rect className="ask-agent-ink" x="62.51" y="32.68" width="8.40" height="12" rx="4.20" />
                <circle className="ask-agent-spark" cx="67.25" cy="35.85" r="1.58" />
              </g>
              <path className="ask-agent-stroke ask-agent-eye-closed" d="M63.36,39.40 Q66.95,35.89 70.54,39.40" strokeWidth="2.23" />
            </g>
          </g>
          <path className="ask-agent-stroke ask-agent-mouth" d="M51.36,65.63 Q60.93,73.05 70.58,65.47" strokeWidth="3.35" />
          <path className="ask-agent-stroke ask-agent-mouth-sad" d="M51.36,71.20 Q60.93,63.90 70.58,71.00" strokeWidth="3.35" />
        </g>
      </svg>
    </span>
  );
}
