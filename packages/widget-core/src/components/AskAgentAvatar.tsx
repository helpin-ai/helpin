import { ASK_AGENT_BODY_PATH } from '@helpin-ai/shared';

/** The same Helpin mark as the app, rendered without a React dependency. */
export function AskAgentAvatar() {
  return (
    <svg className="helpin-message-avatar" viewBox="0 0 100 100" aria-hidden="true" focusable="false">
      <circle cx="50" cy="50" r="50" fill="#1E1C1A" />
      <path d={ASK_AGENT_BODY_PATH} fill="#F7F5F2" fillRule="evenodd" />
      <g fill="#1E1C1A">
        <rect x="35.03" y="32.62" width="8.40" height="12" rx="4.20" />
        <rect x="62.51" y="32.68" width="8.40" height="12" rx="4.20" />
      </g>
      <g fill="#F7F5F2">
        <circle cx="39.82" cy="35.85" r="1.58" />
        <circle cx="67.25" cy="35.85" r="1.58" />
      </g>
      <path d="M51.36,65.63 Q60.93,73.05 70.58,65.47" fill="none" stroke="#1E1C1A" strokeWidth="3.35" strokeLinecap="round" />
    </svg>
  );
}
