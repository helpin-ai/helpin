import { FunctionComponent } from 'preact';
import { MessageCircleIcon, CircleHelpIcon, LifeBuoyIcon, XIcon } from './icons';

type LauncherIcon = 'chat_bubble' | 'question_mark' | 'help';

interface WidgetLauncherProps {
  onClick: () => void;
  isOpen: boolean;
  unreadCount?: number;
  brandColor?: string;
  buttonColor?: string;
  buttonIconColor?: string;
  icon?: LauncherIcon;
}

export const WidgetLauncher: FunctionComponent<WidgetLauncherProps> = ({
  onClick,
  isOpen,
  unreadCount = 0,
  brandColor = '#6366f1',
  buttonColor,
  buttonIconColor,
  icon = 'chat_bubble',
}) => {
  const bgColor = buttonColor || brandColor;
  const iconColor = buttonIconColor || '#ffffff';

  const LauncherIconMap: Record<string, FunctionComponent<{ size?: number; color?: string }>> = {
    chat_bubble: MessageCircleIcon,
    question_mark: CircleHelpIcon,
    help: LifeBuoyIcon,
  };
  const ActiveIcon = isOpen ? XIcon : (LauncherIconMap[icon] || MessageCircleIcon);

  return (
    <button
      className={`helpin-launcher ${isOpen ? 'helpin-launcher--open' : ''}`}
      onClick={onClick}
      style={{ backgroundColor: bgColor }}
      aria-label={isOpen ? 'Close chat' : 'Open chat'}
    >
      <ActiveIcon size={28} color={iconColor} />
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
