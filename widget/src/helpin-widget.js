/**
 * Helpin Support Chat Widget
 * Embeddable customer support chat — zero dependencies, Shadow DOM isolated.
 *
 * Usage:
 *   <script src="https://cdn.helpin.ai/widget/helpin-widget.js"></script>
 *   <script>
 *     HelpinWidget.init({
 *       widgetKey: "your-widget-key",
 *       apiUrl: "https://helpin.example.com",
 *       position: "bottom-right",
 *       theme: "light",
 *       primaryColor: "#6C5CE7",
 *       greeting: "Hi there! How can we help you today?",
 *       preChatForm: true,
 *       companyName: "Helpin",
 *       soundEnabled: true
 *     });
 *   </script>
 */
(function () {
  "use strict";

  if (window.HelpinWidget && window.HelpinWidget._initialized) return;

  // ─── Notification Sound (base64 tiny blip) ───
  const NOTIFICATION_SOUND =
    "data:audio/wav;base64,UklGRl9vT19teleXBhbGV0dGUAAQABAEAA" +
    "AEAAAAEACABAAEAAEABAAAAABAACAAAQAAAQAAAQAAAQAA";

  // We'll generate a proper blip via AudioContext
  function playNotificationSound() {
    try {
      const ctx = new (window.AudioContext || window.webkitAudioContext)();
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.connect(gain);
      gain.connect(ctx.destination);
      osc.frequency.setValueAtTime(880, ctx.currentTime);
      osc.frequency.exponentialRampToValueAtTime(440, ctx.currentTime + 0.15);
      gain.gain.setValueAtTime(0.3, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 0.3);
      osc.start(ctx.currentTime);
      osc.stop(ctx.currentTime + 0.3);
      setTimeout(() => ctx.close(), 500);
    } catch (e) {
      // Audio not available — silent fail
    }
  }

  const HELPIN_AI_DISPLAY_NAME = "Helpin AI";

  function parseMessageMetadata(metadata) {
    if (!metadata) return null;
    try {
      return typeof metadata === "string" ? JSON.parse(metadata) : metadata;
    } catch (_error) {
      return null;
    }
  }

  function parseAIMessageMetadata(metadata) {
    const parsed = parseMessageMetadata(metadata);
    return parsed && (parsed.ai_auto_reply || parsed.ai_agent_id) ? parsed : null;
  }

  function parseLinkPreviews(metadata) {
    const parsed = parseMessageMetadata(metadata);
    if (!parsed || !Array.isArray(parsed.link_previews)) return [];
    return parsed.link_previews.filter((preview) => preview && typeof preview.url === "string" && typeof preview.title === "string");
  }

  function isAIMessage(msg) {
    if (!msg) return false;
    if (msg.sender_type === "ai") return true;
    return !!parseAIMessageMetadata(msg.metadata);
  }

  function getMessageSenderKey(msg) {
    return isAIMessage(msg) ? "ai" : (msg && msg.sender_type) || "";
  }

  function getMessageSenderName(msg) {
    return isAIMessage(msg) ? HELPIN_AI_DISPLAY_NAME : msg.sender_display_name;
  }

  // ─── Styles ───
  function getStyles(config) {
    const primary = config.primaryColor || "#6C5CE7";
    const isRight = config.position !== "bottom-left";
    const isDark = config.theme === "dark";

    // Derive lighter/darker shades
    const primaryRGB = hexToRGB(primary);
    const primaryLight = `rgba(${primaryRGB.r}, ${primaryRGB.g}, ${primaryRGB.b}, 0.08)`;
    const primaryMedium = `rgba(${primaryRGB.r}, ${primaryRGB.g}, ${primaryRGB.b}, 0.15)`;
    const primaryGlow = `rgba(${primaryRGB.r}, ${primaryRGB.g}, ${primaryRGB.b}, 0.35)`;

    const bg = isDark ? "#1a1a2e" : "#ffffff";
    const bgSecondary = isDark ? "#16213e" : "#f8f9fc";
    const bgTertiary = isDark ? "#0f3460" : "#f0f1f5";
    const textPrimary = isDark ? "#e8e8e8" : "#1a1a2e";
    const textSecondary = isDark ? "#a0a0b8" : "#6b7280";
    const textMuted = isDark ? "#6b7280" : "#9ca3af";
    const border = isDark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";
    const borderLight = isDark ? "rgba(255,255,255,0.03)" : "rgba(0,0,0,0.03)";
    const shadow = isDark
      ? "0 24px 80px rgba(0,0,0,0.6), 0 8px 24px rgba(0,0,0,0.4)"
      : "0 24px 80px rgba(0,0,0,0.12), 0 8px 24px rgba(0,0,0,0.06)";
    const bubbleShadow = isDark
      ? "0 8px 32px rgba(0,0,0,0.5)"
      : `0 8px 32px ${primaryGlow}`;

    const customerBubbleBg = primary;
    const customerBubbleText = "#ffffff";
    const teamBubbleBg = isDark ? "#222244" : "#f0f1f5";
    const teamBubbleText = textPrimary;

    const inputBg = isDark ? "#16213e" : "#ffffff";
    const inputBorder = isDark ? "rgba(255,255,255,0.1)" : "rgba(0,0,0,0.1)";

    return `
      @import url('https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,300;0,9..40,400;0,9..40,500;0,9..40,600;1,9..40,400&display=swap');

      :host {
        --tp-primary: ${primary};
        --tp-primary-rgb: ${primaryRGB.r}, ${primaryRGB.g}, ${primaryRGB.b};
        --tp-primary-light: ${primaryLight};
        --tp-primary-medium: ${primaryMedium};
        --tp-primary-glow: ${primaryGlow};
        --tp-bg: ${bg};
        --tp-bg-secondary: ${bgSecondary};
        --tp-bg-tertiary: ${bgTertiary};
        --tp-text: ${textPrimary};
        --tp-text-secondary: ${textSecondary};
        --tp-text-muted: ${textMuted};
        --tp-border: ${border};
        --tp-border-light: ${borderLight};
        --tp-shadow: ${shadow};
        --tp-bubble-shadow: ${bubbleShadow};
        --tp-customer-bg: ${customerBubbleBg};
        --tp-customer-text: ${customerBubbleText};
        --tp-team-bg: ${teamBubbleBg};
        --tp-team-text: ${teamBubbleText};
        --tp-input-bg: ${inputBg};
        --tp-input-border: ${inputBorder};
        --tp-font: 'DM Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;

        all: initial;
        font-family: var(--tp-font);
        font-size: 14px;
        line-height: 1.5;
        color: var(--tp-text);
        position: fixed;
        bottom: 24px;
        ${isRight ? "right: 24px;" : "left: 24px;"}
        z-index: 2147483647;
        direction: ltr;
        text-align: left;
      }

      *, *::before, *::after {
        box-sizing: border-box;
        margin: 0;
        padding: 0;
      }

      /* ─── Launcher Bubble ─── */
      .tp-launcher {
        width: 60px;
        height: 60px;
        border-radius: 50%;
        background: var(--tp-primary);
        border: none;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        box-shadow: var(--tp-bubble-shadow);
        transition: transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1),
                    box-shadow 0.3s ease;
        position: relative;
        outline: none;
      }

      .tp-launcher:hover {
        transform: scale(1.08);
        box-shadow: 0 12px 40px var(--tp-primary-glow);
      }

      .tp-launcher:active {
        transform: scale(0.95);
      }

      .tp-launcher svg {
        width: 28px;
        height: 28px;
        fill: #ffffff;
        transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1),
                    opacity 0.2s ease;
      }

      .tp-launcher .tp-icon-close {
        position: absolute;
        opacity: 0;
        transform: rotate(-90deg) scale(0.5);
      }

      .tp-launcher.tp-open .tp-icon-chat {
        opacity: 0;
        transform: rotate(90deg) scale(0.5);
      }

      .tp-launcher.tp-open .tp-icon-close {
        opacity: 1;
        transform: rotate(0deg) scale(1);
      }

      /* ─── Unread Badge ─── */
      .tp-badge {
        position: absolute;
        top: -4px;
        right: -4px;
        min-width: 22px;
        height: 22px;
        border-radius: 11px;
        background: #ef4444;
        color: #ffffff;
        font-size: 11px;
        font-weight: 600;
        font-family: var(--tp-font);
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0 6px;
        box-shadow: 0 2px 8px rgba(239, 68, 68, 0.4);
        transform: scale(0);
        transition: transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
        pointer-events: none;
      }

      .tp-badge.tp-visible {
        transform: scale(1);
      }

      /* ─── Chat Window ─── */
      .tp-window {
        position: absolute;
        bottom: 76px;
        ${isRight ? "right: 0;" : "left: 0;"}
        width: 400px;
        max-width: calc(100vw - 48px);
        height: 600px;
        max-height: calc(100vh - 140px);
        min-height: 400px;
        background: var(--tp-bg);
        border-radius: 20px;
        box-shadow: var(--tp-shadow);
        border: 1px solid var(--tp-border);
        display: flex;
        flex-direction: column;
        overflow: hidden;
        opacity: 0;
        transform: translateY(16px) scale(0.96);
        pointer-events: none;
        transition: opacity 0.35s cubic-bezier(0.16, 1, 0.3, 1),
                    transform 0.45s cubic-bezier(0.34, 1.56, 0.64, 1);
      }

      .tp-window.tp-visible {
        opacity: 1;
        transform: translateY(0) scale(1);
        pointer-events: auto;
      }

      /* ─── Header ─── */
      .tp-header {
        padding: 24px 24px 20px;
        background: linear-gradient(135deg, var(--tp-primary) 0%, ${adjustColor(primary, -20)} 100%);
        color: #ffffff;
        position: relative;
        overflow: hidden;
        flex-shrink: 0;
      }

      .tp-header::before {
        content: '';
        position: absolute;
        top: -50%;
        right: -20%;
        width: 200px;
        height: 200px;
        border-radius: 50%;
        background: rgba(255,255,255,0.06);
      }

      .tp-header::after {
        content: '';
        position: absolute;
        bottom: -60%;
        left: -10%;
        width: 160px;
        height: 160px;
        border-radius: 50%;
        background: rgba(255,255,255,0.04);
      }

      .tp-header-content {
        position: relative;
        z-index: 1;
      }

      .tp-header-company {
        font-size: 17px;
        font-weight: 600;
        letter-spacing: -0.01em;
        margin-bottom: 4px;
      }

      .tp-header-status {
        font-size: 12.5px;
        opacity: 0.8;
        display: flex;
        align-items: center;
        gap: 6px;
        font-weight: 400;
      }

      .tp-header-status::before {
        content: '';
        width: 7px;
        height: 7px;
        border-radius: 50%;
        background: #4ade80;
        display: inline-block;
        box-shadow: 0 0 6px rgba(74, 222, 128, 0.5);
      }

      /* ─── Messages Area ─── */
      .tp-messages {
        flex: 1;
        overflow-y: auto;
        padding: 20px 20px 8px;
        scroll-behavior: smooth;
        overscroll-behavior: contain;
      }

      .tp-messages::-webkit-scrollbar {
        width: 5px;
      }

      .tp-messages::-webkit-scrollbar-track {
        background: transparent;
      }

      .tp-messages::-webkit-scrollbar-thumb {
        background: var(--tp-border);
        border-radius: 3px;
      }

      /* ─── Greeting ─── */
      .tp-greeting {
        display: flex;
        gap: 10px;
        margin-bottom: 20px;
        animation: tp-fadeUp 0.5s cubic-bezier(0.16, 1, 0.3, 1) both;
      }

      .tp-greeting-avatar {
        width: 36px;
        height: 36px;
        border-radius: 50%;
        background: var(--tp-primary-medium);
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
      }

      .tp-greeting-avatar svg {
        width: 18px;
        height: 18px;
        fill: var(--tp-primary);
      }

      .tp-greeting-bubble {
        background: var(--tp-team-bg);
        color: var(--tp-team-text);
        padding: 12px 16px;
        border-radius: 4px 18px 18px 18px;
        font-size: 14px;
        line-height: 1.55;
        max-width: 280px;
      }

      /* ─── Date Separator ─── */
      .tp-date-sep {
        text-align: center;
        margin: 20px 0 16px;
        position: relative;
      }

      .tp-date-sep::before {
        content: '';
        position: absolute;
        top: 50%;
        left: 0;
        right: 0;
        height: 1px;
        background: var(--tp-border);
      }

      .tp-date-sep span {
        position: relative;
        background: var(--tp-bg);
        padding: 0 12px;
        font-size: 11px;
        font-weight: 500;
        color: var(--tp-text-muted);
        text-transform: uppercase;
        letter-spacing: 0.05em;
      }

      /* ─── Message Row ─── */
      .tp-msg-row {
        display: flex;
        margin-bottom: 6px;
        animation: tp-fadeUp 0.35s cubic-bezier(0.16, 1, 0.3, 1) both;
      }

      .tp-msg-row.tp-customer {
        justify-content: flex-end;
      }

      .tp-msg-row.tp-team {
        justify-content: flex-start;
        gap: 8px;
      }

      .tp-msg-row.tp-team + .tp-msg-row.tp-team {
        margin-top: -2px;
      }

      .tp-msg-row.tp-customer + .tp-msg-row.tp-customer {
        margin-top: -2px;
      }

      .tp-msg-avatar {
        width: 30px;
        height: 30px;
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        flex-shrink: 0;
        margin-top: auto;
      }

      .tp-msg-avatar.tp-agent {
        background: var(--tp-primary-medium);
      }

      .tp-msg-avatar.tp-agent svg {
        width: 15px;
        height: 15px;
        fill: var(--tp-primary);
      }

      .tp-msg-avatar.tp-human {
        background: var(--tp-bg-tertiary);
      }

      .tp-msg-avatar.tp-human svg {
        width: 15px;
        height: 15px;
        fill: var(--tp-text-secondary);
      }

      .tp-msg-avatar.tp-hidden {
        visibility: hidden;
      }

      .tp-msg-content {
        max-width: 280px;
        display: flex;
        flex-direction: column;
      }

      .tp-msg-sender {
        font-size: 11px;
        font-weight: 500;
        color: var(--tp-text-muted);
        margin-bottom: 3px;
        margin-left: 4px;
      }

      .tp-msg-bubble {
        padding: 10px 16px;
        font-size: 14px;
        line-height: 1.55;
        word-wrap: break-word;
        overflow-wrap: break-word;
      }

      .tp-customer .tp-msg-bubble {
        background: var(--tp-customer-bg);
        color: var(--tp-customer-text);
        border-radius: 18px 18px 4px 18px;
      }

      .tp-customer + .tp-customer .tp-msg-bubble {
        border-radius: 18px 4px 4px 18px;
      }

      .tp-team .tp-msg-bubble {
        background: var(--tp-team-bg);
        color: var(--tp-team-text);
        border-radius: 18px 18px 18px 4px;
      }

      .tp-team + .tp-team .tp-msg-bubble {
        border-radius: 4px 18px 18px 4px;
      }

      .tp-link-previews {
        display: flex;
        flex-direction: column;
        gap: 8px;
        margin-top: 8px;
      }

      .tp-link-preview {
        display: block;
        overflow: hidden;
        text-decoration: none;
        border-radius: 14px;
        border: 1px solid ${isDark ? "rgba(255,255,255,0.08)" : "rgba(0,0,0,0.08)"};
        background: ${isDark ? "rgba(255,255,255,0.04)" : "#ffffff"};
        color: inherit;
      }

      .tp-link-preview.tp-outgoing {
        border-color: rgba(255,255,255,0.18);
        background: rgba(255,255,255,0.12);
      }

      .tp-link-preview-image {
        display: block;
        width: 100%;
        height: 132px;
        object-fit: cover;
      }

      .tp-link-preview-body {
        padding: 10px 12px;
      }

      .tp-link-preview-host {
        font-size: 10px;
        font-weight: 700;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        color: ${isDark ? "#a0a0b8" : "#6b7280"};
      }

      .tp-link-preview.tp-outgoing .tp-link-preview-host {
        color: rgba(255,255,255,0.78);
      }

      .tp-link-preview-title {
        margin-top: 4px;
        font-size: 13px;
        font-weight: 600;
        line-height: 1.35;
      }

      .tp-link-preview-desc {
        margin-top: 4px;
        font-size: 12px;
        line-height: 1.45;
        color: ${isDark ? "#c4c4d0" : "#6b7280"};
      }

      .tp-link-preview.tp-outgoing .tp-link-preview-desc {
        color: rgba(255,255,255,0.86);
      }

      .tp-msg-time {
        font-size: 10.5px;
        color: var(--tp-text-muted);
        margin-top: 4px;
        padding: 0 4px;
      }

      .tp-customer .tp-msg-time {
        text-align: right;
      }

      /* ─── Typing Indicator ─── */
      .tp-typing {
        display: none;
        align-items: center;
        gap: 8px;
        padding: 8px 0;
        margin-bottom: 8px;
      }

      .tp-typing.tp-visible {
        display: flex;
      }

      .tp-typing-dots {
        display: flex;
        gap: 4px;
        background: var(--tp-team-bg);
        padding: 12px 16px;
        border-radius: 18px;
      }

      .tp-typing-dot {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: var(--tp-text-muted);
        animation: tp-bounce 1.4s ease-in-out infinite;
      }

      .tp-typing-dot:nth-child(2) { animation-delay: 0.15s; }
      .tp-typing-dot:nth-child(3) { animation-delay: 0.3s; }

      /* ─── Input Area ─── */
      .tp-input-area {
        padding: 16px 20px 16px;
        border-top: 1px solid var(--tp-border-light);
        background: var(--tp-bg);
        flex-shrink: 0;
      }

      .tp-input-wrap {
        display: flex;
        align-items: flex-end;
        gap: 10px;
        background: var(--tp-input-bg);
        border: 1.5px solid var(--tp-input-border);
        border-radius: 14px;
        padding: 6px 6px 6px 16px;
        transition: border-color 0.2s ease, box-shadow 0.2s ease;
      }

      .tp-input-wrap:focus-within {
        border-color: var(--tp-primary);
        box-shadow: 0 0 0 3px var(--tp-primary-light);
      }

      .tp-input-wrap textarea {
        flex: 1;
        border: none;
        background: transparent;
        resize: none;
        font-family: var(--tp-font);
        font-size: 14px;
        line-height: 1.5;
        color: var(--tp-text);
        outline: none;
        max-height: 120px;
        min-height: 24px;
        padding: 6px 0;
      }

      .tp-input-wrap textarea::placeholder {
        color: var(--tp-text-muted);
      }

      .tp-send-btn {
        width: 36px;
        height: 36px;
        border-radius: 10px;
        background: var(--tp-primary);
        border: none;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: transform 0.15s ease, opacity 0.15s ease;
        flex-shrink: 0;
        opacity: 0.4;
      }

      .tp-send-btn.tp-active {
        opacity: 1;
      }

      .tp-send-btn.tp-active:hover {
        transform: scale(1.06);
      }

      .tp-send-btn.tp-active:active {
        transform: scale(0.94);
      }

      .tp-send-btn svg {
        width: 18px;
        height: 18px;
        fill: #ffffff;
      }

      /* ─── Emoji Button ─── */
      .tp-emoji-btn {
        width: 36px;
        height: 36px;
        border-radius: 10px;
        background: transparent;
        border: none;
        cursor: pointer;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background 0.15s ease;
        flex-shrink: 0;
        color: var(--tp-text-muted);
      }

      .tp-emoji-btn:hover {
        background: var(--tp-bg-tertiary);
        color: var(--tp-text);
      }

      .tp-emoji-btn svg {
        width: 20px;
        height: 20px;
      }

      /* ─── Emoji Picker ─── */
      .tp-emoji-picker {
        position: absolute;
        bottom: calc(100% + 8px);
        left: 0;
        width: 300px;
        max-height: 320px;
        background: var(--tp-bg);
        border: 1px solid var(--tp-border);
        border-radius: 12px;
        box-shadow: 0 8px 32px rgba(0,0,0,0.12);
        z-index: 1000;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        animation: tp-emoji-in 0.15s ease-out;
      }

      @keyframes tp-emoji-in {
        from { opacity: 0; transform: translateY(4px) scale(0.98); }
        to { opacity: 1; transform: translateY(0) scale(1); }
      }

      .tp-emoji-search {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 10px 12px;
        border-bottom: 1px solid var(--tp-border-light);
        color: var(--tp-text-muted);
      }

      .tp-emoji-search input {
        flex: 1;
        border: none;
        background: transparent;
        font-family: var(--tp-font);
        font-size: 13px;
        color: var(--tp-text);
        outline: none;
      }

      .tp-emoji-search input::placeholder {
        color: var(--tp-text-muted);
      }

      .tp-emoji-search-clear {
        background: transparent;
        border: none;
        cursor: pointer;
        color: var(--tp-text-muted);
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 2px;
        border-radius: 4px;
      }

      .tp-emoji-search-clear:hover {
        background: var(--tp-bg-tertiary);
        color: var(--tp-text);
      }

      .tp-emoji-categories {
        display: flex;
        gap: 2px;
        padding: 6px 8px;
        border-bottom: 1px solid var(--tp-border-light);
        overflow-x: auto;
      }

      .tp-emoji-category-btn {
        width: 32px;
        height: 32px;
        border: none;
        background: transparent;
        border-radius: 6px;
        cursor: pointer;
        font-size: 16px;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background 0.15s;
        flex-shrink: 0;
      }

      .tp-emoji-category-btn:hover {
        background: var(--tp-bg-tertiary);
      }

      .tp-emoji-category-btn.active {
        background: var(--tp-primary);
      }

      .tp-emoji-grid {
        display: flex;
        flex-wrap: wrap;
        gap: 2px;
        padding: 8px;
        max-height: 200px;
        overflow-y: auto;
      }

      .tp-emoji-btn-pick {
        width: 34px;
        height: 34px;
        border: none;
        background: transparent;
        border-radius: 6px;
        cursor: pointer;
        font-size: 22px;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: background 0.15s;
      }

      .tp-emoji-btn-pick:hover {
        background: var(--tp-bg-tertiary);
      }

      .tp-emoji-empty {
        width: 100%;
        text-align: center;
        padding: 24px 0;
        font-size: 13px;
        color: var(--tp-text-muted);
      }

      .tp-emoji-loading {
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 40px 0;
        color: var(--tp-text-muted);
        font-size: 13px;
      }

      /* ─── Footer ─── */
      .tp-footer {
        text-align: center;
        padding: 8px 0 14px;
        background: var(--tp-bg);
        flex-shrink: 0;
      }

      .tp-footer a {
        font-size: 11px;
        color: var(--tp-text-muted);
        text-decoration: none;
        font-family: var(--tp-font);
        transition: color 0.2s ease;
      }

      .tp-footer a:hover {
        color: var(--tp-text-secondary);
      }

      /* ─── Pre-chat Form ─── */
      .tp-prechat {
        flex: 1;
        padding: 28px 24px;
        display: flex;
        flex-direction: column;
        gap: 20px;
        overflow-y: auto;
      }

      .tp-prechat-title {
        font-size: 18px;
        font-weight: 600;
        color: var(--tp-text);
        letter-spacing: -0.01em;
      }

      .tp-prechat-subtitle {
        font-size: 13.5px;
        color: var(--tp-text-secondary);
        margin-top: -12px;
        line-height: 1.5;
      }

      .tp-field {
        position: relative;
      }

      .tp-field input {
        width: 100%;
        padding: 14px 16px;
        border: 1.5px solid var(--tp-input-border);
        border-radius: 12px;
        font-family: var(--tp-font);
        font-size: 14px;
        color: var(--tp-text);
        background: var(--tp-input-bg);
        outline: none;
        transition: border-color 0.2s ease, box-shadow 0.2s ease;
      }

      .tp-field input::placeholder {
        color: transparent;
      }

      .tp-field label {
        position: absolute;
        left: 16px;
        top: 50%;
        transform: translateY(-50%);
        font-size: 14px;
        color: var(--tp-text-muted);
        pointer-events: none;
        transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
        background: var(--tp-input-bg);
        padding: 0;
        font-family: var(--tp-font);
      }

      .tp-field input:focus,
      .tp-field input:not(:placeholder-shown) {
        border-color: var(--tp-primary);
        box-shadow: 0 0 0 3px var(--tp-primary-light);
      }

      .tp-field input:focus + label,
      .tp-field input:not(:placeholder-shown) + label {
        top: 0;
        transform: translateY(-50%);
        font-size: 11px;
        font-weight: 500;
        color: var(--tp-primary);
        padding: 0 6px;
      }

      .tp-prechat-btn {
        padding: 14px 24px;
        border: none;
        border-radius: 12px;
        background: var(--tp-primary);
        color: #ffffff;
        font-family: var(--tp-font);
        font-size: 15px;
        font-weight: 600;
        cursor: pointer;
        transition: transform 0.15s ease, opacity 0.2s ease;
        letter-spacing: -0.01em;
        margin-top: 4px;
      }

      .tp-prechat-btn:hover {
        transform: translateY(-1px);
        opacity: 0.92;
      }

      .tp-prechat-btn:active {
        transform: translateY(0);
      }

      .tp-prechat-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
        transform: none;
      }

      .tp-prechat-skip {
        text-align: center;
      }

      .tp-prechat-skip button {
        background: none;
        border: none;
        color: var(--tp-text-muted);
        font-family: var(--tp-font);
        font-size: 13px;
        cursor: pointer;
        padding: 4px 8px;
        transition: color 0.2s ease;
      }

      .tp-prechat-skip button:hover {
        color: var(--tp-text-secondary);
      }

      /* ─── Empty State ─── */
      .tp-empty {
        flex: 1;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        padding: 40px 24px;
        text-align: center;
      }

      .tp-empty-icon {
        width: 56px;
        height: 56px;
        border-radius: 16px;
        background: var(--tp-primary-light);
        display: flex;
        align-items: center;
        justify-content: center;
        margin-bottom: 16px;
      }

      .tp-empty-icon svg {
        width: 28px;
        height: 28px;
        fill: var(--tp-primary);
      }

      .tp-empty-title {
        font-size: 16px;
        font-weight: 600;
        color: var(--tp-text);
        margin-bottom: 6px;
      }

      .tp-empty-desc {
        font-size: 13.5px;
        color: var(--tp-text-secondary);
        max-width: 240px;
        line-height: 1.55;
      }

      /* ─── Connection Error ─── */
      .tp-error-bar {
        padding: 8px 16px;
        background: #fef2f2;
        color: #dc2626;
        font-size: 12px;
        text-align: center;
        display: none;
        font-family: var(--tp-font);
      }

      .tp-error-bar.tp-visible {
        display: block;
      }

      /* ─── Animations ─── */
      @keyframes tp-fadeUp {
        from {
          opacity: 0;
          transform: translateY(8px);
        }
        to {
          opacity: 1;
          transform: translateY(0);
        }
      }

      @keyframes tp-bounce {
        0%, 60%, 100% { transform: translateY(0); }
        30% { transform: translateY(-4px); }
      }

      /* ─── Mobile ─── */
      @media (max-width: 480px) {
        :host {
          bottom: 16px;
          ${isRight ? "right: 16px;" : "left: 16px;"}
        }

        .tp-window {
          width: calc(100vw - 32px);
          height: calc(100vh - 120px);
          max-height: calc(100vh - 120px);
          bottom: 72px;
          ${isRight ? "right: -8px;" : "left: -8px;"}
          border-radius: 16px;
        }

        .tp-launcher {
          width: 54px;
          height: 54px;
        }

        .tp-launcher svg {
          width: 24px;
          height: 24px;
        }
      }
    `;
  }

  // ─── SVG Icons ───
  const ICONS = {
    chat: `<svg viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H5.17L4 17.17V4h16v12z"/><path d="M7 9h10v2H7zm0-3h10v2H7z"/></svg>`,
    close: `<svg viewBox="0 0 24 24"><path d="M18.3 5.71a1 1 0 0 0-1.41 0L12 10.59 7.11 5.7A1 1 0 0 0 5.7 7.11L10.59 12 5.7 16.89a1 1 0 1 0 1.41 1.41L12 13.41l4.89 4.89a1 1 0 0 0 1.41-1.41L13.41 12l4.89-4.89a1 1 0 0 0 0-1.4z"/></svg>`,
    send: `<svg viewBox="0 0 24 24"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>`,
    bot: `<svg viewBox="0 0 24 24"><path d="M12 2a2 2 0 0 1 2 2c0 .74-.4 1.39-1 1.73V7h1a7 7 0 0 1 7 7v1a3 3 0 0 1-3 3H6a3 3 0 0 1-3-3v-1a7 7 0 0 1 7-7h1V5.73A2 2 0 0 1 12 2zM9 13a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3zm6 0a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3z"/></svg>`,
    person: `<svg viewBox="0 0 24 24"><path d="M12 12c2.21 0 4-1.79 4-4s-1.79-4-4-4-4 1.79-4 4 1.79 4 4 4zm0 2c-2.67 0-8 1.34-8 4v2h16v-2c0-2.66-5.33-4-8-4z"/></svg>`,
    wave: `<svg viewBox="0 0 24 24"><path d="M7.03 4.95L3.49 8.49c-3.32 3.32-3.32 8.7 0 12.02s8.7 3.32 12.02 0l6.01-6.01a2.517 2.517 0 00-.39-3.86l.39-.39c.97-.97.97-2.56 0-3.54-.16-.16-.35-.3-.54-.41a2.497 2.497 0 00-3.72-3.05 2.517 2.517 0 00-3.88-.42l-2.51 2.51a2.493 2.493 0 00-3.84 3.11zm1.41 1.42c.2-.2.51-.2.71 0 .2.2.2.51 0 .71l-3.18 3.18a1 1 0 101.41 1.41l4.6-4.6a.5.5 0 01.7.71l-4.59 4.6a1 1 0 001.41 1.41l4.6-4.6a.5.5 0 01.7.7l-4.59 4.6a1 1 0 101.41 1.42l4.6-4.6a.5.5 0 01.7.7l-2.98 2.98c-2.15 2.15-5.63 2.15-7.78 0s-2.15-5.63 0-7.78l3.28-3.24z"/></svg>`,
    messages: `<svg viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-1.99.9-1.99 2L2 22l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm-2 12H6v-2h12v2zm0-3H6V9h12v2zm0-3H6V6h12v2z"/></svg>`,
    emoji: `<svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="10" fill="none" stroke="currentColor" stroke-width="2"/><path d="M8 14s1.5 2 4 2 4-2 4-2" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"/><line x1="9" y1="9" x2="9.01" y2="9" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/><line x1="15" y1="9" x2="15.01" y2="9" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>`,
  };

  // ─── Helpers ───
  function hexToRGB(hex) {
    hex = hex.replace("#", "");
    if (hex.length === 3) hex = hex[0] + hex[0] + hex[1] + hex[1] + hex[2] + hex[2];
    return {
      r: parseInt(hex.substring(0, 2), 16),
      g: parseInt(hex.substring(2, 4), 16),
      b: parseInt(hex.substring(4, 6), 16),
    };
  }

  function adjustColor(hex, amount) {
    const rgb = hexToRGB(hex);
    const r = Math.max(0, Math.min(255, rgb.r + amount));
    const g = Math.max(0, Math.min(255, rgb.g + amount));
    const b = Math.max(0, Math.min(255, rgb.b + amount));
    return `rgb(${r}, ${g}, ${b})`;
  }

  function escapeHTML(str) {
    const div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
  }

  function resolveWidgetAssetBase() {
    const currentScript = document.currentScript;
    if (currentScript && currentScript.src) {
      return new URL(".", currentScript.src).toString();
    }

    const scripts = Array.from(document.querySelectorAll('script[src]'));
    for (let idx = scripts.length - 1; idx >= 0; idx--) {
      const src = scripts[idx].src || "";
      if (/helpin-widget(\.min)?\.js(?:\?.*)?$/i.test(src)) {
        return new URL(".", src).toString();
      }
    }

    return "https://cdn.helpin.ai/widget/";
  }

  const WIDGET_ASSET_BASE = resolveWidgetAssetBase();

  function getWidgetAssetURL(filename) {
    return new URL(filename, WIDGET_ASSET_BASE).toString();
  }

  function loadEmojiBundle() {
    if (window.HelpinEmojiBundle) {
      return Promise.resolve(window.HelpinEmojiBundle);
    }

    if (window.HelpinEmojiBundlePromise) {
      return window.HelpinEmojiBundlePromise;
    }

    window.HelpinEmojiBundlePromise = new Promise((resolve, reject) => {
      const emojiScript = document.createElement("script");
      emojiScript.src = getWidgetAssetURL("emoji-data.js");
      emojiScript.crossOrigin = "anonymous";
      emojiScript.onload = () => {
        if (window.HelpinEmojiBundle) {
          resolve(window.HelpinEmojiBundle);
          return;
        }
        reject(new Error("emoji bundle loaded without global payload"));
      };
      emojiScript.onerror = () => reject(new Error("failed to load emoji bundle"));
      document.head.appendChild(emojiScript);
    }).catch((error) => {
      window.HelpinEmojiBundlePromise = null;
      throw error;
    });

    return window.HelpinEmojiBundlePromise;
  }

  function formatTime(isoString) {
    const d = new Date(isoString);
    const h = d.getHours();
    const m = d.getMinutes().toString().padStart(2, "0");
    const ampm = h >= 12 ? "PM" : "AM";
    return `${h % 12 || 12}:${m} ${ampm}`;
  }

  function getDateLabel(isoString) {
    const d = new Date(isoString);
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
    const msgDay = new Date(d.getFullYear(), d.getMonth(), d.getDate());
    const diff = (today - msgDay) / (1000 * 60 * 60 * 24);

    if (diff === 0) return "Today";
    if (diff === 1) return "Yesterday";
    return d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
  }

  function linkify(text) {
    const escaped = escapeHTML(text);
    return escaped.replace(/(^|[\s(>])(https?:\/\/[^\s<]+)/g, function (_match, prefix, rawUrl) {
      let url = rawUrl;
      let suffix = "";

      while (url) {
        const last = url[url.length - 1];
        if (/[.,!?;:]/.test(last)) {
          suffix = last + suffix;
          url = url.slice(0, -1);
          continue;
        }
        if (last === ")" && (url.match(/\(/g) || []).length < (url.match(/\)/g) || []).length) {
          suffix = last + suffix;
          url = url.slice(0, -1);
          continue;
        }
        break;
      }

      return prefix + '<a href="' + url + '" target="_blank" rel="noopener noreferrer" style="color:inherit;text-decoration:underline;">' + url + "</a>" + suffix;
    });
  }

  function safePreviewURL(url) {
    return /^https?:\/\//i.test(url || "") ? url : "#";
  }

  function previewHost(preview) {
    try {
      return new URL(preview.url).hostname.replace(/^www\./, "");
    } catch (_error) {
      return (preview.host || "").replace(/^www\./, "");
    }
  }

  function renderLinkPreviews(previews, isCustomer) {
    if (!Array.isArray(previews) || previews.length === 0) return "";
    return `<div class="tp-link-previews">` + previews.map((preview) => {
      const href = escapeHTML(safePreviewURL(preview.url));
      const title = escapeHTML(preview.title);
      const host = escapeHTML(preview.site_name || previewHost(preview));
      const desc = preview.description ? `<div class="tp-link-preview-desc">${escapeHTML(preview.description)}</div>` : "";
      const image = preview.image_url && /^https?:\/\//i.test(preview.image_url)
        ? `<img class="tp-link-preview-image" src="${escapeHTML(preview.image_url)}" alt="${title}" loading="lazy" />`
        : "";
      return `
        <a class="tp-link-preview ${isCustomer ? "tp-outgoing" : ""}" href="${href}" target="_blank" rel="noopener noreferrer">
          ${image}
          <div class="tp-link-preview-body">
            <div class="tp-link-preview-host">${host}</div>
            <div class="tp-link-preview-title">${title}</div>
            ${desc}
          </div>
        </a>
      `;
    }).join("") + `</div>`;
  }

  // ─── Storage ───
  const STORAGE_KEY = "tp_widget_session";

  function saveSession(data) {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
    } catch (e) {}
  }

  function loadSession() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return null;
      const data = JSON.parse(raw);
      if (data.expires_at && new Date(data.expires_at) < new Date()) {
        localStorage.removeItem(STORAGE_KEY);
        return null;
      }
      return data;
    } catch (e) {
      return null;
    }
  }

  function clearSession() {
    try {
      localStorage.removeItem(STORAGE_KEY);
    } catch (e) {}
  }

  // ─── API Client ───
  function createAPI(config) {
    const base = (config.apiUrl || "").replace(/\/+$/, "") + "/api/widget/support";

    async function request(method, path, body) {
      const opts = {
        method,
        headers: { "Content-Type": "application/json" },
      };
      if (body) opts.body = JSON.stringify(body);

      const res = await fetch(base + path, opts);
      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || err.message || "Request failed");
      }
      return res.json();
    }

    return {
      validateKey: () => request("GET", `/config?widget_key=${encodeURIComponent(config.widgetKey)}`),
      createSession: (name, email) =>
        request("POST", "/session", {
          widget_key: config.widgetKey,
          customer_name: name || undefined,
          customer_email: email || undefined,
        }),
      sendMessage: (sessionToken, content) =>
        request("POST", "/messages", { session_token: sessionToken, content }),
      getMessages: (sessionToken) =>
        request("GET", `/messages?session_token=${encodeURIComponent(sessionToken)}`),
    };
  }

  // ─── Widget Class ───
  class HelpinChat {
    constructor(config) {
      this.config = Object.assign(
        {
          widgetKey: "",
          apiUrl: "",
          position: "bottom-right",
          theme: "light",
          primaryColor: "#6C5CE7",
          greeting: "Hi there! How can we help you today?",
          preChatForm: true,
          companyName: "Helpin",
          soundEnabled: true,
          placeholder: "Type a message...",
        },
        config
      );

      this.isOpen = false;
      this.messages = [];
      this.sessionToken = null;
      this.sessionExpiry = null;
      this.customerName = null;
      this.customerEmail = null;
      this.unreadCount = 0;
      this.pollInterval = null;
      this.lastMessageId = null;
      this.preChatCompleted = false;
      this.hasError = false;
      this.sending = false;

      this.api = createAPI(this.config);
      this._init();
    }

    async _init() {
      // Create Shadow DOM host
      this.host = document.createElement("div");
      this.host.id = "helpin-widget-host";
      this.shadow = this.host.attachShadow({ mode: "open" });

      // Inject styles
      const style = document.createElement("style");
      style.textContent = getStyles(this.config);
      this.shadow.appendChild(style);

      // Build DOM
      this._buildDOM();

      // Append to page
      document.body.appendChild(this.host);

      // Restore session
      const saved = loadSession();
      if (saved && saved.session_token) {
        this.sessionToken = saved.session_token;
        this.sessionExpiry = saved.expires_at;
        this.customerName = saved.customer_name;
        this.customerEmail = saved.customer_email;
        this.preChatCompleted = true;
        this._showChatView();
        await this._loadMessages();
      }

      // Start polling in background (for unread count even when closed)
      this._startBackgroundPoll();

      // Keyboard handler
      document.addEventListener("keydown", (e) => {
        if (e.key === "Escape" && this.isOpen) this.toggle();
      });
    }

    _buildDOM() {
      const container = document.createElement("div");
      container.className = "tp-container";

      // Launcher button
      this.launcher = document.createElement("button");
      this.launcher.className = "tp-launcher";
      this.launcher.setAttribute("aria-label", "Open support chat");
      this.launcher.innerHTML = `
        <span class="tp-icon-chat">${ICONS.chat}</span>
        <span class="tp-icon-close">${ICONS.close}</span>
        <span class="tp-badge" aria-hidden="true">0</span>
      `;
      this.launcher.addEventListener("click", () => this.toggle());
      this.badge = this.launcher.querySelector(".tp-badge");

      // Chat window
      this.window = document.createElement("div");
      this.window.className = "tp-window";
      this.window.setAttribute("role", "dialog");
      this.window.setAttribute("aria-label", "Support chat");

      // Header
      const header = document.createElement("div");
      header.className = "tp-header";
      header.innerHTML = `
        <div class="tp-header-content">
          <div class="tp-header-company">${escapeHTML(this.config.companyName)}</div>
          <div class="tp-header-status">We typically reply in a few minutes</div>
        </div>
      `;

      // Error bar
      this.errorBar = document.createElement("div");
      this.errorBar.className = "tp-error-bar";
      this.errorBar.textContent = "Connection lost. Retrying...";

      // Pre-chat form
      this.preChatView = document.createElement("div");
      this.preChatView.className = "tp-prechat";
      this.preChatView.innerHTML = `
        <div class="tp-prechat-title">Start a conversation</div>
        <div class="tp-prechat-subtitle">We'd love to hear from you. Fill in your details to get started.</div>
        <div class="tp-field">
          <input type="text" id="tp-name" placeholder="Name" autocomplete="name" />
          <label for="tp-name">Your name</label>
        </div>
        <div class="tp-field">
          <input type="email" id="tp-email" placeholder="Email" autocomplete="email" />
          <label for="tp-email">Email address</label>
        </div>
        <button class="tp-prechat-btn" type="button">Start chatting</button>
        <div class="tp-prechat-skip">
          <button type="button">Continue without details</button>
        </div>
      `;

      const startBtn = this.preChatView.querySelector(".tp-prechat-btn");
      const skipBtn = this.preChatView.querySelector(".tp-prechat-skip button");
      const nameInput = this.preChatView.querySelector("#tp-name");
      const emailInput = this.preChatView.querySelector("#tp-email");

      startBtn.addEventListener("click", () => {
        this.customerName = nameInput.value.trim() || null;
        this.customerEmail = emailInput.value.trim() || null;
        this._completePreChat();
      });

      skipBtn.addEventListener("click", () => {
        this._completePreChat();
      });

      // Enter key on email field starts chat
      emailInput.addEventListener("keydown", (e) => {
        if (e.key === "Enter") startBtn.click();
      });

      // Chat view (messages + input)
      this.chatView = document.createElement("div");
      this.chatView.style.cssText = "display:flex;flex-direction:column;flex:1;overflow:hidden;";

      // Messages area
      this.messagesEl = document.createElement("div");
      this.messagesEl.className = "tp-messages";

      // Typing indicator
      this.typingEl = document.createElement("div");
      this.typingEl.className = "tp-typing";
      this.typingEl.innerHTML = `
        <div class="tp-typing-dots">
          <div class="tp-typing-dot"></div>
          <div class="tp-typing-dot"></div>
          <div class="tp-typing-dot"></div>
        </div>
      `;

      // Input area
      this.inputArea = document.createElement("div");
      this.inputArea.className = "tp-input-area";
      this.inputArea.innerHTML = `
        <div class="tp-input-wrap">
          <textarea rows="1" placeholder="${escapeHTML(this.config.placeholder)}" aria-label="Message"></textarea>
          <button class="tp-emoji-btn" aria-label="Open emoji picker" type="button">
            ${ICONS.emoji}
          </button>
          <button class="tp-send-btn" aria-label="Send message" type="button">
            ${ICONS.send}
          </button>
        </div>
      `;

      this.textarea = this.inputArea.querySelector("textarea");
      this.sendBtn = this.inputArea.querySelector(".tp-send-btn");
      this.emojiBtn = this.inputArea.querySelector(".tp-emoji-btn");
      this.emojiPicker = null;
      this.emojiData = null;
      this.emojiOpen = false;

      // Emoji button click - lazy load emoji data
      this.emojiBtn.addEventListener("click", () => this._toggleEmojiPicker());

      // Auto-resize textarea
      this.textarea.addEventListener("input", () => {
        this.textarea.style.height = "auto";
        this.textarea.style.height = Math.min(this.textarea.scrollHeight, 120) + "px";
        this._updateSendBtn();
      });

      this.textarea.addEventListener("keydown", (e) => {
        if (e.key === "Enter" && !e.shiftKey) {
          e.preventDefault();
          this._sendMessage();
        }
      });

      this.sendBtn.addEventListener("click", () => this._sendMessage());

      // Footer
      const footer = document.createElement("div");
      footer.className = "tp-footer";
      footer.innerHTML = `<a href="https://helpin.ai" target="_blank" rel="noopener noreferrer">Powered by Helpin</a>`;

      // Assemble chat view
      this.chatView.appendChild(this.messagesEl);
      this.chatView.appendChild(this.inputArea);

      // Assemble window
      this.window.appendChild(header);
      this.window.appendChild(this.errorBar);

      if (this.config.preChatForm && !this.preChatCompleted) {
        this.window.appendChild(this.preChatView);
      } else {
        this.preChatCompleted = true;
        this.window.appendChild(this.chatView);
      }

      this.window.appendChild(footer);

      container.appendChild(this.window);
      container.appendChild(this.launcher);

      this.shadow.appendChild(container);
    }

    _showChatView() {
      if (this.preChatView.parentNode) {
        this.preChatView.parentNode.replaceChild(this.chatView, this.preChatView);
      }
      this._renderMessages();
    }

    async _completePreChat() {
      this.preChatCompleted = true;

      if (!this.sessionToken) {
        try {
          const session = await this.api.createSession(this.customerName, this.customerEmail);
          this.sessionToken = session.session_token;
          this.sessionExpiry = session.expires_at;
          saveSession({
            session_token: this.sessionToken,
            expires_at: this.sessionExpiry,
            customer_name: this.customerName,
            customer_email: this.customerEmail,
          });
        } catch (e) {
          this._showError(true);
          return;
        }
      }

      this._showChatView();
      this._renderGreeting();
      this.textarea.focus();
    }

    toggle() {
      this.isOpen = !this.isOpen;

      if (this.isOpen) {
        this.window.classList.add("tp-visible");
        this.launcher.classList.add("tp-open");
        this.unreadCount = 0;
        this._updateBadge();

        if (this.preChatCompleted) {
          setTimeout(() => {
            this.textarea.focus();
            this._scrollToBottom();
          }, 100);
        } else {
          setTimeout(() => {
            const nameInput = this.preChatView.querySelector("#tp-name");
            if (nameInput) nameInput.focus();
          }, 100);
        }
      } else {
        this.window.classList.remove("tp-visible");
        this.launcher.classList.remove("tp-open");
      }
    }

    _updateBadge() {
      if (this.unreadCount > 0 && !this.isOpen) {
        this.badge.textContent = this.unreadCount > 9 ? "9+" : this.unreadCount;
        this.badge.classList.add("tp-visible");
      } else {
        this.badge.classList.remove("tp-visible");
      }
    }

    _updateSendBtn() {
      if (this.textarea.value.trim()) {
        this.sendBtn.classList.add("tp-active");
      } else {
        this.sendBtn.classList.remove("tp-active");
      }
    }

    _toggleEmojiPicker() {
      // Close if open
      if (this.emojiOpen && this.emojiPicker) {
        this.emojiPicker.remove();
        this.emojiPicker = null;
        this.emojiOpen = false;
        return;
      }

      // Create picker container
      this.emojiPicker = document.createElement("div");
      this.emojiPicker.className = "tp-emoji-picker";
      this.emojiOpen = true;
      this.emojiPicker.innerHTML = '<div class="tp-emoji-loading">Loading emojis...</div>';
      this.inputArea.appendChild(this.emojiPicker);

      loadEmojiBundle()
        .then((bundle) => {
          if (!this.emojiPicker || !this.emojiOpen) {
            return;
          }
          this._renderEmojiPicker(bundle);
        })
        .catch(() => {
          if (!this.emojiPicker) {
            return;
          }
          this.emojiPicker.innerHTML = '<div class="tp-emoji-loading">Failed to load emojis</div>';
        });
    }

    _bindEmojiImageFallbacks() {
      if (!this.emojiPicker) {
        return;
      }

      this.emojiPicker.querySelectorAll(".tp-emoji-btn-pick img").forEach((img) => {
        img.addEventListener("error", () => {
          img.hidden = true;
          const fallback = img.nextElementSibling;
          if (fallback) {
            fallback.hidden = false;
          }
        }, { once: true });
      });
    }

    _renderEmojiPicker(bundle) {
      if (!this.emojiPicker || !bundle) return;

      const { emojis, categories, getEmojiImageUrl, hexToEmoji, searchEmojis } = bundle;

      // Category ID mapping for legacy support
      const categoryIdMap = {
        'smileys': 'smileys_people',
        'animals': 'animals_nature',
        'food': 'food_drink',
        'travel': 'travel_places',
        'recent': 'recent'
      };

      // Filter emojis by category
      const getEmojisByCategory = (catId) => {
        const mappedId = categoryIdMap[catId] || catId;
        return emojis.filter(e => {
          // Simple category mapping based on emoji unicode ranges
          const code = parseInt(e.u, 16);
          if (mappedId === 'recent') return false; // No recent yet
          if (mappedId === 'smileys_people') return code >= 0x1f600 && code <= 0x1f64f;
          if (mappedId === 'animals_nature') return (code >= 0x1f400 && code <= 0x1f43f) || (code >= 0x2600 && code <= 0x26ff);
          if (mappedId === 'food_drink') return (code >= 0x1f340 && code <= 0x1f37f) || (code >= 0x1f950 && code <= 0x1f96f);
          if (mappedId === 'activities') return code >= 0x1f3c0 && code <= 0x1f3df;
          if (mappedId === 'travel_places') return (code >= 0x1f680 && code <= 0x1f6ff) || (code >= 0x1f300 && code <= 0x1f3df);
          if (mappedId === 'objects') return (code >= 0x1f3a0 && code <= 0x1f3f7) || (code >= 0x1f4b0 && code <= 0x1f5ff);
          if (mappedId === 'symbols') return (code >= 0x1f300 && code <= 0x1f3ff) || (code >= 0x2600 && code <= 0x26ff) || (code >= 0x2700 && code <= 0x27bf);
          if (mappedId === 'flags') return (code >= 0x1f1e6 && code <= 0x1f1ff) || (code >= 0x1f3f3 && code <= 0x1f3ff);
          return false;
        });
      };

      let activeCategory = 'smileys_people';
      let search = "";

      const renderGrid = () => {
        let displayEmojis = [];
        if (search.trim()) {
          displayEmojis = searchEmojis(search);
        } else {
          displayEmojis = getEmojisByCategory(activeCategory);
        }

        this.emojiPicker.innerHTML = `
          <div class="tp-emoji-search">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>
            </svg>
            <input type="text" placeholder="Search emojis..." value="${escapeHTML(search)}" />
            ${search ? '<button class="tp-emoji-search-clear"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 6 6 18M6 6l12 12"/></svg></button>' : ''}
          </div>
          <div class="tp-emoji-categories">
            ${categories.map(c => `
              <button class="tp-emoji-category-btn ${c.id === activeCategory ? 'active' : ''}" data-cat="${c.id}" title="${c.label}">
                ${c.icon}
              </button>
            `).join('')}
          </div>
          <div class="tp-emoji-grid">
            ${displayEmojis.length > 0 ? displayEmojis.slice(0, 60).map((emoji, i) => `
              <button class="tp-emoji-btn-pick" data-unicode="${emoji.u}" data-display="${hexToEmoji(emoji.u)}" title="${emoji.n[0]}">
                <img src="${getEmojiImageUrl(emoji.u)}" width="24" height="24" alt="" aria-hidden="true" loading="lazy" />
                <span class="tp-emoji-fallback" hidden>${hexToEmoji(emoji.u)}</span>
              </button>
            `).join('') : '<p class="tp-emoji-empty">No emojis found</p>'}
          </div>
        `;

        this._bindEmojiImageFallbacks();

        // Category buttons
        this.emojiPicker.querySelectorAll(".tp-emoji-category-btn").forEach(btn => {
          btn.addEventListener("click", () => {
            activeCategory = btn.dataset.cat;
            search = "";
            renderGrid();
          });
        });

        // Search input
        const searchInput = this.emojiPicker.querySelector("input");
        searchInput.addEventListener("input", (e) => {
          search = e.target.value;
          renderGrid();
        });

        // Clear search
        const clearBtn = this.emojiPicker.querySelector(".tp-emoji-search-clear");
        if (clearBtn) {
          clearBtn.addEventListener("click", () => {
            search = "";
            renderGrid();
          });
        }

        // Emoji selection
        this.emojiPicker.querySelectorAll(".tp-emoji-btn-pick").forEach(btn => {
          btn.addEventListener("click", () => {
            const display = btn.dataset.display;
            this._insertEmoji(display);
            this._toggleEmojiPicker(); // Close picker
          });
        });
      };

      renderGrid();
    }

    _insertEmoji(emoji) {
      const start = this.textarea.selectionStart;
      const end = this.textarea.selectionEnd;
      const value = this.textarea.value;
      this.textarea.value = value.slice(0, start) + emoji + value.slice(end);
      this.textarea.selectionStart = this.textarea.selectionEnd = start + emoji.length;
      this.textarea.focus();
      this._updateSendBtn();
    }

    async _sendMessage() {
      const content = this.textarea.value.trim();
      if (!content || this.sending) return;

      if (!this.sessionToken) {
        // Create session on the fly if no pre-chat
        try {
          const session = await this.api.createSession(this.customerName, this.customerEmail);
          this.sessionToken = session.session_token;
          this.sessionExpiry = session.expires_at;
          saveSession({
            session_token: this.sessionToken,
            expires_at: this.sessionExpiry,
            customer_name: this.customerName,
            customer_email: this.customerEmail,
          });
        } catch (e) {
          this._showError(true);
          return;
        }
      }

      // Optimistic UI
      const optimisticMsg = {
        id: "temp-" + Date.now(),
        sender_type: "customer",
        sender_display_name: this.customerName || "You",
        content,
        created_at: new Date().toISOString(),
        _sending: true,
      };

      this.messages.push(optimisticMsg);
      this._renderMessages();
      this._scrollToBottom();

      this.textarea.value = "";
      this.textarea.style.height = "auto";
      this._updateSendBtn();
      this.sending = true;

      try {
        const msg = await this.api.sendMessage(this.sessionToken, content);
        // Replace optimistic message
        const idx = this.messages.findIndex((m) => m.id === optimisticMsg.id);
        if (idx !== -1) this.messages[idx] = msg;
        this.lastMessageId = msg.id;
        this._showError(false);
      } catch (e) {
        // Mark as failed
        optimisticMsg._failed = true;
        optimisticMsg._sending = false;
        this._showError(true);
      }

      this.sending = false;
      this._renderMessages();
    }

    async _loadMessages() {
      if (!this.sessionToken) return;

      try {
        const msgs = await this.api.getMessages(this.sessionToken);
        if (Array.isArray(msgs)) {
          const oldCount = this.messages.length;
          this.messages = msgs;

          // Check for new team messages
          if (msgs.length > 0) {
            const lastMsg = msgs[msgs.length - 1];
            if (lastMsg.id !== this.lastMessageId && lastMsg.sender_type !== "customer") {
              const isNew = oldCount > 0 && msgs.length > oldCount;
              if (isNew) {
                if (!this.isOpen) {
                  this.unreadCount += msgs.length - oldCount;
                  this._updateBadge();
                }
                if (this.config.soundEnabled) {
                  playNotificationSound();
                }
              }
            }
            this.lastMessageId = lastMsg.id;
          }

          this._renderMessages();
          this._showError(false);
        }
      } catch (e) {
        if (e.message && e.message.includes("session expired")) {
          clearSession();
          this.sessionToken = null;
          this.preChatCompleted = false;
          this.messages = [];
        }
      }
    }

    _renderGreeting() {
      if (!this.config.greeting || this.messages.length > 0) return;

      const existing = this.messagesEl.querySelector(".tp-greeting");
      if (existing) return;

      const greeting = document.createElement("div");
      greeting.className = "tp-greeting";
      greeting.innerHTML = `
        <div class="tp-greeting-avatar">${ICONS.wave}</div>
        <div class="tp-greeting-bubble">${escapeHTML(this.config.greeting)}</div>
      `;
      this.messagesEl.insertBefore(greeting, this.messagesEl.firstChild);
    }

    _renderMessages() {
      // Preserve greeting if no messages
      const greetingEl = this.messagesEl.querySelector(".tp-greeting");
      this.messagesEl.innerHTML = "";

      if (this.messages.length === 0) {
        if (greetingEl) {
          this.messagesEl.appendChild(greetingEl);
        } else {
          this._renderGreeting();
        }
        this.messagesEl.appendChild(this.typingEl);
        return;
      }

      // Remove greeting if there are messages
      let lastDateLabel = null;
      let lastSenderKey = null;

      this.messages.forEach((msg, idx) => {
        const dateLabel = getDateLabel(msg.created_at);

        // Date separator
        if (dateLabel !== lastDateLabel) {
          const sep = document.createElement("div");
          sep.className = "tp-date-sep";
          sep.innerHTML = `<span>${escapeHTML(dateLabel)}</span>`;
          this.messagesEl.appendChild(sep);
          lastDateLabel = dateLabel;
          lastSenderKey = null; // Reset grouping after date
        }

        const isCustomer = msg.sender_type === "customer";
        const isAI = isAIMessage(msg);
        const senderKey = getMessageSenderKey(msg);
        const nextSenderKey =
          idx === this.messages.length - 1 ? null : getMessageSenderKey(this.messages[idx + 1]);
        const senderName = getMessageSenderName(msg);
        const isSameAsPrev = senderKey === lastSenderKey;
        const isLastInGroup =
          idx === this.messages.length - 1 || nextSenderKey !== senderKey;

        const row = document.createElement("div");
        row.className = `tp-msg-row ${isCustomer ? "tp-customer" : "tp-team"}`;
        row.style.animationDelay = `${Math.min(idx * 0.03, 0.3)}s`;

        let avatarHTML = "";
        if (!isCustomer) {
          const avatarClass = isAI ? "tp-agent" : "tp-human";
          const avatarIcon = isAI ? ICONS.bot : ICONS.person;
          const hidden = !isLastInGroup ? "tp-hidden" : "";
          avatarHTML = `<div class="tp-msg-avatar ${avatarClass} ${hidden}">${avatarIcon}</div>`;
        }

        const senderHTML =
          !isCustomer && !isSameAsPrev && senderName
            ? `<div class="tp-msg-sender">${escapeHTML(senderName)}</div>`
            : "";

        const timeHTML = isLastInGroup
          ? `<div class="tp-msg-time">${formatTime(msg.created_at)}</div>`
          : "";

        const contentHTML = linkify(msg.content);
        const linkPreviewsHTML = renderLinkPreviews(parseLinkPreviews(msg.metadata), isCustomer);

        let statusClass = "";
        if (msg._sending) statusClass = ' style="opacity:0.6;"';
        if (msg._failed) statusClass = ' style="opacity:0.5;border:1px dashed #ef4444;border-radius:18px;"';

        row.innerHTML = `
          ${!isCustomer ? avatarHTML : ""}
          <div class="tp-msg-content">
            ${senderHTML}
            <div class="tp-msg-bubble"${statusClass}>${contentHTML}${linkPreviewsHTML}</div>
            ${timeHTML}
          </div>
        `;

        this.messagesEl.appendChild(row);
        lastSenderKey = senderKey;
      });

      // Typing indicator at bottom
      this.messagesEl.appendChild(this.typingEl);
    }

    _scrollToBottom() {
      requestAnimationFrame(() => {
        this.messagesEl.scrollTop = this.messagesEl.scrollHeight;
      });
    }

    _showError(show) {
      this.hasError = show;
      if (show) {
        this.errorBar.classList.add("tp-visible");
      } else {
        this.errorBar.classList.remove("tp-visible");
      }
    }

    _startBackgroundPoll() {
      // Poll every 3s when open, 15s when closed
      const poll = async () => {
        if (this.sessionToken) {
          await this._loadMessages();
          if (this.isOpen) this._scrollToBottom();
        }
      };

      setInterval(() => {
        const interval = this.isOpen ? 3000 : 15000;
        if (!this._lastPoll || Date.now() - this._lastPoll >= interval) {
          this._lastPoll = Date.now();
          poll();
        }
      }, 3000);
    }

    // Public methods
    open() {
      if (!this.isOpen) this.toggle();
    }

    close() {
      if (this.isOpen) this.toggle();
    }

    destroy() {
      if (this.host && this.host.parentNode) {
        this.host.parentNode.removeChild(this.host);
      }
    }
  }

  // ─── Public API ───
  window.HelpinWidget = {
    _initialized: false,
    _instance: null,

    init: function (config) {
      if (this._initialized) {
        console.warn("HelpinWidget: Already initialized.");
        return;
      }
      if (!config || !config.widgetKey) {
        console.error("HelpinWidget: widgetKey is required.");
        return;
      }
      if (!config.apiUrl) {
        console.error("HelpinWidget: apiUrl is required.");
        return;
      }

      this._initialized = true;
      this._instance = new HelpinChat(config);
      return this._instance;
    },

    open: function () {
      if (this._instance) this._instance.open();
    },

    close: function () {
      if (this._instance) this._instance.close();
    },

    destroy: function () {
      if (this._instance) {
        this._instance.destroy();
        this._instance = null;
        this._initialized = false;
      }
    },
  };
})();
