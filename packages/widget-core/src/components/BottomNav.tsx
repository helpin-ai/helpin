import { FunctionComponent } from 'preact';

export type WidgetBaseView = 'home' | 'messages' | 'help';
export type WidgetView = WidgetBaseView | 'conversation';

interface BottomNavProps {
  activeView: WidgetView;
  onNavigate: (view: WidgetBaseView) => void;
  brandColor?: string;
}

const HOME_ICON = 'M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z';
const MESSAGES_ICON = 'M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z';
const HELP_ICON = 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z';

const NAV_ITEMS: { view: WidgetBaseView; label: string; icon: string }[] = [
  { view: 'home', label: 'Home', icon: HOME_ICON },
  { view: 'messages', label: 'Messages', icon: MESSAGES_ICON },
  { view: 'help', label: 'Help', icon: HELP_ICON },
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
            <svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor">
              <path d={item.icon} />
            </svg>
            <span className="helpin-bottom-nav-label">{item.label}</span>
          </button>
        );
      })}
    </div>
  );
};
