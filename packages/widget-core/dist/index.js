import { options as w, Fragment as z } from "preact";
var $ = 0;
function i(e, t, n, r, l, c) {
  t || (t = {});
  var s, o, a = t;
  if ("ref" in a) for (o in a = {}, t) o == "ref" ? s = t[o] : a[o] = t[o];
  var h = { type: e, props: a, key: n, ref: s, __k: null, __: null, __b: 0, __e: null, __c: null, constructor: void 0, __v: --$, __i: -1, __u: 0, __source: l, __self: c };
  if (typeof e == "function" && (s = e.defaultProps)) for (o in s) a[o] === void 0 && (a[o] = s[o]);
  return w.vnode && w.vnode(h), h;
}
const K = ({
  workspaceName: e,
  logoUrl: t,
  onClose: n,
  brandColor: r = "#6366f1",
  showBranding: l = !0
}) => /* @__PURE__ */ i("div", { className: "helpin-widget-header", style: { backgroundColor: r }, children: [
  /* @__PURE__ */ i("div", { className: "helpin-header-content", children: t ? /* @__PURE__ */ i("img", { src: t, alt: e, className: "helpin-header-logo" }) : /* @__PURE__ */ i("div", { className: "helpin-header-title", children: e }) }),
  /* @__PURE__ */ i("button", { className: "helpin-header-close", onClick: n, "aria-label": "Close", children: /* @__PURE__ */ i("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "white", children: /* @__PURE__ */ i("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) }),
  l && /* @__PURE__ */ i("div", { className: "helpin-header-branding", children: "Powered by Helpin" })
] });
var v, u, N, x, y = 0, R = [], d = w, F = d.__b, j = d.__r, I = d.diffed, B = d.__c, q = d.unmount, E = d.__;
function H(e, t) {
  d.__h && d.__h(u, e, y || t), y = 0;
  var n = u.__H || (u.__H = { __: [], __h: [] });
  return e >= n.__.length && n.__.push({}), n.__[e];
}
function p(e) {
  return y = 1, V(W, e);
}
function V(e, t, n) {
  var r = H(v++, 2);
  if (r.t = e, !r.__c && (r.__ = [W(void 0, t), function(o) {
    var a = r.__N ? r.__N[0] : r.__[0], h = r.t(a, o);
    a !== h && (r.__N = [h, r.__[1]], r.__c.setState({}));
  }], r.__c = u, !u.__f)) {
    var l = function(o, a, h) {
      if (!r.__c.__H) return !0;
      var m = r.__c.__H.__.filter(function(_) {
        return _.__c;
      });
      if (m.every(function(_) {
        return !_.__N;
      })) return !c || c.call(this, o, a, h);
      var f = r.__c.props !== o;
      return m.some(function(_) {
        if (_.__N) {
          var g = _.__[0];
          _.__ = _.__N, _.__N = void 0, g !== _.__[0] && (f = !0);
        }
      }), c && c.call(this, o, a, h) || f;
    };
    u.__f = !0;
    var c = u.shouldComponentUpdate, s = u.componentWillUpdate;
    u.componentWillUpdate = function(o, a, h) {
      if (this.__e) {
        var m = c;
        c = void 0, l(o, a, h), c = m;
      }
      s && s.call(this, o, a, h);
    }, u.shouldComponentUpdate = l;
  }
  return r.__N || r.__;
}
function k(e, t) {
  var n = H(v++, 3);
  !d.__s && U(n.__H, t) && (n.__ = e, n.u = t, u.__H.__h.push(n));
}
function P(e) {
  return y = 5, G(function() {
    return { current: e };
  }, []);
}
function G(e, t) {
  var n = H(v++, 7);
  return U(n.__H, t) && (n.__ = e(), n.__H = t, n.__h = e), n.__;
}
function J() {
  for (var e; e = R.shift(); ) {
    var t = e.__H;
    if (e.__P && t) try {
      t.__h.some(b), t.__h.some(D), t.__h = [];
    } catch (n) {
      t.__h = [], d.__e(n, e.__v);
    }
  }
}
d.__b = function(e) {
  u = null, F && F(e);
}, d.__ = function(e, t) {
  e && t.__k && t.__k.__m && (e.__m = t.__k.__m), E && E(e, t);
}, d.__r = function(e) {
  j && j(e), v = 0;
  var t = (u = e.__c).__H;
  t && (N === u ? (t.__h = [], u.__h = [], t.__.some(function(n) {
    n.__N && (n.__ = n.__N), n.u = n.__N = void 0;
  })) : (t.__h.some(b), t.__h.some(D), t.__h = [], v = 0)), N = u;
}, d.diffed = function(e) {
  I && I(e);
  var t = e.__c;
  t && t.__H && (t.__H.__h.length && (R.push(t) !== 1 && x === d.requestAnimationFrame || ((x = d.requestAnimationFrame) || Q)(J)), t.__H.__.some(function(n) {
    n.u && (n.__H = n.u), n.u = void 0;
  })), N = u = null;
}, d.__c = function(e, t) {
  t.some(function(n) {
    try {
      n.__h.some(b), n.__h = n.__h.filter(function(r) {
        return !r.__ || D(r);
      });
    } catch (r) {
      t.some(function(l) {
        l.__h && (l.__h = []);
      }), t = [], d.__e(r, n.__v);
    }
  }), B && B(e, t);
}, d.unmount = function(e) {
  q && q(e);
  var t, n = e.__c;
  n && n.__H && (n.__H.__.some(function(r) {
    try {
      b(r);
    } catch (l) {
      t = l;
    }
  }), n.__H = void 0, t && d.__e(t, n.__v));
};
var L = typeof requestAnimationFrame == "function";
function Q(e) {
  var t, n = function() {
    clearTimeout(r), L && cancelAnimationFrame(t), setTimeout(e);
  }, r = setTimeout(n, 35);
  L && (t = requestAnimationFrame(n));
}
function b(e) {
  var t = u, n = e.__c;
  typeof n == "function" && (e.__c = void 0, n()), u = t;
}
function D(e) {
  var t = u;
  e.__c = e.__(), u = t;
}
function U(e, t) {
  return !e || e.length !== t.length || t.some(function(n, r) {
    return n !== e[r];
  });
}
function W(e, t) {
  return typeof t == "function" ? t(e) : t;
}
const Y = ({
  requireEmail: e = !0,
  requireName: t = !0,
  welcomeMessage: n = "Hi! How can we help you today?",
  onSubmit: r
}) => {
  const [l, c] = p(""), [s, o] = p(""), [a, h] = p("email"), m = (_) => {
    _.preventDefault(), s.trim() && t ? h("name") : (r({ name: "", email: s.trim() }), h("done"));
  }, f = (_) => {
    _.preventDefault(), r({ name: l.trim(), email: s.trim() }), h("done");
  };
  return a === "done" ? null : /* @__PURE__ */ i("div", { className: "helpin-pre-chat-form", children: [
    /* @__PURE__ */ i("div", { className: "helpin-pre-chat-welcome", children: n }),
    a === "email" && /* @__PURE__ */ i("form", { onSubmit: m, children: [
      /* @__PURE__ */ i(
        "input",
        {
          type: "email",
          className: "helpin-input",
          placeholder: "Enter your email",
          value: s,
          onInput: (_) => o(_.target.value),
          required: e,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ i("button", { type: "submit", className: "helpin-btn-primary", children: "Continue" })
    ] }),
    a === "name" && /* @__PURE__ */ i("form", { onSubmit: f, children: [
      /* @__PURE__ */ i(
        "input",
        {
          type: "text",
          className: "helpin-input",
          placeholder: "Enter your name",
          value: l,
          onInput: (_) => c(_.target.value),
          required: t,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ i("button", { type: "submit", className: "helpin-btn-primary", children: "Start Chat" })
    ] })
  ] });
}, O = ({ message: e }) => {
  const t = e.role === "customer", n = e.role === "ai", r = e.role === "agent", l = e.role === "system", c = [
    "helpin-message-bubble",
    t && "helpin-message--customer",
    r && "helpin-message--agent",
    n && "helpin-message--ai",
    l && "helpin-message--system",
    e.isInternal && "helpin-message--internal"
  ].filter(Boolean).join(" "), s = (o) => new Date(o).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return /* @__PURE__ */ i("div", { className: c, children: [
    /* @__PURE__ */ i("div", { className: "helpin-message-content", children: e.content }),
    e.sources && e.sources.length > 0 && /* @__PURE__ */ i("div", { className: "helpin-message-sources", children: e.sources.map((o, a) => /* @__PURE__ */ i("div", { className: "helpin-source-item", children: [
      "📄 ",
      o.title
    ] }, a)) }),
    e.aiConfidence !== void 0 && /* @__PURE__ */ i("div", { className: "helpin-message-confidence", children: [
      "Confidence: ",
      Math.round(e.aiConfidence * 100),
      "%"
    ] }),
    /* @__PURE__ */ i("div", { className: "helpin-message-time", children: s(e.createdAt) })
  ] });
}, X = ({ messages: e }) => {
  const t = P(null);
  k(() => {
    t.current && (t.current.scrollTop = t.current.scrollHeight);
  }, [e]);
  const n = (l) => {
    const c = new Date(l), s = /* @__PURE__ */ new Date(), o = new Date(s);
    return o.setDate(o.getDate() - 1), c.toDateString() === s.toDateString() ? "Today" : c.toDateString() === o.toDateString() ? "Yesterday" : c.toLocaleDateString();
  }, r = (l, c) => {
    if (c === 0) return n(l);
    const s = new Date(e[c - 1].createdAt), o = new Date(l);
    return s.toDateString() !== o.toDateString() ? n(l) : null;
  };
  return /* @__PURE__ */ i("div", { className: "helpin-message-list", ref: t, children: e.map((l, c) => {
    const s = r(l.createdAt, c);
    return /* @__PURE__ */ i("div", { children: [
      s && /* @__PURE__ */ i("div", { className: "helpin-date-separator", children: /* @__PURE__ */ i("span", { children: s }) }),
      /* @__PURE__ */ i(O, { message: l })
    ] }, l.id);
  }) });
}, Z = ({
  onSend: e,
  disabled: t = !1,
  placeholder: n = "Type a message..."
}) => {
  const [r, l] = p(""), c = P(null);
  k(() => {
    c.current && (c.current.style.height = "auto", c.current.style.height = `${Math.min(c.current.scrollHeight, 120)}px`);
  }, [r]);
  const s = (a) => {
    a == null || a.preventDefault(), r.trim() && !t && (e(r.trim()), l(""));
  };
  return /* @__PURE__ */ i("form", { className: "helpin-compose-bar", onSubmit: s, children: [
    /* @__PURE__ */ i(
      "textarea",
      {
        ref: c,
        className: "helpin-compose-input",
        value: r,
        onInput: (a) => l(a.target.value),
        onKeyDown: (a) => {
          a.key === "Enter" && !a.shiftKey && (a.preventDefault(), s());
        },
        placeholder: n,
        disabled: t,
        rows: 1
      }
    ),
    /* @__PURE__ */ i(
      "button",
      {
        type: "submit",
        className: "helpin-compose-send",
        disabled: t || !r.trim(),
        children: /* @__PURE__ */ i("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ i("path", { d: "M2.01 21L23 12 2.01 3 2 10l15 2-15 2z" }) })
      }
    )
  ] });
}, ee = ({
  replies: e,
  onSelect: t
}) => /* @__PURE__ */ i("div", { className: "helpin-quick-replies", children: e.map((n, r) => /* @__PURE__ */ i(
  "button",
  {
    className: "helpin-quick-reply",
    onClick: () => t(n),
    children: n
  },
  r
)) }), te = ({
  label: e = "is typing..."
}) => /* @__PURE__ */ i("div", { className: "helpin-typing-indicator", children: [
  /* @__PURE__ */ i("span", { className: "helpin-typing-dots", children: [
    /* @__PURE__ */ i("span", {}),
    /* @__PURE__ */ i("span", {}),
    /* @__PURE__ */ i("span", {})
  ] }),
  /* @__PURE__ */ i("span", { className: "helpin-typing-label", children: e })
] }), re = ({
  config: e,
  messages: t,
  isOpen: n,
  onClose: r,
  onSendMessage: l,
  onQuickReply: c,
  showPreChatForm: s,
  onPreChatSubmit: o,
  isTyping: a = !1,
  quickReplies: h = []
}) => {
  var _, g, S, C, T, A, M;
  if (!n) return null;
  const m = ((_ = e.branding) == null ? void 0 : _.widgetPosition) || "bottom-right", f = ((g = e.branding) == null ? void 0 : g.primaryColor) || "#6366f1";
  return /* @__PURE__ */ i(
    "div",
    {
      className: "helpin-chat-window",
      style: { [m.includes("left") ? "left" : "right"]: "20px" },
      children: [
        /* @__PURE__ */ i(
          K,
          {
            workspaceName: e.workspaceId || "Support",
            logoUrl: (S = e.branding) == null ? void 0 : S.logoUrl,
            onClose: r,
            brandColor: f,
            showBranding: (C = e.features) == null ? void 0 : C.csatRating
          }
        ),
        /* @__PURE__ */ i("div", { className: "helpin-chat-content", children: s ? /* @__PURE__ */ i(
          Y,
          {
            requireEmail: (T = e.features) == null ? void 0 : T.preChatForm,
            welcomeMessage: (A = e.branding) == null ? void 0 : A.welcomeMessage,
            onSubmit: o
          }
        ) : /* @__PURE__ */ i(z, { children: [
          /* @__PURE__ */ i(X, { messages: t }),
          a && /* @__PURE__ */ i(te, {}),
          h.length > 0 && /* @__PURE__ */ i(ee, { replies: h, onSelect: c }),
          /* @__PURE__ */ i(
            Z,
            {
              onSend: l,
              disabled: (M = e.features) == null ? void 0 : M.aiEnabled
            }
          )
        ] }) })
      ]
    }
  );
}, ae = ({
  onClick: e,
  isOpen: t,
  unreadCount: n = 0,
  brandColor: r = "#6366f1"
}) => /* @__PURE__ */ i(
  "button",
  {
    className: `helpin-launcher ${t ? "helpin-launcher--open" : ""}`,
    onClick: e,
    style: { backgroundColor: r },
    "aria-label": t ? "Close chat" : "Open chat",
    children: t ? /* @__PURE__ */ i("svg", { viewBox: "0 0 24 24", width: "28", height: "28", fill: "white", children: /* @__PURE__ */ i("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) : /* @__PURE__ */ i(z, { children: [
      /* @__PURE__ */ i("svg", { viewBox: "0 0 24 24", width: "28", height: "28", fill: "white", children: /* @__PURE__ */ i("path", { d: "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z" }) }),
      n > 0 && /* @__PURE__ */ i("span", { className: "helpin-unread-badge", style: { backgroundColor: "#ef4444" }, children: n > 9 ? "9+" : n })
    ] })
  }
), ne = [
  { value: 1, emoji: "😞" },
  { value: 2, emoji: "😕" },
  { value: 3, emoji: "😐" },
  { value: 4, emoji: "🙂" },
  { value: 5, emoji: "😄" }
], le = ({ onSubmit: e }) => {
  const [t, n] = p(null), [r, l] = p(""), [c, s] = p(!1), o = () => {
    t !== null && (e(t, r), s(!0));
  };
  return c ? /* @__PURE__ */ i("div", { className: "helpin-csat-rating helpin-csat-rating--submitted", children: "Thank you for your feedback!" }) : /* @__PURE__ */ i("div", { className: "helpin-csat-rating", children: [
    /* @__PURE__ */ i("div", { className: "helpin-csat-question", children: "How would you rate your experience?" }),
    /* @__PURE__ */ i("div", { className: "helpin-csat-emojis", children: ne.map(({ value: a, emoji: h }) => /* @__PURE__ */ i(
      "button",
      {
        className: `helpin-csat-emoji ${t === a ? "helpin-csat-emoji--selected" : ""}`,
        onClick: () => n(a),
        children: h
      },
      a
    )) }),
    t !== null && /* @__PURE__ */ i("div", { className: "helpin-csat-feedback", children: [
      /* @__PURE__ */ i(
        "textarea",
        {
          placeholder: "Any additional feedback?",
          value: r,
          onInput: (a) => l(a.target.value)
        }
      ),
      /* @__PURE__ */ i("button", { onClick: o, className: "helpin-btn-primary", children: "Submit" })
    ] })
  ] });
}, oe = ({
  text: e,
  isStreaming: t,
  onComplete: n
}) => {
  const [r, l] = p("");
  return k(() => {
    if (t && r.length < e.length) {
      const c = setTimeout(() => {
        l(e.slice(0, r.length + 1));
      }, 30);
      return () => clearTimeout(c);
    } else !t && e !== r && (l(e), n == null || n());
  }, [e, t, r, n]), /* @__PURE__ */ i("div", { className: "helpin-streaming-text", children: [
    /* @__PURE__ */ i("span", { children: r }),
    t && /* @__PURE__ */ i("span", { className: "helpin-cursor", children: "▊" })
  ] });
};
export {
  re as ChatWindow,
  Z as ComposeBar,
  le as CsatRating,
  O as MessageBubble,
  X as MessageList,
  Y as PreChatForm,
  ee as QuickReplies,
  oe as StreamingText,
  te as TypingIndicator,
  K as WidgetHeader,
  ae as WidgetLauncher
};
