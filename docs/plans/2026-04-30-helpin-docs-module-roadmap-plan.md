# Helpin Docs module roadmap


This historical roadmap explains the April priorities for workspace documentation.
It is not the current release checklist: several items listed as remaining or
indefinitely deferred now have implementations.

## Current implementation and limits

Source-compared on 2026-09-18. These notes establish source availability, not
production rollout, universal permission correctness, or fresh test results.

- [AI-section regeneration](../../server/internal/service/docs_ai_section.go)
  starts a document-targeted agent run with output type
  `docs_ai_section_candidate` and includes `publish_ai_section_candidate` in its
  tool selection. Approval compares the candidate's source content against the
  current block before applying it. The remaining-work claim that generation
  still needs to become agent-backed is outdated; this does not prove every
  proposed provenance field or visual-diff requirement is complete.
- [Entity embeds](../../frontend/src/components/docs/EntityEmbedExtension.ts)
  include CRM deal, contact, and company types. The
  [renderer](../../frontend/src/components/docs/EntityEmbedNodeView.tsx) imports
  epic drawer navigation. CRM embed absence and mandatory full-page epic
  navigation should not be treated as current findings.
- [Saved-view embeds](../../frontend/src/components/docs/SavedViewEmbedNodeView.tsx)
  fetch PM, CRM, and support data. CRM/support options include predefined views,
  such as all deals or open conversations; this is not proof of full support for
  arbitrary user-created saved views in every module.
- [Task metadata controls](../../frontend/src/components/docs/TaskItemMetadataToolbar.tsx)
  store assignee IDs and due dates and construct PM task creation requests.
  The old metadata-only description is incomplete. Source inspection alone does
  not establish complete notification or conversion parity with PM checklists.
- [DocsEditor](../../frontend/src/components/docs/DocsEditor.tsx) registers an
  Excalidraw extension. The indefinitely-deferred whiteboard decision below has
  been superseded in code; its presence does not prove every export/indexing
  contract handles that node.

The original P0–P2 completion summary, library preferences, estimates of remaining
work, and deferred list are historical planning judgments. Use current source and
the [repository roadmap](../../ROADMAP.md) before choosing new work. No browser,
provider, agent-run, or application-test validation was performed for this review.

## Original roadmap

## Status

P0-P2 first implementation pass is complete. The remaining work is hardening and deepening the shipped slices rather than starting the module from scratch.

Implemented so far:

- AI section block with regeneration, candidate storage, preview, approve/reject, stale-candidate protection, docs/support source refs, activity logging, and `ai_section.regenerated` / `ai_section.approved` automation events.
- PM task/story, epic, and support conversation entity embeds.
- Doc body mentions with `@user`, `@team`, and `@agent` behavior; agent mentions can start document-targeted runs.
- Anchored doc comments for blocks/ranges, with replies and reactions.
- `doc`, `doc_block`, and `ai_section` automation target support plus `doc.published`.
- References panel for doc links, embeds, anchored comments, citations, and agent runs.
- Typed block registry and agent-readable block projections.
- Permission/redaction fields for entity and citation refs.
- Block and AI-section audit activity logging.
- Support coverage stale markers for docs/blocks.
- PM saved-view embed first pass.
- Semantic callouts, task-list metadata, toggles, image captions, file attachment blocks, inline table of contents, table sort/merge/split controls, and rich embeds.

Remaining hardening:

1. Make AI sections truly agent-owned: the agent run output should become the candidate, with stored `model`, `prompt_hash`, richer provenance, and a proper visual diff before approval.
2. Use a ProseMirror/Tiptap-aware diff for AI section review. Prefer `@tiptap/pm/changeset` because the frontend already depends on `@tiptap/pm` and Tiptap exposes the matching ProseMirror packages; use plain `diff`/jsdiff only for fallback text comparisons. Avoid the older `react-diff-viewer` package for the primary editor diff path because it is text-oriented and stale for React 19.
3. Fix epic embeds to open through the same drawer/navigation behavior as task/story embeds instead of full-page route transitions.
4. Add comment resolve/reopen and emit comment/block audit events such as `block.commented`.
5. Add CRM entity embeds for deal, contact, and company.
6. Upgrade saved-view embeds to render actual PM views, then add CRM/support saved views.
7. Harden permission-aware source rendering, especially support conversations or CRM records the viewer cannot access.
8. Upgrade task-list assignee/due-date metadata to real member IDs, picker UI, notification behavior, and optional PM task/story conversion.
9. Complete TOC sidebar behavior, row/column drag and CSV paste cleanup for tables, and richer backend-backed unfurls for embeds.
10. Ensure doc-targeted one-shot agent runs get the same useful tool surface that command runs get, including web search/research, entity lookup, PM/CRM/support/docs read tools, and any safe write tools needed to complete the requested task.

## Thesis

Helpin Docs should not try to become a generic Confluence or Notion clone first. The wedge is workspace-native, agent-assisted documentation that is connected to PM execution, CRM context, and support knowledge gaps.

The next roadmap should therefore ship one differentiated vertical slice early, then extract shared block infrastructure from the real requirements of that slice. The platform should be shaped by AI sections, entity embeds, comments, mentions, citations, and automation targets rather than designed abstractly before those features exist.

## Priority Roadmap

### P0: Differentiated Vertical Slice

1. **AI section block**
   - A region of a document owned by an agent.
   - Supports regenerate, generated candidate preview, diff, approval, rejection, and last-generated metadata.
   - Tracks enough provenance to answer who or what generated the section and from which sources.
   - Diff should be implemented against the structured TipTap/ProseMirror document when possible, not only markdown text.

2. **Entity embed blocks**
   - Ship PM task/story/epic and support conversation/ticket embeds in the first slice.
   - Push CRM deal/contact/company embeds to late-P0 or early-P1 so the first slice stays bounded.
   - Embeds should be live, permission-aware, click-through, and readable by agents.

3. **Inline comments and mentions**
   - Comments anchor to a block or selected text range.
   - Support replies, resolve/reopen, and lightweight reactions.
   - Mentions support `@user`, `@team`, and `@agent`, using the existing Helpin mention and notification patterns.

4. **Docs and blocks as agent and automation targets**
   - First-class targets: `doc`, `doc_block`, `ai_section`.
   - Initial events: `ai_section.regenerated`, `ai_section.approved`, and `doc.published`.
   - Later events such as `block.commented`, `block.stale`, and `doc.updated` should wait for P1 audit and review work.
   - Automation rules should be able to launch agents against a doc or block target using the existing generic target contract.

5. **Citations and sources for AI sections**
   - Scope this narrowly to AI-generated sections first.
   - Ship end-to-end citations for docs chunks and support conversations first.
   - Treat CRM activity, PM objects, and broader agent retrieval traces as follow-up source types.
   - Render provenance in the section UI without requiring a fully generic citation system on day one.

6. **Doc-level versioning plus section diff**
   - Keep existing document-level versioning as the primary rollback model.
   - For AI sections, store generated candidate content and approved content so users can diff the section without requiring full block-level versioning.

7. **Backlinks and references panel**
   - Show docs, PM objects, CRM records, support conversations, comments, citations, and agent runs that reference the current doc or block.
   - This should make the Helpin knowledge graph visible early.

### P1: Stabilize The Block Platform

1. **Typed block registry**
   - Extract after the first vertical slice proves the shape.
   - Registry entries should define kind, attrs schema, TipTap extension, renderer, serializer, search text, agent-readable form, permission behavior, and audit behavior.
   - Agent-readable form means a structured JSON projection with block kind, stable ID, relevant attrs, plain text, linked entity refs, citation refs, and action affordances. Markdown can be a display/export fallback, not the agent contract.
   - Migrate existing custom blocks into the registry incrementally: callout, image, video, HTML, code, table, entity embed, AI section.

2. **Permission-aware block rendering and agent-readable forms**
   - Blocks that reference PM, CRM, or support entities must degrade safely when the viewer lacks access.
   - Agents should receive structured block forms, not markdown-only text.

3. **Meaningful block audit events**
   - Track comment created/resolved, mention created, AI section regenerated/approved/rejected, entity linked/unlinked, block marked stale, and doc published.
   - Avoid exhaustive per-keystroke audit logs.

4. **Support-to-docs feedback loop**
   - First pass only marks relevant docs or blocks as stale from failed support answers, repeated ticket themes, or coverage gaps.
   - Suggestion creation and automatic AI section regeneration should follow after stale markers are visible and auditable.
   - Link stale markers back to the originating support evidence.

5. **Saved-view embed block**
   - Start narrower than a live query language.
   - Embed an existing PM, CRM, or support saved view by reference and render it as a table/list/card set.
   - Defer custom SQL or DSL until saved-view embeds prove demand.

### P2: Editor Completeness

1. **Semantic callouts**
   - Map current color variants to `info`, `tip`, `warning`, `danger`, and `success`.

2. **Task lists with assignee and due date**
   - Support check state, assignee, due date, and optional PM task/story conversion.
   - Mention and notification behavior should match PM checklist items where practical.

3. **Toggle and collapsible sections**
   - Useful for long agent-generated plans, specs, and support runbooks.

4. **Image captions and file attachment blocks**
   - Keep current image alt/alignment behavior.
   - Add first-class captions and generic file blocks for PDFs, CSVs, logs, screenshots, and documents.

5. **Table of contents**
   - Provide sidebar and inline variants backed by stable heading and block anchors.

6. **Tables v2**
   - Add merge/split cells, sorting, row/column drag, and better CSV paste cleanup if usage justifies the work.

7. **General rich embeds**
   - Expand beyond video to rich unfurls for tools such as Figma, GitHub, Linear, Loom, YouTube, and Notion.

## Integration Model

### Project Management

- Embed PM tasks, stories, epics, objectives, and sprints in docs.
- Convert doc task-list items into PM work items.
- Let specs, release notes, and planning docs publish status changes back to PM.
- Use doc comments and mentions as part of PM notification loops.

### CRM

- Embed contacts, companies, deals, buyer signals, summaries, and CRM activity.
- Generate deal briefs and account plans as AI sections.
- Cite customer evidence from emails, calendar events, notes, and support conversations.
- Let docs become reusable sales and success playbooks tied to live CRM data.

### Support

- Embed support tickets, conversations, coverage gaps, and help-center articles.
- Use repeated support issues to mark blocks stale or trigger AI section regeneration.
- Carry source provenance from support answers into docs.
- Let help-center publishing and support answer quality form a closed feedback loop.

### Agents And Automation

- Docs, blocks, and AI sections should be targetable by manual runs, automation rules, and scheduled maintenance agents.
- Doc-targeted one-shot agent runs should inherit the same practical tool access as command runs, including web search/research and workspace read tools, so agents can complete documentation tasks without being artificially blind.
- Agent action blocks should trigger Temporal workflows with audit logging.
- Automation events should be explicit and reusable rather than hidden inside editor-specific code paths.

## Deferred Or Cut

- **KaTeX/math**: cut until there is clear customer pull. It adds schema, rendering, indexing, export, and test surface.
- **Excalidraw/whiteboards**: defer indefinitely unless a strong workspace-planning use case appears.
- **Columns/grid layout**: defer indefinitely; lower leverage than entity embeds and AI sections for Helpin's wedge.
- **PlantUML, D2, draw.io**: defer. Mermaid plus future rich embeds should cover most near-term needs.
- **Custom live query DSL or SQL block**: defer. Saved-view embeds are the lower-risk first version.
- **Full block-level versioning**: defer. Use doc-level versions plus AI-section candidate/approval history first.

## Implementation Notes

- Do not start with a large abstract registry project. Build AI section, entity embed, comments, mentions, citations, and automation targeting first.
- Keep using the current addressable block model and compatibility document aggregate while the vertical slice is built.
- Extract the typed block registry once the first new native blocks reveal the required schema, render, serialization, permission, and agent-readable contracts.
- Treat markdown as an import/export format, not the canonical representation for Helpin-native blocks.
- Prefer saved references to existing PM/CRM/support objects over copied snapshots. Render snapshots only as fallbacks when permissions or deleted records require it.
- AI section provenance must be permission-aware. If a viewer cannot access a cited support conversation, CRM record, PM object, or source chunk, show a redacted source label and confidence/status metadata, but do not expose restricted text or links.
- AI section diffs should compare TipTap JSON / ProseMirror nodes for approval UI. `@tiptap/pm/changeset` is the preferred low-dependency option because it stays version-aligned with TipTap; `diff`/jsdiff is acceptable only for plain-text fallback summaries.
- One-shot document agents should use the same target-aware tool selection path as command-bar runs. When the target is a doc or block, include web search/research tools and PM/CRM/support/docs context tools according to permissions and the task prompt.

## Assumptions

- The first product milestone is internal docs plus workspace-native docs, not a public help-center editor refresh.
- PM and support embeds should ship before CRM embeds because they are closest to current docs, planning, and support coverage flows.
- The AI section block is the main differentiator and should drive requirements for citations, diffs, approvals, and registry extraction.
- The existing generic agent target contract should be extended to docs rather than adding bespoke docs-only launch paths.
