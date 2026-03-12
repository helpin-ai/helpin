import { options as T } from "preact";
var Z = 0;
function e(n, l, i, t, a, r) {
  l || (l = {});
  var o, c, s = l;
  if ("ref" in s) for (c in s = {}, l) c == "ref" ? o = l[c] : s[c] = l[c];
  var h = { type: n, props: s, key: i, ref: o, __k: null, __: null, __b: 0, __e: null, __c: null, constructor: void 0, __v: --Z, __i: -1, __u: 0, __source: a, __self: r };
  if (typeof n == "function" && (o = n.defaultProps)) for (c in o) s[c] === void 0 && (s[c] = o[c]);
  return T.vnode && T.vnode(h), h;
}
var C, m, L, $, x = 0, Y = [], p = T, q = p.__b, j = p.__r, O = p.diffed, R = p.__c, U = p.unmount, W = p.__;
function V(n, l) {
  p.__h && p.__h(m, n, x || l), x = 0;
  var i = m.__H || (m.__H = { __: [], __h: [] });
  return n >= i.__.length && i.__.push({}), i.__[n];
}
function _(n) {
  return x = 1, ee(X, n);
}
function ee(n, l, i) {
  var t = V(C++, 2);
  if (t.t = n, !t.__c && (t.__ = [X(void 0, l), function(c) {
    var s = t.__N ? t.__N[0] : t.__[0], h = t.t(s, c);
    s !== h && (t.__N = [h, t.__[1]], t.__c.setState({}));
  }], t.__c = m, !m.__f)) {
    var a = function(c, s, h) {
      if (!t.__c.__H) return !0;
      var v = t.__c.__H.__.filter(function(d) {
        return d.__c;
      });
      if (v.every(function(d) {
        return !d.__N;
      })) return !r || r.call(this, c, s, h);
      var f = t.__c.props !== c;
      return v.some(function(d) {
        if (d.__N) {
          var g = d.__[0];
          d.__ = d.__N, d.__N = void 0, g !== d.__[0] && (f = !0);
        }
      }), r && r.call(this, c, s, h) || f;
    };
    m.__f = !0;
    var r = m.shouldComponentUpdate, o = m.componentWillUpdate;
    m.componentWillUpdate = function(c, s, h) {
      if (this.__e) {
        var v = r;
        r = void 0, a(c, s, h), r = v;
      }
      o && o.call(this, c, s, h);
    }, m.shouldComponentUpdate = a;
  }
  return t.__N || t.__;
}
function E(n, l) {
  var i = V(C++, 3);
  !p.__s && J(i.__H, l) && (i.__ = n, i.u = l, m.__H.__h.push(i));
}
function G(n) {
  return x = 5, ne(function() {
    return { current: n };
  }, []);
}
function ne(n, l) {
  var i = V(C++, 7);
  return J(i.__H, l) && (i.__ = n(), i.__H = l, i.__h = n), i.__;
}
function le() {
  for (var n; n = Y.shift(); ) {
    var l = n.__H;
    if (n.__P && l) try {
      l.__h.some(M), l.__h.some(I), l.__h = [];
    } catch (i) {
      l.__h = [], p.__e(i, n.__v);
    }
  }
}
p.__b = function(n) {
  m = null, q && q(n);
}, p.__ = function(n, l) {
  n && l.__k && l.__k.__m && (n.__m = l.__k.__m), W && W(n, l);
}, p.__r = function(n) {
  j && j(n), C = 0;
  var l = (m = n.__c).__H;
  l && (L === m ? (l.__h = [], m.__h = [], l.__.some(function(i) {
    i.__N && (i.__ = i.__N), i.u = i.__N = void 0;
  })) : (l.__h.some(M), l.__h.some(I), l.__h = [], C = 0)), L = m;
}, p.diffed = function(n) {
  O && O(n);
  var l = n.__c;
  l && l.__H && (l.__H.__h.length && (Y.push(l) !== 1 && $ === p.requestAnimationFrame || (($ = p.requestAnimationFrame) || ie)(le)), l.__H.__.some(function(i) {
    i.u && (i.__H = i.u), i.u = void 0;
  })), L = m = null;
}, p.__c = function(n, l) {
  l.some(function(i) {
    try {
      i.__h.some(M), i.__h = i.__h.filter(function(t) {
        return !t.__ || I(t);
      });
    } catch (t) {
      l.some(function(a) {
        a.__h && (a.__h = []);
      }), l = [], p.__e(t, i.__v);
    }
  }), R && R(n, l);
}, p.unmount = function(n) {
  U && U(n);
  var l, i = n.__c;
  i && i.__H && (i.__H.__.some(function(t) {
    try {
      M(t);
    } catch (a) {
      l = a;
    }
  }), i.__H = void 0, l && p.__e(l, i.__v));
};
var K = typeof requestAnimationFrame == "function";
function ie(n) {
  var l, i = function() {
    clearTimeout(t), K && cancelAnimationFrame(l), setTimeout(n);
  }, t = setTimeout(i, 35);
  K && (l = requestAnimationFrame(i));
}
function M(n) {
  var l = m, i = n.__c;
  typeof i == "function" && (n.__c = void 0, i()), m = l;
}
function I(n) {
  var l = m;
  n.__c = n.__(), m = l;
}
function J(n, l) {
  return !n || n.length !== l.length || l.some(function(i, t) {
    return i !== n[t];
  });
}
function X(n, l) {
  return typeof l == "function" ? l(n) : l;
}
const te = "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z", ae = "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z", se = "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z", re = [
  { view: "home", label: "Home", icon: te },
  { view: "messages", label: "Messages", icon: ae },
  { view: "help", label: "Help", icon: se }
], ce = ({
  activeView: n,
  onNavigate: l,
  brandColor: i = "#6366f1"
}) => /* @__PURE__ */ e("div", { className: "helpin-bottom-nav", children: re.map((t) => {
  const a = n === t.view;
  return /* @__PURE__ */ e(
    "button",
    {
      className: `helpin-bottom-nav-item ${a ? "helpin-bottom-nav-item--active" : ""}`,
      onClick: () => l(t.view),
      style: a ? { color: i } : void 0,
      children: [
        /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "22", height: "22", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: t.icon }) }),
        /* @__PURE__ */ e("span", { className: "helpin-bottom-nav-label", children: t.label })
      ]
    },
    t.view
  );
}) }), A = "M3.4 20.4l17.45-7.48a1 1 0 000-1.84L3.4 3.6a.993.993 0 00-1.39.91L2 9.12c0 .5.37.93.87.99L17 12 2.87 13.88c-.5.07-.87.5-.87 1l.01 4.61c0 .71.73 1.2 1.39.91z", oe = ({
  config: n,
  onSendMessage: l,
  onNavigate: i,
  showPreChatForm: t,
  onPreChatSubmit: a
}) => {
  var w, y, N;
  const [r, o] = _(""), [c, s] = _(""), [h, v] = _(""), [f, d] = _("email"), g = ((w = n.branding) == null ? void 0 : w.primaryColor) || "#6366f1", H = (y = n.branding) == null ? void 0 : y.logoUrl, D = ((N = n.branding) == null ? void 0 : N.welcomeMessage) || "How can we help?", k = n.workspaceName || "Support", b = () => {
    const u = r.trim();
    u && (l(u), o(""));
  }, B = (u) => {
    u.key === "Enter" && !u.shiftKey && (u.preventDefault(), b());
  }, S = (u) => {
    var F;
    u.preventDefault(), c.trim() && ((F = n.features) != null && F.preChatForm ? d("name") : a({ name: "", email: c.trim() }));
  }, z = (u) => {
    u.preventDefault(), a({ name: h.trim(), email: c.trim() });
  };
  return /* @__PURE__ */ e("div", { className: "helpin-home-view", children: [
    /* @__PURE__ */ e("div", { className: "helpin-home-header", style: { background: `linear-gradient(135deg, ${g}, ${g}88)` }, children: H ? /* @__PURE__ */ e("img", { src: H, alt: k, className: "helpin-home-logo" }) : /* @__PURE__ */ e("div", { className: "helpin-home-logo-placeholder", style: { backgroundColor: "#ffffff" }, children: /* @__PURE__ */ e("span", { children: k.charAt(0).toUpperCase() }) }) }),
    /* @__PURE__ */ e("div", { className: "helpin-home-content", children: [
      /* @__PURE__ */ e("h2", { className: "helpin-home-welcome", children: D }),
      t ? /* @__PURE__ */ e("div", { className: "helpin-home-prechat", children: f === "email" ? /* @__PURE__ */ e("form", { onSubmit: S, className: "helpin-home-form", children: [
        /* @__PURE__ */ e(
          "input",
          {
            type: "email",
            className: "helpin-home-input",
            placeholder: "Enter your email to get started...",
            value: c,
            onInput: (u) => s(u.target.value),
            required: !0
          }
        ),
        /* @__PURE__ */ e(
          "button",
          {
            type: "submit",
            className: "helpin-home-send",
            style: { backgroundColor: g },
            disabled: !c.trim(),
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ e("path", { d: A }) })
          }
        )
      ] }) : /* @__PURE__ */ e("form", { onSubmit: z, className: "helpin-home-form", children: [
        /* @__PURE__ */ e(
          "input",
          {
            type: "text",
            className: "helpin-home-input",
            placeholder: "What's your name?",
            value: h,
            onInput: (u) => v(u.target.value)
          }
        ),
        /* @__PURE__ */ e(
          "button",
          {
            type: "submit",
            className: "helpin-home-send",
            style: { backgroundColor: g },
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ e("path", { d: A }) })
          }
        )
      ] }) }) : /* @__PURE__ */ e("div", { className: "helpin-home-search", children: [
        /* @__PURE__ */ e(
          "input",
          {
            type: "text",
            className: "helpin-home-input",
            placeholder: "Ask me anything...",
            value: r,
            onInput: (u) => o(u.target.value),
            onKeyDown: B
          }
        ),
        /* @__PURE__ */ e(
          "button",
          {
            className: "helpin-home-send",
            onClick: b,
            style: { backgroundColor: g },
            disabled: !r.trim(),
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ e("path", { d: A }) })
          }
        )
      ] }),
      /* @__PURE__ */ e("div", { className: "helpin-home-actions", children: [
        /* @__PURE__ */ e("button", { className: "helpin-home-action", onClick: () => i("messages"), children: [
          /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: "M20 2H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h14l4 4V4c0-1.1-.9-2-2-2zm-2 12H6v-2h12v2zm0-3H6V9h12v2zm0-3H6V6h12v2z" }) }),
          /* @__PURE__ */ e("div", { className: "helpin-home-action-text", children: [
            /* @__PURE__ */ e("span", { className: "helpin-home-action-title", children: "Send us a message" }),
            /* @__PURE__ */ e("span", { className: "helpin-home-action-desc", children: "We typically reply in a few minutes" })
          ] }),
          /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-home-action-arrow", children: /* @__PURE__ */ e("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
        ] }),
        /* @__PURE__ */ e("button", { className: "helpin-home-action", onClick: () => i("help"), children: [
          /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z" }) }),
          /* @__PURE__ */ e("div", { className: "helpin-home-action-text", children: [
            /* @__PURE__ */ e("span", { className: "helpin-home-action-title", children: "Help center" }),
            /* @__PURE__ */ e("span", { className: "helpin-home-action-desc", children: "Find answers to common questions" })
          ] }),
          /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-home-action-arrow", children: /* @__PURE__ */ e("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
        ] })
      ] })
    ] })
  ] });
};
function he(n) {
  const l = document.createElement("div");
  return l.appendChild(document.createTextNode(n)), l.innerHTML;
}
const de = ({ message: n }) => {
  const l = n.role === "customer", i = n.role === "ai", t = n.role === "agent", a = n.role === "system", r = [
    "helpin-message-bubble",
    l && "helpin-message--customer",
    t && "helpin-message--agent",
    i && "helpin-message--ai",
    a && "helpin-message--system",
    n.isInternal && "helpin-message--internal"
  ].filter(Boolean).join(" "), o = l ? "You" : i ? "AI assistant" : t ? "Support agent" : "System", c = (s) => new Date(s).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  return /* @__PURE__ */ e("div", { className: r, role: "listitem", "aria-label": `${o} message`, children: [
    /* @__PURE__ */ e(
      "div",
      {
        className: "helpin-message-content",
        dangerouslySetInnerHTML: { __html: he(n.content) }
      }
    ),
    n.sources && n.sources.length > 0 && /* @__PURE__ */ e("div", { className: "helpin-message-sources", children: n.sources.map((s, h) => /* @__PURE__ */ e("div", { className: "helpin-source-item", children: s.title }, h)) }),
    n.aiConfidence !== void 0 && /* @__PURE__ */ e("div", { className: "helpin-message-confidence", children: [
      "Confidence: ",
      Math.round(n.aiConfidence * 100),
      "%"
    ] }),
    /* @__PURE__ */ e("div", { className: "helpin-message-time", "aria-label": `Sent at ${c(n.createdAt)}`, children: c(n.createdAt) })
  ] });
}, me = ({ messages: n }) => {
  const l = G(null);
  E(() => {
    if (l.current) {
      const a = l.current;
      (a.scrollHeight - a.scrollTop - a.clientHeight < 100 || n.length <= 1) && (a.scrollTop = a.scrollHeight);
    }
  }, [n]);
  const i = (a) => {
    const r = new Date(a), o = /* @__PURE__ */ new Date(), c = new Date(o);
    return c.setDate(c.getDate() - 1), r.toDateString() === o.toDateString() ? "Today" : r.toDateString() === c.toDateString() ? "Yesterday" : r.toLocaleDateString();
  }, t = (a, r) => {
    if (r === 0) return i(a);
    const o = new Date(n[r - 1].createdAt), c = new Date(a);
    return o.toDateString() !== c.toDateString() ? i(a) : null;
  };
  return /* @__PURE__ */ e("div", { className: "helpin-message-list", ref: l, role: "list", "aria-label": "Messages", children: n.map((a, r) => {
    const o = t(a.createdAt, r);
    return /* @__PURE__ */ e("div", { children: [
      o && /* @__PURE__ */ e("div", { className: "helpin-date-separator", children: /* @__PURE__ */ e("span", { children: o }) }),
      /* @__PURE__ */ e(de, { message: a })
    ] }, a.id);
  }) });
}, P = ({
  onSend: n,
  disabled: l = !1,
  placeholder: i = "Type a message..."
}) => {
  const [t, a] = _(""), r = G(null);
  E(() => {
    r.current && (r.current.style.height = "auto", r.current.style.height = `${Math.min(r.current.scrollHeight, 120)}px`);
  }, [t]);
  const o = (s) => {
    s == null || s.preventDefault(), t.trim() && !l && (n(t.trim()), a(""));
  };
  return /* @__PURE__ */ e("form", { className: "helpin-compose-bar", onSubmit: o, children: [
    /* @__PURE__ */ e(
      "textarea",
      {
        ref: r,
        className: "helpin-compose-input",
        value: t,
        onInput: (s) => a(s.target.value),
        onKeyDown: (s) => {
          s.key === "Enter" && !s.shiftKey && (s.preventDefault(), o());
        },
        placeholder: i,
        disabled: l,
        rows: 1,
        "aria-label": i
      }
    ),
    /* @__PURE__ */ e(
      "button",
      {
        type: "submit",
        className: "helpin-compose-send",
        disabled: l || !t.trim(),
        "aria-label": "Send message",
        children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: "M2.01 21L23 12 2.01 3 2 10l15 2-15 2z" }) })
      }
    )
  ] });
}, pe = ({
  label: n = "is typing..."
}) => /* @__PURE__ */ e("div", { className: "helpin-typing-indicator", children: [
  /* @__PURE__ */ e("span", { className: "helpin-typing-dots", children: [
    /* @__PURE__ */ e("span", {}),
    /* @__PURE__ */ e("span", {}),
    /* @__PURE__ */ e("span", {})
  ] }),
  /* @__PURE__ */ e("span", { className: "helpin-typing-label", children: n })
] }), ue = ({
  replies: n,
  onSelect: l
}) => /* @__PURE__ */ e("div", { className: "helpin-quick-replies", role: "group", "aria-label": "Quick replies", children: n.map((i, t) => /* @__PURE__ */ e(
  "button",
  {
    className: "helpin-quick-reply",
    onClick: () => l(i),
    "aria-label": `Quick reply: ${i}`,
    children: i
  },
  t
)) }), _e = "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z", ve = ({
  config: n,
  messages: l,
  onSendMessage: i,
  onQuickReply: t,
  isTyping: a = !1,
  quickReplies: r = [],
  hasConversation: o
}) => !o || l.length === 0 ? /* @__PURE__ */ e("div", { className: "helpin-messages-view", children: [
  /* @__PURE__ */ e("div", { className: "helpin-messages-header", children: /* @__PURE__ */ e("span", { className: "helpin-messages-title", children: "Messages" }) }),
  /* @__PURE__ */ e("div", { className: "helpin-messages-empty", children: [
    /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "48", height: "48", fill: "currentColor", className: "helpin-messages-empty-icon", children: /* @__PURE__ */ e("path", { d: _e }) }),
    /* @__PURE__ */ e("h3", { className: "helpin-messages-empty-title", children: "No messages" }),
    /* @__PURE__ */ e("p", { className: "helpin-messages-empty-desc", children: "Messages from the team will be shown here" })
  ] }),
  /* @__PURE__ */ e("div", { className: "helpin-messages-new-container", children: /* @__PURE__ */ e(
    P,
    {
      onSend: i,
      placeholder: "Ask a question"
    }
  ) })
] }) : /* @__PURE__ */ e("div", { className: "helpin-messages-view", children: [
  /* @__PURE__ */ e("div", { className: "helpin-messages-header", children: /* @__PURE__ */ e("span", { className: "helpin-messages-title", children: "Messages" }) }),
  /* @__PURE__ */ e("div", { className: "helpin-messages-thread", children: [
    /* @__PURE__ */ e(me, { messages: l }),
    a && /* @__PURE__ */ e(pe, {}),
    r.length > 0 && /* @__PURE__ */ e(ue, { replies: r, onSelect: t })
  ] }),
  /* @__PURE__ */ e(P, { onSend: i })
] }), fe = ({
  config: n,
  onNavigate: l
}) => /* @__PURE__ */ e("div", { className: "helpin-help-view", children: [
  /* @__PURE__ */ e("div", { className: "helpin-help-header", children: /* @__PURE__ */ e("span", { className: "helpin-help-title", children: "Help" }) }),
  /* @__PURE__ */ e("div", { className: "helpin-help-content", children: [
    /* @__PURE__ */ e("div", { className: "helpin-help-search", children: [
      /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", className: "helpin-help-search-icon", children: /* @__PURE__ */ e("path", { d: "M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z" }) }),
      /* @__PURE__ */ e(
        "input",
        {
          type: "text",
          className: "helpin-help-search-input",
          placeholder: "Search for help..."
        }
      )
    ] }),
    /* @__PURE__ */ e("div", { className: "helpin-help-links", children: [
      /* @__PURE__ */ e("button", { className: "helpin-help-link", onClick: () => l("messages"), children: [
        /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: "M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z" }) }),
        /* @__PURE__ */ e("div", { className: "helpin-help-link-text", children: [
          /* @__PURE__ */ e("span", { className: "helpin-help-link-title", children: "Contact us" }),
          /* @__PURE__ */ e("span", { className: "helpin-help-link-desc", children: "Send us a message and we'll get back to you" })
        ] }),
        /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-help-link-arrow", children: /* @__PURE__ */ e("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
      ] }),
      /* @__PURE__ */ e("div", { className: "helpin-help-divider" }),
      /* @__PURE__ */ e("a", { className: "helpin-help-link", href: "#", target: "_blank", rel: "noopener noreferrer", children: [
        /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: "M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z" }) }),
        /* @__PURE__ */ e("div", { className: "helpin-help-link-text", children: [
          /* @__PURE__ */ e("span", { className: "helpin-help-link-title", children: "Browse our docs" }),
          /* @__PURE__ */ e("span", { className: "helpin-help-link-desc", children: "Find detailed guides and documentation" })
        ] }),
        /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-help-link-arrow", children: /* @__PURE__ */ e("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
      ] })
    ] })
  ] })
] }), we = ({
  config: n,
  messages: l,
  isOpen: i,
  onClose: t,
  onSendMessage: a,
  onQuickReply: r,
  showPreChatForm: o,
  onPreChatSubmit: c,
  isTyping: s = !1,
  quickReplies: h = [],
  initialView: v = "home"
}) => {
  var S, z, w, y;
  const [f, d] = _(v);
  if (!i) return null;
  const g = ((S = n.branding) == null ? void 0 : S.widgetPosition) || "bottom-right", H = ((z = n.branding) == null ? void 0 : z.primaryColor) || "#6366f1", D = ((w = n.branding) == null ? void 0 : w.showBranding) ?? !0, k = ((y = n.branding) == null ? void 0 : y.colorScheme) || "light", b = (N) => {
    d(N);
  }, B = (N) => {
    a(N), d("messages");
  };
  return /* @__PURE__ */ e(
    "div",
    {
      className: `helpin-chat-window helpin-theme-${k}`,
      style: {
        [g.includes("left") ? "left" : "right"]: "20px",
        bottom: "20px"
      },
      children: [
        /* @__PURE__ */ e("button", { className: "helpin-window-close", onClick: t, "aria-label": "Close", children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) }),
        /* @__PURE__ */ e("div", { className: "helpin-view-container", children: [
          f === "home" && /* @__PURE__ */ e(
            oe,
            {
              config: n,
              onSendMessage: B,
              onNavigate: b,
              showPreChatForm: o,
              onPreChatSubmit: c
            }
          ),
          f === "messages" && /* @__PURE__ */ e(
            ve,
            {
              config: n,
              messages: l,
              onSendMessage: a,
              onQuickReply: r,
              isTyping: s,
              quickReplies: h,
              hasConversation: l.length > 0
            }
          ),
          f === "help" && /* @__PURE__ */ e(fe, { config: n, onNavigate: b })
        ] }),
        D && /* @__PURE__ */ e("div", { className: "helpin-powered-by", children: [
          /* @__PURE__ */ e("span", { children: "Powered by" }),
          /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "14", height: "14", fill: "currentColor", className: "helpin-powered-by-icon", children: /* @__PURE__ */ e("path", { d: "M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" }) }),
          /* @__PURE__ */ e("span", { className: "helpin-powered-by-name", children: "Helpin" })
        ] }),
        /* @__PURE__ */ e(
          ce,
          {
            activeView: f,
            onNavigate: d,
            brandColor: H
          }
        )
      ]
    }
  );
}, ye = ({
  workspaceName: n,
  logoUrl: l,
  onClose: i,
  brandColor: t = "#6366f1"
}) => /* @__PURE__ */ e("div", { className: "helpin-widget-header", style: { backgroundColor: t }, children: [
  /* @__PURE__ */ e("div", { className: "helpin-header-content", children: l ? /* @__PURE__ */ e("img", { src: l, alt: n, className: "helpin-header-logo" }) : /* @__PURE__ */ e("div", { className: "helpin-header-title", children: n }) }),
  /* @__PURE__ */ e("button", { className: "helpin-header-close", onClick: i, "aria-label": "Close", children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "white", children: /* @__PURE__ */ e("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) })
] }), Q = {
  chat_bubble: "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z",
  question_mark: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z",
  help: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17v-2h2v2h-2zm2.07-7.75l-.9.92c-.5.51-.82.89-.99 1.37-.13.36-.18.76-.18 1.46h-2v-.5a4.5 4.5 0 0 1 .52-2.08c.3-.55.71-1.04 1.24-1.52l1.24-1.26c.37-.36.59-.86.59-1.41a2.22 2.22 0 0 0-.73-1.64A2.33 2.33 0 0 0 12 7c-.85 0-1.55.3-2.08.83-.53.52-.8 1.16-.87 1.94H7.07c.08-1.42.62-2.57 1.63-3.44C9.71 5.44 10.76 5 12 5c1.3 0 2.4.42 3.3 1.26.9.84 1.37 1.86 1.37 3.07 0 .88-.36 1.68-.93 2.25-.18.18-.37.35-.57.5l-.1.07z"
}, ge = "M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6z", Ce = ({
  onClick: n,
  isOpen: l,
  unreadCount: i = 0,
  brandColor: t = "#6366f1",
  buttonColor: a,
  buttonIconColor: r,
  icon: o = "chat_bubble"
}) => {
  const c = a || t, s = r || "#ffffff", h = l ? ge : Q[o] || Q.chat_bubble;
  return /* @__PURE__ */ e(
    "button",
    {
      className: `helpin-launcher ${l ? "helpin-launcher--open" : ""}`,
      onClick: n,
      style: { backgroundColor: c },
      "aria-label": l ? "Close chat" : "Open chat",
      children: [
        /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "28", height: "28", fill: s, children: /* @__PURE__ */ e("path", { d: h }) }),
        !l && i > 0 && /* @__PURE__ */ e(
          "span",
          {
            className: "helpin-unread-badge",
            style: { backgroundColor: "#ef4444" },
            "aria-label": `${i} unread messages`,
            children: i > 9 ? "9+" : i
          }
        )
      ]
    }
  );
}, He = ({
  requireEmail: n = !0,
  requireName: l = !0,
  welcomeMessage: i = "Hi! How can we help you today?",
  onSubmit: t
}) => {
  const [a, r] = _(""), [o, c] = _(""), [s, h] = _("email"), v = (d) => {
    d.preventDefault(), o.trim() && l ? h("name") : (t({ name: "", email: o.trim() }), h("done"));
  }, f = (d) => {
    d.preventDefault(), t({ name: a.trim(), email: o.trim() }), h("done");
  };
  return s === "done" ? null : /* @__PURE__ */ e("div", { className: "helpin-pre-chat-form", children: [
    /* @__PURE__ */ e("div", { className: "helpin-pre-chat-welcome", children: i }),
    s === "email" && /* @__PURE__ */ e("form", { onSubmit: v, children: [
      /* @__PURE__ */ e("label", { className: "helpin-sr-only", htmlFor: "helpin-email-input", children: "Email address" }),
      /* @__PURE__ */ e(
        "input",
        {
          id: "helpin-email-input",
          type: "email",
          className: "helpin-input",
          placeholder: "Enter your email",
          value: o,
          onInput: (d) => c(d.target.value),
          required: n,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ e("button", { type: "submit", className: "helpin-btn-primary", children: "Continue" })
    ] }),
    s === "name" && /* @__PURE__ */ e("form", { onSubmit: f, children: [
      /* @__PURE__ */ e("label", { className: "helpin-sr-only", htmlFor: "helpin-name-input", children: "Your name" }),
      /* @__PURE__ */ e(
        "input",
        {
          id: "helpin-name-input",
          type: "text",
          className: "helpin-input",
          placeholder: "Enter your name",
          value: a,
          onInput: (d) => r(d.target.value),
          required: l,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ e("button", { type: "submit", className: "helpin-btn-primary", children: "Start Chat" })
    ] })
  ] });
}, Ne = [
  { value: 1, emoji: "😞", label: "Very unsatisfied" },
  { value: 2, emoji: "😕", label: "Unsatisfied" },
  { value: 3, emoji: "😐", label: "Neutral" },
  { value: 4, emoji: "🙂", label: "Satisfied" },
  { value: 5, emoji: "😄", label: "Very satisfied" }
], ke = ({ onSubmit: n }) => {
  const [l, i] = _(null), [t, a] = _(""), [r, o] = _(!1), c = () => {
    l !== null && (n(l, t), o(!0));
  };
  return r ? /* @__PURE__ */ e("div", { className: "helpin-csat-rating helpin-csat-rating--submitted", children: "Thank you for your feedback!" }) : /* @__PURE__ */ e("div", { className: "helpin-csat-rating", children: [
    /* @__PURE__ */ e("div", { className: "helpin-csat-question", children: "How would you rate your experience?" }),
    /* @__PURE__ */ e("div", { className: "helpin-csat-emojis", role: "radiogroup", "aria-label": "Rate your experience", children: Ne.map(({ value: s, emoji: h, label: v }) => /* @__PURE__ */ e(
      "button",
      {
        className: `helpin-csat-emoji ${l === s ? "helpin-csat-emoji--selected" : ""}`,
        onClick: () => i(s),
        role: "radio",
        "aria-checked": l === s,
        "aria-label": v,
        children: h
      },
      s
    )) }),
    l !== null && /* @__PURE__ */ e("div", { className: "helpin-csat-feedback", children: [
      /* @__PURE__ */ e(
        "textarea",
        {
          placeholder: "Any additional feedback?",
          value: t,
          onInput: (s) => a(s.target.value),
          "aria-label": "Additional feedback",
          maxLength: 1e3
        }
      ),
      /* @__PURE__ */ e("button", { onClick: c, className: "helpin-btn-primary", children: "Submit" })
    ] })
  ] });
}, Se = ({
  text: n,
  isStreaming: l,
  onComplete: i,
  charDelayMs: t = 30
}) => {
  const [a, r] = _("");
  return E(() => {
    if (l && a.length < n.length) {
      const o = setTimeout(() => {
        r(n.slice(0, a.length + 1));
      }, t);
      return () => clearTimeout(o);
    } else !l && n !== a && (r(n), i == null || i());
  }, [n, l, a, i]), /* @__PURE__ */ e("div", { className: "helpin-streaming-text", children: [
    /* @__PURE__ */ e("span", { children: a }),
    l && /* @__PURE__ */ e("span", { className: "helpin-cursor", children: "▊" })
  ] });
};
export {
  ce as BottomNav,
  we as ChatWindow,
  P as ComposeBar,
  ke as CsatRating,
  fe as HelpView,
  oe as HomeView,
  de as MessageBubble,
  me as MessageList,
  ve as MessagesView,
  He as PreChatForm,
  ue as QuickReplies,
  Se as StreamingText,
  pe as TypingIndicator,
  ye as WidgetHeader,
  Ce as WidgetLauncher
};
