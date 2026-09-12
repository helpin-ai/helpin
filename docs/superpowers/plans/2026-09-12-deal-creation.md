# Deal creation implementation plan

Implement directly in waqar-fixes with no sub-agents. This is the single plan for the approved form.

Goal: all fields visible in this order: company, contacts, editable generated name, amount/currency/revenue type, pipeline/stage, owner/close date, probability; no heading subtext.

Architecture: reuse CRM associations for the customer, primary person and participants. Extend atomic deal creation for additional contact IDs and persist a revenue period. Use centralized Quiet dropdowns and underline inputs. Mount form state only while open to reset safely. Keep caller context explicit.

- [x] Add tests for atomic participant creation, workspace validation and revenue periods; implement backend DTO/model/service/repository and additive migration.
- [x] Build tested form defaults/revenue helpers and company/contact picker with inline creation, association lookup, stale-search protection and clear errors.
- [x] Replace CreateDealDialog with the approved visible form; inherit pipeline/stage from Deals and board columns; default owner to current member. Preserve edited names and explicit probability overrides.
- [x] Show/edit revenue period in deal details, calculate potential MRR/ARR, and support changing primary contact while retaining people.
- [x] Verify targeted Go and frontend tests, TypeScript/lint and rendered form interactions. Review diff and commit only task files.

Compatibility: preserve custom properties, company-only and independent-person deals, existing creation callbacks, toasts and routes. Existing amounts retain one-time semantics. The app currently has no workspace currency preference: preserve USD fallback and remember the user's last explicit currency per workspace.


Follow-up scope approved in the same task: match task state controls and reliable moves, and add editable stage colors.

- [ ] Add persisted stage colors with the task color picker, safe defaults and color validation. Preserve colors through stage edits, reorders and pipeline duplication.
- [ ] Reuse the task state content and picker for deal stage selection; show stage colors in board/list/detail/create/settings.
- [ ] Reuse task board drop-target and commit helpers; isolate optimistic rollback per deal and guard in-flight/context changes. Verify successful, failed and concurrent moves.
- [ ] Verify stage UI and transitions, then commit the follow-up changes.

Verified creation: 18 frontend unit checks, 7 browser cases (including task-style toast/Open and inline contact retry), CRM service tests, repository/dbmigrate tests, TypeScript, new-form lint, and a disposable PostgreSQL migration check. Existing board/card/table lint findings are being assessed in the follow-up stage work. Toolbar from waqar-images/pipeline.png is hidden for workspaces with zero deals.
