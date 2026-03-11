# @helpin/widget-core

React/Preact components for building a chat widget.

## Installation

```bash
pnpm add @helpin/widget-core
```

## Usage

```tsx
import { 
  ChatWindow, 
  WidgetLauncher, 
  MessageList, 
  MessageBubble,
  ComposeBar,
  WidgetHeader,
  PreChatForm,
  QuickReplies,
  TypingIndicator,
  CsatRating,
  StreamingText
} from '@helpin/widget-core';
import '@helpin/widget-core/dist/style.css';
```

## Components

| Component | Description |
|-----------|-------------|
| `ChatWindow` | Main container for the chat widget |
| `WidgetLauncher` | Floating button to open/close chat |
| `MessageList` | Scrollable list of messages |
| `MessageBubble` | Individual message with role styling |
| `ComposeBar` | Text input and send button |
| `WidgetHeader` | Header with title and close button |
| `PreChatForm` | Email/name capture before chat |
| `QuickReplies` | Quick reply buttons |
| `TypingIndicator` | Animated typing dots |
| `CsatRating` | Customer satisfaction rating |
| `StreamingText` | Animated text for AI responses |

## Types

```typescript
import type { 
  Message, 
  WidgetConfig, 
  CustomerInfo,
  WidgetAdapter,
  AiSource,
  Attachment 
} from '@helpin/widget-core';
```

## Development

### Build

```bash
pnpm build
```

### Test

```bash
pnpm test
```

### Test Coverage

| Component | Tests |
|-----------|-------|
| `MessageBubble` | 7 |
| `MessageList` | 6 |
| `WidgetLauncher` | 7 |
| `ComposeBar` | 12 |
| `PreChatForm` | 7 |
| `QuickReplies` | 5 |
| `TypingIndicator` | 5 |

Run tests with coverage:

```bash
pnpm test --coverage
```
