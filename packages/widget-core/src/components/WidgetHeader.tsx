import { FunctionComponent } from 'preact';
import { XIcon } from './icons';

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
        <XIcon size={20} color="white" />
      </button>
    </div>
  );
};
