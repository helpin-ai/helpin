import { useRef, useEffect, useState } from 'react';
import { mountWidget, unmountWidget, type Message, type WidgetConfig } from '@helpin-ai/widget-core';

// Widget-core CSS -- helpin-* prefixed classes, no conflicts with dashboard
import '@helpin-ai/widget-core/styles';

interface WidgetPreviewProps {
  privacyNotice?: WidgetConfig['privacyNotice'];
  initialView?: 'home' | 'conversation';
  brandColor: string;
  showBranding: boolean;
  launcherPosition: string;
  launcherIcon: string;
  welcomeMessage: string;
  workspaceName?: string;
  aiDisplayName?: string;
  avatarUrl?: string;
  workspaceLogoUrl?: string;
  colorScheme?: string;
  buttonColor?: string;
  buttonIconColor?: string;
  logoUrl?: string;
  helpSpaces?: Array<{ id: string; name: string; slug: string }>;
  availability?: WidgetConfig['availability'];
  aiFirst?: boolean;
  showTalkToHuman?: boolean;
  escalationMessage?: string;
  widgetKey?: string;
  host?: string;
  /** A conversation to show, for example the onboarding preview's answers. */
  messages?: Message[];
  isAIThinking?: boolean;
  onSendMessage?: (content: string) => void;
  /** The chat window alone, filling its container: no backdrop, launcher, or controls that would leave the conversation. */
  bare?: boolean;
}

/**
 * Full-height sticky widget preview panel.
 * Uses mountWidget() from @helpin-ai/widget-core so the widget renders
 * with its own bundled Preact instance — no dual-instance __H errors.
 */
export function WidgetPreview({
  privacyNotice,
  initialView = 'home',
  brandColor,
  showBranding,
  launcherPosition,
  launcherIcon,
  welcomeMessage,
  workspaceName = 'Your Company',
  aiDisplayName,
  avatarUrl,
  workspaceLogoUrl,
  colorScheme = 'light',
  buttonColor,
  buttonIconColor,
  logoUrl,
  helpSpaces,
  availability,
  aiFirst = false,
  showTalkToHuman = false,
  escalationMessage,
  widgetKey,
  host,
  messages,
  isAIThinking,
  onSendMessage,
  bare = false,
}: WidgetPreviewProps) {
  const mountRef = useRef<HTMLDivElement>(null);
  const [isOpen, setIsOpen] = useState(true);

  // Re-render on prop changes — Preact diffs internally, no flicker
  useEffect(() => {
    const el = mountRef.current;
    if (!el) return;

    const resolvedLogoUrl = logoUrl || workspaceLogoUrl;

    mountWidget(el, {
      config: {
        workspaceId: workspaceName,
        privacyNotice,
        workspaceName,
        aiDisplayName,
        branding: {
          primaryColor: brandColor,
          avatarUrl,
          logoUrl: resolvedLogoUrl,
          welcomeMessage,
          widgetPosition: (launcherPosition === 'bottom_left' ? 'bottom-left' : 'bottom-right') as 'bottom-left' | 'bottom-right',
          showBranding,
          launcherIcon: launcherIcon as 'chat_bubble' | 'question_mark' | 'help',
          colorScheme: (colorScheme || 'light') as 'system' | 'light' | 'dark',
          buttonColor,
          buttonIconColor,
        },
        features: {
          aiEnabled: aiFirst,
          aiFirst,
          showTalkToHuman,
          escalationMessage,
          fileUploads: false,
          preChatForm: false,
          requirePhone: false,
          csatRating: false,
          forceIdentify: false,
        },
        availability: availability ?? {
          isOnline: true,
          statusText: 'Online now',
          replyTimeText: 'We typically reply in a few minutes',
        },
        helpSpaces,
      },
      isOpen,
      showLauncher: !bare,
      onClose: () => setIsOpen(false),
      onLauncherClick: () => setIsOpen((open) => !open),
      initialView,
      widgetKey,
      host,
      messages,
      isAIThinking,
      onSendMessage,
    });
  }, [privacyNotice, initialView, brandColor, showBranding, launcherPosition, launcherIcon, welcomeMessage, workspaceName, aiDisplayName, avatarUrl, workspaceLogoUrl, colorScheme, buttonColor, buttonIconColor, logoUrl, isOpen, helpSpaces, availability, aiFirst, showTalkToHuman, escalationMessage, widgetKey, host, messages, isAIThinking, onSendMessage, bare]);

  // Unmount only when the React component itself unmounts
  useEffect(() => {
    return () => {
      const el = mountRef.current;
      if (el) unmountWidget(el);
    };
  }, []);

  const positionSide = launcherPosition === 'bottom_left' ? 'left' : 'right';

  return (
    <>
      <style>{`
        ${brandOverride(brandColor)}
        .widget-preview-panel {
          position: relative;
          width: 100%;
          height: 100%;
          min-height: 0;
          padding: 16px;
          box-sizing: border-box;
          overflow: hidden;
          contain: paint;
          background: repeating-conic-gradient(
            rgba(0,0,0,0.03) 0% 25%, transparent 0% 50%
          ) 50% / 16px 16px;
          border-radius: 8px;
          border: 1px solid var(--border, #e5e7eb);
        }

        /* Chat window — full natural size, anchored bottom-right */
        .widget-preview-panel .helpin-chat-window {
          position: absolute;
          width: 370px;
          max-width: calc(100% - 32px);
          height: calc(100% - 116px);
          max-height: 680px;
          bottom: 96px;
          ${positionSide}: 16px;
          animation: none;
          border-radius: 16px;
          box-shadow: 0 8px 30px rgba(0,0,0,0.12);
        }

        /* Launcher — natural size */
        .widget-preview-panel .helpin-launcher {
          position: absolute;
          width: 56px;
          height: 56px;
          bottom: 16px;
          ${positionSide}: 16px;
          left: ${positionSide === 'left' ? '16px' : 'auto'};
          right: ${positionSide === 'right' ? '16px' : 'auto'};
        }
        ${bare ? `
        /* Unclipped so the window's shadow shows; isolated so its on-site z-index stays inside the panel. */
        .widget-preview-panel { padding: 0; overflow: visible; contain: none; isolation: isolate; background: none; border: 0; }
        .widget-preview-panel .helpin-chat-window { inset: 0; width: 100%; max-width: none; height: 100%; max-height: none; }
        .widget-preview-panel .helpin-conversation-back,
        .widget-preview-panel .helpin-window-close-inline { display: none; }
        ` : ''}
      `}</style>
      <div className="widget-preview-panel">
        <div ref={mountRef} style={{ width: '100%', height: '100%' }} />
      </div>
    </>
  );
}

/** The brand color as the live widget applies it: its style variables, with readable text on top. */
function brandOverride(color: string) {
  const match = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(color.trim());
  if (!match) return '';
  const [r, g, b] = match.slice(1).map((part) => parseInt(part, 16));
  const foreground = (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.5 ? '#000000' : '#ffffff';
  return `.widget-preview-panel .helpin-widget {
          --helpin-primary: ${match[0]};
          --helpin-primary-hover: color-mix(in srgb, ${match[0]} 85%, black);
          --helpin-primary-foreground: ${foreground};
        }`;
}
