# @helpin-ai/widget-core

The internal Preact component library that powers the Helpin chat widget. It is consumed by `@helpin-ai/sdk-js` and is not published on npm.

## Use in this repository

This package is marked `private: true`; it is an internal workspace dependency,
not a public npm installation target. For an application integration, start with
the [JavaScript SDK](../sdk-js/README.md) or a framework wrapper.

From the repository root:

```bash
pnpm install --frozen-lockfile
pnpm --filter @helpin-ai/widget-core build
```

The build bundles Preact. Use the mount API below when embedding in a React host
to avoid mixing renderer instances. Supply your own `WidgetConfig`, state, and
callbacks; widget-core does not create an authenticated support session for you.

## Components

### High-level

| Export | Description |
| --- | --- |
| `mountWidget` | Mount the complete widget into a DOM container |
| `unmountWidget` | Unmount a previously mounted widget |
| `ChatWindow` | Main widget window |
| `WidgetLauncher` | Floating launcher button |

### Views

| Export | Description |
| --- | --- |
| `ConversationView` | Active conversation panel |
| `ConversationListView` | Conversation list / messages view |
| `HelpView` | Help-center home |
| `HelpSpaceView` | Help-center space |
| `HelpCollectionView` | Help-center collection |
| `HelpArticleView` | Help-center article |
| `HomeView` | Widget home screen |
| `MessagesView` | Messages screen |

### Building blocks

| Export | Description |
| --- | --- |
| `MessageList` | Scrollable message list |
| `MessageBubble` | Individual message bubble |
| `ComposeBar` | Message input area |
| `WidgetHeader` | Widget top bar |
| `PreChatForm` | Pre-chat data collection form |
| `QuickReplies` | Quick-reply button row |
| `TypingIndicator` | Typing animation |
| `CsatRating` | Customer satisfaction rating |
| `StreamingText` | Streaming text display |
| `BottomNav` | Bottom navigation (home / messages / help) |
| `ImageLightbox` | Full-screen image preview |
| `SpecialNoticeBanner` | Conversation notice banner |

## Types

```ts
import type {
  WidgetAdapter,
  WidgetConfig,
  WidgetView,
  Message,
  Conversation,
  ActiveTeammate,
  CustomerInfo,
  AiSource,
  Attachment,
  PendingAttachment,
  EmojiCatalog,
} from '@helpin-ai/widget-core';
```

## Mount API

Use `mountWidget` and `unmountWidget` to manage the widget lifecycle imperatively:

```ts
import { mountWidget, unmountWidget } from '@helpin-ai/widget-core';

const container = document.getElementById('helpin-root')!;

mountWidget(container, {
  config,
  isOpen: true,
  messages: [],
  onSendMessage: (content) => {
    console.log('send', content);
  },
  onClose: () => {
    console.log('close');
  },
});

// Later, when you're done:
unmountWidget(container);
```

### Common `mountWidget` options

See [`MountWidgetOptions`](src/index.ts) for the complete contract, including
contact capture, answer feedback, escalation, and customer satisfaction callbacks.

**Required**

| Option | Description |
| --- | --- |
| `config` | Widget configuration object |

**State**

| Option | Description |
| --- | --- |
| `messages` | Current message list |
| `isOpen` | Whether the widget window is open |
| `isTyping` / `isAIThinking` | Show typing or AI-thinking indicators |
| `activeTeammate` | Currently assigned teammate |
| `unreadCount` | Badge count on the launcher |
| `connectionStatus` | Connection state for reconnect UI |
| `conversations` / `activeConversation` | Conversation list and active selection |
| `isConversationExpanded` | Whether the conversation view is expanded |
| `transcriptEmail` | Email address for transcript requests |
| `initialView` | Starting `WidgetView` |
| `openArticleRequest` | Programmatically open a help-center article |
| `widgetKey` / `host` | Context for help-center content fetching |

**Callbacks**

| Option | Description |
| --- | --- |
| `onClose` | Widget close |
| `onSendMessage` | Message sent from conversation |
| `onSendMessageFromHome` | Message sent from the home view |
| `onUploadAttachment` | Attachment upload |
| `onQuickReply` | Quick-reply button clicked |
| `onTyping` | User typing state changed |
| `onPreChatSubmit` | Pre-chat form submitted |
| `onLauncherClick` | Launcher button clicked |
| `onRetryConnection` | Reconnect attempt |
| `onSelectConversation` / `onStartNewConversation` | Conversation navigation |
| `onViewChange` | Bottom-nav view changed |
| `onToggleConversationExpanded` | Conversation expand/collapse toggled |
| `onRequestTranscript` | Transcript requested |
| `onImageClick` | Image clicked (for custom handling) |

**UI toggles**

| Option | Description |
| --- | --- |
| `showPreChatForm` | Display the pre-chat form |
| `showLauncher` | Display the floating launcher |

## Utilities

| Export | Description |
| --- | --- |
| `loadEmojiCatalog` | Lazy-load the emoji catalog for the picker |

## Development

```bash
pnpm --filter @helpin-ai/widget-core build
pnpm --filter @helpin-ai/widget-core test
```
