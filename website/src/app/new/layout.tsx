import { previewMetadata } from './_components/preview-metadata';
import { MarketingShell } from './_components/MarketingShell';

// Preview route. Not indexed until the direction is approved and it replaces /.
export const metadata = previewMetadata("Helpin — Support, projects, CRM and docs on one customer history, with AI agents.", "/new");

export default function NewHomeLayout({ children }: { children: React.ReactNode }) {
  return <MarketingShell>{children}</MarketingShell>;
}
