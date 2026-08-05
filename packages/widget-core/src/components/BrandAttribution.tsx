import { FunctionComponent } from 'preact';
import { HelpinMark } from './HelpinMark';
import { buildHelpinAttributionUrl } from '../utils/helpinAttribution';

interface BrandAttributionProps {
  label: string;
  className: string;
  workspaceId?: string;
  workspaceName?: string;
  content: 'chat_widget_footer' | 'chat_widget_composer';
}

export const BrandAttribution: FunctionComponent<BrandAttributionProps> = ({
  label,
  className,
  workspaceId,
  workspaceName,
  content,
}) => {
  const attributionUrl = buildHelpinAttributionUrl(workspaceName, workspaceId, content);

  return (
    <a
      href={attributionUrl}
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
};
