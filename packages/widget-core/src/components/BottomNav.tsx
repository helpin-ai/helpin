import { FunctionComponent } from 'preact';
import { HomeIcon, MessageSquareIcon, CircleHelpIcon } from './icons';

export type WidgetBaseView = 'home' | 'messages' | 'help';
export type WidgetView = WidgetBaseView | 'conversation';

interface BottomNavProps {
  activeView: WidgetView;
  onNavigate: (view: WidgetBaseView) => void;
  brandColor?: string;
}

const NAV_ITEMS: { view: WidgetBaseView; label: string; Icon: FunctionComponent<{ size?: number; color?: string }> }[] = [
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
            className={`helpin-bottom-nav-item ${isActive ? 'helpin-bottom-nav-item--active' : ''}`}
            onClick={() => onNavigate(item.view)}
            style={isActive ? { color: brandColor } : undefined}
          >
            <item.Icon size={22} />
            <span className="helpin-bottom-nav-label">{item.label}</span>
          </button>
        );
      })}
    </div>
  );
};
