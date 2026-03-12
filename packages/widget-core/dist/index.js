import { options as w, Fragment as V } from "preact";
var Q = 0;
function i(e, t, n, a, r, o) {
  t || (t = {});
  var c, s, l = t;
  if ("ref" in l) for (s in l = {}, t) s == "ref" ? c = t[s] : l[s] = t[s];
  var u = { type: e, props: l, key: n, ref: c, __k: null, __: null, __b: 0, __e: null, __c: null, constructor: void 0, __v: --Q, __i: -1, __u: 0, __source: r, __self: o };
  if (typeof e == "function" && (c = e.defaultProps)) for (s in c) l[s] === void 0 && (l[s] = c[s]);
  return w.vnode && w.vnode(u), u;
}
const Y = ({
  workspaceName: e,
  logoUrl: t,
  onClose: n,
  brandColor: a = "#6366f1"
}) => /* @__PURE__ */ i("div", { className: "helpin-widget-header", style: { backgroundColor: a }, children: [
  /* @__PURE__ */ i("div", { className: "helpin-header-content", children: t ? /* @__PURE__ */ i("img", { src: t, alt: e, className: "helpin-header-logo" }) : /* @__PURE__ */ i("div", { className: "helpin-header-title", children: e }) }),
  /* @__PURE__ */ i("button", { className: "helpin-header-close", onClick: n, "aria-label": "Close", children: /* @__PURE__ */ i("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "white", children: /* @__PURE__ */ i("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) })
] });
var v, _, N, B, y = 0, U = [], d = w, F = d.__b, L = d.__r, z = d.diffed, E = d.__c, j = d.unmount, q = d.__;
function S(e, t) {
  d.__h && d.__h(_, e, y || t), y = 0;
  var n = _.__H || (_.__H = { __: [], __h: [] });
  return e >= n.__.length && n.__.push({}), n.__[e];
}
function p(e) {
  return y = 1, O(K, e);
}
function O(e, t, n) {
  var a = S(v++, 2);
  if (a.t = e, !a.__c && (a.__ = [K(void 0, t), function(s) {
    var l = a.__N ? a.__N[0] : a.__[0], u = a.t(l, s);
    l !== u && (a.__N = [u, a.__[1]], a.__c.setState({}));
  }], a.__c = _, !_.__f)) {
    var r = function(s, l, u) {
      if (!a.__c.__H) return !0;
      var m = a.__c.__H.__.filter(function(h) {
        return h.__c;
      });
      if (m.every(function(h) {
        return !h.__N;
      })) return !o || o.call(this, s, l, u);
      var f = a.__c.props !== s;
      return m.some(function(h) {
        if (h.__N) {
          var b = h.__[0];
          h.__ = h.__N, h.__N = void 0, b !== h.__[0] && (f = !0);
        }
      }), o && o.call(this, s, l, u) || f;
    };
    _.__f = !0;
    var o = _.shouldComponentUpdate, c = _.componentWillUpdate;
    _.componentWillUpdate = function(s, l, u) {
      if (this.__e) {
        var m = o;
        o = void 0, r(s, l, u), o = m;
      }
      c && c.call(this, s, l, u);
    }, _.shouldComponentUpdate = r;
  }
  return a.__N || a.__;
}
function k(e, t) {
  var n = S(v++, 3);
  !d.__s && W(n.__H, t) && (n.__ = e, n.u = t, _.__H.__h.push(n));
}
function P(e) {
  return y = 5, G(function() {
    return { current: e };
  }, []);
}
function G(e, t) {
  var n = S(v++, 7);
  return W(n.__H, t) && (n.__ = e(), n.__H = t, n.__h = e), n.__;
}
function J() {
  for (var e; e = U.shift(); ) {
    var t = e.__H;
    if (e.__P && t) try {
      t.__h.some(g), t.__h.some(H), t.__h = [];
    } catch (n) {
      t.__h = [], d.__e(n, e.__v);
    }
  }
}
d.__b = function(e) {
  _ = null, F && F(e);
}, d.__ = function(e, t) {
  e && t.__k && t.__k.__m && (e.__m = t.__k.__m), q && q(e, t);
}, d.__r = function(e) {
  L && L(e), v = 0;
  var t = (_ = e.__c).__H;
  t && (N === _ ? (t.__h = [], _.__h = [], t.__.some(function(n) {
    n.__N && (n.__ = n.__N), n.u = n.__N = void 0;
  })) : (t.__h.some(g), t.__h.some(H), t.__h = [], v = 0)), N = _;
}, d.diffed = function(e) {
  z && z(e);
  var t = e.__c;
  t && t.__H && (t.__H.__h.length && (U.push(t) !== 1 && B === d.requestAnimationFrame || ((B = d.requestAnimationFrame) || X)(J)), t.__H.__.some(function(n) {
    n.u && (n.__H = n.u), n.u = void 0;
  })), N = _ = null;
}, d.__c = function(e, t) {
  t.some(function(n) {
    try {
      n.__h.some(g), n.__h = n.__h.filter(function(a) {
        return !a.__ || H(a);
      });
    } catch (a) {
      t.some(function(r) {
        r.__h && (r.__h = []);
      }), t = [], d.__e(a, n.__v);
    }
  }), E && E(e, t);
}, d.unmount = function(e) {
  j && j(e);
  var t, n = e.__c;
  n && n.__H && (n.__H.__.some(function(a) {
    try {
      g(a);
    } catch (r) {
      t = r;
    }
  }), n.__H = void 0, t && d.__e(t, n.__v));
};
var $ = typeof requestAnimationFrame == "function";
function X(e) {
  var t, n = function() {
    clearTimeout(a), $ && cancelAnimationFrame(t), setTimeout(e);
  }, a = setTimeout(n, 35);
  $ && (t = requestAnimationFrame(n));
}
function g(e) {
  var t = _, n = e.__c;
  typeof n == "function" && (e.__c = void 0, n()), _ = t;
}
function H(e) {
  var t = _;
  e.__c = e.__(), _ = t;
}
function W(e, t) {
  return !e || e.length !== t.length || t.some(function(n, a) {
    return n !== e[a];
  });
}
function K(e, t) {
  return typeof t == "function" ? t(e) : t;
}
const Z = ({
  requireEmail: e = !0,
  requireName: t = !0,
  welcomeMessage: n = "Hi! How can we help you today?",
  onSubmit: a
}) => {
  const [r, o] = p(""), [c, s] = p(""), [l, u] = p("email"), m = (h) => {
    h.preventDefault(), c.trim() && t ? u("name") : (a({ name: "", email: c.trim() }), u("done"));
  }, f = (h) => {
    h.preventDefault(), a({ name: r.trim(), email: c.trim() }), u("done");
  };
  return l === "done" ? null : /* @__PURE__ */ i("div", { className: "helpin-pre-chat-form", children: [
    /* @__PURE__ */ i("div", { className: "helpin-pre-chat-welcome", children: n }),
    l === "email" && /* @__PURE__ */ i("form", { onSubmit: m, children: [
      /* @__PURE__ */ i("label", { className: "helpin-sr-only", htmlFor: "helpin-email-input", children: "Email address" }),
      /* @__PURE__ */ i(
        "input",
        {
          id: "helpin-email-input",
          type: "email",
          className: "helpin-input",
          placeholder: "Enter your email",
          value: c,
          onInput: (h) => s(h.target.value),
          required: e,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ i("button", { type: "submit", className: "helpin-btn-primary", children: "Continue" })
    ] }),
    l === "name" && /* @__PURE__ */ i("form", { onSubmit: f, children: [
      /* @__PURE__ */ i("label", { className: "helpin-sr-only", htmlFor: "helpin-name-input", children: "Your name" }),
      /* @__PURE__ */ i(
        "input",
        {
          id: "helpin-name-input",
          type: "text",
          className: "helpin-input",
          placeholder: "Enter your name",
          value: r,
          onInput: (h) => o(h.target.value),
          required: t,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ i("button", { type: "submit", className: "helpin-btn-primary", children: "Start Chat" })
    ] })
  ] });
};
function ee(e) {
  const t = document.createElement("div");
  return t.appendChild(document.createTextNode(e)), t.innerHTML;
}
const te = ({ message: e }) => {
  const t = e.role === "customer", n = e.role === "ai", a = e.role === "agent", r = e.role === "system", o = [
    "helpin-message-bubble",
    t && "helpin-message--customer",
    a && "helpin-message--agent",
    n && "helpin-message--ai",
    r && "helpin-message--system",
    e.isInternal && "helpin-message--internal"
  ].filter(Boolean).join(" "), c = t ? "You" : n ? "AI assistant" : a ? "Support agent" : "System", s = (l) => new Date(l).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return /* @__PURE__ */ i("div", { className: o, role: "listitem", "aria-label": `${c} message`, children: [
    /* @__PURE__ */ i(
      "div",
      {
        className: "helpin-message-content",
        dangerouslySetInnerHTML: { __html: ee(e.content) }
      }
    ),
    e.sources && e.sources.length > 0 && /* @__PURE__ */ i("div", { className: "helpin-message-sources", children: e.sources.map((l, u) => /* @__PURE__ */ i("div", { className: "helpin-source-item", children: l.title }, u)) }),
    e.aiConfidence !== void 0 && /* @__PURE__ */ i("div", { className: "helpin-message-confidence", children: [
      "Confidence: ",
      Math.round(e.aiConfidence * 100),
      "%"
    ] }),
    /* @__PURE__ */ i("div", { className: "helpin-message-time", "aria-label": `Sent at ${s(e.createdAt)}`, children: s(e.createdAt) })
  ] });
}, ne = ({ messages: e }) => {
  const t = P(null);
  k(() => {
    if (t.current) {
      const r = t.current;
      (r.scrollHeight - r.scrollTop - r.clientHeight < 100 || e.length <= 1) && (r.scrollTop = r.scrollHeight);
    }
  }, [e]);
  const n = (r) => {
    const o = new Date(r), c = /* @__PURE__ */ new Date(), s = new Date(c);
    return s.setDate(s.getDate() - 1), o.toDateString() === c.toDateString() ? "Today" : o.toDateString() === s.toDateString() ? "Yesterday" : o.toLocaleDateString();
  }, a = (r, o) => {
    if (o === 0) return n(r);
    const c = new Date(e[o - 1].createdAt), s = new Date(r);
    return c.toDateString() !== s.toDateString() ? n(r) : null;
  };
  return /* @__PURE__ */ i("div", { className: "helpin-message-list", ref: t, role: "list", "aria-label": "Messages", children: e.map((r, o) => {
    const c = a(r.createdAt, o);
    return /* @__PURE__ */ i("div", { children: [
      c && /* @__PURE__ */ i("div", { className: "helpin-date-separator", children: /* @__PURE__ */ i("span", { children: c }) }),
      /* @__PURE__ */ i(te, { message: r })
    ] }, r.id);
  }) });
}, ie = ({
  onSend: e,
  disabled: t = !1,
  placeholder: n = "Type a message..."
}) => {
  const [a, r] = p(""), o = P(null);
  k(() => {
    o.current && (o.current.style.height = "auto", o.current.style.height = `${Math.min(o.current.scrollHeight, 120)}px`);
  }, [a]);
  const c = (l) => {
    l == null || l.preventDefault(), a.trim() && !t && (e(a.trim()), r(""));
  };
  return /* @__PURE__ */ i("form", { className: "helpin-compose-bar", onSubmit: c, children: [
    /* @__PURE__ */ i(
      "textarea",
      {
        ref: o,
        className: "helpin-compose-input",
        value: a,
        onInput: (l) => r(l.target.value),
        onKeyDown: (l) => {
          l.key === "Enter" && !l.shiftKey && (l.preventDefault(), c());
        },
        placeholder: n,
        disabled: t,
        rows: 1,
        "aria-label": n
      }
    ),
    /* @__PURE__ */ i(
      "button",
      {
        type: "submit",
        className: "helpin-compose-send",
        disabled: t || !a.trim(),
        "aria-label": "Send message",
        children: /* @__PURE__ */ i("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ i("path", { d: "M2.01 21L23 12 2.01 3 2 10l15 2-15 2z" }) })
      }
    )
  ] });
}, ae = ({
  replies: e,
  onSelect: t
}) => /* @__PURE__ */ i("div", { className: "helpin-quick-replies", role: "group", "aria-label": "Quick replies", children: e.map((n, a) => /* @__PURE__ */ i(
  "button",
  {
    className: "helpin-quick-reply",
    onClick: () => t(n),
    "aria-label": `Quick reply: ${n}`,
    children: n
  },
  a
)) }), re = ({
  label: e = "is typing..."
}) => /* @__PURE__ */ i("div", { className: "helpin-typing-indicator", children: [
  /* @__PURE__ */ i("span", { className: "helpin-typing-dots", children: [
    /* @__PURE__ */ i("span", {}),
    /* @__PURE__ */ i("span", {}),
    /* @__PURE__ */ i("span", {})
  ] }),
  /* @__PURE__ */ i("span", { className: "helpin-typing-label", children: e })
] }), ce = ({
  config: e,
  messages: t,
  isOpen: n,
  onClose: a,
  onSendMessage: r,
  onQuickReply: o,
  showPreChatForm: c,
  onPreChatSubmit: s,
  isTyping: l = !1,
  quickReplies: u = []
}) => {
  var b, C, D, T, A, M, x, I;
  if (!n) return null;
  const m = ((b = e.branding) == null ? void 0 : b.widgetPosition) || "bottom-right", f = ((C = e.branding) == null ? void 0 : C.primaryColor) || "#6366f1", h = ((D = e.branding) == null ? void 0 : D.showBranding) ?? !0;
  return /* @__PURE__ */ i(
    "div",
    {
      className: "helpin-chat-window",
      style: {
        [m.includes("left") ? "left" : "right"]: "20px",
        bottom: "20px"
      },
      children: [
        /* @__PURE__ */ i(
          Y,
          {
            workspaceName: e.workspaceId || "Support",
            logoUrl: (T = e.branding) == null ? void 0 : T.logoUrl,
            onClose: a,
            brandColor: f,
            showBranding: ((A = e.branding) == null ? void 0 : A.showBranding) ?? !0
          }
        ),
        /* @__PURE__ */ i("div", { className: "helpin-chat-content", children: c ? /* @__PURE__ */ i(
          Z,
          {
            requireEmail: (M = e.features) == null ? void 0 : M.preChatForm,
            welcomeMessage: (x = e.branding) == null ? void 0 : x.welcomeMessage,
            onSubmit: s
          }
        ) : /* @__PURE__ */ i(V, { children: [
          /* @__PURE__ */ i(ne, { messages: t }),
          l && /* @__PURE__ */ i(re, {}),
          u.length > 0 && /* @__PURE__ */ i(ae, { replies: u, onSelect: o }),
          /* @__PURE__ */ i(
            ie,
            {
              onSend: r,
              disabled: (I = e.features) == null ? void 0 : I.aiEnabled
            }
          )
        ] }) }),
        h && /* @__PURE__ */ i("div", { className: "helpin-footer-branding", children: "Powered by Helpin" })
      ]
    }
  );
}, R = {
  chat_bubble: "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z",
  question_mark: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z",
  help: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17v-2h2v2h-2zm2.07-7.75l-.9.92c-.5.51-.82.89-.99 1.37-.13.36-.18.76-.18 1.46h-2v-.5a4.5 4.5 0 0 1 .52-2.08c.3-.55.71-1.04 1.24-1.52l1.24-1.26c.37-.36.59-.86.59-1.41a2.22 2.22 0 0 0-.73-1.64A2.33 2.33 0 0 0 12 7c-.85 0-1.55.3-2.08.83-.53.52-.8 1.16-.87 1.94H7.07c.08-1.42.62-2.57 1.63-3.44C9.71 5.44 10.76 5 12 5c1.3 0 2.4.42 3.3 1.26.9.84 1.37 1.86 1.37 3.07 0 .88-.36 1.68-.93 2.25-.18.18-.37.35-.57.5l-.1.07z"
}, le = "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z", ue = ({
  onClick: e,
  isOpen: t,
  unreadCount: n = 0,
  brandColor: a = "#6366f1",
  icon: r = "chat_bubble"
}) => {
  const o = t ? le : R[r] || R.chat_bubble;
  return /* @__PURE__ */ i(
    "button",
    {
      className: `helpin-launcher ${t ? "helpin-launcher--open" : ""}`,
      onClick: e,
      style: { backgroundColor: a },
      "aria-label": t ? "Close chat" : "Open chat",
      children: [
        /* @__PURE__ */ i("svg", { viewBox: "0 0 24 24", width: "28", height: "28", fill: "white", children: /* @__PURE__ */ i("path", { d: o }) }),
        !t && n > 0 && /* @__PURE__ */ i(
          "span",
          {
            className: "helpin-unread-badge",
            style: { backgroundColor: "#ef4444" },
            "aria-label": `${n} unread messages`,
            children: n > 9 ? "9+" : n
          }
        )
      ]
    }
  );
}, oe = [
  { value: 1, emoji: "😞", label: "Very unsatisfied" },
  { value: 2, emoji: "😕", label: "Unsatisfied" },
  { value: 3, emoji: "😐", label: "Neutral" },
  { value: 4, emoji: "🙂", label: "Satisfied" },
  { value: 5, emoji: "😄", label: "Very satisfied" }
], _e = ({ onSubmit: e }) => {
  const [t, n] = p(null), [a, r] = p(""), [o, c] = p(!1), s = () => {
    t !== null && (e(t, a), c(!0));
  };
  return o ? /* @__PURE__ */ i("div", { className: "helpin-csat-rating helpin-csat-rating--submitted", children: "Thank you for your feedback!" }) : /* @__PURE__ */ i("div", { className: "helpin-csat-rating", children: [
    /* @__PURE__ */ i("div", { className: "helpin-csat-question", children: "How would you rate your experience?" }),
    /* @__PURE__ */ i("div", { className: "helpin-csat-emojis", role: "radiogroup", "aria-label": "Rate your experience", children: oe.map(({ value: l, emoji: u, label: m }) => /* @__PURE__ */ i(
      "button",
      {
        className: `helpin-csat-emoji ${t === l ? "helpin-csat-emoji--selected" : ""}`,
        onClick: () => n(l),
        role: "radio",
        "aria-checked": t === l,
        "aria-label": m,
        children: u
      },
      l
    )) }),
    t !== null && /* @__PURE__ */ i("div", { className: "helpin-csat-feedback", children: [
      /* @__PURE__ */ i(
        "textarea",
        {
          placeholder: "Any additional feedback?",
          value: a,
          onInput: (l) => r(l.target.value),
          "aria-label": "Additional feedback",
          maxLength: 1e3
        }
      ),
      /* @__PURE__ */ i("button", { onClick: s, className: "helpin-btn-primary", children: "Submit" })
    ] })
  ] });
}, he = ({
  text: e,
  isStreaming: t,
  onComplete: n,
  charDelayMs: a = 30
}) => {
  const [r, o] = p("");
  return k(() => {
    if (t && r.length < e.length) {
      const c = setTimeout(() => {
        o(e.slice(0, r.length + 1));
      }, a);
      return () => clearTimeout(c);
    } else !t && e !== r && (o(e), n == null || n());
  }, [e, t, r, n]), /* @__PURE__ */ i("div", { className: "helpin-streaming-text", children: [
    /* @__PURE__ */ i("span", { children: r }),
    t && /* @__PURE__ */ i("span", { className: "helpin-cursor", children: "▊" })
  ] });
};
export {
  ce as ChatWindow,
  ie as ComposeBar,
  _e as CsatRating,
  te as MessageBubble,
  ne as MessageList,
  Z as PreChatForm,
  ae as QuickReplies,
  he as StreamingText,
  re as TypingIndicator,
  Y as WidgetHeader,
  ue as WidgetLauncher
};
