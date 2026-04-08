# @helpin-ai/widget-core

Low-level React/Preact widget components and mount helpers used by the Helpin widget runtime.

This package is primarily an internal building block for `@helpin-ai/sdk-js`, but it can also be used directly when you want to mount the chat UI yourself.

## Install

```bash
npm install @helpin-ai/widget-core
```

## Main Exports

| Export | Description |
| --- | --- |
| `mountWidget` | Mount the full widget into a DOM container |
| `unmountWidget` | Unmount a previously mounted widget |
| `ChatWindow` | Main widget window |
| `WidgetLauncher` | Floating launcher button |
| `ConversationView` | Active conversation panel |
| `ConversationListView` | Conversation list/messages view |
| `HelpView` | Help-center home view |
| `HelpSpaceView` | Help-center space view |
| `HelpCollectionView` | Help collection view |
| `HelpArticleView` | Help article view |
| `ImageLightbox` | Full-screen image preview |
| `BottomNav` | Bottom navigation for home/messages/help |
| `MessageList`, `MessageBubble`, `ComposeBar`, `WidgetHeader`, `PreChatForm`, `QuickReplies`, `TypingIndicator`, `CsatRating`, `StreamingText`, `HomeView`, `MessagesView` | UI building blocks |

## Types

The package exports these core types:

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

// later
unmountWidget(container);
```

Common `mountWidget(...)` options:

| Option | Description |
| --- | --- |
| `config` | Required widget configuration |
| `messages` | Current message list |
| `isOpen` | Whether the window is open |
| `onClose` | Called when the widget closes |
| `onSendMessage` | Send message handler |
| `onSendMessageFromHome` | Send handler from the home view |
| `onUploadAttachment` | Attachment upload handler |
| `onQuickReply` | Quick-reply click handler |
| `onTyping` | Typing indicator handler |
| `showPreChatForm` / `onPreChatSubmit` | Pre-chat form controls |
| `isTyping` / `isAIThinking` | Typing/loading indicators |
| `activeTeammate` | Active assigned teammate |
| `initialView` | Initial `WidgetView` |
| `showLauncher` / `onLauncherClick` | Launcher controls |
| `unreadCount` | Launcher unread badge |
| `connectionStatus` / `onRetryConnection` | Reconnect state |
| `conversations` / `activeConversation` | Conversation list state |
| `onSelectConversation` / `onStartNewConversation` | Conversation actions |
| `onViewChange` | Bottom-nav view changes |
| `isConversationExpanded` / `onToggleConversationExpanded` | Expanded conversation mode |
| `transcriptEmail` / `onRequestTranscript` | Transcript request flow |
| `widgetKey` / `host` | Help-center fetching context |
| `openArticleRequest` | Open a help-center article programmatically |
| `onImageClick` | Custom image click handling |

## Emoji Helpers

| Export | Description |
| --- | --- |
| `loadEmojiCatalog` | Lazy-load the emoji catalog used by the picker |

## Development

```bash
pnpm --filter @helpin-ai/widget-core build
pnpm --filter @helpin-ai/widget-core test
```
