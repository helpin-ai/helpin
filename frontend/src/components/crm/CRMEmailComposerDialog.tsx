import { EmailTemplatePicker } from "./outreach/EmailTemplatePicker";
import { EmailTemplateEditor } from "./outreach/EmailTemplateEditor";
import { crmOutreachService } from "@/lib/services/crmOutreachService";
import { useEffect, useMemo, useRef, useState } from "react";
import { EditorContent, useEditor } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { ArrowLeft02Icon, Mail01Icon } from "@/lib/icons";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  QuietSelect,
  QuietUnderlineInput,
} from "@/components/design-system/quiet";
import { ConfirmDialog } from "@/components/pm/ConfirmDialog";
import {
  EmailChipInput,
  classifyEmailChipInput,
} from "@/components/ui/email-chip-input";
import { useWorkspaceStore } from "@/stores/workspaceStore";
import { emailTextToHTML, withEmailSignature } from "./emailComposition";
import { EmojiPicker } from "@/components/support/EmojiPicker";
import { LinkInsertModal } from "@/components/support/LinkInsertModal";
import type { CRMEmailAccount } from "@/lib/crmTypes";
import { crmEmailService } from "@/lib/services/crmService";
import { unwrap } from "@/lib/queryUtils";
import {
  QuietComposerAITools,
  QuietComposerEditorSurface,
  QuietComposerToolbar,
  QuietConversationComposer,
  type ConversationRewriteOperation,
} from "@/components/design-system/quiet";
import { useCRMEmailAttachments } from "@/hooks/useCRMEmailAttachments";
import { CRMEmailAttachmentStrip } from "./CRMEmailAttachmentStrip";
import { UpgradeRequiredDialog } from "@edition";
import {
  getUpgradeRequiredReason,
  type UpgradeRequiredReason,
} from "@edition/errors";

export interface EmailDraft {
  title?: string;
  dealId?: string;
  dealName?: string;
  to?: string[];
  cc?: string[];
  subject?: string;
  body?: string;
}

interface CRMEmailComposerDialogProps {
  workspaceId: string;
  accounts: CRMEmailAccount[];
  open: boolean;
  draft?: EmailDraft;
  onOpenChange: (open: boolean) => void;
}

export function CRMEmailComposerDialog({
  workspaceId,
  accounts,
  open,
  draft,
  onOpenChange,
}: CRMEmailComposerDialogProps) {
  const queryClient = useQueryClient();
  const availableAccounts = useMemo(
    () =>
      accounts.filter(
        (account) =>
          account.can_send === true &&
          account.is_active &&
          account.status === "connected" &&
          account.provider === "gmail",
      ),
    [accounts],
  );
  const [accountId, setAccountId] = useState(availableAccounts[0]?.id ?? "");
  const [to, setTo] = useState<string[]>(draft?.to ?? []);
  const [toInput, setToInput] = useState("");
  const [cc, setCC] = useState<string[]>(draft?.cc ?? []);
  const [ccInput, setCCInput] = useState("");
  const [discardOpen, setDiscardOpen] = useState(false);
  const [includeSignature, setIncludeSignature] = useState(true);
  const sendLock = useRef(false);
  const sendButton = useRef<HTMLButtonElement>(null);
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const [showCC, setShowCC] = useState(Boolean(draft?.cc?.length));
  const [saveTemplateOpen, setSaveTemplateOpen] = useState(false);
  const [replacement, setReplacement] = useState<{
    subject: string;
    body_html: string;
  }>();
  const [templateLoading, setTemplateLoading] = useState(false);
  const [subject, setSubject] = useState(draft?.subject ?? "");
  const [bodyHTML, setBodyHTML] = useState(() =>
    emailTextToHTML(draft?.body ?? ""),
  );
  const [bodyText, setBodyText] = useState(draft?.body ?? "");
  const [sending, setSending] = useState(false);
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkInitial, setLinkInitial] = useState({ label: "", url: "" });
  const [focused, setFocused] = useState(false);
  const [rewriting, setRewriting] = useState(false);
  const emailAttachments = useCRMEmailAttachments(workspaceId);
  const [upgradeReason, setUpgradeReason] =
    useState<UpgradeRequiredReason | null>(null);

  const extensions = useMemo(
    () => [
      StarterKit.configure({
        heading: false,
        codeBlock: false,
        horizontalRule: false,
        link: {
          openOnClick: false,
          autolink: true,
          linkOnPaste: true,
          HTMLAttributes: {
            target: "_blank",
            rel: "noopener noreferrer nofollow",
          },
        },
      }),
      Placeholder.configure({ placeholder: "Write your message..." }),
    ],
    [],
  );

  const editor = useEditor({
    extensions,
    content: emailTextToHTML(draft?.body ?? ""),
    editable: !sending,
    immediatelyRender: false,
    editorProps: {
      attributes: {
        class:
          "rich-text-soft prose prose-sm dark:prose-invert max-w-none min-h-[220px] max-h-[45vh] overflow-y-auto px-4 py-3 text-sm leading-relaxed focus:outline-none",
      },
      handleKeyDown: (_view, event) => {
        if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
          event.preventDefault();
          sendButton.current?.click();
          return true;
        }
        return false;
      },
    },
    onUpdate: ({ editor: currentEditor }) => {
      setBodyHTML(currentEditor.getHTML());
      setBodyText(currentEditor.getText());
    },
    onFocus: () => setFocused(true),
    onBlur: () => setFocused(false),
  });

  useEffect(() => {
    editor?.setEditable(!sending && !templateLoading);
  }, [editor, sending, templateLoading]);

  const effectiveAccountId = availableAccounts.some(
    (account) => account.id === accountId,
  )
    ? accountId
    : (availableAccounts[0]?.id ?? "");

  const signature = availableAccounts.find(
    (account) => account.id === effectiveAccountId,
  )?.signature;
  const dirty =
    bodyHTML !== emailTextToHTML(draft?.body ?? "") ||
    subject !== (draft?.subject ?? "") ||
    to.join(",") !== (draft?.to ?? []).join(",") ||
    cc.join(",") !== (draft?.cc ?? []).join(",") ||
    Boolean(toInput || ccInput) ||
    emailAttachments.attachments.length > 0;
  useEffect(() => {
    if (!dirty && !sending) return;
    const protect = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("beforeunload", protect);
    return () => window.removeEventListener("beforeunload", protect);
  }, [dirty, sending]);
  const openLinkModal = () => {
    if (!editor) return;
    const attrs = editor.getAttributes("link") as { href?: string };
    const { from, to: selectionTo, empty } = editor.state.selection;
    let label = "";
    if (editor.isActive("link")) {
      editor.chain().focus().extendMarkRange("link").run();
      const expanded = editor.state.selection;
      label = editor.state.doc.textBetween(expanded.from, expanded.to, " ");
    } else if (!empty) {
      label = editor.state.doc.textBetween(from, selectionTo, " ");
    }
    setLinkInitial({ label, url: attrs.href ?? "" });
    setLinkOpen(true);
  };

  const insertLink = (label: string, url: string) => {
    if (!editor) return;
    if (editor.isActive("link")) {
      editor.chain().focus().extendMarkRange("link").unsetLink().run();
    }
    const { from, to: selectionTo, empty } = editor.state.selection;
    if (empty) {
      editor
        .chain()
        .focus()
        .insertContent({
          type: "text",
          text: label,
          marks: [{ type: "link", attrs: { href: url } }],
        })
        .run();
      return;
    }
    editor
      .chain()
      .focus()
      .insertContentAt(
        { from, to: selectionTo },
        {
          type: "text",
          text: label,
          marks: [{ type: "link", attrs: { href: url } }],
        },
      )
      .run();
  };

  const send = async () => {
    if (
      sendLock.current ||
      emailAttachments.uploading ||
      emailAttachments.hasFailedUploads ||
      rewriting ||
      templateLoading ||
      replacement
    )
      return;
    const pendingTo = classifyEmailChipInput(toInput);
    const pendingCC = classifyEmailChipInput(ccInput);
    if (pendingTo.invalid.length || pendingCC.invalid.length) {
      toast.error("Enter valid email addresses");
      return;
    }
    const recipients = [...new Set([...to, ...pendingTo.valid])];
    if (subject.includes("{{") || bodyHTML.includes("{{")) {
      toast.error("Resolve the template variables before sending.");
      return;
    }
    const trimmedSubject = subject.trim();
    if (
      !effectiveAccountId ||
      recipients.length === 0 ||
      !trimmedSubject ||
      !bodyText.trim()
    ) {
      toast.error("Choose a sender and add a recipient, subject, and message");
      return;
    }

    sendLock.current = true;
    setSending(true);
    try {
      const result = unwrap(
        await crmEmailService.sendEmail(workspaceId, {
          account_id: effectiveAccountId,
          to: recipients,
          cc: [...new Set([...cc, ...pendingCC.valid])],
          deal_id: draft?.dealId,
          subject: trimmedSubject,
          body_html: withEmailSignature(
            bodyHTML,
            includeSignature ? signature : undefined,
          ),
          draft_id: emailAttachments.draftId,
          attachment_ids: emailAttachments.attachmentIds,
        }),
      );
      await queryClient.invalidateQueries({ queryKey: ["crm", workspaceId] });
      if (result.association_warning) toast.warning(result.association_warning);
      else toast.success("Email sent");
      emailAttachments.reset();
      onOpenChange(false);
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : "Email could not be sent",
      );
    } finally {
      sendLock.current = false;
      setSending(false);
    }
  };

  const canSend = Boolean(
    effectiveAccountId &&
    (to.length > 0 || classifyEmailChipInput(toInput).valid.length > 0) &&
    !classifyEmailChipInput(toInput).invalid.length &&
    !classifyEmailChipInput(ccInput).invalid.length &&
    !rewriting &&
    subject.trim() &&
    bodyText.trim() &&
    !emailAttachments.uploading &&
    !emailAttachments.hasFailedUploads &&
    !sending &&
    !templateLoading &&
    !replacement,
  );
  const discard = () => {
    emailAttachments.reset();
    onOpenChange(false);
  };
  const closeComposer = () => {
    if (sendLock.current || rewriting || emailAttachments.uploading) return;
    if (dirty) setDiscardOpen(true);
    else discard();
  };

  const rewrite = async (operation: ConversationRewriteOperation) => {
    if (
      !editor ||
      !bodyText.trim() ||
      rewriting ||
      templateLoading ||
      replacement
    )
      return;
    setRewriting(true);
    try {
      const result = unwrap(
        await crmEmailService.rewriteDraft(workspaceId, bodyHTML, operation),
      );
      editor.commands.setContent(result.content);
      editor.commands.focus("end");
    } catch (error) {
      const reason = getUpgradeRequiredReason(error);
      if (reason) setUpgradeReason(reason);
      else
        toast.error(
          error instanceof Error
            ? error.message
            : "Email could not be rewritten",
        );
    } finally {
      setRewriting(false);
    }
  };

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) closeComposer();
        }}
      >
        <DialogContent className="max-h-[92vh] w-[min(760px,calc(100vw-2rem))] max-w-none gap-0 overflow-y-auto p-0 sm:max-w-none">
          <DialogHeader className="border-b border-border/60 px-5 py-4 text-left">
            <DialogTitle className="flex items-center gap-2 text-sm">
              <Mail01Icon className="h-4 w-4 text-muted-foreground" />
              {draft?.title ?? "New email"}
            </DialogTitle>
            <DialogDescription
              className={draft?.dealName ? "text-xs" : "sr-only"}
            >
              {draft?.dealName
                ? `Linked to ${draft.dealName}`
                : "Compose an email from your connected mailbox."}
            </DialogDescription>
          </DialogHeader>

          <div className="min-w-0 divide-y divide-border/60">
            <div className="grid grid-cols-[64px_minmax(0,1fr)] items-center gap-2 px-5 py-2.5">
              <span className="text-xs font-medium text-muted-foreground">
                From
              </span>
              <QuietSelect
                label="From"
                value={effectiveAccountId}
                onChange={setAccountId}
                disabled={sending || templateLoading}
                options={availableAccounts.map((account) => ({
                  value: account.id,
                  label: account.email_address,
                }))}
              />
            </div>

            <div className="grid grid-cols-[64px_minmax(0,1fr)_auto] items-center gap-2 px-5 py-2.5">
              <span className="text-xs font-medium text-muted-foreground">
                To
              </span>
              <EmailChipInput
                ariaLabel="To"
                value={to}
                onValueChange={setTo}
                inputValue={toInput}
                onInputValueChange={setToInput}
                placeholder="name@company.com"
                className="min-w-0 rounded-none border-0 bg-transparent p-0 shadow-none focus-within:ring-0"
                disabled={sending || templateLoading}
              />
              {!showCC && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs text-muted-foreground"
                  onClick={() => setShowCC(true)}
                >
                  Cc
                </Button>
              )}
            </div>

            {showCC && (
              <div className="grid grid-cols-[64px_minmax(0,1fr)] items-center gap-2 px-5 py-2.5">
                <span className="text-xs font-medium text-muted-foreground">
                  Cc
                </span>
                <EmailChipInput
                  ariaLabel="Cc"
                  value={cc}
                  onValueChange={setCC}
                  inputValue={ccInput}
                  onInputValueChange={setCCInput}
                  placeholder="name@company.com"
                  className="min-w-0 rounded-none border-0 bg-transparent p-0 shadow-none focus-within:ring-0"
                  disabled={sending || templateLoading}
                />
              </div>
            )}

            <div className="grid grid-cols-[64px_minmax(0,1fr)] items-center gap-2 px-5 py-2.5">
              <span className="text-xs font-medium text-muted-foreground">
                Subject
              </span>
              <QuietUnderlineInput
                aria-label="Subject"
                value={subject}
                onChange={(event) => setSubject(event.target.value)}
                placeholder="Email subject"
                className="h-8 min-w-0 border-b-transparent px-0 font-medium"
                disabled={sending || templateLoading}
              />
            </div>
          </div>

          {availableAccounts.length === 0 ? (
            <div className="mx-5 mt-4 rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm text-muted-foreground">
              Connect your Gmail account to send emails.
              {workspace?.slug && (
                <a
                  className="ml-2 underline"
                  href={`/w/${workspace.slug}/settings/crm-email`}
                >
                  Connect mailbox
                </a>
              )}
            </div>
          ) : null}

          {editor ? (
            <QuietConversationComposer
              focused={focused}
              className="m-5 min-w-0"
            >
              <div className="flex flex-wrap items-center gap-2 px-3 pt-2">
                <EmailTemplatePicker
                  workspaceId={workspaceId}
                  disabled={sending || templateLoading}
                  onSelect={async (template) => {
                    setTemplateLoading(true);
                    try {
                      const recipients = to;
                      if (recipients.length !== 1)
                        throw new Error(
                          "Choose one recipient before personalizing a template.",
                        );
                      const rendered = unwrap(
                        await crmOutreachService.renderTemplate(
                          workspaceId,
                          template.id,
                          {
                            email: recipients[0],
                            account_id: effectiveAccountId,
                            deal_id: draft?.dealId,
                          },
                        ),
                      );
                      if (bodyText.trim() || subject.trim())
                        setReplacement(rendered);
                      else {
                        setSubject(rendered.subject);
                        editor.commands.setContent(rendered.body_html);
                        setBodyHTML(editor.getHTML());
                        setBodyText(editor.getText());
                      }
                    } catch (error) {
                      toast.error(
                        error instanceof Error
                          ? error.message
                          : "Could not apply template",
                      );
                    } finally {
                      setTemplateLoading(false);
                    }
                  }}
                />
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={sending || !bodyText.trim() || !subject.trim()}
                  onClick={() => setSaveTemplateOpen(true)}
                >
                  Save template
                </Button>
                <QuietComposerAITools
                  disabled={!bodyText.trim()}
                  pending={rewriting}
                  onSelect={rewrite}
                />
              </div>
              <QuietComposerEditorSurface>
                <EditorContent editor={editor} />
              </QuietComposerEditorSurface>
              {signature && (
                <div className="px-4 pb-3 text-sm">
                  <label className="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
                    <input
                      type="checkbox"
                      checked={includeSignature}
                      onChange={(event) =>
                        setIncludeSignature(event.target.checked)
                      }
                      disabled={sending || templateLoading}
                    />
                    Include signature
                  </label>
                  {includeSignature && (
                    <div className="whitespace-pre-wrap break-words">
                      {signature}
                    </div>
                  )}
                </div>
              )}
              <CRMEmailAttachmentStrip
                attachments={emailAttachments.attachments}
                onRemove={(id) => void emailAttachments.remove(id)}
              />
              <QuietComposerToolbar
                editor={editor}
                emoji={
                  <EmojiPicker
                    onEmojiSelect={(emoji) =>
                      editor?.chain().focus().insertContent(emoji).run()
                    }
                    side="top"
                  />
                }
                onLink={openLinkModal}
                onAttach={emailAttachments.pickFiles}
                trailing={
                  <Button
                    className="hidden sm:inline-flex"
                    variant="ghost"
                    size="sm"
                    onClick={closeComposer}
                    disabled={sending || templateLoading}
                  >
                    <ArrowLeft02Icon className="h-3.5 w-3.5" />
                    Cancel
                  </Button>
                }
                onSubmit={() => void send()}
                submitLabel="Send"
                submitDisabled={!canSend}
                submitting={sending}
              />
              <button
                ref={sendButton}
                disabled={!canSend}
                type="button"
                className="hidden"
                onClick={() => void send()}
              />
            </QuietConversationComposer>
          ) : null}

          <LinkInsertModal
            open={linkOpen}
            onOpenChange={setLinkOpen}
            workspaceId={workspaceId}
            initialLabel={linkInitial.label}
            initialUrl={linkInitial.url}
            onInsert={insertLink}
            onRemove={
              editor?.isActive("link")
                ? () =>
                    editor
                      .chain()
                      .focus()
                      .extendMarkRange("link")
                      .unsetLink()
                      .run()
                : undefined
            }
          />
        </DialogContent>
      </Dialog>
      {saveTemplateOpen && (
        <EmailTemplateEditor
          workspaceId={workspaceId}
          initial={{ subject, body_html: bodyHTML }}
          onClose={() => setSaveTemplateOpen(false)}
        />
      )}
      <ConfirmDialog
        open={Boolean(replacement)}
        onOpenChange={(open) => !open && setReplacement(undefined)}
        title="Replace this draft?"
        description="The template will replace your current subject and message."
        confirmLabel="Use template"
        cancelLabel="Keep draft"
        onConfirm={() => {
          if (!replacement || !editor) return;
          setSubject(replacement.subject);
          editor.commands.setContent(replacement.body_html);
          setBodyHTML(editor.getHTML());
          setBodyText(editor.getText());
          setReplacement(undefined);
        }}
      />
      <ConfirmDialog
        open={discardOpen}
        onOpenChange={setDiscardOpen}
        title="Discard this email?"
        description="Your message and attachments will be removed."
        confirmLabel="Discard email"
        cancelLabel="Keep writing"
        onConfirm={discard}
      />
      <UpgradeRequiredDialog
        open={upgradeReason !== null}
        onOpenChange={(dialogOpen) => {
          if (!dialogOpen) setUpgradeReason(null);
        }}
        reason={upgradeReason}
      />
    </>
  );
}
