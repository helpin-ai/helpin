# Linear Notification System — Research

> **Status: historical competitor research.** This page preserves design input,
> not Helpin's implemented feature set or a current verification of the vendor's
> product. Vendor limits, settings, shortcuts, and integrations may have changed.
> Consult the [notification index](README.md) for Helpin's current code entry points.

## 1. Events That Trigger Notifications

Linear generates notifications for subscribed issues. A user is **automatically subscribed** to an issue when they:
- Create the issue
- Are assigned to the issue
- Are @mentioned in the issue description or a comment
- Manually subscribe via the Activity section menu or `Shift S`

If @mentioned in a **comment thread**, the user is subscribed to that thread only — not the overall issue.

### Notification event types (per-channel toggleable):

**Issue-level events:**
- Issue assigned to you
- Issue marked urgent or breaching SLA
- Status change (grouped with priority change and blocking-relationship change — these cannot be separated)
- Comment added to a subscribed issue
- @mention in issue description or comment
- Emoji reaction to your comment
- Issue completed or canceled
- Issue auto-closed by Linear

**Project-level events (personal project notifications):**
- New issue created in a subscribed project
- Comment or change to the project description
- Issue in the project marked as completed or canceled
- New project update posted

**Initiative/Project updates:**
- Structured health-indicator updates posted by project/initiative leads
- Configurable reminders for leads to post updates at a cadence (weekly, biweekly, etc.)

**Team-level events (Slack channel notifications):**
- Issues created in that team
- New comments on team issues
- Status updates on team issues

**View subscriptions (Slack):**
- Issue added to a view
- Issue completed/canceled in a view

---

## 2. Notification Preferences & Settings Granularity

Settings location: **Settings > Account > Notifications**

### Channel-level control
Each notification **channel** (Desktop, Mobile, Email, Slack) can be independently enabled/disabled. A green dot = enabled, gray dot = disabled.

### Per-channel, per-event-type toggles
Within each channel, users see a list of notification event types they can selectively toggle on/off. For example:
- Receive Slack notifications only when assigned an issue
- Receive Slack + Desktop + Email when an assigned issue is marked urgent or blocking
- Turn off email for comments but keep status change notifications via email
- Enable/disable @mention notifications per channel

### Notification grouping (not separable)
Some event types are grouped together. If you enable "status changes," you will also be notified for priority changes and blocking-relationship changes on that issue. You **cannot** select only status changes independently.

### Email-specific timing controls
- Send email immediately if issue assigned to you is marked **urgent** or **breaches SLA**
- Delay low-priority emails outside work hours until the next day
- Work hours defined as 8 AM - 6 PM in the user's timezone

### Subscription-based model
You do NOT choose which notifications appear in the Inbox — all notifications for subscribed issues arrive there. The per-channel settings control whether the notification is **also** sent via Desktop push, Mobile push, Email digest, or Slack DM.

### Project-level subscriptions
- Click the bell icon on any project page to subscribe/unsubscribe
- Toggle personal notifications for: issue created, description changed, issue completed/canceled, project update posted

---

## 3. Notification Channels

### In-App Inbox (always on)
- All notifications arrive in the Inbox — this cannot be customized
- Accessible via sidebar or keyboard shortcut `G` then `I`
- Maximum 500 notifications retained at once; older ones are dropped

### Desktop Push Notifications
- Real-time delivery
- Sent via Linear desktop app
- Optional notification badge on desktop app icon (configurable in Preferences > Desktop Application)

### Mobile Push Notifications
- Real-time delivery
- Sent via Linear mobile app

### Email Digests
- NOT real-time — delayed based on urgency
- Only sent if the user has NOT already read the Linear inbox notification (smart deduplication)
- Summary format of unread notifications
- Timing configurable (immediate for urgent/SLA, delay low-priority to next business day)

### Slack (Personal DM)
- Real-time delivery
- Appears as DM from the Linear app in Slack
- Same notification types as other channels, independently configurable
- Setup: Settings > Account > Notifications > Slack section, authenticate to Slack

### Slack (Team/Project/View Channel Notifications)
- Posts to a specific Slack channel (e.g., `#linear-team`, `#p-project-name`)
- Team notifications: issue created, comments, status changes
- Project notifications: issue created, comments, status changes, project updates
- View notifications: issue added to view, issue completed/canceled
- Non-Linear Slack members can see basic notification info (if workspace integration is connected)
- Linear members can take actions directly from Slack (update assignee, comment, subscribe/unsubscribe)
- Rich unfurls for issue/project/document/initiative links with action buttons

---

## 4. In-App Notification Inbox

### Layout & Navigation
- Central notification center in the sidebar
- Keyboard shortcut: `G` then `I` to jump to Inbox from anywhere
- Navigate through notifications with `J`/`K` or arrow keys
- Click into a notification to view the issue in a special **Inbox view** (inline issue detail with inbox actions)

### Read/Unread
- `U` — toggle read/unread on selected notification
- `Option/Alt U` — mark ALL as read or unread
- Display option toggle: **Show read** (can hide read notifications)

### Snooze
- `H` — snooze selected notification
- Temporarily hides the notification; re-appears as new/unread at the specified time
- Predefined intervals: 1 hour, tomorrow, next cycle start, and other predefined options
- Custom date/time input: "Jan 3 10am", "next quarter", "til March", "for 2 weeks"
- Display option toggle: **Show snoozed** (can show/hide snoozed)
- Snooze also works in Triage

### Delete (Linear uses "delete" not "archive")
- `Backspace` — delete selected notification
- `Shift Backspace` — delete ALL notifications
- `Cmd/Ctrl D` — delete all READ notifications
- Right-click context menu for delete action
- Linear does NOT support archiving notifications (confirmed in FAQ — may add in future)

### Quick Search / Filtering
- `Cmd/Ctrl F` — opens quick search bar within Inbox
- Filter by: title, issue ID, notification type, or assignee
- Example filters: show only comments, show only auto-closed issues
- `Esc` clears the search

### Issue Reminders (distinct from Snooze)
- Set a reminder on any issue, document, project, or initiative
- Reminder creates a NEW inbox notification at the specified time
- Shows at the top of the issue; can be rescheduled or canceled
- Accessible via `...` menu > "Remind Me" or command menu
- Same time picker as snooze (predefined + custom dates)
- Key difference from snooze: Reminders are set on issues/documents proactively; snooze delays an existing notification

### Unsubscribe
- `Shift S` — unsubscribe from the issue (must click into the notification first)
- Also available via top menu bar or Activity feed unsubscribe option

---

## 5. Notification Grouping & Batching

### Event-type grouping (settings level)
Notification types are grouped at the settings level. For example, "status changes" is grouped with "priority changes" and "blocking-relationship changes." You cannot separate these — enabling one enables all in the group.

### Email digest batching
- Email notifications are batched into digests, NOT sent per-event
- Timing varies by urgency: urgent/SLA-breach can be immediate; low-priority can be delayed to next business day
- Digests only sent if the inbox notification hasn't been read yet (avoids redundant emails)

### Per-issue notification consolidation
- Multiple events on the same issue appear as updates to that issue's notification entry in the Inbox
- The Inbox view is issue-centric: you see a list of issues with notification indicators, not a flat list of individual events

### No explicit notification grouping UI
- Linear does not have a visual "group by" or thread-collapsing feature in the Inbox like some email clients
- Instead, the issue-centric model naturally consolidates — one issue = one row in the Inbox, showing the latest activity

---

## 6. Unique UX Patterns

### Keyboard-first design
Linear's notification system is heavily keyboard-driven:
- `G I` — go to Inbox
- `J/K` or arrows — navigate notifications
- `U` — toggle read/unread
- `H` — snooze
- `Backspace` — delete
- `Shift S` — unsubscribe
- `Cmd/Ctrl F` — search within inbox
- `Cmd/Ctrl D` — delete all read
- Right-click context menus for all actions

### Smart email deduplication
Emails are only sent if the user hasn't already read the inbox notification. This prevents redundant alerts across channels.

### Snooze with natural language dates
The snooze/reminder date picker supports natural language input like "next quarter", "til March", "for 2 weeks", "Jan 3 10am" — not just predefined options.

### Snooze until next cycle
A unique option to snooze until the team's next sprint/cycle starts — integrates notification timing with project management cadence.

### Issue-centric Inbox (not event-centric)
Unlike email-style notification lists, Linear's Inbox is issue-centric. Each row represents an issue, and clicking in shows the full issue detail with all recent activity. This avoids the "100 notifications for 5 issues" problem.

### Inline issue editing from Inbox
When you click into a notification, you get a full issue view where you can update properties (status, assignee, priority, etc.) directly — no need to navigate away to a separate issue page.

### Triage as a team-level shared inbox
Separate from personal Inbox, Linear has a "Triage" workflow where issues created by integrations or non-team members appear in a shared team queue. The team reviews, categorizes, and moves issues into the workflow. This acts as a team-level notification/intake system with rotation responsibility.

### View-level Slack subscriptions
Users can subscribe a Slack channel to any custom view (not just teams/projects), getting notified when issues enter or complete within that view.

### Desktop badge control
Users can disable the notification badge on the desktop app icon (Preferences > Desktop Application), which is a subtle but important focus feature.

### Subscription model transparency
Linear surfaces subscription status clearly: users can see all subscribed issues under "My Issues > Subscribed" and manage them with clear keyboard shortcuts.

### 500 notification limit
The Inbox caps at 500 notifications. Older ones are silently dropped. This forces a "stay on top of it" discipline rather than infinite accumulation.

---

## Sources

- [Notifications – Linear Docs](https://linear.app/docs/notifications)
- [Inbox – Linear Docs](https://linear.app/docs/inbox)
- [Project notifications – Linear Docs](https://linear.app/docs/project-notifications)
- [Slack – Linear Docs](https://linear.app/docs/slack)
- [Preferences – Linear Docs](https://linear.app/docs/account-preferences)
- [Initiative and Project updates – Linear Docs](https://linear.app/docs/initiative-and-project-updates)
- [Comments and reactions – Linear Docs](https://linear.app/docs/comment-on-issues)
- [Notification settings changelog (2020)](https://linear.app/changelog/2020-11-19-notification-settings)
- [Personal Slack notifications – Changelog](https://linear.app/changelog/personal-slack-notifications)
- [Triage – Linear Docs](https://linear.app/docs/triage)
