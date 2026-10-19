# Inbox routing, AI replies and sender controls

Approved scope: combine rule-based and AI routing statuses; add an AI-reply column showing effective status; make sender addresses clickable with a new-address link. Work in waqar-fixes without subagents.

- Preserve routing badges, tooltips, warnings, inbox ordering, memberships, forwarding, archived states and existing assignments.
- AI replies inherits existing behavior unless explicitly disabled for an inbox. Show On, Off, Internal notes, Email only or Chat only as appropriate. Manual Ask AI remains available.
- Enforce the inbox policy before automatic processing and again under the publication lock; scheduled follow-ups recheck the same policy.
- Sender selection must require verified DNS and change only the selected inbox. Preserve other inbox assignments and the workspace default. New sender opens the existing sender setup screen.
- Verify persistence, suppression, sender reassignment, permissions, effective status, setup navigation and responsive rendering; run relevant backend and frontend checks.

Implemented in waqar-fixes. Both original routing-label explanations and badge tooltips are preserved; narrow containers use labelled rows without horizontal scrolling.

Validation: 41 focused frontend tests passed; support model/repository/service regression checks passed; TypeScript checking, Go vet/build and the frontend production build passed. New control files pass ESLint. The two existing inbox components retain the same 16 lint errors and one warning present in HEAD (React refresh/effect rules).

No live inbox settings were changed. Deployment remains pending. Existing inboxes continue their current AI behavior until explicitly disabled; the new boolean column is added by AutoMigrate with a false default.
