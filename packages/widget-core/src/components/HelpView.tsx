import { FunctionComponent } from 'preact';
import type { WidgetConfig } from '../types';
import { SearchIcon, MailIcon, FileTextIcon, ChevronRightIcon } from './icons';

interface HelpViewProps {
  config: WidgetConfig;
  onNavigate: (view: 'conversation' | 'messages') => void;
}

export const HelpView: FunctionComponent<HelpViewProps> = ({
  config: _config,
  onNavigate,
}) => {
  return (
    <div className="helpin-help-view">
      <div className="helpin-help-header">
        <span className="helpin-help-title">Help</span>
      </div>
      <div className="helpin-help-content">
        <div className="helpin-help-search">
          <SearchIcon size={18} class="helpin-help-search-icon" />
          <input
            type="text"
            className="helpin-help-search-input"
            placeholder="Search for help..."
          />
        </div>

        <div className="helpin-help-links">
          <button className="helpin-help-link" onClick={() => onNavigate('conversation')}>
            <MailIcon size={20} />
            <div className="helpin-help-link-text">
              <span className="helpin-help-link-title">Contact us</span>
              <span className="helpin-help-link-desc">Send us a message and we'll get back to you</span>
            </div>
            <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
          </button>

          <div className="helpin-help-divider" />

          <a className="helpin-help-link" href="#" target="_blank" rel="noopener noreferrer">
            <FileTextIcon size={20} />
            <div className="helpin-help-link-text">
              <span className="helpin-help-link-title">Browse our docs</span>
              <span className="helpin-help-link-desc">Find detailed guides and documentation</span>
            </div>
            <ChevronRightIcon size={16} class="helpin-help-link-arrow" />
          </a>
        </div>
      </div>
    </div>
  );
};
