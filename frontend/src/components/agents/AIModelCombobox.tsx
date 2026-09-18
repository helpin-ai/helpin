import { useMemo, useState } from "react";
import { QuietDropdown } from "@/components/design-system/quiet-dropdown";
import { quietUnderlineControlClassName } from "@/components/design-system/quiet";
import { catalogLabel, catalogTier, modelCatalogFor } from "@/lib/aiProviders";
import { ArrowUpDownIcon } from "@/lib/icons";
import { cn } from "@/lib/utils";

const CUSTOM_PREFIX = "custom:";

/**
 * Model picker with catalog suggestions grouped by tier. Any identifier can still
 * be typed, which is what compatible endpoints and unlisted models need.
 */
export function AIModelCombobox({
  id,
  provider,
  value,
  disabled,
  onChange,
}: {
  id?: string;
  provider: string;
  value: string;
  disabled?: boolean;
  onChange: (model: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const catalog = useMemo(() => modelCatalogFor(provider), [provider]);
  const trimmed = query.trim();
  const known = catalog.some((group) =>
    group.models.some((model) => model.selectionModel === trimmed),
  );

  const groups = useMemo(() => {
    const catalogGroups = catalog.map((group) => ({
      id: group.key,
      label: group.label,
      options: group.models.map((model) => ({
        value: model.selectionModel,
        label: model.label,
        keywords: [model.selectionModel, model.canonicalModel],
        content: (
          <span className="flex min-w-0 flex-col">
            <span>{model.label}</span>
            <span className="font-mono text-[11px] text-muted-foreground">
              {model.selectionModel}
            </span>
          </span>
        ),
      })),
    }));
    if (!trimmed || known || provider === "openai_chatgpt") return catalogGroups;
    return [
      {
        id: "custom",
        label: "Custom",
        options: [
          {
            value: `${CUSTOM_PREFIX}${trimmed}`,
            label: `Use “${trimmed}”`,
            keywords: [trimmed],
          },
        ],
      },
      ...catalogGroups,
    ];
  }, [catalog, trimmed, known, provider]);

  const tier = provider === "openai_chatgpt" ? undefined : catalogTier(provider, value);

  return (
    <div className="space-y-1">
      <QuietDropdown
        label="Model"
        open={open}
        onOpenChange={(next) => {
          setOpen(next);
          setQuery("");
        }}
        disabled={disabled}
        searchMode="always"
        searchPlaceholder={provider === "openai_chatgpt" ? "Search models" : "Search or type a model id"}
        query={query}
        onQueryChange={setQuery}
        groups={groups}
        selected={value ? [value] : []}
        onSelect={(selected) => {
          onChange(
            selected.startsWith(CUSTOM_PREFIX) ? selected.slice(CUSTOM_PREFIX.length) : selected,
          );
          setOpen(false);
        }}
        empty={
          provider === "openai_chatgpt" ? <span className="px-2 py-1.5 text-xs text-muted-foreground">No matching models.</span> : trimmed ? (
            <span className="px-2 py-1.5 text-xs text-muted-foreground">
              Press Enter to use “{trimmed}”.
            </span>
          ) : (
            <span className="px-2 py-1.5 text-xs text-muted-foreground">
              Type the exact model id from your provider.
            </span>
          )
        }
        trigger={
          <button
            type="button"
            id={id}
            role="combobox"
            aria-label="Model"
            aria-expanded={open}
            disabled={disabled}
            className={cn(
              quietUnderlineControlClassName,
              "flex w-full items-center justify-between gap-2 px-0.5 text-left disabled:opacity-60",
            )}
          >
            <span className={cn("truncate", !value && "text-muted-foreground")}>
              {catalogLabel(provider, value) || value || "Choose model"}
            </span>
            <ArrowUpDownIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          </button>
        }
      />
      {disabled ? (
        <p className="text-xs text-muted-foreground">
          Choose a connection to see suggested models.
        </p>
      ) : tier ? (
        <p className="text-xs text-muted-foreground">
          {tier.label} · {tier.description}
        </p>
      ) : value && provider !== "openai_chatgpt" && !catalogLabel(provider, value) ? (
        <p className="text-xs text-muted-foreground">
          Custom model. Make sure your provider accepts this id.
        </p>
      ) : null}
    </div>
  );
}
