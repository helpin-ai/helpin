import { useRef, useState } from "react";
import type { Editor, JSONContent } from "@tiptap/core";
import { DocsEditor } from "@/components/docs/DocsEditor";
import { Button } from "@/components/ui/button";
import { QuietUnderlineInput } from "@/components/design-system/quiet";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/design-system/quiet-dropdown-select";
import type { DocsCollection, DocsSpace } from "@/lib/docsTypes";
import type {
  CoverageSuggestionReview,
  SupportCoverageGapDetail,
  SupportGapSuggestion,
} from "@/lib/supportCoverageTypes";
import { Loading01Icon } from "@/lib/icons";
import { buildCoverageCollectionOptions } from "./coverageCollectionOptions";

export function GapProposalReview({
  gap,
  suggestion,
  wsSlug,
  spaces,
  collections,
  targetSpaceId,
  targetCollectionId,
  canEdit,
  applying,
  onSpaceChange,
  onCollectionChange,
  onApply,
  onDiscard,
}: {
  gap: SupportCoverageGapDetail;
  suggestion: SupportGapSuggestion;
  wsSlug: string;
  spaces: DocsSpace[];
  collections?: DocsCollection[];
  targetSpaceId: string;
  targetCollectionId: string;
  canEdit: boolean;
  applying: boolean;
  onSpaceChange: (id: string) => void;
  onCollectionChange: (id: string) => void;
  onApply: (
    id: string,
    review?: CoverageSuggestionReview,
  ) => Promise<string | null>;
  onDiscard: (id: string) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [title, setTitle] = useState(suggestion.title);
  const [content, setContent] = useState(suggestion.content as JSONContent);
  const editor = useRef<Editor | null>(null);
  const [route, setRoute] = useState<"create_article" | "update_article">(
    suggestion.suggestion_type === "update_article"
      ? "update_article"
      : "create_article",
  );
  const [documentId, setDocumentId] = useState(
    suggestion.target_document_id || gap.related_articles[0]?.document_id || "",
  );
  const spaceId = targetSpaceId || suggestion.target_space_id || "";
  const collectionId = targetSpaceId
    ? targetCollectionId
    : suggestion.target_collection_id || "";
  const collectionOptions = buildCoverageCollectionOptions(
    spaceId,
    collections ?? [],
  );
  const documentTitle =
    gap.related_articles.find((article) => article.document_id === documentId)
      ?.article_title ||
    suggestion.target_document_title ||
    "linked article";
  const validDestination =
    route === "update_article" ? Boolean(documentId) : Boolean(spaceId);
  const save = () => {
    return onApply(suggestion.id, {
      route,
      title: title.trim(),
      content: editor.current?.getJSON() ?? content,
      target_document_id: route === "update_article" ? documentId : undefined,
      target_space_id: route === "create_article" ? spaceId : undefined,
      target_collection_id:
        route === "create_article" ? collectionId : undefined,
    });
  };
  const saveAndOpen = async () => {
    const savedDocumentId = await save();
    if (savedDocumentId)
      window.location.assign(
        `/w/${encodeURIComponent(wsSlug)}/docs/documents/${encodeURIComponent(savedDocumentId)}`,
      );
  };
  return (
    <section
      aria-label="Review proposed fix"
      className="border-t border-border/50 px-5 py-5 sm:px-7"
    >
      <div className="flex items-center justify-between gap-3">
        <h4 className="text-sm font-semibold">Review proposed fix</h4>
        {canEdit && (
          <Button
            variant="ghost"
            size="sm"
            disabled={applying}
            onClick={() => setEditing(!editing)}
          >
            {editing ? "Preview" : "Edit draft"}
          </Button>
        )}
      </div>
      <label
        className="mt-3 block text-xs text-muted-foreground"
        htmlFor={`proposal-title-${suggestion.id}`}
      >
        {route === "update_article" ? "Proposal title" : "Article title"}
      </label>
      <QuietUnderlineInput
        id={`proposal-title-${suggestion.id}`}
        aria-label={
          route === "update_article" ? "Proposal title" : "Article title"
        }
        value={title}
        disabled={!canEdit || applying}
        onChange={(event) => setTitle(event.target.value)}
        className="mt-1 w-full text-base font-medium"
      />
      <div
        className="mt-3 h-[360px] min-w-0 overflow-hidden border-y border-border/40"
        data-coverage-proposal-editor
      >
        <DocsEditor
          showTitle={false}
          showFileMenu={false}
          initialContent={content}
          readOnly={!editing || !canEdit || applying}
          onSave={async (value) => {
            setContent(value);
          }}
          onEditorReady={(value) => {
            editor.current = value;
          }}
          contentWidth="reading"
        />
      </div>
      {suggestion.evidence_summary && (
        <p className="mt-2 text-xs text-muted-foreground">
          Based on: {suggestion.evidence_summary}
        </p>
      )}
      {canEdit && (
        <div className="mt-4 space-y-3">
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-xs text-muted-foreground">Save as</span>
            <Select
              value={route}
              onValueChange={(value) => setRoute(value as typeof route)}
              disabled={applying}
            >
              <SelectTrigger aria-label="Save route" className="w-48">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="create_article">
                  New draft article
                </SelectItem>
                <SelectItem value="update_article" disabled={!documentId}>
                  Additions to an article
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          {route === "update_article" ? (
            <Select
              value={documentId}
              onValueChange={setDocumentId}
              disabled={applying}
            >
              <SelectTrigger aria-label="Target article" className="w-full">
                <SelectValue placeholder="Choose article" />
              </SelectTrigger>
              <SelectContent>
                {gap.related_articles.map((article) => (
                  <SelectItem
                    key={article.document_id}
                    value={article.document_id}
                  >
                    {article.article_title || "Untitled article"}
                  </SelectItem>
                ))}
                {suggestion.target_document_id &&
                  !gap.related_articles.some(
                    (article) =>
                      article.document_id === suggestion.target_document_id,
                  ) && (
                    <SelectItem value={suggestion.target_document_id}>
                      {suggestion.target_document_title || "Suggested article"}
                    </SelectItem>
                  )}
              </SelectContent>
            </Select>
          ) : (
            <div className="flex flex-col gap-2 sm:flex-row">
              <Select
                value={spaceId}
                onValueChange={onSpaceChange}
                disabled={applying}
              >
                <SelectTrigger
                  aria-label="Docs space"
                  className="w-full sm:flex-1"
                >
                  <SelectValue placeholder="Choose docs space" />
                </SelectTrigger>
                <SelectContent>
                  {spaces.map((space) => (
                    <SelectItem key={space.id} value={space.id}>
                      {space.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Select
                value={collectionId || "_root"}
                onValueChange={(value) =>
                  onCollectionChange(value === "_root" ? "" : value)
                }
                disabled={!spaceId || applying}
              >
                <SelectTrigger
                  aria-label="Collection"
                  className="w-full sm:flex-1"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="_root">Space root</SelectItem>
                  {collectionOptions.map((collection) => (
                    <SelectItem key={collection.id} value={collection.id}>
                      {collection.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}
          <p className="text-xs leading-5 text-muted-foreground">
            {route === "update_article"
              ? `Adds these sections to “${documentTitle}”. Existing content is preserved.`
              : "Creates a new draft in the selected location."}{" "}
            Publication stays under your control. The gap stays open for
            verification.
          </p>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              disabled={applying || !validDestination || !title.trim()}
              onClick={() => void save()}
            >
              {applying && <Loading01Icon className="animate-spin" />}
              {applying ? "Saving…" : "Save draft for review"}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={applying}
              onClick={() => onDiscard(suggestion.id)}
            >
              Discard
            </Button>
            <Button
              variant="link"
              size="sm"
              disabled={applying || !validDestination || !title.trim()}
              onClick={() => void saveAndOpen()}
            >
              Save and open editor
            </Button>
          </div>
        </div>
      )}
    </section>
  );
}
