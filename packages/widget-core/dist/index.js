var K, v, He, $, pe, Me, xe, ze, ie, Z, ee, O = {}, W = [], Ue = /acit|ex(?:s|g|n|p|$)|rph|grid|ows|mnc|ntw|ine[ch]|zoo|^ord|itera/i, Q = Array.isArray;
function I(n, e) {
  for (var t in e) n[t] = e[t];
  return n;
}
function re(n) {
  n && n.parentNode && n.parentNode.removeChild(n);
}
function P(n, e, t) {
  var r, o, i, s = {};
  for (i in e) i == "key" ? r = e[i] : i == "ref" ? o = e[i] : s[i] = e[i];
  if (arguments.length > 2 && (s.children = arguments.length > 3 ? K.call(arguments, 2) : t), typeof n == "function" && n.defaultProps != null) for (i in n.defaultProps) s[i] === void 0 && (s[i] = n.defaultProps[i]);
  return F(n, s, r, o, null);
}
function F(n, e, t, r, o) {
  var i = { type: n, props: e, key: t, ref: r, __k: null, __: null, __b: 0, __e: null, __c: null, constructor: void 0, __v: o ?? ++He, __i: -1, __u: 0 };
  return o == null && v.vnode != null && v.vnode(i), i;
}
function E(n) {
  return n.children;
}
function R(n, e) {
  this.props = n, this.context = e;
}
function L(n, e) {
  if (e == null) return n.__ ? L(n.__, n.__i + 1) : null;
  for (var t; e < n.__k.length; e++) if ((t = n.__k[e]) != null && t.__e != null) return t.__e;
  return typeof n.type == "function" ? L(n) : null;
}
function Oe(n) {
  if (n.__P && n.__d) {
    var e = n.__v, t = e.__e, r = [], o = [], i = I({}, e);
    i.__v = e.__v + 1, v.vnode && v.vnode(i), oe(n.__P, i, e, n.__n, n.__P.namespaceURI, 32 & e.__u ? [t] : null, r, t ?? L(e), !!(32 & e.__u), o), i.__v = e.__v, i.__.__k[i.__i] = i, $e(r, i, o), e.__e = e.__ = null, i.__e != t && De(i);
  }
}
function De(n) {
  if ((n = n.__) != null && n.__c != null) return n.__e = n.__c.base = null, n.__k.some(function(e) {
    if (e != null && e.__e != null) return n.__e = n.__c.base = e.__e;
  }), De(n);
}
function ue(n) {
  (!n.__d && (n.__d = !0) && $.push(n) && !q.__r++ || pe != v.debounceRendering) && ((pe = v.debounceRendering) || Me)(q);
}
function q() {
  try {
    for (var n, e = 1; $.length; ) $.length > e && $.sort(xe), n = $.shift(), e = $.length, Oe(n);
  } finally {
    $.length = q.__r = 0;
  }
}
function Ie(n, e, t, r, o, i, s, h, _, c, u) {
  var a, p, d, g, C, b, f, m = r && r.__k || W, H = e.length;
  for (_ = We(t, e, m, _, H), a = 0; a < H; a++) (d = t.__k[a]) != null && (p = d.__i != -1 && m[d.__i] || O, d.__i = a, b = oe(n, d, p, o, i, s, h, _, c, u), g = d.__e, d.ref && p.ref != d.ref && (p.ref && ae(p.ref, null, d), u.push(d.ref, d.__c || g, d)), C == null && g != null && (C = g), (f = !!(4 & d.__u)) || p.__k === d.__k ? _ = Ae(d, _, n, f) : typeof d.type == "function" && b !== void 0 ? _ = b : g && (_ = g.nextSibling), d.__u &= -7);
  return t.__e = C, _;
}
function We(n, e, t, r, o) {
  var i, s, h, _, c, u = t.length, a = u, p = 0;
  for (n.__k = new Array(o), i = 0; i < o; i++) (s = e[i]) != null && typeof s != "boolean" && typeof s != "function" ? (typeof s == "string" || typeof s == "number" || typeof s == "bigint" || s.constructor == String ? s = n.__k[i] = F(null, s, null, null, null) : Q(s) ? s = n.__k[i] = F(E, { children: s }, null, null, null) : s.constructor === void 0 && s.__b > 0 ? s = n.__k[i] = F(s.type, s.props, s.key, s.ref ? s.ref : null, s.__v) : n.__k[i] = s, _ = i + p, s.__ = n, s.__b = n.__b + 1, h = null, (c = s.__i = qe(s, t, _, a)) != -1 && (a--, (h = t[c]) && (h.__u |= 2)), h == null || h.__v == null ? (c == -1 && (o > u ? p-- : o < u && p++), typeof s.type != "function" && (s.__u |= 4)) : c != _ && (c == _ - 1 ? p-- : c == _ + 1 ? p++ : (c > _ ? p-- : p++, s.__u |= 4))) : n.__k[i] = null;
  if (a) for (i = 0; i < u; i++) (h = t[i]) != null && (2 & h.__u) == 0 && (h.__e == r && (r = L(h)), Le(h, h));
  return r;
}
function Ae(n, e, t, r) {
  var o, i;
  if (typeof n.type == "function") {
    for (o = n.__k, i = 0; o && i < o.length; i++) o[i] && (o[i].__ = n, e = Ae(o[i], e, t, r));
    return e;
  }
  n.__e != e && (r && (e && n.type && !e.parentNode && (e = L(n)), t.insertBefore(n.__e, e || null)), e = n.__e);
  do
    e = e && e.nextSibling;
  while (e != null && e.nodeType == 8);
  return e;
}
function qe(n, e, t, r) {
  var o, i, s, h = n.key, _ = n.type, c = e[t], u = c != null && (2 & c.__u) == 0;
  if (c === null && h == null || u && h == c.key && _ == c.type) return t;
  if (r > (u ? 1 : 0)) {
    for (o = t - 1, i = t + 1; o >= 0 || i < e.length; ) if ((c = e[s = o >= 0 ? o-- : i++]) != null && (2 & c.__u) == 0 && h == c.key && _ == c.type) return s;
  }
  return -1;
}
function me(n, e, t) {
  e[0] == "-" ? n.setProperty(e, t ?? "") : n[e] = t == null ? "" : typeof t != "number" || Ue.test(e) ? t : t + "px";
}
function V(n, e, t, r, o) {
  var i, s;
  e: if (e == "style") if (typeof t == "string") n.style.cssText = t;
  else {
    if (typeof r == "string" && (n.style.cssText = r = ""), r) for (e in r) t && e in t || me(n.style, e, "");
    if (t) for (e in t) r && t[e] == r[e] || me(n.style, e, t[e]);
  }
  else if (e[0] == "o" && e[1] == "n") i = e != (e = e.replace(ze, "$1")), s = e.toLowerCase(), e = s in n || e == "onFocusOut" || e == "onFocusIn" ? s.slice(2) : e.slice(2), n.l || (n.l = {}), n.l[e + i] = t, t ? r ? t.u = r.u : (t.u = ie, n.addEventListener(e, i ? ee : Z, i)) : n.removeEventListener(e, i ? ee : Z, i);
  else {
    if (o == "http://www.w3.org/2000/svg") e = e.replace(/xlink(H|:h)/, "h").replace(/sName$/, "s");
    else if (e != "width" && e != "height" && e != "href" && e != "list" && e != "form" && e != "tabIndex" && e != "download" && e != "rowSpan" && e != "colSpan" && e != "role" && e != "popover" && e in n) try {
      n[e] = t ?? "";
      break e;
    } catch {
    }
    typeof t == "function" || (t == null || t === !1 && e[4] != "-" ? n.removeAttribute(e) : n.setAttribute(e, e == "popover" && t == 1 ? "" : t));
  }
}
function fe(n) {
  return function(e) {
    if (this.l) {
      var t = this.l[e.type + n];
      if (e.t == null) e.t = ie++;
      else if (e.t < t.u) return;
      return t(v.event ? v.event(e) : e);
    }
  };
}
function oe(n, e, t, r, o, i, s, h, _, c) {
  var u, a, p, d, g, C, b, f, m, H, x, z, T, D, y, k = e.type;
  if (e.constructor !== void 0) return null;
  128 & t.__u && (_ = !!(32 & t.__u), i = [h = e.__e = t.__e]), (u = v.__b) && u(e);
  e: if (typeof k == "function") try {
    if (f = e.props, m = k.prototype && k.prototype.render, H = (u = k.contextType) && r[u.__c], x = u ? H ? H.props.value : u.__ : r, t.__c ? b = (a = e.__c = t.__c).__ = a.__E : (m ? e.__c = a = new k(f, x) : (e.__c = a = new R(f, x), a.constructor = k, a.render = Ke), H && H.sub(a), a.state || (a.state = {}), a.__n = r, p = a.__d = !0, a.__h = [], a._sb = []), m && a.__s == null && (a.__s = a.state), m && k.getDerivedStateFromProps != null && (a.__s == a.state && (a.__s = I({}, a.__s)), I(a.__s, k.getDerivedStateFromProps(f, a.__s))), d = a.props, g = a.state, a.__v = e, p) m && k.getDerivedStateFromProps == null && a.componentWillMount != null && a.componentWillMount(), m && a.componentDidMount != null && a.__h.push(a.componentDidMount);
    else {
      if (m && k.getDerivedStateFromProps == null && f !== d && a.componentWillReceiveProps != null && a.componentWillReceiveProps(f, x), e.__v == t.__v || !a.__e && a.shouldComponentUpdate != null && a.shouldComponentUpdate(f, a.__s, x) === !1) {
        e.__v != t.__v && (a.props = f, a.state = a.__s, a.__d = !1), e.__e = t.__e, e.__k = t.__k, e.__k.some(function(A) {
          A && (A.__ = e);
        }), W.push.apply(a.__h, a._sb), a._sb = [], a.__h.length && s.push(a);
        break e;
      }
      a.componentWillUpdate != null && a.componentWillUpdate(f, a.__s, x), m && a.componentDidUpdate != null && a.__h.push(function() {
        a.componentDidUpdate(d, g, C);
      });
    }
    if (a.context = x, a.props = f, a.__P = n, a.__e = !1, z = v.__r, T = 0, m) a.state = a.__s, a.__d = !1, z && z(e), u = a.render(a.props, a.state, a.context), W.push.apply(a.__h, a._sb), a._sb = [];
    else do
      a.__d = !1, z && z(e), u = a.render(a.props, a.state, a.context), a.state = a.__s;
    while (a.__d && ++T < 25);
    a.state = a.__s, a.getChildContext != null && (r = I(I({}, r), a.getChildContext())), m && !p && a.getSnapshotBeforeUpdate != null && (C = a.getSnapshotBeforeUpdate(d, g)), D = u != null && u.type === E && u.key == null ? Te(u.props.children) : u, h = Ie(n, Q(D) ? D : [D], e, t, r, o, i, s, h, _, c), a.base = e.__e, e.__u &= -161, a.__h.length && s.push(a), b && (a.__E = a.__ = null);
  } catch (A) {
    if (e.__v = null, _ || i != null) if (A.then) {
      for (e.__u |= _ ? 160 : 128; h && h.nodeType == 8 && h.nextSibling; ) h = h.nextSibling;
      i[i.indexOf(h)] = null, e.__e = h;
    } else {
      for (y = i.length; y--; ) re(i[y]);
      ne(e);
    }
    else e.__e = t.__e, e.__k = t.__k, A.then || ne(e);
    v.__e(A, e, t);
  }
  else i == null && e.__v == t.__v ? (e.__k = t.__k, e.__e = t.__e) : h = e.__e = je(t.__e, e, t, r, o, i, s, _, c);
  return (u = v.diffed) && u(e), 128 & e.__u ? void 0 : h;
}
function ne(n) {
  n && (n.__c && (n.__c.__e = !0), n.__k && n.__k.some(ne));
}
function $e(n, e, t) {
  for (var r = 0; r < t.length; r++) ae(t[r], t[++r], t[++r]);
  v.__c && v.__c(e, n), n.some(function(o) {
    try {
      n = o.__h, o.__h = [], n.some(function(i) {
        i.call(o);
      });
    } catch (i) {
      v.__e(i, o.__v);
    }
  });
}
function Te(n) {
  return typeof n != "object" || n == null || n.__b > 0 ? n : Q(n) ? n.map(Te) : I({}, n);
}
function je(n, e, t, r, o, i, s, h, _) {
  var c, u, a, p, d, g, C, b = t.props || O, f = e.props, m = e.type;
  if (m == "svg" ? o = "http://www.w3.org/2000/svg" : m == "math" ? o = "http://www.w3.org/1998/Math/MathML" : o || (o = "http://www.w3.org/1999/xhtml"), i != null) {
    for (c = 0; c < i.length; c++) if ((d = i[c]) && "setAttribute" in d == !!m && (m ? d.localName == m : d.nodeType == 3)) {
      n = d, i[c] = null;
      break;
    }
  }
  if (n == null) {
    if (m == null) return document.createTextNode(f);
    n = document.createElementNS(o, m, f.is && f), h && (v.__m && v.__m(e, i), h = !1), i = null;
  }
  if (m == null) b === f || h && n.data == f || (n.data = f);
  else {
    if (i = i && K.call(n.childNodes), !h && i != null) for (b = {}, c = 0; c < n.attributes.length; c++) b[(d = n.attributes[c]).name] = d.value;
    for (c in b) d = b[c], c == "dangerouslySetInnerHTML" ? a = d : c == "children" || c in f || c == "value" && "defaultValue" in f || c == "checked" && "defaultChecked" in f || V(n, c, null, d, o);
    for (c in f) d = f[c], c == "children" ? p = d : c == "dangerouslySetInnerHTML" ? u = d : c == "value" ? g = d : c == "checked" ? C = d : h && typeof d != "function" || b[c] === d || V(n, c, d, b[c], o);
    if (u) h || a && (u.__html == a.__html || u.__html == n.innerHTML) || (n.innerHTML = u.__html), e.__k = [];
    else if (a && (n.innerHTML = ""), Ie(e.type == "template" ? n.content : n, Q(p) ? p : [p], e, t, r, m == "foreignObject" ? "http://www.w3.org/1999/xhtml" : o, i, s, i ? i[0] : t.__k && L(t, 0), h, _), i != null) for (c = i.length; c--; ) re(i[c]);
    h || (c = "value", m == "progress" && g == null ? n.removeAttribute("value") : g != null && (g !== n[c] || m == "progress" && !g || m == "option" && g != b[c]) && V(n, c, g, b[c], o), c = "checked", C != null && C != n[c] && V(n, c, C, b[c], o));
  }
  return n;
}
function ae(n, e, t) {
  try {
    if (typeof n == "function") {
      var r = typeof n.__u == "function";
      r && n.__u(), r && e == null || (n.__u = n(e));
    } else n.current = e;
  } catch (o) {
    v.__e(o, t);
  }
}
function Le(n, e, t) {
  var r, o;
  if (v.unmount && v.unmount(n), (r = n.ref) && (r.current && r.current != n.__e || ae(r, null, e)), (r = n.__c) != null) {
    if (r.componentWillUnmount) try {
      r.componentWillUnmount();
    } catch (i) {
      v.__e(i, e);
    }
    r.base = r.__P = null;
  }
  if (r = n.__k) for (o = 0; o < r.length; o++) r[o] && Le(r[o], e, t || typeof n.type != "function");
  t || re(n.__e), n.__c = n.__ = n.__e = void 0;
}
function Ke(n, e, t) {
  return this.constructor(n, t);
}
function Be(n, e, t) {
  var r, o, i, s;
  e == document && (e = document.documentElement), v.__ && v.__(n, e), o = (r = !1) ? null : e.__k, i = [], s = [], oe(e, n = e.__k = P(E, null, [n]), o || O, O, e.namespaceURI, o ? null : e.firstChild ? K.call(e.childNodes) : null, i, o ? o.__e : e.firstChild, r, s), $e(i, n, s);
}
K = W.slice, v = { __e: function(n, e, t, r) {
  for (var o, i, s; e = e.__; ) if ((o = e.__c) && !o.__) try {
    if ((i = o.constructor) && i.getDerivedStateFromError != null && (o.setState(i.getDerivedStateFromError(n)), s = o.__d), o.componentDidCatch != null && (o.componentDidCatch(n, r || {}), s = o.__d), s) return o.__E = o;
  } catch (h) {
    n = h;
  }
  throw n;
} }, He = 0, R.prototype.setState = function(n, e) {
  var t;
  t = this.__s != null && this.__s != this.state ? this.__s : this.__s = I({}, this.state), typeof n == "function" && (n = n(I({}, t), this.props)), n && I(t, n), n != null && this.__v && (e && this._sb.push(e), ue(this));
}, R.prototype.forceUpdate = function(n) {
  this.__v && (this.__e = !0, n && this.__h.push(n), ue(this));
}, R.prototype.render = E, $ = [], Me = typeof Promise == "function" ? Promise.prototype.then.bind(Promise.resolve()) : setTimeout, xe = function(n, e) {
  return n.__v.__b - e.__v.__b;
}, q.__r = 0, ze = /(PointerCapture)$|Capture$/i, ie = 0, Z = fe(!1), ee = fe(!0);
var Qe = 0;
function l(n, e, t, r, o, i) {
  e || (e = {});
  var s, h, _ = e;
  if ("ref" in _) for (h in _ = {}, e) h == "ref" ? s = e[h] : _[h] = e[h];
  var c = { type: n, props: _, key: t, ref: s, __k: null, __: null, __b: 0, __e: null, __c: null, constructor: void 0, __v: --Qe, __i: -1, __u: 0, __source: o, __self: i };
  if (typeof n == "function" && (s = n.defaultProps)) for (h in s) _[h] === void 0 && (_[h] = s[h]);
  return v.vnode && v.vnode(c), c;
}
var B, N, Y, ve, j = 0, Ee = [], w = v, ge = w.__b, be = w.__r, ye = w.diffed, Ne = w.__c, we = w.unmount, Ce = w.__;
function se(n, e) {
  w.__h && w.__h(N, n, j || e), j = 0;
  var t = N.__H || (N.__H = { __: [], __h: [] });
  return n >= t.__.length && t.__.push({}), t.__[n];
}
function S(n) {
  return j = 1, Je(Fe, n);
}
function Je(n, e, t) {
  var r = se(B++, 2);
  if (r.t = n, !r.__c && (r.__ = [Fe(void 0, e), function(h) {
    var _ = r.__N ? r.__N[0] : r.__[0], c = r.t(_, h);
    _ !== c && (r.__N = [c, r.__[1]], r.__c.setState({}));
  }], r.__c = N, !N.__f)) {
    var o = function(h, _, c) {
      if (!r.__c.__H) return !0;
      var u = r.__c.__H.__.filter(function(p) {
        return p.__c;
      });
      if (u.every(function(p) {
        return !p.__N;
      })) return !i || i.call(this, h, _, c);
      var a = r.__c.props !== h;
      return u.some(function(p) {
        if (p.__N) {
          var d = p.__[0];
          p.__ = p.__N, p.__N = void 0, d !== p.__[0] && (a = !0);
        }
      }), i && i.call(this, h, _, c) || a;
    };
    N.__f = !0;
    var i = N.shouldComponentUpdate, s = N.componentWillUpdate;
    N.componentWillUpdate = function(h, _, c) {
      if (this.__e) {
        var u = i;
        i = void 0, o(h, _, c), i = u;
      }
      s && s.call(this, h, _, c);
    }, N.shouldComponentUpdate = o;
  }
  return r.__N || r.__;
}
function J(n, e) {
  var t = se(B++, 3);
  !w.__s && Pe(t.__H, e) && (t.__ = n, t.u = e, N.__H.__h.push(t));
}
function Ve(n) {
  return j = 5, Ge(function() {
    return { current: n };
  }, []);
}
function Ge(n, e) {
  var t = se(B++, 7);
  return Pe(t.__H, e) && (t.__ = n(), t.__H = e, t.__h = n), t.__;
}
function Ye() {
  for (var n; n = Ee.shift(); ) {
    var e = n.__H;
    if (n.__P && e) try {
      e.__h.some(U), e.__h.some(te), e.__h = [];
    } catch (t) {
      e.__h = [], w.__e(t, n.__v);
    }
  }
}
w.__b = function(n) {
  N = null, ge && ge(n);
}, w.__ = function(n, e) {
  n && e.__k && e.__k.__m && (n.__m = e.__k.__m), Ce && Ce(n, e);
}, w.__r = function(n) {
  be && be(n), B = 0;
  var e = (N = n.__c).__H;
  e && (Y === N ? (e.__h = [], N.__h = [], e.__.some(function(t) {
    t.__N && (t.__ = t.__N), t.u = t.__N = void 0;
  })) : (e.__h.some(U), e.__h.some(te), e.__h = [], B = 0)), Y = N;
}, w.diffed = function(n) {
  ye && ye(n);
  var e = n.__c;
  e && e.__H && (e.__H.__h.length && (Ee.push(e) !== 1 && ve === w.requestAnimationFrame || ((ve = w.requestAnimationFrame) || Xe)(Ye)), e.__H.__.some(function(t) {
    t.u && (t.__H = t.u), t.u = void 0;
  })), Y = N = null;
}, w.__c = function(n, e) {
  e.some(function(t) {
    try {
      t.__h.some(U), t.__h = t.__h.filter(function(r) {
        return !r.__ || te(r);
      });
    } catch (r) {
      e.some(function(o) {
        o.__h && (o.__h = []);
      }), e = [], w.__e(r, t.__v);
    }
  }), Ne && Ne(n, e);
}, w.unmount = function(n) {
  we && we(n);
  var e, t = n.__c;
  t && t.__H && (t.__H.__.some(function(r) {
    try {
      U(r);
    } catch (o) {
      e = o;
    }
  }), t.__H = void 0, e && w.__e(e, t.__v));
};
var ke = typeof requestAnimationFrame == "function";
function Xe(n) {
  var e, t = function() {
    clearTimeout(r), ke && cancelAnimationFrame(e), setTimeout(n);
  }, r = setTimeout(t, 35);
  ke && (e = requestAnimationFrame(t));
}
function U(n) {
  var e = N, t = n.__c;
  typeof t == "function" && (n.__c = void 0, t()), N = e;
}
function te(n) {
  var e = N;
  n.__c = n.__(), N = e;
}
function Pe(n, e) {
  return !n || n.length !== e.length || e.some(function(t, r) {
    return t !== n[r];
  });
}
function Fe(n, e) {
  return typeof e == "function" ? e(n) : e;
}
const Ze = "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z", en = "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z", nn = "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z", tn = [
  { view: "home", label: "Home", icon: Ze },
  { view: "messages", label: "Messages", icon: en },
  { view: "help", label: "Help", icon: nn }
], ln = ({
  activeView: n,
  onNavigate: e,
  brandColor: t = "#6366f1"
}) => /* @__PURE__ */ l("div", { className: "helpin-bottom-nav", children: tn.map((r) => {
  const o = n === r.view;
  return /* @__PURE__ */ l(
    "button",
    {
      className: `helpin-bottom-nav-item ${o ? "helpin-bottom-nav-item--active" : ""}`,
      onClick: () => e(r.view),
      style: o ? { color: t } : void 0,
      children: [
        /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "22", height: "22", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: r.icon }) }),
        /* @__PURE__ */ l("span", { className: "helpin-bottom-nav-label", children: r.label })
      ]
    },
    r.view
  );
}) }), X = "M3.4 20.4l17.45-7.48a1 1 0 000-1.84L3.4 3.6a.993.993 0 00-1.39.91L2 9.12c0 .5.37.93.87.99L17 12 2.87 13.88c-.5.07-.87.5-.87 1l.01 4.61c0 .71.73 1.2 1.39.91z", rn = ({
  config: n,
  onSendMessage: e,
  onNavigate: t,
  showPreChatForm: r,
  onPreChatSubmit: o
}) => {
  var z, T, D;
  const [i, s] = S(""), [h, _] = S(""), [c, u] = S(""), [a, p] = S("email"), d = ((z = n.branding) == null ? void 0 : z.primaryColor) || "#6366f1", g = (T = n.branding) == null ? void 0 : T.logoUrl, C = ((D = n.branding) == null ? void 0 : D.welcomeMessage) || "How can we help?", b = n.workspaceName || "Support", f = () => {
    const y = i.trim();
    y && (e(y), s(""));
  }, m = (y) => {
    y.key === "Enter" && !y.shiftKey && (y.preventDefault(), f());
  }, H = (y) => {
    var k;
    y.preventDefault(), h.trim() && ((k = n.features) != null && k.preChatForm ? p("name") : o({ name: "", email: h.trim() }));
  }, x = (y) => {
    y.preventDefault(), o({ name: c.trim(), email: h.trim() });
  };
  return /* @__PURE__ */ l("div", { className: "helpin-home-view", children: [
    /* @__PURE__ */ l("div", { className: "helpin-home-header", style: { background: `linear-gradient(135deg, ${d}, ${d}88)` }, children: g ? /* @__PURE__ */ l("img", { src: g, alt: b, className: "helpin-home-logo" }) : /* @__PURE__ */ l("div", { className: "helpin-home-logo-placeholder", style: { backgroundColor: "#ffffff" }, children: /* @__PURE__ */ l("span", { children: b.charAt(0).toUpperCase() }) }) }),
    /* @__PURE__ */ l("div", { className: "helpin-home-content", children: [
      /* @__PURE__ */ l("h2", { className: "helpin-home-welcome", children: C }),
      r ? /* @__PURE__ */ l("div", { className: "helpin-home-prechat", children: a === "email" ? /* @__PURE__ */ l("form", { onSubmit: H, className: "helpin-home-form", children: [
        /* @__PURE__ */ l(
          "input",
          {
            type: "email",
            className: "helpin-home-input",
            placeholder: "Enter your email to get started...",
            value: h,
            onInput: (y) => _(y.target.value),
            required: !0
          }
        ),
        /* @__PURE__ */ l(
          "button",
          {
            type: "submit",
            className: "helpin-home-send",
            style: { backgroundColor: d },
            disabled: !h.trim(),
            children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ l("path", { d: X }) })
          }
        )
      ] }) : /* @__PURE__ */ l("form", { onSubmit: x, className: "helpin-home-form", children: [
        /* @__PURE__ */ l(
          "input",
          {
            type: "text",
            className: "helpin-home-input",
            placeholder: "What's your name?",
            value: c,
            onInput: (y) => u(y.target.value)
          }
        ),
        /* @__PURE__ */ l(
          "button",
          {
            type: "submit",
            className: "helpin-home-send",
            style: { backgroundColor: d },
            children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ l("path", { d: X }) })
          }
        )
      ] }) }) : /* @__PURE__ */ l("div", { className: "helpin-home-search", children: [
        /* @__PURE__ */ l(
          "input",
          {
            type: "text",
            className: "helpin-home-input",
            placeholder: "Ask me anything...",
            value: i,
            onInput: (y) => s(y.target.value),
            onKeyDown: m
          }
        ),
        /* @__PURE__ */ l(
          "button",
          {
            className: "helpin-home-send",
            onClick: f,
            style: { backgroundColor: d },
            disabled: !i.trim(),
            children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "white", children: /* @__PURE__ */ l("path", { d: X }) })
          }
        )
      ] }),
      /* @__PURE__ */ l("div", { className: "helpin-home-actions", children: [
        /* @__PURE__ */ l("button", { className: "helpin-home-action", onClick: () => t("conversation"), children: [
          /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: "M20 2H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h14l4 4V4c0-1.1-.9-2-2-2zm-2 12H6v-2h12v2zm0-3H6V9h12v2zm0-3H6V6h12v2z" }) }),
          /* @__PURE__ */ l("div", { className: "helpin-home-action-text", children: [
            /* @__PURE__ */ l("span", { className: "helpin-home-action-title", children: "Send us a message" }),
            /* @__PURE__ */ l("span", { className: "helpin-home-action-desc", children: "We typically reply in a few minutes" })
          ] }),
          /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-home-action-arrow", children: /* @__PURE__ */ l("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
        ] }),
        /* @__PURE__ */ l("button", { className: "helpin-home-action", onClick: () => t("help"), children: [
          /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z" }) }),
          /* @__PURE__ */ l("div", { className: "helpin-home-action-text", children: [
            /* @__PURE__ */ l("span", { className: "helpin-home-action-title", children: "Help center" }),
            /* @__PURE__ */ l("span", { className: "helpin-home-action-desc", children: "Find answers to common questions" })
          ] }),
          /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-home-action-arrow", children: /* @__PURE__ */ l("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
        ] })
      ] })
    ] })
  ] });
};
function on(n) {
  const e = document.createElement("div");
  return e.appendChild(document.createTextNode(n)), e.innerHTML;
}
function an(n) {
  const e = Date.now(), t = new Date(n).getTime(), r = e - t, o = Math.floor(r / 6e4);
  if (o < 1) return "Just now";
  if (o < 60) return `${o}m ago`;
  const i = Math.floor(o / 60);
  return i < 24 ? `${i}h ago` : new Date(n).toLocaleDateString();
}
const sn = ({ message: n, config: e }) => {
  const t = n.role === "customer", r = n.role === "ai", o = n.role === "agent", i = n.role === "system", s = [
    "helpin-message-bubble",
    t && "helpin-message--customer",
    o && "helpin-message--agent",
    r && "helpin-message--ai",
    i && "helpin-message--system",
    n.isInternal && "helpin-message--internal"
  ].filter(Boolean).join(" "), h = r ? "AI Agent" : o ? "Agent" : i ? "System" : "", _ = t ? "" : (e == null ? void 0 : e.workspaceName) || "Support";
  return /* @__PURE__ */ l(
    "div",
    {
      className: `helpin-message-row ${t ? "helpin-message-row--customer" : "helpin-message-row--agent"}`,
      role: "listitem",
      "aria-label": `${h || "You"} message`,
      children: [
        /* @__PURE__ */ l("div", { className: s, children: [
          /* @__PURE__ */ l(
            "div",
            {
              className: "helpin-message-content",
              dangerouslySetInnerHTML: { __html: on(n.content) }
            }
          ),
          n.sources && n.sources.length > 0 && /* @__PURE__ */ l("div", { className: "helpin-message-sources", children: n.sources.map((c, u) => /* @__PURE__ */ l("div", { className: "helpin-source-item", children: c.title }, u)) }),
          n.aiConfidence !== void 0 && /* @__PURE__ */ l("div", { className: "helpin-message-confidence", children: [
            "Confidence: ",
            Math.round(n.aiConfidence * 100),
            "%"
          ] })
        ] }),
        !t && _ && /* @__PURE__ */ l("div", { className: "helpin-message-attribution", children: [
          /* @__PURE__ */ l("span", { className: "helpin-message-sender", children: _ }),
          h && /* @__PURE__ */ l(E, { children: [
            /* @__PURE__ */ l("span", { className: "helpin-message-attr-dot", children: "·" }),
            /* @__PURE__ */ l("span", { children: h })
          ] }),
          /* @__PURE__ */ l("span", { className: "helpin-message-attr-dot", children: "·" }),
          /* @__PURE__ */ l("span", { children: an(n.createdAt) })
        ] })
      ]
    }
  );
}, Re = ({
  messages: n,
  showDateSeparators: e = !0,
  config: t
}) => {
  const r = Ve(null);
  J(() => {
    if (r.current) {
      const s = r.current;
      (s.scrollHeight - s.scrollTop - s.clientHeight < 100 || n.length <= 1) && (s.scrollTop = s.scrollHeight);
    }
  }, [n]);
  const o = (s) => {
    const h = new Date(s), _ = /* @__PURE__ */ new Date(), c = new Date(_);
    return c.setDate(c.getDate() - 1), h.toDateString() === _.toDateString() ? "Today" : h.toDateString() === c.toDateString() ? "Yesterday" : h.toLocaleDateString();
  }, i = (s, h) => {
    if (!e) return null;
    if (h === 0) return o(s);
    const _ = new Date(n[h - 1].createdAt), c = new Date(s);
    return _.toDateString() !== c.toDateString() ? o(s) : null;
  };
  return /* @__PURE__ */ l("div", { className: "helpin-message-list", ref: r, role: "list", "aria-label": "Messages", children: n.map((s, h) => {
    const _ = i(s.createdAt, h);
    return /* @__PURE__ */ l("div", { children: [
      _ && /* @__PURE__ */ l("div", { className: "helpin-date-separator", children: /* @__PURE__ */ l("span", { children: _ }) }),
      /* @__PURE__ */ l(sn, { message: s, config: t })
    ] }, s.id);
  }) });
}, cn = "M16.5 6v11.5a4 4 0 0 1-8 0V5a2.5 2.5 0 0 1 5 0v10.5a1 1 0 0 1-2 0V6h-1.5v9.5a2.5 2.5 0 0 0 5 0V5a4 4 0 0 0-8 0v12.5a5.5 5.5 0 0 0 11 0V6z", hn = "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-4-8c.79 0 1.5-.71 1.5-1.5S8.79 9 8 9s-1.5.71-1.5 1.5S7.21 12 8 12zm8 0c.79 0 1.5-.71 1.5-1.5S16.79 9 16 9s-1.5.71-1.5 1.5.71 1.5 1.5 1.5zm-4 5.5c2.33 0 4.31-1.46 5.11-3.5H6.89c.8 2.04 2.78 3.5 5.11 3.5z", _n = "M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8z", le = ({
  onSend: n,
  disabled: e = !1,
  placeholder: t = "Ask a question..."
}) => {
  const [r, o] = S(""), i = Ve(null);
  J(() => {
    i.current && (i.current.style.height = "auto", i.current.style.height = `${Math.min(i.current.scrollHeight, 120)}px`);
  }, [r]);
  const s = (c) => {
    c == null || c.preventDefault(), r.trim() && !e && (n(r.trim()), o(""));
  }, h = (c) => {
    c.key === "Enter" && !c.shiftKey && (c.preventDefault(), s());
  }, _ = r.trim().length > 0 && !e;
  return /* @__PURE__ */ l("div", { className: "helpin-compose-wrapper", children: [
    /* @__PURE__ */ l("form", { className: "helpin-compose-bar", onSubmit: s, children: [
      /* @__PURE__ */ l(
        "textarea",
        {
          ref: i,
          className: "helpin-compose-input",
          value: r,
          onInput: (c) => o(c.target.value),
          onKeyDown: h,
          placeholder: t,
          disabled: e,
          rows: 1,
          "aria-label": t
        }
      ),
      /* @__PURE__ */ l("div", { className: "helpin-compose-actions", children: [
        /* @__PURE__ */ l("div", { className: "helpin-compose-tools", children: [
          /* @__PURE__ */ l(
            "button",
            {
              type: "button",
              className: "helpin-compose-tool-btn",
              "aria-label": "Attach file",
              tabIndex: 0,
              children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: cn }) })
            }
          ),
          /* @__PURE__ */ l(
            "button",
            {
              type: "button",
              className: "helpin-compose-tool-btn",
              "aria-label": "Add emoji",
              tabIndex: 0,
              children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: hn }) })
            }
          )
        ] }),
        /* @__PURE__ */ l(
          "button",
          {
            type: "submit",
            className: `helpin-compose-send ${_ ? "helpin-compose-send--active" : ""}`,
            disabled: !_,
            "aria-label": "Send message",
            children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: _n }) })
          }
        )
      ] })
    ] }),
    /* @__PURE__ */ l("div", { className: "helpin-compose-footer", children: [
      "By chatting with us, you agree to our",
      " ",
      /* @__PURE__ */ l("a", { href: "#", className: "helpin-compose-footer-link", children: "Privacy Policy" })
    ] })
  ] });
}, dn = ({
  label: n = "is typing..."
}) => /* @__PURE__ */ l("div", { className: "helpin-typing-indicator", children: [
  /* @__PURE__ */ l("span", { className: "helpin-typing-dots", children: [
    /* @__PURE__ */ l("span", {}),
    /* @__PURE__ */ l("span", {}),
    /* @__PURE__ */ l("span", {})
  ] }),
  /* @__PURE__ */ l("span", { className: "helpin-typing-label", children: n })
] }), pn = ({
  replies: n,
  onSelect: e
}) => /* @__PURE__ */ l("div", { className: "helpin-quick-replies", role: "group", "aria-label": "Quick replies", children: n.map((t, r) => /* @__PURE__ */ l(
  "button",
  {
    className: "helpin-quick-reply",
    onClick: () => e(t),
    "aria-label": `Quick reply: ${t}`,
    children: t
  },
  r
)) }), un = "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z", mn = ({
  config: n,
  messages: e,
  onSendMessage: t,
  onQuickReply: r,
  isTyping: o = !1,
  quickReplies: i = [],
  hasConversation: s
}) => !s || e.length === 0 ? /* @__PURE__ */ l("div", { className: "helpin-messages-view", children: [
  /* @__PURE__ */ l("div", { className: "helpin-messages-header", children: /* @__PURE__ */ l("span", { className: "helpin-messages-title", children: "Messages" }) }),
  /* @__PURE__ */ l("div", { className: "helpin-messages-empty", children: [
    /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "48", height: "48", fill: "currentColor", className: "helpin-messages-empty-icon", children: /* @__PURE__ */ l("path", { d: un }) }),
    /* @__PURE__ */ l("h3", { className: "helpin-messages-empty-title", children: "No messages" }),
    /* @__PURE__ */ l("p", { className: "helpin-messages-empty-desc", children: "Messages from the team will be shown here" })
  ] }),
  /* @__PURE__ */ l("div", { className: "helpin-messages-new-container", children: /* @__PURE__ */ l(
    le,
    {
      onSend: t,
      placeholder: "Ask a question"
    }
  ) })
] }) : /* @__PURE__ */ l("div", { className: "helpin-messages-view", children: [
  /* @__PURE__ */ l("div", { className: "helpin-messages-header", children: /* @__PURE__ */ l("span", { className: "helpin-messages-title", children: "Messages" }) }),
  /* @__PURE__ */ l("div", { className: "helpin-messages-thread", children: [
    /* @__PURE__ */ l(Re, { messages: e }),
    o && /* @__PURE__ */ l(dn, {}),
    i.length > 0 && /* @__PURE__ */ l(pn, { replies: i, onSelect: r })
  ] }),
  /* @__PURE__ */ l(le, { onSend: t })
] }), fn = ({
  config: n,
  onNavigate: e
}) => /* @__PURE__ */ l("div", { className: "helpin-help-view", children: [
  /* @__PURE__ */ l("div", { className: "helpin-help-header", children: /* @__PURE__ */ l("span", { className: "helpin-help-title", children: "Help" }) }),
  /* @__PURE__ */ l("div", { className: "helpin-help-content", children: [
    /* @__PURE__ */ l("div", { className: "helpin-help-search", children: [
      /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", className: "helpin-help-search-icon", children: /* @__PURE__ */ l("path", { d: "M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z" }) }),
      /* @__PURE__ */ l(
        "input",
        {
          type: "text",
          className: "helpin-help-search-input",
          placeholder: "Search for help..."
        }
      )
    ] }),
    /* @__PURE__ */ l("div", { className: "helpin-help-links", children: [
      /* @__PURE__ */ l("button", { className: "helpin-help-link", onClick: () => e("conversation"), children: [
        /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: "M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z" }) }),
        /* @__PURE__ */ l("div", { className: "helpin-help-link-text", children: [
          /* @__PURE__ */ l("span", { className: "helpin-help-link-title", children: "Contact us" }),
          /* @__PURE__ */ l("span", { className: "helpin-help-link-desc", children: "Send us a message and we'll get back to you" })
        ] }),
        /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-help-link-arrow", children: /* @__PURE__ */ l("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
      ] }),
      /* @__PURE__ */ l("div", { className: "helpin-help-divider" }),
      /* @__PURE__ */ l("a", { className: "helpin-help-link", href: "#", target: "_blank", rel: "noopener noreferrer", children: [
        /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: "M14 2H6c-1.1 0-1.99.9-1.99 2L4 20c0 1.1.89 2 1.99 2H18c1.1 0 2-.9 2-2V8l-6-6zm2 16H8v-2h8v2zm0-4H8v-2h8v2zm-3-5V3.5L18.5 9H13z" }) }),
        /* @__PURE__ */ l("div", { className: "helpin-help-link-text", children: [
          /* @__PURE__ */ l("span", { className: "helpin-help-link-title", children: "Browse our docs" }),
          /* @__PURE__ */ l("span", { className: "helpin-help-link-desc", children: "Find detailed guides and documentation" })
        ] }),
        /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "16", height: "16", fill: "currentColor", className: "helpin-help-link-arrow", children: /* @__PURE__ */ l("path", { d: "M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6z" }) })
      ] })
    ] })
  ] })
] }), vn = "M15.41 7.41 14 6l-6 6 6 6 1.41-1.41L10.83 12z", gn = "M12 8c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z", bn = "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z", yn = ({
  config: n,
  messages: e,
  onSendMessage: t,
  onBack: r,
  onClose: o
}) => {
  var a, p;
  const [i] = S(() => (/* @__PURE__ */ new Date()).toISOString()), s = n.workspaceName || "Support", h = (a = n.branding) == null ? void 0 : a.logoUrl, _ = ((p = n.branding) == null ? void 0 : p.welcomeMessage) || "Hi there. How can we help?", u = e.some((d) => d.role !== "customer") || e.length === 0 ? e.length === 0 ? [{
    id: "__intro__",
    conversationId: "__intro__",
    role: "agent",
    content: _,
    isInternal: !1,
    createdAt: i
  }] : e : [{
    id: "__intro__",
    conversationId: "__intro__",
    role: "agent",
    content: _,
    isInternal: !1,
    createdAt: i
  }, ...e];
  return /* @__PURE__ */ l("div", { className: "helpin-conversation-view", children: [
    /* @__PURE__ */ l("div", { className: "helpin-conversation-header", children: [
      /* @__PURE__ */ l(
        "button",
        {
          type: "button",
          className: "helpin-conversation-back",
          onClick: r,
          "aria-label": "Back",
          children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: vn }) })
        }
      ),
      /* @__PURE__ */ l("div", { className: "helpin-conversation-brand", children: [
        h ? /* @__PURE__ */ l("img", { src: h, alt: s, className: "helpin-conversation-logo" }) : /* @__PURE__ */ l("div", { className: "helpin-conversation-logo-placeholder", children: /* @__PURE__ */ l("span", { children: s.charAt(0).toUpperCase() }) }),
        /* @__PURE__ */ l("div", { className: "helpin-conversation-brand-copy", children: [
          /* @__PURE__ */ l("span", { className: "helpin-conversation-title", children: s }),
          /* @__PURE__ */ l("span", { className: "helpin-conversation-subtitle", children: "The team can also help" })
        ] })
      ] }),
      /* @__PURE__ */ l("div", { className: "helpin-conversation-header-actions", children: [
        /* @__PURE__ */ l(
          "button",
          {
            type: "button",
            className: "helpin-conversation-header-btn",
            "aria-label": "More options",
            children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: gn }) })
          }
        ),
        o && /* @__PURE__ */ l(
          "button",
          {
            type: "button",
            className: "helpin-conversation-header-btn",
            onClick: o,
            "aria-label": "Close",
            children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: bn }) })
          }
        )
      ] })
    ] }),
    /* @__PURE__ */ l("div", { className: "helpin-conversation-thread", children: /* @__PURE__ */ l(
      Re,
      {
        messages: u,
        showDateSeparators: !1,
        config: n
      }
    ) }),
    /* @__PURE__ */ l(le, { onSend: t })
  ] });
}, Nn = ({
  config: n,
  messages: e,
  isOpen: t,
  onClose: r,
  onSendMessage: o,
  onQuickReply: i,
  showPreChatForm: s,
  onPreChatSubmit: h,
  isTyping: _ = !1,
  quickReplies: c = [],
  initialView: u = "home"
}) => {
  var ce, he, _e, de;
  const [a, p] = S(u), [d, g] = S(
    u === "conversation" ? "home" : u
  ), [C, b] = S(t), [f, m] = S(t);
  if (J(() => {
    let M, G;
    return t ? (b(!0), typeof window < "u" ? M = window.requestAnimationFrame(() => m(!0)) : m(!0)) : C && (m(!1), G = globalThis.setTimeout(() => b(!1), 220)), () => {
      M !== void 0 && typeof window < "u" && window.cancelAnimationFrame(M), G !== void 0 && globalThis.clearTimeout(G);
    };
  }, [t, C]), !C) return null;
  const H = ((ce = n.branding) == null ? void 0 : ce.widgetPosition) || "bottom-right", x = ((he = n.branding) == null ? void 0 : he.primaryColor) || "#6366f1", z = ((_e = n.branding) == null ? void 0 : _e.showBranding) ?? !0, T = ((de = n.branding) == null ? void 0 : de.colorScheme) || "light", D = H.includes("left") ? "helpin-chat-window--left" : "helpin-chat-window--right", y = (M) => {
    p(M);
  }, k = (M) => {
    g(M), p("conversation");
  }, A = (M) => {
    o(M), k("home");
  };
  return /* @__PURE__ */ l(
    "div",
    {
      className: `helpin-chat-window ${D} ${f ? "helpin-chat-window--visible" : "helpin-chat-window--hidden"} helpin-theme-${T}`,
      children: [
        a !== "conversation" && /* @__PURE__ */ l("button", { className: "helpin-window-close", onClick: r, "aria-label": "Close", children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "18", height: "18", fill: "currentColor", children: /* @__PURE__ */ l("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) }),
        /* @__PURE__ */ l("div", { className: "helpin-view-container", children: [
          a === "home" && /* @__PURE__ */ l(
            rn,
            {
              config: n,
              onSendMessage: A,
              onNavigate: (M) => {
                if (M === "conversation") {
                  k("home");
                  return;
                }
                y(M);
              },
              showPreChatForm: s,
              onPreChatSubmit: h
            }
          ),
          a === "conversation" && /* @__PURE__ */ l(
            yn,
            {
              config: n,
              messages: e,
              onSendMessage: o,
              onBack: () => p(d),
              onClose: r
            }
          ),
          a === "messages" && /* @__PURE__ */ l(
            mn,
            {
              config: n,
              messages: e,
              onSendMessage: o,
              onQuickReply: i,
              isTyping: _,
              quickReplies: c,
              hasConversation: e.length > 0
            }
          ),
          a === "help" && /* @__PURE__ */ l(
            fn,
            {
              config: n,
              onNavigate: (M) => {
                if (M === "conversation") {
                  k("help");
                  return;
                }
                y(M);
              }
            }
          )
        ] }),
        z && a !== "conversation" && /* @__PURE__ */ l("div", { className: "helpin-powered-by", children: [
          /* @__PURE__ */ l("span", { children: "Powered by" }),
          /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "14", height: "14", fill: "currentColor", className: "helpin-powered-by-icon", children: /* @__PURE__ */ l("path", { d: "M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" }) }),
          /* @__PURE__ */ l("span", { className: "helpin-powered-by-name", children: "Helpin" })
        ] }),
        a !== "conversation" && /* @__PURE__ */ l(
          ln,
          {
            activeView: a,
            onNavigate: p,
            brandColor: x
          }
        )
      ]
    }
  );
}, Se = {
  chat_bubble: "M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z",
  question_mark: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 17h-2v-2h2v2zm2.07-7.75l-.9.92C13.45 12.9 13 13.5 13 15h-2v-.5c0-1.1.45-2.1 1.17-2.83l1.24-1.26c.37-.36.59-.86.59-1.41 0-1.1-.9-2-2-2s-2 .9-2 2H8c0-2.21 1.79-4 4-4s4 1.79 4 4c0 .88-.36 1.68-.93 2.25z",
  help: "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17v-2h2v2h-2zm2.07-7.75l-.9.92c-.5.51-.82.89-.99 1.37-.13.36-.18.76-.18 1.46h-2v-.5a4.5 4.5 0 0 1 .52-2.08c.3-.55.71-1.04 1.24-1.52l1.24-1.26c.37-.36.59-.86.59-1.41a2.22 2.22 0 0 0-.73-1.64A2.33 2.33 0 0 0 12 7c-.85 0-1.55.3-2.08.83-.53.52-.8 1.16-.87 1.94H7.07c.08-1.42.62-2.57 1.63-3.44C9.71 5.44 10.76 5 12 5c1.3 0 2.4.42 3.3 1.26.9.84 1.37 1.86 1.37 3.07 0 .88-.36 1.68-.93 2.25-.18.18-.37.35-.57.5l-.1.07z"
}, wn = "M7.41 8.59L12 13.17l4.59-4.58L18 10l-6 6-6-6z", Cn = ({
  onClick: n,
  isOpen: e,
  unreadCount: t = 0,
  brandColor: r = "#6366f1",
  buttonColor: o,
  buttonIconColor: i,
  icon: s = "chat_bubble"
}) => {
  const h = o || r, _ = i || "#ffffff", c = e ? wn : Se[s] || Se.chat_bubble;
  return /* @__PURE__ */ l(
    "button",
    {
      className: `helpin-launcher ${e ? "helpin-launcher--open" : ""}`,
      onClick: n,
      style: { backgroundColor: h },
      "aria-label": e ? "Close chat" : "Open chat",
      children: [
        /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "28", height: "28", fill: _, children: /* @__PURE__ */ l("path", { d: c }) }),
        !e && t > 0 && /* @__PURE__ */ l(
          "span",
          {
            className: "helpin-unread-badge",
            style: { backgroundColor: "#ef4444" },
            "aria-label": `${t} unread messages`,
            children: t > 9 ? "9+" : t
          }
        )
      ]
    }
  );
}, Sn = ({
  workspaceName: n,
  logoUrl: e,
  onClose: t,
  brandColor: r = "#6366f1"
}) => /* @__PURE__ */ l("div", { className: "helpin-widget-header", style: { backgroundColor: r }, children: [
  /* @__PURE__ */ l("div", { className: "helpin-header-content", children: e ? /* @__PURE__ */ l("img", { src: e, alt: n, className: "helpin-header-logo" }) : /* @__PURE__ */ l("div", { className: "helpin-header-title", children: n }) }),
  /* @__PURE__ */ l("button", { className: "helpin-header-close", onClick: t, "aria-label": "Close", children: /* @__PURE__ */ l("svg", { viewBox: "0 0 24 24", width: "20", height: "20", fill: "white", children: /* @__PURE__ */ l("path", { d: "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" }) }) })
] }), Hn = ({
  requireEmail: n = !0,
  requireName: e = !0,
  welcomeMessage: t = "Hi! How can we help you today?",
  onSubmit: r
}) => {
  const [o, i] = S(""), [s, h] = S(""), [_, c] = S("email"), u = (p) => {
    p.preventDefault(), s.trim() && e ? c("name") : (r({ name: "", email: s.trim() }), c("done"));
  }, a = (p) => {
    p.preventDefault(), r({ name: o.trim(), email: s.trim() }), c("done");
  };
  return _ === "done" ? null : /* @__PURE__ */ l("div", { className: "helpin-pre-chat-form", children: [
    /* @__PURE__ */ l("div", { className: "helpin-pre-chat-welcome", children: t }),
    _ === "email" && /* @__PURE__ */ l("form", { onSubmit: u, children: [
      /* @__PURE__ */ l("label", { className: "helpin-sr-only", htmlFor: "helpin-email-input", children: "Email address" }),
      /* @__PURE__ */ l(
        "input",
        {
          id: "helpin-email-input",
          type: "email",
          className: "helpin-input",
          placeholder: "Enter your email",
          value: s,
          onInput: (p) => h(p.target.value),
          required: n,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ l("button", { type: "submit", className: "helpin-btn-primary", children: "Continue" })
    ] }),
    _ === "name" && /* @__PURE__ */ l("form", { onSubmit: a, children: [
      /* @__PURE__ */ l("label", { className: "helpin-sr-only", htmlFor: "helpin-name-input", children: "Your name" }),
      /* @__PURE__ */ l(
        "input",
        {
          id: "helpin-name-input",
          type: "text",
          className: "helpin-input",
          placeholder: "Enter your name",
          value: o,
          onInput: (p) => i(p.target.value),
          required: e,
          autoFocus: !0
        }
      ),
      /* @__PURE__ */ l("button", { type: "submit", className: "helpin-btn-primary", children: "Start Chat" })
    ] })
  ] });
}, kn = [
  { value: 1, emoji: "😞", label: "Very unsatisfied" },
  { value: 2, emoji: "😕", label: "Unsatisfied" },
  { value: 3, emoji: "😐", label: "Neutral" },
  { value: 4, emoji: "🙂", label: "Satisfied" },
  { value: 5, emoji: "😄", label: "Very satisfied" }
], Mn = ({ onSubmit: n }) => {
  const [e, t] = S(null), [r, o] = S(""), [i, s] = S(!1), h = () => {
    e !== null && (n(e, r), s(!0));
  };
  return i ? /* @__PURE__ */ l("div", { className: "helpin-csat-rating helpin-csat-rating--submitted", children: "Thank you for your feedback!" }) : /* @__PURE__ */ l("div", { className: "helpin-csat-rating", children: [
    /* @__PURE__ */ l("div", { className: "helpin-csat-question", children: "How would you rate your experience?" }),
    /* @__PURE__ */ l("div", { className: "helpin-csat-emojis", role: "radiogroup", "aria-label": "Rate your experience", children: kn.map(({ value: _, emoji: c, label: u }) => /* @__PURE__ */ l(
      "button",
      {
        className: `helpin-csat-emoji ${e === _ ? "helpin-csat-emoji--selected" : ""}`,
        onClick: () => t(_),
        role: "radio",
        "aria-checked": e === _,
        "aria-label": u,
        children: c
      },
      _
    )) }),
    e !== null && /* @__PURE__ */ l("div", { className: "helpin-csat-feedback", children: [
      /* @__PURE__ */ l(
        "textarea",
        {
          placeholder: "Any additional feedback?",
          value: r,
          onInput: (_) => o(_.target.value),
          "aria-label": "Additional feedback",
          maxLength: 1e3
        }
      ),
      /* @__PURE__ */ l("button", { onClick: h, className: "helpin-btn-primary", children: "Submit" })
    ] })
  ] });
}, xn = ({
  text: n,
  isStreaming: e,
  onComplete: t,
  charDelayMs: r = 30
}) => {
  const [o, i] = S("");
  return J(() => {
    if (e && o.length < n.length) {
      const s = setTimeout(() => {
        i(n.slice(0, o.length + 1));
      }, r);
      return () => clearTimeout(s);
    } else !e && n !== o && (i(n), t == null || t());
  }, [n, e, o, t]), /* @__PURE__ */ l("div", { className: "helpin-streaming-text", children: [
    /* @__PURE__ */ l("span", { children: o }),
    e && /* @__PURE__ */ l("span", { className: "helpin-cursor", children: "▊" })
  ] });
};
function zn(n, e) {
  var f, m, H, x;
  const {
    config: t,
    messages: r = [],
    isOpen: o = !0,
    onClose: i = () => {
    },
    onSendMessage: s = () => {
    },
    onQuickReply: h = () => {
    },
    showPreChatForm: _ = !1,
    onPreChatSubmit: c = () => {
    },
    isTyping: u = !1,
    quickReplies: a = [],
    initialView: p = "home",
    showLauncher: d = !0,
    onLauncherClick: g,
    unreadCount: C = 0
  } = e, b = P(
    "div",
    { className: "helpin-widget", style: { width: "100%", height: "100%" } },
    P(Nn, {
      config: t,
      messages: r,
      isOpen: o,
      onClose: i,
      onSendMessage: s,
      onQuickReply: h,
      showPreChatForm: _,
      onPreChatSubmit: c,
      isTyping: u,
      quickReplies: a,
      initialView: p
    }),
    d ? P(Cn, {
      onClick: g || i,
      isOpen: o,
      unreadCount: C,
      brandColor: ((f = t.branding) == null ? void 0 : f.primaryColor) || "#6366f1",
      buttonColor: (m = t.branding) == null ? void 0 : m.buttonColor,
      buttonIconColor: (H = t.branding) == null ? void 0 : H.buttonIconColor,
      icon: ((x = t.branding) == null ? void 0 : x.launcherIcon) || "chat_bubble"
    }) : null
  );
  Be(b, n);
}
function Dn(n) {
  Be(null, n);
}
export {
  ln as BottomNav,
  Nn as ChatWindow,
  le as ComposeBar,
  yn as ConversationView,
  Mn as CsatRating,
  fn as HelpView,
  rn as HomeView,
  sn as MessageBubble,
  Re as MessageList,
  mn as MessagesView,
  Hn as PreChatForm,
  pn as QuickReplies,
  xn as StreamingText,
  dn as TypingIndicator,
  Sn as WidgetHeader,
  Cn as WidgetLauncher,
  zn as mountWidget,
  Dn as unmountWidget
};
