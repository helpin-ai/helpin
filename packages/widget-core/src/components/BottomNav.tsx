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

export const BottomNav: FunctionComponent<BottomNavProps> = ({
  activeView,
  onNavigate,
  brandColor = '#6366f1',
}) => {
  return (
    <div className="helpin-bottom-nav">
      {NAV_ITEMS.map((item) => {
        const isActive = activeView === item.view;
        return (
          <button
            key={item.view}
            type="button"
            className={`helpin-bottom-nav-item ${isActive ? 'helpin-bottom-nav-item--active' : ''}`}
            onClick={() => onNavigate(item.view)}
            style={isActive ? { color: brandColor, '--helpin-nav-active-color': brandColor } : undefined}
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
