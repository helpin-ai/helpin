# Historical conversation triage and routing scenarios

This checklist describes an earlier routing UI. Use it as a source of test ideas,
not as current click-by-click instructions or evidence that the scenarios pass.
Source comparison on 2026-09-17 found the following changes:

- The inbox wizard has Details, Members & Assignment, and Routing steps, followed
  by a completion screen. Routing contains rule-based conditions and an **AI
  routing** switch; the old “Eligible for AI Routing” label is obsolete.
- **Automated routing** is the global setting. The current UI explains that it
  checks manual rules first and then AI. Turning it off disables both; the inbox
  dialog allows saving rules and prompts while displaying a warning that they
  will not run.
- Current conditions include message text, sender email, and email domain. The
  rule-card layout, section order, labels, and validation messages in the older
  scenarios below must be updated when turning them into executable tests.

Use the [inbox wizard](../frontend/src/components/support/TeamInboxDialog.tsx),
[step definitions](../frontend/src/components/support/teamInboxDialogFlow.ts), and
[routing settings](../frontend/src/components/settings/ConversationRoutingTab.tsx)
as the current UI references. This source review did not execute the manual
scenarios, verify model-generated routing outcomes, or validate their remaining
backend expectations.


## Prerequisites
- A workspace with at least 2 team inboxes created (e.g. "Sales", "Technical Support")
- Each inbox should have a routing prompt configured (Step 3 of inbox creation)
- At least one workspace member assigned to each inbox

---

## A. Team Inbox Creation (3-Step Wizard)

### A1. Create inbox with routing prompt
1. Go to Settings > Inboxes & Routing > Inbox Catalog
2. Create a new team inbox
3. **Step 1 (Details):** Name it "Billing", handle auto-generates to `#billing`, add description
4. **Step 2 (Members):** Add 2 members
5. **Step 3 (Routing):** Enable "Eligible for AI Routing", set routing prompt to: *"Billing questions, refund requests, payment issues, invoice disputes, and subscription changes."*
6. Click "Create Inbox"
- **Expected:** Inbox appears in Inbox Catalog with routing prompt visible. Shows "Yes" under Eligible column.

### A2. Create manual-only inbox (no AI routing)
1. Create inbox "Internal Escalations"
2. In Step 3, disable "Eligible for AI Routing"
- **Expected:** Routing prompt field is hidden. Inbox appears in catalog with "No" under Eligible. AI triage will never route to this inbox.

### A3. Edit existing inbox routing
1. Click Edit on an existing inbox in the Inbox Catalog
2. Navigate to Step 3, change the routing prompt
3. Save
- **Expected:** Updated prompt shows in the catalog table

---

## B. Routing Rules (Deterministic)

### B1. Create a keyword-based rule
1. Go to Routing Rules section, click "+ New Rule"
2. Name: "Refund requests", Priority: 1, Target: Billing inbox
3. Under Conditions > Text Contains, type `refund` and press Enter, then `money back` and press Enter
4. Click "Create Rule"
- **Expected:** Rule card appears with priority badge "1", chips showing `refund` and `money back`, active status dot

### B2. Create a domain-based rule
1. Create rule: "Enterprise clients", Priority: 2, Target: Sales inbox
2. Under Email Domain Equals, add `bigcorp.com` and `enterprise.co`
3. Create
- **Expected:** Rule card shows domain chips. Any email from those domains routes to Sales.

### B3. Create a combined rule (keyword + domain)
1. Create rule with both Text Contains (`partnership`) AND Email Domain Equals (`agency.com`)
2. Create
- **Expected:** Rule only matches when BOTH conditions are true (AND logic)

### B4. Rule priority ordering
1. Create Rule A: Priority 0, Text Contains: `urgent`, Target: Technical Support
2. Create Rule B: Priority 1, Text Contains: `urgent`, Target: Sales
3. Send a conversation containing "urgent"
- **Expected:** Routes to Technical Support (Rule A wins because priority 0 < 1)

### B5. Disable a rule
1. Edit an existing rule
2. Toggle "Rule Enabled" off at the bottom of the dialog
3. Save
- **Expected:** Rule card shows grey inactive dot. Rule is skipped during evaluation. A conversation that previously matched this rule now falls through to next rule or AI triage.

### B6. Delete a rule
1. Click trash icon on a rule card
2. Confirm deletion
- **Expected:** Rule removed immediately. Conversations no longer match this rule.

### B7. Edit rule chips
1. Edit an existing rule
2. Remove a keyword chip by clicking the X on it
3. Add new keywords
4. Save
- **Expected:** Updated chips show on the rule card

### B8. Validation
1. Try creating a rule with no name
- **Expected:** Toast error "Rule name is required"
2. Try creating a rule with no conditions (no keywords, no domains)
- **Expected:** Toast error "Add at least one condition"
3. Try creating a rule with no target inbox
- **Expected:** Toast error "Choose a target inbox"

---

## C. AI Triage Settings

### C1. Enable AI triage
1. Expand the AI Triage section
2. Toggle "Enable AI Triage" on
3. Verify Triage Channels checkboxes appear (Widget, Email, Internal & API)
4. Save
- **Expected:** Floating save bar appears. After save, AI triage is active for new conversations.

### C2. Confidence threshold
1. Set threshold slider to 70%
2. Save
- **Expected:** AI suggestions with confidence >= 70% qualify for auto-move. Lower confidence suggestions are shown but not auto-moved.

### C3. Auto-move enabled
1. Enable AI Triage + Auto-Move
2. Set confidence threshold to 90%
3. Send a conversation that clearly matches one inbox's routing prompt
- **Expected:** Conversation automatically moves to the matched inbox without agent intervention. Triage status = `auto_moved`.

### C4. Auto-move disabled (suggestion mode)
1. Enable AI Triage, disable Auto-Move
2. Send a conversation
- **Expected:** Conversation stays in shared inbox. Agent sees a triage suggestion banner with the recommended inbox. Triage status = `suggested`.

### C5. Dismiss a suggestion
1. With a suggested triage on a conversation, click dismiss
- **Expected:** Suggestion disappears. Triage status = `dismissed`. Won't re-suggest for this conversation.

### C6. Override a suggestion
1. With a suggested triage, manually move the conversation to a different inbox than suggested
- **Expected:** Triage status = `overridden`.

### C7. Channel filtering
1. Enable AI Triage, check only "Widget", uncheck "Email"
2. Send an email conversation and a widget conversation
- **Expected:** Widget conversation gets triaged. Email conversation does NOT get triaged (stays in shared inbox).

---

## D. Advanced Settings

### D1. Fallback behavior - Shared
1. Set fallback to "Keep in Shared Inbox"
2. Send a conversation that doesn't match any rule and AI confidence is below threshold
- **Expected:** Conversation stays in Shared Inbox

### D2. Fallback behavior - Default
1. Set fallback to "Keep in Default Inbox"
2. Same test
- **Expected:** Conversation goes to the default team inbox

### D3. Daily AI budget
1. Set daily budget to 3
2. Send 5 conversations
- **Expected:** First 3 get AI triage. Last 2 fall to fallback behavior without AI classification.

### D4. Skip spam conversations
1. Enable "Skip Spam Conversations"
2. Mark a conversation as spam, then have a new similar message come in
- **Expected:** Spam-flagged conversations don't consume AI budget

### D5. Deduplicate first messages
1. Enable "Deduplicate First Messages"
2. Send 3 identical messages in quick succession
- **Expected:** Only 1 AI classification runs. All 3 get the same routing result.

### D6. Re-run on meaning change
1. Enable "Re-Run When Meaning Changes"
2. A conversation starts about billing (routed to Billing inbox)
3. Follow-up message changes topic to a technical issue
- **Expected:** AI re-classifies and suggests moving to Technical Support

### D7. Re-run disabled
1. Disable "Re-Run When Meaning Changes"
2. Same scenario
- **Expected:** Conversation stays in Billing inbox regardless of topic change

---

## E. Pipeline Integration (Rules + AI)

### E1. Rule match bypasses AI
1. Create a rule: Text Contains `refund`, Target: Billing
2. Enable AI Triage
3. Send "I want a refund for my last order"
- **Expected:** Routed by RULE to Billing. AI triage does NOT run (no budget consumed). Triage source = `rule`.

### E2. No rule match falls through to AI
1. No rules match the conversation
2. AI Triage is enabled
3. Send "How do I integrate your API with my React app?"
- **Expected:** AI triage runs, matches Technical Support inbox based on routing prompt. Triage source = `ai`.

### E3. No rule match, AI disabled
1. Disable AI Triage
2. No matching rules
3. Send a conversation
- **Expected:** Conversation stays in shared/default inbox per fallback setting. No triage runs.

### E4. No rule match, AI low confidence
1. AI Triage enabled, threshold at 90%
2. Send an ambiguous message like "Hello"
- **Expected:** AI confidence likely below 90%. Falls back to shared/default inbox.

### E5. Multiple inboxes with overlapping prompts
1. Inbox A prompt: "Technical questions about APIs and integrations"
2. Inbox B prompt: "Developer onboarding and API getting started guides"
3. Send: "How do I get started with your API?"
- **Expected:** AI picks the best match based on semantic similarity. Verify the confidence score makes sense.

---

## F. UI/UX Verification

### F1. Pipeline banner
- **Expected:** Top of routing page shows: "How routing works: Rules -> AI Triage -> Fallback Inbox"

### F2. Section ordering
- **Expected:** Sections appear in order: Routing Rules, AI Triage, Inboxes & Routing Prompts, Advanced Settings

### F3. Chip input interactions
1. Type a keyword, press Enter -> chip added
2. Type with comma -> chip added
3. Paste "refund, invoice, billing" -> 3 chips added
4. Click X on chip -> removed
5. Press Backspace in empty input -> last chip removed
- **Expected:** All interactions work smoothly

### F4. Rule card display
- **Expected:** Each rule shows: priority badge, name, active dot, target inbox, condition chips (outline style, read-only)

### F5. Unsaved changes bar
1. Toggle any AI Triage setting
- **Expected:** Floating "Unsaved changes" bar appears at bottom-right with Save button

### F6. Empty state
- **Expected:** With no rules, shows "No routing rules yet" with sparkles icon

### F7. Settings sidebar icon
- **Expected:** "Inboxes & Routing" shows the Route icon (not MessageSquare)
