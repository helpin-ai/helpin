# Shortcut (formerly Clubhouse) Notification System Research

> Research date: 2026-03-08

## 1. Events That Trigger Notifications

Shortcut generates notifications for the following events on Stories and Epics that a user is **following**:

### Story-Level Events
- **Story state/status changes** (e.g., moved from "In Progress" to "Done")
- **Story description** changes
- **Story title** changes
- **Story points/estimate** changes
- **Story type** changes (feature, bug, chore)
- **Story due date** changes
- **Story iteration** changes
- **Story epic** assignment changes
- **Story labels/custom fields** changes
- **Comments** added to a story
- **@-mentions** in story descriptions or comments (both direct user mentions and Team mentions)

### Epic-Level Events
- Changes to Epics the user follows
- Changes to Stories within followed Epics
- Comments and @-mentions in Epic descriptions or discussions

### What Does NOT Trigger Notifications
- Simply being a Requester does not guarantee notifications — the user must be a Follower (though Requesters are auto-added as Followers by default)

---

## 2. Notification Preferences & Settings Granularity

### Following Model (Core Concept)
Shortcut uses a **"Follower" model** as the foundation of its notification system:
- **Owners** of a Story are automatically added as Followers
- **Requesters** of a Story are automatically added as Followers
- **Commenters** are automatically added as Followers when they comment on a Story
- Users can manually follow/unfollow any Story or Epic
- Only Followers receive notifications about changes

### Email Notification Preferences
Users can configure email notifications in their Profile Settings with these options:
- **Daily summary email**: A single daily digest of all events on followed Stories/Epics, with timestamps (default UTC offset, adjustable per user)
- **Immediate @-mention emails**: Direct @-mentions and Team @-mentions are sent immediately, separately from the daily summary
- **None**: Disable email notifications entirely by selecting "None" in settings

### Activity Badge Preferences
Users can configure the in-app Activity Badge to appear for:
- **All new comments** added to Stories/Epics they follow
- **Only @-mentions** — badge only appears when the user is directly mentioned

### Event Type Granularity
Users can choose which types of changes trigger notifications:
- Story Description changes
- Story State changes
- Epic assignment changes
- Title changes
- Points/estimate changes
- Due Date changes
- Type changes
- Iteration changes
- Labels/Fields changes

---

## 3. Notification Channels

### Channel 1: In-App Activity Feed
- Accessed via the **Activity Button** in the upper-right corner of the UI
- Shows a real-time feed of changes to followed items
- Has an **Activity Badge** (unread count indicator)

### Channel 2: Email Notifications
- **Daily summary email**: Aggregated digest of all events, sent once per day
- **Immediate mention emails**: @-mentions delivered in real-time, separate from the daily digest
- Activated by default; can be disabled in Profile Settings
- Timestamps adjustable via UTC offset in Profile

### Channel 3: Browser Notifications (Push)
- Supported in **Google Chrome, Safari, and Firefox**
- Triggered when the user's Shortcut username is **@-mentioned** in a Story comment or Epic discussion
- Requires browser-level permission (e.g., Chrome: Settings > Privacy and Security > Site Settings > Notifications > add `https://app.shortcut.com`)

### Channel 4: Slack Integration
Two modes of Slack notifications:

#### a) Personal Slack DMs
- User receives a DM from the "Shortcut" bot when:
  - They are @-mentioned in a Story description, comment, or Epic description/comment
  - A Team they belong to is @-mentioned
- Users can **reply in-thread** in Slack to push a comment back to the Story in Shortcut
- Emoji reactions in Slack are synced back to Shortcut

#### b) Team/Channel Notifications
- Link a **Shortcut Team** or a **Custom Field value** to a specific Slack channel
- Story updates (comments, new stories, status changes) for that Team/Field are posted automatically to the channel
- Attributed to the "Shortcut Bot" in Slack
- Setup: Profile Avatar > Integrations > Slack > Link Team / Link a Field Value to Slack Channel

#### c) Slack Thread Sync
- Link any Slack thread to a Shortcut Story via Slack message actions
- Existing Slack comments are copied to Shortcut as Story comments (with links back to Slack)
- Bi-directional: comments added in Shortcut also appear in the linked Slack thread

---

## 4. In-App Notification Inbox / Activity Feed

### Location & Access
- **Activity Button** in the upper-right corner of the Shortcut UI
- Shows a badge count of unread notifications

### Three Tabs / Filter Views
1. **All Activity**: An in-order chronological stream of all activity on followed items
2. **Comments**: Filtered to show only comments on Stories the user owns or is following
3. **Mentions**: Filtered to show only @-mentions of the user or groups/teams they belong to

### Content Shown
- Story-level activities (state changes, field updates, etc.)
- Comments on followed Stories and Epics
- @-mentions across the system
- Updates are scoped to Stories, Projects, and Epics the user is following

### Badge Behavior
- Configurable: can show badge for **all comments** on followed items, or **only for @-mentions**
- Helps users control notification noise level

---

## 5. Notification Grouping & Batching

### Email Batching
- **Daily summary email**: All events from followed Stories/Epics are batched into a single daily digest
- Each event in the summary includes a timestamp
- Timestamps default to Eastern Time UTC offset but are configurable per user in Profile Settings
- **Exception**: @-mention emails are sent immediately and are NOT batched into the daily summary

### In-App Feed
- The Activity Feed displays events in **reverse chronological order** (newest first)
- No explicit grouping by Story or time window is documented — events appear as a flat stream
- The three-tab filter system (All / Comments / Mentions) serves as the primary way to reduce noise

### Slack
- Team channel notifications appear as individual messages per event (comment, story addition)
- Personal DMs are sent per @-mention event

---

## 6. Unique UX Patterns

### 1. Follower-Based Notification Model
Unlike tools that notify based on role assignment alone, Shortcut uses an explicit **Follower list** per Story/Epic. While owners, requesters, and commenters are auto-added, users have full control to follow/unfollow. This gives fine-grained, per-item control over what generates notifications.

### 2. Three-Tab Activity Feed
The Activity Feed's three-tab design (All Activity / Comments / Mentions) is a distinctive pattern that lets users quickly triage between "everything happening," "discussion threads," and "things directed at me."

### 3. Slack Bi-Directional Threading
The ability to reply to a Shortcut notification DM in Slack and have that reply posted as a comment on the Story is a notable convenience. Combined with emoji react syncing, it reduces context-switching.

### 4. Slack Thread Sync (Link Any Thread)
Users can retroactively link an existing Slack conversation thread to a Shortcut Story, copying over all prior comments. This captures decisions made in Slack directly in the Story's history.

### 5. Team-to-Channel Mapping
Linking a Shortcut Team (or Custom Field value) to a Slack channel creates a persistent, automatic notification pipeline for squad-level awareness without individual opt-in.

### 6. Configurable Activity Badge Sensitivity
The ability to toggle the in-app badge between "all comments" and "mentions only" is a simple but effective noise control mechanism that many PM tools lack.

### 7. Separation of Digest vs. Immediate Emails
Rather than offering complex per-event email frequency settings, Shortcut uses a clean two-tier model: daily digest for general activity + immediate delivery for @-mentions. This balances awareness with urgency.

---

## Sources

- [Shortcut Email Notifications – Help Center](https://help.shortcut.com/hc/en-us/articles/205268919-Shortcut-Email-Notifications)
- [Shortcut Browser Notifications – Help Center](https://help.shortcut.com/hc/en-us/articles/115002887226-Shortcut-Browser-Notifications)
- [The Shortcut Activity Feed – Help Center](https://help.shortcut.com/hc/en-us/articles/207373703-The-Shortcut-Activity-Feed)
- [@-mentions – Help Center](https://help.shortcut.com/hc/en-us/articles/115005567583--mentions)
- [The Slack Integration (with Slack Actions) – Help Center](https://help.shortcut.com/hc/en-us/articles/205268749-The-Slack-Integration-with-Slack-Actions)
- [Slack Integration for a Team – Help Center](https://help.shortcut.com/hc/en-us/articles/360058887631-Slack-Integration-for-a-Team)
- [Details of a Story – Help Center](https://help.shortcut.com/hc/en-us/articles/360043978792-Details-of-a-Story)
- [Profile Settings – Help Center](https://help.shortcut.com/hc/en-us/articles/360044503211-Profile-Settings)
- [Communicate faster with @mention notifications in Slack – Blog](https://www.shortcut.com/blog/slack-personal-notifications)
- [Slack Thread Sync – Blog](https://www.shortcut.com/blog/keep-your-conversations-seamlessly-connected-with-slack-thread-sync)
- [Slack Integration – Shortcut](https://www.shortcut.com/integrations/slack)
