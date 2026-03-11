import { h, FunctionComponent } from 'preact';

interface WidgetLauncherProps {
  onClick: () => void;
  isOpen: boolean;
  unreadCount?: number;
  brandColor?: string;
}

export const WidgetLauncher: FunctionComponent<WidgetLauncherProps> = ({
  onClick,
  isOpen,
  unreadCount = 0,
  brandColor = '#6366f1',
}) => {
  return (
    <button
      className={`helpin-launcher ${isOpen ? 'helpin-launcher--open' : ''}`}
      onClick={onClick}
      style={{ backgroundColor: brandColor }}
      aria-label={isOpen ? 'Close chat' : 'Open chat'}
    >
      {!isOpen ? (
        <>
          <svg viewBox="0 0 24 24" width="28" height="28" fill="white">
            <path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z" />
          </svg>
          {unreadCount > 0 && (
            <span className="helpin-unread-badge" style={{ backgroundColor: '#ef4444' }}>
              {unreadCount > 9 ? '9+' : unreadCount}
            </span>
          )}
        </>
      ) : (
        <svg viewBox="0 0 24 24" width="28" height="28" fill="white">
          <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" />
        </svg>
      )}
    </button>
  );
};
