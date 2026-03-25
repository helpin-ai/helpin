import { useRef, useEffect, useState } from 'react';
import { mountWidget, unmountWidget, type WidgetConfig } from '@helpin/widget-core';

// Widget-core CSS -- helpin-* prefixed classes, no conflicts with dashboard
import '@helpin/widget-core/styles';

interface WidgetPreviewProps {
  brandColor: string;
  showBranding: boolean;
  launcherPosition: string;
  launcherIcon: string;
  welcomeMessage: string;
  workspaceName?: string;
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
}

/**
 * Full-height sticky widget preview panel (Intercom-style).
 * Uses mountWidget() from @helpin/widget-core so the widget renders
 * with its own bundled Preact instance — no dual-instance __H errors.
 */
export function WidgetPreview({
  brandColor,
  showBranding,
  launcherPosition,
  launcherIcon,
  welcomeMessage,
  workspaceName = 'Your Company',
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
        workspaceName,
        branding: {
          primaryColor: brandColor,
          logoUrl: resolvedLogoUrl,
          welcomeMessage: welcomeMessage || 'How can we help?',
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
      onClose: () => setIsOpen(false),
      onLauncherClick: () => setIsOpen((open) => !open),
      initialView: 'home',
      widgetKey,
      host,
    });
  }, [brandColor, showBranding, launcherPosition, launcherIcon, welcomeMessage, workspaceName, workspaceLogoUrl, colorScheme, buttonColor, buttonIconColor, logoUrl, isOpen, helpSpaces, availability, aiFirst, showTalkToHuman, escalationMessage, widgetKey, host]);

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
      `}</style>
      <div className="widget-preview-panel">
        <div ref={mountRef} style={{ width: '100%', height: '100%' }} />
      </div>
    </>
  );
}
