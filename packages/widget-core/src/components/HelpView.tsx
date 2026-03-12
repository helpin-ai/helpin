import { FunctionComponent } from 'preact';
import type { WidgetConfig } from '../types';

interface HelpViewProps {
  config: WidgetConfig;
  onNavigate: (view: 'messages') => void;
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
          <svg viewBox="0 0 24 24" width="18" height="18" fill="currentColor" className="helpin-help-search-icon">
            <path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z" />
          </svg>
          <input
            type="text"
            className="helpin-help-search-input"
            placeholder="Search for help..."
          />
        </div>

        <div className="helpin-help-links">
          <button className="helpin-help-link" onClick={() => onNavigate('messages')}>
            <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
              <path d="M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z" />
            </svg>
            <div className="helpin-help-link-text">
              <span className="helpin-help-link-title">Contact us</span>
              <span className="helpin-help-link-desc">Send us a message and we'll get back to you</span>
            </div>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" className="helpin-help-link-arrow">
              <path d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" />
            </svg>
          </button>

          <div className="helpin-help-divider" />

          <a className="helpin-help-link" href="#" target="_blank" rel="noopener noreferrer">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
              <path d="M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z" />
            </svg>
            <div className="helpin-help-link-text">
              <span className="helpin-help-link-title">Browse our docs</span>
              <span className="helpin-help-link-desc">Find detailed guides and documentation</span>
            </div>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" className="helpin-help-link-arrow">
              <path d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" />
            </svg>
          </a>
        </div>
      </div>
    </div>
  );
};
