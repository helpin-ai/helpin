import { FunctionComponent } from 'preact';

type LauncherIcon = 'chat_bubble' | 'question_mark' | 'help';

const LAUNCHER_ICONS: Record<LauncherIcon, string> = {
  chat_bubble: 'M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z',
  question_mark: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z',
  help: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17v-2h2v2h-2zm2.07-7.75l-.9.92c-.5.51-.82.89-.99 1.37-.13.36-.18.76-.18 1.46h-2v-.5a4.5 4.5 0 0 1 .52-2.08c.3-.55.71-1.04 1.24-1.52l1.24-1.26c.37-.36.59-.86.59-1.41a2.22 2.22 0 0 0-.73-1.64A2.33 2.33 0 0 0 12 7c-.85 0-1.55.3-2.08.83-.53.52-.8 1.16-.87 1.94H7.07c.08-1.42.62-2.57 1.63-3.44C9.71 5.44 10.76 5 12 5c1.3 0 2.4.42 3.3 1.26.9.84 1.37 1.86 1.37 3.07 0 .88-.36 1.68-.93 2.25-.18.18-.37.35-.57.5l-.1.07z',
};

const CLOSE_ICON = 'M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z';

interface WidgetLauncherProps {
  onClick: () => void;
  isOpen: boolean;
  unreadCount?: number;
  brandColor?: string;
  icon?: LauncherIcon;
}

export const WidgetLauncher: FunctionComponent<WidgetLauncherProps> = ({
  onClick,
  isOpen,
  unreadCount = 0,
  brandColor = '#6366f1',
  icon = 'chat_bubble',
}) => {
  const iconPath = isOpen ? CLOSE_ICON : (LAUNCHER_ICONS[icon] || LAUNCHER_ICONS.chat_bubble);

  return (
    <button
      className={`helpin-launcher ${isOpen ? 'helpin-launcher--open' : ''}`}
      onClick={onClick}
      style={{ backgroundColor: brandColor }}
      aria-label={isOpen ? 'Close chat' : 'Open chat'}
    >
      <svg viewBox="0 0 24 24" width="28" height="28" fill="white">
        <path d={iconPath} />
      </svg>
      {!isOpen && unreadCount > 0 && (
        <span
          className="helpin-unread-badge"
          style={{ backgroundColor: '#ef4444' }}
          aria-label={`${unreadCount} unread messages`}
        >
          {unreadCount > 9 ? '9+' : unreadCount}
        </span>
      )}
    </button>
  );
};
