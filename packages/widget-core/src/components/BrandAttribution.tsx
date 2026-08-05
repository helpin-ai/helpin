import { FunctionComponent } from 'preact';
import { HelpinMark } from './HelpinMark';

const HELPIN_BRANDING_URL = 'https://helpin.ai/?utm_source=helpin_widget&utm_medium=widget&utm_campaign=powered_by';

interface BrandAttributionProps {
  label: string;
  className: string;
}

export const BrandAttribution: FunctionComponent<BrandAttributionProps> = ({ label, className }) => (
  <a
    href={HELPIN_BRANDING_URL}
    target="_blank"
    rel="noopener noreferrer"
    className={`helpin-brand-attribution ${className}`}
  >
    <span className="helpin-brand-attribution-label">{label}</span>
    <span className="helpin-brand-attribution-brand">
      <HelpinMark className="helpin-brand-attribution-icon" />
      <span className="helpin-brand-attribution-name">Helpin</span>
    </span>
  </a>
);
