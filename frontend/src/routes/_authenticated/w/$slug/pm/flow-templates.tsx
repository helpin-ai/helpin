import { createFileRoute } from '@tanstack/react-router';
import { FlowTemplateEditorPage } from '@/pages/pm/FlowTemplateEditor';

export const Route = createFileRoute('/_authenticated/w/$slug/pm/flow-templates')({
  component: FlowTemplateEditorRoute,
});

function FlowTemplateEditorRoute() {
  return (
    <div className="h-full overflow-auto p-4 md:p-6">
      <FlowTemplateEditorPage />
    </div>
  );
}
