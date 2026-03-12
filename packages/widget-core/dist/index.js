import { options as F, Fragment as re } from "preact";
var oe = 0;
function e(n, i, l, t, a, c) {
  i || (i = {});
  var s, r, o = i;
  if ("ref" in o) for (r in o = {}, i) r == "ref" ? s = i[r] : o[r] = i[r];
  var h = { type: n, props: o, key: l, ref: s, __k: null, __: null, __b: 0, __e: null, __c: null, constructor: void 0, __v: --oe, __i: -1, __u: 0, __source: a, __self: c };
  if (typeof n == "function" && (s = n.defaultProps)) for (r in s) o[r] === void 0 && (o[r] = s[r]);
  return F.vnode && F.vnode(h), h;
}
var H, m, V, W, D = 0, ne = [], p = F, K = p.__b, Q = p.__r, J = p.diffed, Y = p.__c, G = p.unmount, X = p.__;
function R(n, i) {
  p.__h && p.__h(m, n, D || i), D = 0;
  var l = m.__H || (m.__H = { __: [], __h: [] });
  return n >= l.__.length && l.__.push({}), l.__[n];
}
function f(n) {
  return D = 1, ce(te, n);
}
function ce(n, i, l) {
  var t = R(H++, 2);
  if (t.t = n, !t.__c && (t.__ = [te(void 0, i), function(r) {
    var o = t.__N ? t.__N[0] : t.__[0], h = t.t(o, r);
    o !== h && (t.__N = [h, t.__[1]], t.__c.setState({}));
  }], t.__c = m, !m.__f)) {
    var a = function(r, o, h) {
      if (!t.__c.__H) return !0;
      var v = t.__c.__H.__.filter(function(d) {
        return d.__c;
      });
      if (v.every(function(d) {
        return !d.__N;
      })) return !c || c.call(this, r, o, h);
      var _ = t.__c.props !== r;
      return v.some(function(d) {
        if (d.__N) {
          var N = d.__[0];
          d.__ = d.__N, d.__N = void 0, N !== d.__[0] && (_ = !0);
        }
      }), c && c.call(this, r, o, h) || _;
    };
    m.__f = !0;
    var c = m.shouldComponentUpdate, s = m.componentWillUpdate;
    m.componentWillUpdate = function(r, o, h) {
      if (this.__e) {
        var v = c;
        c = void 0, a(r, o, h), c = v;
      }
      s && s.call(this, r, o, h);
    }, m.shouldComponentUpdate = a;
  }
  return t.__N || t.__;
}
function I(n, i) {
  var l = R(H++, 3);
  !p.__s && le(l.__H, i) && (l.__ = n, l.u = i, m.__H.__h.push(l));
}
function ie(n) {
  return D = 5, he(function() {
    return { current: n };
  }, []);
}
function he(n, i) {
  var l = R(H++, 7);
  return le(l.__H, i) && (l.__ = n(), l.__H = i, l.__h = n), l.__;
}
function de() {
  for (var n; n = ne.shift(); ) {
    var i = n.__H;
    if (n.__P && i) try {
      i.__h.some(A), i.__h.some($), i.__h = [];
    } catch (l) {
      i.__h = [], p.__e(l, n.__v);
    }
  }
}
p.__b = function(n) {
  m = null, K && K(n);
}, p.__ = function(n, i) {
  n && i.__k && i.__k.__m && (n.__m = i.__k.__m), X && X(n, i);
}, p.__r = function(n) {
  Q && Q(n), H = 0;
  var i = (m = n.__c).__H;
  i && (V === m ? (i.__h = [], m.__h = [], i.__.some(function(l) {
    l.__N && (l.__ = l.__N), l.u = l.__N = void 0;
  })) : (i.__h.some(A), i.__h.some($), i.__h = [], H = 0)), V = m;
}, p.diffed = function(n) {
  J && J(n);
  var i = n.__c;
  i && i.__H && (i.__H.__h.length && (ne.push(i) !== 1 && W === p.requestAnimationFrame || ((W = p.requestAnimationFrame) || me)(de)), i.__H.__.some(function(l) {
    l.u && (l.__H = l.u), l.u = void 0;
  })), V = m = null;
}, p.__c = function(n, i) {
  i.some(function(l) {
    try {
      l.__h.some(A), l.__h = l.__h.filter(function(t) {
        return !t.__ || $(t);
      });
    } catch (t) {
      i.some(function(a) {
        a.__h && (a.__h = []);
      }), i = [], p.__e(t, l.__v);
    }
  }), Y && Y(n, i);
}, p.unmount = function(n) {
  G && G(n);
  var i, l = n.__c;
  l && l.__H && (l.__H.__.some(function(t) {
    try {
      A(t);
    } catch (a) {
      i = a;
    }
  }), l.__H = void 0, i && p.__e(i, l.__v));
};
var Z = typeof requestAnimationFrame == "function";
function me(n) {
  var i, l = function() {
    clearTimeout(t), Z && cancelAnimationFrame(i), setTimeout(n);
  }, t = setTimeout(l, 35);
  Z && (i = requestAnimationFrame(l));
}
function A(n) {
  var i = m, l = n.__c;
  typeof l == "function" && (n.__c = void 0, l()), m = i;
}
function $(n) {
  var i = m;
  n.__c = n.__(), m = i;
}
function le(n, i) {
  return !n || n.length !== i.length || i.some(function(l, t) {
    return l !== n[t];
  });
}
function te(n, i) {
  return typeof i == "function" ? i(n) : i;
}
const pe = "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z", ue = "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z", ve = "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z", _e = [
  { view: "home", label: "Home", icon: pe },
  { view: "messages", label: "Messages", icon: ue },
  { view: "help", label: "Help", icon: ve }
], fe = ({
  activeView: n,
  onNavigate: i,
  brandColor: l = "#6366f1"
}) => /* @__PURE__ */ e("div", { className: "helpin-bottom-nav", children: _e.map((t) => {
  const a = n === t.view;
  return /* @__PURE__ */ e(
    "button",
    {
      className: `helpin-bottom-nav-item ${a ? "helpin-bottom-nav-item--active" : ""}`,
      onClick: () => i(t.view),
      style: a ? { color: l } : void 0,
      children: [
        /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "22", height: "22", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: t.icon }) }),
        /* @__PURE__ */ e("span", { className: "helpin-bottom-nav-label", children: t.label })
      ]
    },
    t.view
  );
}) }), E = "M3.4 20.4l17.45-7.48a1 1 0 000-1.84L3.4 3.6a.993.993 0 00-1.39.91L2 9.12c0 .5.37.93.87.99L17 12 2.87 13.88c-.5.07-.87.5-.87 1l.01 4.61c0 .71.73 1.2 1.39.91z", ge = ({
  config: n,
  onSendMessage: i,
  onNavigate: l,
  showPreChatForm: t,
  onPreChatSubmit: a
}) => {
  var M, z, x;
  const [c, s] = f(""), [r, o] = f(""), [h, v] = f(""), [_, d] = f("email"), N = ((M = n.branding) == null ? void 0 : M.primaryColor) || "#6366f1", k = (z = n.branding) == null ? void 0 : z.logoUrl, w = ((x = n.branding) == null ? void 0 : x.welcomeMessage) || "How can we help?", y = n.workspaceName || "Support", S = () => {
    const u = c.trim();
    u && (i(u), s(""));
  }, C = (u) => {
    u.key === "Enter" && !u.shiftKey && (u.preventDefault(), S());
  }, B = (u) => {
    var b;
    u.preventDefault(), r.trim() && ((b = n.features) != null && b.preChatForm ? d("name") : a({ name: "", email: r.trim() }));
  }, L = (u) => {
    u.preventDefault(), a({ name: h.trim(), email: r.trim() });
  };
  return /* @__PURE__ */ e("div", { className: "helpin-home-view", children: [
    /* @__PURE__ */ e("div", { className: "helpin-home-header", style: { background: `linear-gradient(135deg, ${N}, ${N}88)` }, children: k ? /* @__PURE__ */ e("img", { src: k, alt: y, className: "helpin-home-logo" }) : /* @__PURE__ */ e("div", { className: "helpin-home-logo-placeholder", style: { backgroundColor: "#ffffff" }, children: /* @__PURE__ */ e("span", { children: y.charAt(0).toUpperCase() }) }) }),
    /* @__PURE__ */ e("div", { className: "helpin-home-content", children: [
      /* @__PURE__ */ e("h2", { className: "helpin-home-welcome", children: w }),
      t ? /* @__PURE__ */ e("div", { className: "helpin-home-prechat", children: _ === "email" ? /* @__PURE__ */ e("form", { onSubmit: B, className: "helpin-home-form", children: [
        /* @__PURE__ */ e(
          "input",
          {
            type: "email",
            className: "helpin-home-input",
            placeholder: "Enter your email to get started...",
            value: r,
            onInput: (u) => o(u.target.value),
            required: !0
          }
        ),
        /* @__PURE__ */ e(
          "button",
          {
            type: "submit",
            className: "helpin-home-send",
            style: { backgroundColor: N },
            disabled: !r.trim(),
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ e("path", { d: E }) })
          }
        )
      ] }) : /* @__PURE__ */ e("form", { onSubmit: L, className: "helpin-home-form", children: [
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
            style: { backgroundColor: N },
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ e("path", { d: E }) })
          }
        )
      ] }) }) : /* @__PURE__ */ e("div", { className: "helpin-home-search", children: [
        /* @__PURE__ */ e(
          "input",
          {
            type: "text",
            className: "helpin-home-input",
            placeholder: "Ask me anything...",
            value: c,
            onInput: (u) => s(u.target.value),
            onKeyDown: C
          }
        ),
        /* @__PURE__ */ e(
          "button",
          {
            className: "helpin-home-send",
            onClick: S,
            style: { backgroundColor: N },
            disabled: !c.trim(),
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ e("path", { d: E }) })
          }
        )
      ] }),
      /* @__PURE__ */ e("div", { className: "helpin-home-actions", children: [
        /* @__PURE__ */ e("button", { className: "helpin-home-action", onClick: () => l("conversation"), children: [
          /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: "M20 2H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h14l4 4V4c0-1.1-.9-2-2-2zm-2 12H6v-2h12v2zm0-3H6V9h12v2zm0-3H6V6h12v2z" }) }),
          /* @__PURE__ */ e("div", { className: "helpin-home-action-text", children: [
            /* @__PURE__ */ e("span", { className: "helpin-home-action-title", children: "Send us a message" }),
            /* @__PURE__ */ e("span", { className: "helpin-home-action-desc", children: "We typically reply in a few minutes" })
          ] }),
          /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-home-action-arrow", children: /* @__PURE__ */ e("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
        ] }),
        /* @__PURE__ */ e("button", { className: "helpin-home-action", onClick: () => l("help"), children: [
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
function Ne(n) {
  const i = document.createElement("div");
  return i.appendChild(document.createTextNode(n)), i.innerHTML;
}
function be(n) {
  const i = Date.now(), l = new Date(n).getTime(), t = i - l, a = Math.floor(t / 6e4);
  if (a < 1) return "Just now";
  if (a < 60) return `${a}m ago`;
  const c = Math.floor(a / 60);
  return c < 24 ? `${c}h ago` : new Date(n).toLocaleDateString();
}
const we = ({ message: n, config: i }) => {
  const l = n.role === "customer", t = n.role === "ai", a = n.role === "agent", c = n.role === "system", s = [
    "helpin-message-bubble",
    l && "helpin-message--customer",
    a && "helpin-message--agent",
    t && "helpin-message--ai",
    c && "helpin-message--system",
    n.isInternal && "helpin-message--internal"
  ].filter(Boolean).join(" "), r = t ? "AI Agent" : a ? "Agent" : c ? "System" : "", o = l ? "" : (i == null ? void 0 : i.workspaceName) || "Support";
  return /* @__PURE__ */ e(
    "div",
    {
      className: `helpin-message-row ${l ? "helpin-message-row--customer" : "helpin-message-row--agent"}`,
      role: "listitem",
      "aria-label": `${r || "You"} message`,
      children: [
        /* @__PURE__ */ e("div", { className: s, children: [
          /* @__PURE__ */ e(
            "div",
            {
              className: "helpin-message-content",
              dangerouslySetInnerHTML: { __html: Ne(n.content) }
            }
          ),
          n.sources && n.sources.length > 0 && /* @__PURE__ */ e("div", { className: "helpin-message-sources", children: n.sources.map((h, v) => /* @__PURE__ */ e("div", { className: "helpin-source-item", children: h.title }, v)) }),
          n.aiConfidence !== void 0 && /* @__PURE__ */ e("div", { className: "helpin-message-confidence", children: [
            "Confidence: ",
            Math.round(n.aiConfidence * 100),
            "%"
          ] })
        ] }),
        !l && o && /* @__PURE__ */ e("div", { className: "helpin-message-attribution", children: [
          /* @__PURE__ */ e("span", { className: "helpin-message-sender", children: o }),
          r && /* @__PURE__ */ e(re, { children: [
            /* @__PURE__ */ e("span", { className: "helpin-message-attr-dot", children: "·" }),
            /* @__PURE__ */ e("span", { children: r })
          ] }),
          /* @__PURE__ */ e("span", { className: "helpin-message-attr-dot", children: "·" }),
          /* @__PURE__ */ e("span", { children: be(n.createdAt) })
        ] })
      ]
    }
  );
}, ae = ({
  messages: n,
  showDateSeparators: i = !0,
  config: l
}) => {
  const t = ie(null);
  I(() => {
    if (t.current) {
      const s = t.current;
      (s.scrollHeight - s.scrollTop - s.clientHeight < 100 || n.length <= 1) && (s.scrollTop = s.scrollHeight);
    }
  }, [n]);
  const a = (s) => {
    const r = new Date(s), o = /* @__PURE__ */ new Date(), h = new Date(o);
    return h.setDate(h.getDate() - 1), r.toDateString() === o.toDateString() ? "Today" : r.toDateString() === h.toDateString() ? "Yesterday" : r.toLocaleDateString();
  }, c = (s, r) => {
    if (!i) return null;
    if (r === 0) return a(s);
    const o = new Date(n[r - 1].createdAt), h = new Date(s);
    return o.toDateString() !== h.toDateString() ? a(s) : null;
  };
  return /* @__PURE__ */ e("div", { className: "helpin-message-list", ref: t, role: "list", "aria-label": "Messages", children: n.map((s, r) => {
    const o = c(s.createdAt, r);
    return /* @__PURE__ */ e("div", { children: [
      o && /* @__PURE__ */ e("div", { className: "helpin-date-separator", children: /* @__PURE__ */ e("span", { children: o }) }),
      /* @__PURE__ */ e(we, { message: s, config: l })
    ] }, s.id);
  }) });
}, ye = "M16.5 6v11.5a4 4 0 0 1-8 0V5a2.5 2.5 0 0 1 5 0v10.5a1 1 0 0 1-2 0V6h-1.5v9.5a2.5 2.5 0 0 0 5 0V5a4 4 0 0 0-8 0v12.5a5.5 5.5 0 0 0 11 0V6z", Ce = "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-4-8c.79 0 1.5-.71 1.5-1.5S8.79 9 8 9s-1.5.71-1.5 1.5S7.21 12 8 12zm8 0c.79 0 1.5-.71 1.5-1.5S16.79 9 16 9s-1.5.71-1.5 1.5.71 1.5 1.5 1.5zm-4 5.5c2.33 0 4.31-1.46 5.11-3.5H6.89c.8 2.04 2.78 3.5 5.11 3.5z", He = "M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8z", O = ({
  onSend: n,
  disabled: i = !1,
  placeholder: l = "Ask a question..."
}) => {
  const [t, a] = f(""), c = ie(null);
  I(() => {
    c.current && (c.current.style.height = "auto", c.current.style.height = `${Math.min(c.current.scrollHeight, 120)}px`);
  }, [t]);
  const s = (h) => {
    h == null || h.preventDefault(), t.trim() && !i && (n(t.trim()), a(""));
  }, r = (h) => {
    h.key === "Enter" && !h.shiftKey && (h.preventDefault(), s());
  }, o = t.trim().length > 0 && !i;
  return /* @__PURE__ */ e("div", { className: "helpin-compose-wrapper", children: [
    /* @__PURE__ */ e("form", { className: "helpin-compose-bar", onSubmit: s, children: [
      /* @__PURE__ */ e(
        "textarea",
        {
          ref: c,
          className: "helpin-compose-input",
          value: t,
          onInput: (h) => a(h.target.value),
          onKeyDown: r,
          placeholder: l,
          disabled: i,
          rows: 1,
          "aria-label": l
        }
      ),
      /* @__PURE__ */ e("div", { className: "helpin-compose-actions", children: [
        /* @__PURE__ */ e("div", { className: "helpin-compose-tools", children: [
          /* @__PURE__ */ e(
            "button",
            {
              type: "button",
              className: "helpin-compose-tool-btn",
              "aria-label": "Attach file",
              tabIndex: 0,
              children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: ye }) })
            }
          ),
          /* @__PURE__ */ e(
            "button",
            {
              type: "button",
              className: "helpin-compose-tool-btn",
              "aria-label": "Add emoji",
              tabIndex: 0,
              children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: Ce }) })
            }
          )
        ] }),
        /* @__PURE__ */ e(
          "button",
          {
            type: "submit",
            className: `helpin-compose-send ${o ? "helpin-compose-send--active" : ""}`,
            disabled: !o,
            "aria-label": "Send message",
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: He }) })
          }
        )
      ] })
    ] }),
    /* @__PURE__ */ e("div", { className: "helpin-compose-footer", children: [
      "By chatting with us, you agree to our",
      " ",
      /* @__PURE__ */ e("a", { href: "#", className: "helpin-compose-footer-link", children: "Privacy Policy" })
    ] })
  ] });
}, ke = ({
  label: n = "is typing..."
}) => /* @__PURE__ */ e("div", { className: "helpin-typing-indicator", children: [
  /* @__PURE__ */ e("span", { className: "helpin-typing-dots", children: [
    /* @__PURE__ */ e("span", {}),
    /* @__PURE__ */ e("span", {}),
    /* @__PURE__ */ e("span", {})
  ] }),
  /* @__PURE__ */ e("span", { className: "helpin-typing-label", children: n })
] }), Se = ({
  replies: n,
  onSelect: i
}) => /* @__PURE__ */ e("div", { className: "helpin-quick-replies", role: "group", "aria-label": "Quick replies", children: n.map((l, t) => /* @__PURE__ */ e(
  "button",
  {
    className: "helpin-quick-reply",
    onClick: () => i(l),
    "aria-label": `Quick reply: ${l}`,
    children: l
  },
  t
)) }), Me = "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z", ze = ({
  config: n,
  messages: i,
  onSendMessage: l,
  onQuickReply: t,
  isTyping: a = !1,
  quickReplies: c = [],
  hasConversation: s
}) => !s || i.length === 0 ? /* @__PURE__ */ e("div", { className: "helpin-messages-view", children: [
  /* @__PURE__ */ e("div", { className: "helpin-messages-header", children: /* @__PURE__ */ e("span", { className: "helpin-messages-title", children: "Messages" }) }),
  /* @__PURE__ */ e("div", { className: "helpin-messages-empty", children: [
    /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "48", height: "48", fill: "currentColor", className: "helpin-messages-empty-icon", children: /* @__PURE__ */ e("path", { d: Me }) }),
    /* @__PURE__ */ e("h3", { className: "helpin-messages-empty-title", children: "No messages" }),
    /* @__PURE__ */ e("p", { className: "helpin-messages-empty-desc", children: "Messages from the team will be shown here" })
  ] }),
  /* @__PURE__ */ e("div", { className: "helpin-messages-new-container", children: /* @__PURE__ */ e(
    O,
    {
      onSend: l,
      placeholder: "Ask a question"
    }
  ) })
] }) : /* @__PURE__ */ e("div", { className: "helpin-messages-view", children: [
  /* @__PURE__ */ e("div", { className: "helpin-messages-header", children: /* @__PURE__ */ e("span", { className: "helpin-messages-title", children: "Messages" }) }),
  /* @__PURE__ */ e("div", { className: "helpin-messages-thread", children: [
    /* @__PURE__ */ e(ae, { messages: i }),
    a && /* @__PURE__ */ e(ke, {}),
    c.length > 0 && /* @__PURE__ */ e(Se, { replies: c, onSelect: t })
  ] }),
  /* @__PURE__ */ e(O, { onSend: l })
] }), xe = ({
  config: n,
  onNavigate: i
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
      /* @__PURE__ */ e("button", { className: "helpin-help-link", onClick: () => i("conversation"), children: [
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
] }), Ae = "M15.41 7.41 14 6l-6 6 6 6 1.41-1.41L10.83 12z", De = "M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z", Ie = "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z", Be = ({
  config: n,
  messages: i,
  onSendMessage: l,
  onBack: t,
  onClose: a
}) => {
  var _, d;
  const [c] = f(() => (/* @__PURE__ */ new Date()).toISOString()), s = n.workspaceName || "Support", r = (_ = n.branding) == null ? void 0 : _.logoUrl, o = ((d = n.branding) == null ? void 0 : d.welcomeMessage) || "Hi there. How can we help?", v = i.some((N) => N.role !== "customer") || i.length === 0 ? i.length === 0 ? [{
    id: "__intro__",
    conversationId: "__intro__",
    role: "agent",
    content: o,
    isInternal: !1,
    createdAt: c
  }] : i : [{
    id: "__intro__",
    conversationId: "__intro__",
    role: "agent",
    content: o,
    isInternal: !1,
    createdAt: c
  }, ...i];
  return /* @__PURE__ */ e("div", { className: "helpin-conversation-view", children: [
    /* @__PURE__ */ e("div", { className: "helpin-conversation-header", children: [
      /* @__PURE__ */ e(
        "button",
        {
          type: "button",
          className: "helpin-conversation-back",
          onClick: t,
          "aria-label": "Back",
          children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: Ae }) })
        }
      ),
      /* @__PURE__ */ e("div", { className: "helpin-conversation-brand", children: [
        r ? /* @__PURE__ */ e("img", { src: r, alt: s, className: "helpin-conversation-logo" }) : /* @__PURE__ */ e("div", { className: "helpin-conversation-logo-placeholder", children: /* @__PURE__ */ e("span", { children: s.charAt(0).toUpperCase() }) }),
        /* @__PURE__ */ e("div", { className: "helpin-conversation-brand-copy", children: [
          /* @__PURE__ */ e("span", { className: "helpin-conversation-title", children: s }),
          /* @__PURE__ */ e("span", { className: "helpin-conversation-subtitle", children: "The team can also help" })
        ] })
      ] }),
      /* @__PURE__ */ e("div", { className: "helpin-conversation-header-actions", children: [
        /* @__PURE__ */ e(
          "button",
          {
            type: "button",
            className: "helpin-conversation-header-btn",
            "aria-label": "More options",
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: De }) })
          }
        ),
        a && /* @__PURE__ */ e(
          "button",
          {
            type: "button",
            className: "helpin-conversation-header-btn",
            onClick: a,
            "aria-label": "Close",
            children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: Ie }) })
          }
        )
      ] })
    ] }),
    /* @__PURE__ */ e("div", { className: "helpin-conversation-thread", children: /* @__PURE__ */ e(
      ae,
      {
        messages: v,
        showDateSeparators: !1,
        config: n
      }
    ) }),
    /* @__PURE__ */ e(O, { onSend: l })
  ] });
}, Ee = ({
  config: n,
  messages: i,
  isOpen: l,
  onClose: t,
  onSendMessage: a,
  onQuickReply: c,
  showPreChatForm: s,
  onPreChatSubmit: r,
  isTyping: o = !1,
  quickReplies: h = [],
  initialView: v = "home"
}) => {
  var q, j, U, P;
  const [_, d] = f(v), [N, k] = f(
    v === "conversation" ? "home" : v
  ), [w, y] = f(l), [S, C] = f(l);
  if (I(() => {
    let g, T;
    return l ? (y(!0), typeof window < "u" ? g = window.requestAnimationFrame(() => C(!0)) : C(!0)) : w && (C(!1), T = globalThis.setTimeout(() => y(!1), 220)), () => {
      g !== void 0 && typeof window < "u" && window.cancelAnimationFrame(g), T !== void 0 && globalThis.clearTimeout(T);
    };
  }, [l, w]), !w) return null;
  const B = ((q = n.branding) == null ? void 0 : q.widgetPosition) || "bottom-right", L = ((j = n.branding) == null ? void 0 : j.primaryColor) || "#6366f1", M = ((U = n.branding) == null ? void 0 : U.showBranding) ?? !0, z = ((P = n.branding) == null ? void 0 : P.colorScheme) || "light", x = B.includes("left") ? "helpin-chat-window--left" : "helpin-chat-window--right", u = (g) => {
    d(g);
  }, b = (g) => {
    k(g), d("conversation");
  }, se = (g) => {
    a(g), b("home");
  };
  return /* @__PURE__ */ e(
    "div",
    {
      className: `helpin-chat-window ${x} ${S ? "helpin-chat-window--visible" : "helpin-chat-window--hidden"} helpin-theme-${z}`,
      children: [
        _ !== "conversation" && /* @__PURE__ */ e("button", { className: "helpin-window-close", onClick: t, "aria-label": "Close", children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", children: /* @__PURE__ */ e("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) }),
        /* @__PURE__ */ e("div", { className: "helpin-view-container", children: [
          _ === "home" && /* @__PURE__ */ e(
            ge,
            {
              config: n,
              onSendMessage: se,
              onNavigate: (g) => {
                if (g === "conversation") {
                  b("home");
                  return;
                }
                u(g);
              },
              showPreChatForm: s,
              onPreChatSubmit: r
            }
          ),
          _ === "conversation" && /* @__PURE__ */ e(
            Be,
            {
              config: n,
              messages: i,
              onSendMessage: a,
              onBack: () => d(N),
              onClose: t
            }
          ),
          _ === "messages" && /* @__PURE__ */ e(
            ze,
            {
              config: n,
              messages: i,
              onSendMessage: a,
              onQuickReply: c,
              isTyping: o,
              quickReplies: h,
              hasConversation: i.length > 0
            }
          ),
          _ === "help" && /* @__PURE__ */ e(
            xe,
            {
              config: n,
              onNavigate: (g) => {
                if (g === "conversation") {
                  b("help");
                  return;
                }
                u(g);
              }
            }
          )
        ] }),
        M && _ !== "conversation" && /* @__PURE__ */ e("div", { className: "helpin-powered-by", children: [
          /* @__PURE__ */ e("span", { children: "Powered by" }),
          /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "14", height: "14", fill: "currentColor", className: "helpin-powered-by-icon", children: /* @__PURE__ */ e("path", { d: "M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" }) }),
          /* @__PURE__ */ e("span", { className: "helpin-powered-by-name", children: "Helpin" })
        ] }),
        _ !== "conversation" && /* @__PURE__ */ e(
          fe,
          {
            activeView: _,
            onNavigate: d,
            brandColor: L
          }
        )
      ]
    }
  );
}, Fe = ({
  workspaceName: n,
  logoUrl: i,
  onClose: l,
  brandColor: t = "#6366f1"
}) => /* @__PURE__ */ e("div", { className: "helpin-widget-header", style: { backgroundColor: t }, children: [
  /* @__PURE__ */ e("div", { className: "helpin-header-content", children: i ? /* @__PURE__ */ e("img", { src: i, alt: n, className: "helpin-header-logo" }) : /* @__PURE__ */ e("div", { className: "helpin-header-title", children: n }) }),
  /* @__PURE__ */ e("button", { className: "helpin-header-close", onClick: l, "aria-label": "Close", children: /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "white", children: /* @__PURE__ */ e("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) })
] }), ee = {
  chat_bubble: "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z",
  question_mark: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z",
  help: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17v-2h2v2h-2zm2.07-7.75l-.9.92c-.5.51-.82.89-.99 1.37-.13.36-.18.76-.18 1.46h-2v-.5a4.5 4.5 0 0 1 .52-2.08c.3-.55.71-1.04 1.24-1.52l1.24-1.26c.37-.36.59-.86.59-1.41a2.22 2.22 0 0 0-.73-1.64A2.33 2.33 0 0 0 12 7c-.85 0-1.55.3-2.08.83-.53.52-.8 1.16-.87 1.94H7.07c.08-1.42.62-2.57 1.63-3.44C9.71 5.44 10.76 5 12 5c1.3 0 2.4.42 3.3 1.26.9.84 1.37 1.86 1.37 3.07 0 .88-.36 1.68-.93 2.25-.18.18-.37.35-.57.5l-.1.07z"
}, Le = "M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6z", $e = ({
  onClick: n,
  isOpen: i,
  unreadCount: l = 0,
  brandColor: t = "#6366f1",
  buttonColor: a,
  buttonIconColor: c,
  icon: s = "chat_bubble"
}) => {
  const r = a || t, o = c || "#ffffff", h = i ? Le : ee[s] || ee.chat_bubble;
  return /* @__PURE__ */ e(
    "button",
    {
      className: `helpin-launcher ${i ? "helpin-launcher--open" : ""}`,
      onClick: n,
      style: { backgroundColor: r },
      "aria-label": i ? "Close chat" : "Open chat",
      children: [
        /* @__PURE__ */ e("svg", { viewBox: "0 0 24 24", width: "28", height: "28", fill: o, children: /* @__PURE__ */ e("path", { d: h }) }),
        !i && l > 0 && /* @__PURE__ */ e(
          "span",
          {
            className: "helpin-unread-badge",
            style: { backgroundColor: "#ef4444" },
            "aria-label": `${l} unread messages`,
            children: l > 9 ? "9+" : l
          }
        )
      ]
    }
  );
}, Oe = ({
  requireEmail: n = !0,
  requireName: i = !0,
  welcomeMessage: l = "Hi! How can we help you today?",
  onSubmit: t
}) => {
  const [a, c] = f(""), [s, r] = f(""), [o, h] = f("email"), v = (d) => {
    d.preventDefault(), s.trim() && i ? h("name") : (t({ name: "", email: s.trim() }), h("done"));
  }, _ = (d) => {
    d.preventDefault(), t({ name: a.trim(), email: s.trim() }), h("done");
  };
  return o === "done" ? null : /* @__PURE__ */ e("div", { className: "helpin-pre-chat-form", children: [
    /* @__PURE__ */ e("div", { className: "helpin-pre-chat-welcome", children: l }),
    o === "email" && /* @__PURE__ */ e("form", { onSubmit: v, children: [
      /* @__PURE__ */ e("label", { className: "helpin-sr-only", htmlFor: "helpin-email-input", children: "Email address" }),
      /* @__PURE__ */ e(
        "input",
        {
          id: "helpin-email-input",
          type: "email",
          className: "helpin-input",
          placeholder: "Enter your email",
          value: s,
          onInput: (d) => r(d.target.value),
          required: n,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ e("button", { type: "submit", className: "helpin-btn-primary", children: "Continue" })
    ] }),
    o === "name" && /* @__PURE__ */ e("form", { onSubmit: _, children: [
      /* @__PURE__ */ e("label", { className: "helpin-sr-only", htmlFor: "helpin-name-input", children: "Your name" }),
      /* @__PURE__ */ e(
        "input",
        {
          id: "helpin-name-input",
          type: "text",
          className: "helpin-input",
          placeholder: "Enter your name",
          value: a,
          onInput: (d) => c(d.target.value),
          required: i,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ e("button", { type: "submit", className: "helpin-btn-primary", children: "Start Chat" })
    ] })
  ] });
}, Te = [
  { value: 1, emoji: "😞", label: "Very unsatisfied" },
  { value: 2, emoji: "😕", label: "Unsatisfied" },
  { value: 3, emoji: "😐", label: "Neutral" },
  { value: 4, emoji: "🙂", label: "Satisfied" },
  { value: 5, emoji: "😄", label: "Very satisfied" }
], Re = ({ onSubmit: n }) => {
  const [i, l] = f(null), [t, a] = f(""), [c, s] = f(!1), r = () => {
    i !== null && (n(i, t), s(!0));
  };
  return c ? /* @__PURE__ */ e("div", { className: "helpin-csat-rating helpin-csat-rating--submitted", children: "Thank you for your feedback!" }) : /* @__PURE__ */ e("div", { className: "helpin-csat-rating", children: [
    /* @__PURE__ */ e("div", { className: "helpin-csat-question", children: "How would you rate your experience?" }),
    /* @__PURE__ */ e("div", { className: "helpin-csat-emojis", role: "radiogroup", "aria-label": "Rate your experience", children: Te.map(({ value: o, emoji: h, label: v }) => /* @__PURE__ */ e(
      "button",
      {
        className: `helpin-csat-emoji ${i === o ? "helpin-csat-emoji--selected" : ""}`,
        onClick: () => l(o),
        role: "radio",
        "aria-checked": i === o,
        "aria-label": v,
        children: h
      },
      o
    )) }),
    i !== null && /* @__PURE__ */ e("div", { className: "helpin-csat-feedback", children: [
      /* @__PURE__ */ e(
        "textarea",
        {
          placeholder: "Any additional feedback?",
          value: t,
          onInput: (o) => a(o.target.value),
          "aria-label": "Additional feedback",
          maxLength: 1e3
        }
      ),
      /* @__PURE__ */ e("button", { onClick: r, className: "helpin-btn-primary", children: "Submit" })
    ] })
  ] });
}, qe = ({
  text: n,
  isStreaming: i,
  onComplete: l,
  charDelayMs: t = 30
}) => {
  const [a, c] = f("");
  return I(() => {
    if (i && a.length < n.length) {
      const s = setTimeout(() => {
        c(n.slice(0, a.length + 1));
      }, t);
      return () => clearTimeout(s);
    } else !i && n !== a && (c(n), l == null || l());
  }, [n, i, a, l]), /* @__PURE__ */ e("div", { className: "helpin-streaming-text", children: [
    /* @__PURE__ */ e("span", { children: a }),
    i && /* @__PURE__ */ e("span", { className: "helpin-cursor", children: "▊" })
  ] });
};
export {
  fe as BottomNav,
  Ee as ChatWindow,
  O as ComposeBar,
  Be as ConversationView,
  Re as CsatRating,
  xe as HelpView,
  ge as HomeView,
  we as MessageBubble,
  ae as MessageList,
  ze as MessagesView,
  Oe as PreChatForm,
  Se as QuickReplies,
  qe as StreamingText,
  ke as TypingIndicator,
  Fe as WidgetHeader,
  $e as WidgetLauncher
};
