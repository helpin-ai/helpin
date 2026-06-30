import { FunctionComponent } from 'preact';
import type { WidgetConfig } from '../types';
import { MailIcon, FileTextIcon, ChevronRightIcon, XIcon } from './icons';
import { HelpSpaceView } from './HelpSpaceView';

interface HelpViewProps {
  config: WidgetConfig;
  host?: string;
  widgetKey?: string;
  onContact: () => void;
  onSelectSpace: (spaceSlug: string) => void;
  onSelectCollection: (collectionSlug: string) => void;
  onClose?: () => void;
}

export const HelpView: FunctionComponent<HelpViewProps> = ({
  config,
  host,
  widgetKey,
  onContact,
  onSelectSpace,
  onSelectCollection,
  onClose,
}) => {
  const helpSpaces = config.helpSpaces ?? [];
  const canBrowseDocs = !!host && !!widgetKey && helpSpaces.length > 0;

  return (
    <div className="helpin-help-view">
      <div className="helpin-help-header">
        <div className="helpin-help-header-spacer" />
        <span className="helpin-help-title">Help Center</span>
        {onClose ? (
          <button className="helpin-window-close-inline" onClick={onClose} aria-label="Close">
            <XIcon size={18} />
          </button>
        ) : (
          <div className="helpin-help-header-spacer" />
        )}
      </div>
      <div className="helpin-help-content">
        {!canBrowseDocs && (
          <p className="helpin-help-empty">Articles are not available in this widget yet.</p>
        )}

        {canBrowseDocs && helpSpaces.length === 1 && host && widgetKey && (
          <div className="helpin-help-inline-section">
            <div className="helpin-help-section-label">
              <FileTextIcon size={16} />
              <span>Help Center</span>
            </div>
            <HelpSpaceView
              host={host}
              widgetKey={widgetKey}
              space={helpSpaces[0]}
              showBack={false}
              showHeader={false}
              onBack={() => {}}
              onSelectCollection={onSelectCollection}
            />
          </div>
        )}

        {canBrowseDocs && helpSpaces.length > 1 && (
          <div className="helpin-help-inline-section">
            <div className="helpin-help-section-label">
              <FileTextIcon size={16} />
              <span>Help Center</span>
            </div>
            <div className="helpin-help-list">
              {helpSpaces.map((space) => (
                <button
                  key={space.id}
                  className="helpin-help-link"
                  onClick={() => onSelectSpace(space.slug)}
                >
                  <FileTextIcon size={20} />
                  <div className="helpin-help-link-text">
                    <span className="helpin-help-link-title">{space.name}</span>
                    <span className="helpin-help-link-desc">Browse collections and articles</span>
                  </div>
                  <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="helpin-help-links helpin-help-links--bottom">
          <button className="helpin-help-link" onClick={onContact}>
            <MailIcon size={20} />
            <div className="helpin-help-link-text">
              <span className="helpin-help-link-title">Contact us</span>
              <span className="helpin-help-link-desc">Send us a message and we'll get back to you</span>
            </div>
            <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
          </button>
        </div>
      </div>
    </div>
  );
};
