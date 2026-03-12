import { FunctionComponent } from 'preact';

interface WidgetHeaderProps {
  workspaceName: string;
  logoUrl?: string;
  onClose: () => void;
  brandColor?: string;
  showBranding?: boolean;
}

export const WidgetHeader: FunctionComponent<WidgetHeaderProps> = ({
  workspaceName,
  logoUrl,
  onClose,
  brandColor = '#6366f1',
}) => {
  return (
    <div className="helpin-widget-header" style={{ backgroundColor: brandColor }}>
      <div className="helpin-header-content">
        {logoUrl ? (
          <img src={logoUrl} alt={workspaceName} className="helpin-header-logo" />
        ) : (
          <div className="helpin-header-title">{workspaceName}</div>
        )}
      </div>
      <button className="helpin-header-close" onClick={onClose} aria-label="Close">
        <svg viewBox="0 0 24 24" width="20" height="20" fill="white">
          <path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" />
        </svg>
      </button>
    </div>
  );
};
