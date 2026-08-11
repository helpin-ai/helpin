import { FunctionComponent } from 'preact';

interface AIThinkingMarkProps {
  className?: string;
}

// The four Helpin arms animate independently so the mark feels active without
// turning into a generic spinner or distracting from the response shimmer.
export const AIThinkingMark: FunctionComponent<AIThinkingMarkProps> = ({ className }) => (
  <svg
    viewBox="13.636 13.636 72.727 72.727"
    fill="currentColor"
    aria-hidden="true"
    focusable="false"
    className={className}
  >
    <g className="helpin-ai-thinking-mark-arm helpin-ai-thinking-mark-arm--top">
      <path d="M55.818 41.273H26.727a8.727 8.727 0 0 1 0-17.455h29.091z" />
      <path d="M26.041 20.909a8.727 8.727 0 0 1 13.009 0z" />
    </g>
    <g className="helpin-ai-thinking-mark-arm helpin-ai-thinking-mark-arm--right">
      <path d="M79.091 26.041a8.727 8.727 0 0 1 0 13.009z" />
      <path d="M58.727 55.818V26.727a8.727 8.727 0 0 1 17.455 0v29.091z" />
    </g>
    <g className="helpin-ai-thinking-mark-arm helpin-ai-thinking-mark-arm--bottom">
      <path d="M44.182 58.727h29.091a8.727 8.727 0 0 1 0 17.455H44.182z" />
      <path d="M73.959 79.091a8.727 8.727 0 0 1-13.009 0z" />
    </g>
    <g className="helpin-ai-thinking-mark-arm helpin-ai-thinking-mark-arm--left">
      <path d="M20.909 73.959a8.727 8.727 0 0 1 0-13.009z" />
      <path d="M41.273 44.182v29.091a8.727 8.727 0 0 1-17.455 0V44.182z" />
    </g>
  </svg>
);
