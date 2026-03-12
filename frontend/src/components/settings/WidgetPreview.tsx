import { useRef, useEffect } from 'react';
import { render as preactRender, h } from 'preact';
import { ChatWindow, WidgetLauncher } from '@helpin/widget-core';

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
}

/**
 * Renders actual @helpin/widget-core Preact components inside the React dashboard
 * using an imperative Preact render bridge. The preview container uses CSS `contain: paint`
 * to trap `position: fixed` elements within the preview bounds.
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
}: WidgetPreviewProps) {
  const mountRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = mountRef.current;
    if (!el) return;

    const resolvedLogoUrl = logoUrl || workspaceLogoUrl;

    const config = {
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
        aiEnabled: false,
        fileUploads: false,
        preChatForm: false,
        csatRating: false,
      },
    };

    const tree = h(
      'div',
      { className: 'helpin-widget', style: { width: '100%', height: '100%' } },
      h(ChatWindow, {
        config,
        messages: [],
        isOpen: true,
        onClose: () => {},
        onSendMessage: () => {},
        onQuickReply: () => {},
        showPreChatForm: false,
        onPreChatSubmit: () => {},
        initialView: 'home',
      }),
      h(WidgetLauncher, {
        onClick: () => {},
        isOpen: false,
        unreadCount: 0,
        brandColor,
        buttonColor,
        buttonIconColor,
        icon: launcherIcon as 'chat_bubble' | 'question_mark' | 'help',
      }),
    );

    preactRender(tree, el);
    return () => {
      preactRender(null, el);
    };
  }, [brandColor, showBranding, launcherPosition, launcherIcon, welcomeMessage, workspaceName, workspaceLogoUrl, colorScheme, buttonColor, buttonIconColor, logoUrl]);

  const positionSide = launcherPosition === 'bottom_left' ? 'left' : 'right';

  return (
    <>
      <style>{`
        .widget-preview-scope {
          position: relative;
          height: 480px;
          overflow: hidden;
          contain: paint;
        }
        .widget-preview-scope .helpin-chat-window {
          position: absolute;
          width: 320px;
          height: 400px;
          max-height: none;
          bottom: 70px;
          ${positionSide}: 12px;
          animation: none;
        }
        .widget-preview-scope .helpin-launcher {
          position: absolute;
          width: 48px;
          height: 48px;
          bottom: 12px;
          ${positionSide}: 12px;
          left: ${positionSide === 'left' ? '12px' : 'auto'};
          right: ${positionSide === 'right' ? '12px' : 'auto'};
        }
        .widget-preview-scope .helpin-compose-input {
          font-size: 12px;
          padding: 8px 12px;
        }
        .widget-preview-scope .helpin-compose-send {
          width: 32px;
          height: 32px;
        }
        .widget-preview-scope .helpin-home-welcome {
          font-size: 16px;
        }
        .widget-preview-scope .helpin-home-header {
          padding: 24px 16px 20px;
        }
        .widget-preview-scope .helpin-home-content {
          padding: 12px 14px 0;
        }
        .widget-preview-scope .helpin-home-input {
          padding: 8px 12px;
          font-size: 12px;
        }
        .widget-preview-scope .helpin-home-send {
          width: 32px;
          height: 32px;
        }
        .widget-preview-scope .helpin-home-action {
          padding: 10px 10px;
        }
        .widget-preview-scope .helpin-home-action-title {
          font-size: 12px;
        }
        .widget-preview-scope .helpin-home-action-desc {
          font-size: 10px;
        }
        .widget-preview-scope .helpin-home-logo {
          width: 36px;
          height: 36px;
        }
        .widget-preview-scope .helpin-home-logo-placeholder {
          width: 36px;
          height: 36px;
          font-size: 15px;
        }
        .widget-preview-scope .helpin-bottom-nav-label {
          font-size: 9px;
        }
        .widget-preview-scope .helpin-bottom-nav-item svg {
          width: 18px;
          height: 18px;
        }
        .widget-preview-scope .helpin-powered-by {
          font-size: 9px;
          padding: 4px 12px 2px;
        }
        .widget-preview-scope .helpin-window-close {
          width: 26px;
          height: 26px;
          top: 8px;
          right: 8px;
        }
        .widget-preview-scope .helpin-window-close svg {
          width: 14px;
          height: 14px;
        }
        .widget-preview-scope .helpin-pre-chat-welcome {
          font-size: 13px;
        }
        .widget-preview-scope .helpin-input {
          padding: 8px 12px;
          font-size: 12px;
        }
        .widget-preview-scope .helpin-btn-primary {
          padding: 8px 12px;
          font-size: 12px;
        }
        .widget-preview-scope .helpin-header-branding {
          font-size: 9px;
        }
      `}</style>
      <div className="widget-preview-scope rounded-lg border bg-muted/20">
        <div ref={mountRef} style={{ width: '100%', height: '100%' }} />
      </div>
    </>
  );
}
