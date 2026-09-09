# Settings save feedback implementation plan

Goal: Keep settings save controls and feedback clear of the Helpin launcher without changing transactional settings into autosave.

Approved approach: Shared save feedback near panel headings; top-center global toasts; preserve explicit Save for related configuration and consequential actions. No broad autosave conversion in this change.

Audit:
- ChatGeneralTab (Chat Widget and AI Assistant): fixed bottom/right autosave status; debounced callbacks may overlap; failures have only a toast; query refresh can overwrite edits.
- ConversationRoutingTab: fixed bottom/right explicit save bar.
- CRMEmailSettingsTab and CRMMeetingSettingsTab: explicit bottom-of-form save rows.
- HelpcenterTab: existing StickyFormFooter already at top; retain API with shared presentation.
- GeneralTab: explicit workspace identity save inside its card; preserve its confirmation behavior.
- Global Sonner: default bottom-right overlaps launcher, affects all toast users.

Steps:
1. Add SettingsSaveBar and SettingsSaveStatus with sticky top placement, live status, wrapping layout, persistent error and optional Retry. Retain StickyFormFooter as compatibility wrapper. Test semantics.
2. Move manual routing and CRM save controls to shared top bars. Retain callbacks, permission checks, dirty checks and submission behavior. Inspect General identity card placement; relocate only if needed, preserving its confirmation dialog.
3. Replace ChatGeneralTab's fixed indicator with shared status, serialize debounced mutations, keep current edits during background refresh/in-flight saves, report success only after persistence, and support retry. Preserve existing 800ms text debounce. Remove premature AI toggle success toasts.
4. Set global toaster top-center, preserving caller overrides.
5. Test autosave initialization, edit coalescing, in-flight edits, failure/retry, successful server normalization and cleanup. Run settings tests and frontend TypeScript; inspect responsive layout/browser if feasible. Review scoped diff and commit only task files.

Validation limits: Do not modify unrelated CRM playbook changes in this worktree. Do not push or deploy without a subsequent instruction.

Completed:
- Added shared sticky save bar and accessible save status; migrated the audited manual controls and existing autosave feedback.
- Autosave serializes requests and keeps newer edits, persistent errors, and Retry. Navigation waits for persistence; browser unload warns about unsaved changes. Successful mutation responses seed the query cache before invalidation completes.
- Global toasts default to top-center. Existing explicit-save behavior is preserved.
- Validation: 95 tests across 25 settings/autosave test files passed; frontend `tsc -b --pretty false` passed; `git diff --check` passed.
- Browser validation used a local preview of the actual shared save/status/toaster components with a simulated bottom-right launcher, at 1440×1000 and 390×844, including scrolling. This was not a production workspace test. Temporary preview files were removed.
- Follow-up review found no remaining blockers after adding navigation protection and cache refresh handling.
