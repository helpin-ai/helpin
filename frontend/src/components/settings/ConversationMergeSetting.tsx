import { Switch } from "@/components/ui/switch";
import {
  useChatSettings,
  useUpdateChatSettings,
} from "@/hooks/queries/useSupport";
import { useWorkspaceAccess } from "@/hooks/queries/useSession";
export function ConversationMergeSetting({
  workspaceId,
}: {
  workspaceId: string;
}) {
  const { data } = useChatSettings(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId);
  const update = useUpdateChatSettings(workspaceId);
  if (!access?.permissions.includes("support.admin")) return null;
  return (
    <section className="rounded-xl border p-5">
      <div className="flex items-center justify-between gap-6">
        <div>
          <label htmlFor="cross-customer-merge" className="text-sm font-medium">
            Merge conversations from different customers
          </label>
          <p className="mt-1 text-xs text-muted-foreground">
            Customers will share new replies. Earlier histories stay private.
          </p>
        </div>
        <Switch
          id="cross-customer-merge"
          checked={data?.settings.allow_cross_customer_merge ?? false}
          disabled={!data || update.isPending}
          onCheckedChange={(checked) =>
            update.mutate({ allow_cross_customer_merge: checked })
          }
        />
      </div>
    </section>
  );
}
