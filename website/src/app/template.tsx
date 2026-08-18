import { HelpinWidgetProvider } from '@/components/HelpinWidgetProvider';

export default function Template({ children }: Readonly<{ children: React.ReactNode }>) {
  return <HelpinWidgetProvider>{children}</HelpinWidgetProvider>;
}
