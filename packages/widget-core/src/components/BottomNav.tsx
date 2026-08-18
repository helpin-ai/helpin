import { FunctionComponent } from 'preact';
import { HomeIcon, MessageSquareIcon, CircleHelpIcon } from './icons';

export type WidgetBaseView = 'home' | 'messages' | 'help';
export type WidgetView = WidgetBaseView | 'conversation' | 'help-space' | 'help-collection' | 'help-article';

interface BottomNavProps {
  activeView: WidgetView;
  onNavigate: (view: WidgetBaseView) => void;
  brandColor?: string;
}

const NAV_ITEMS: { view: WidgetBaseView; label: string; Icon: FunctionComponent<{ size?: number; color?: string; strokeWidth?: number }> }[] = [
  { view: 'home', label: 'Home', Icon: HomeIcon },
  { view: 'messages', label: 'Messages', Icon: MessageSquareIcon },
  { view: 'help', label: 'Help', Icon: CircleHelpIcon },
];

function getContrastingIconColor(color: string): '#ffffff' | '#111827' {
  const normalized = color.trim().replace(/^#/, '');
  const expanded = normalized.length === 3
    ? normalized.split('').map(character => character + character).join('')
    : normalized;

  if (!/^[0-9a-f]{6}$/i.test(expanded)) {
    return '#ffffff';
  }

  const channels = [0, 2, 4].map(index => parseInt(expanded.slice(index, index + 2), 16) / 255);
  const [red, green, blue] = channels.map(channel =>
    channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4,
  );
  const luminance = 0.2126 * red + 0.7152 * green + 0.0722 * blue;
  const whiteContrast = 1.05 / (luminance + 0.05);
  const darkContrast = (luminance + 0.05) / 0.059;

  return whiteContrast >= darkContrast ? '#ffffff' : '#111827';
}

export const BottomNav: FunctionComponent<BottomNavProps> = ({
  activeView,
  onNavigate,
  brandColor = '#6366f1',
}) => {
  const activeIconColor = getContrastingIconColor(brandColor);

  return (
    <div
      className="helpin-bottom-nav"
      style={{
        '--helpin-nav-active-color': brandColor,
        '--helpin-nav-active-foreground': activeIconColor,
      }}
    >
      {NAV_ITEMS.map((item) => {
        const isActive = activeView === item.view;
        return (
          <button
            key={item.view}
            type="button"
            className={`helpin-bottom-nav-item ${isActive ? 'helpin-bottom-nav-item--active' : ''}`}
            onClick={() => onNavigate(item.view)}
            aria-current={isActive ? 'page' : undefined}
          >
            <span className="helpin-bottom-nav-icon" aria-hidden="true">
              <item.Icon size={21} strokeWidth={isActive ? 2.15 : 1.85} />
            </span>
            <span className="helpin-bottom-nav-label">{item.label}</span>
          </button>
        );
      })}
    </div>
  );
};
