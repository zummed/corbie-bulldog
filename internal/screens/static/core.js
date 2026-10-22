// What every screen shares: making elements, reaching the API, times,
// parties and statuses, the screens by address, and the stream of changes.
// What is shown is put in as text, never as markup; the one markup put in
// is the icons below, which hold nothing of the office's.
"use strict";

const B = {
  me: null,                                  // /api/me
  routes: [],                                // { pattern, view, nav }
  narrow: matchMedia("(max-width: 640px)"),  // the line of a room runs downward
  starting: new Set(),                       // the requests asked here, which may not have started yet
};

// el makes an element: attrs set attributes, or listen when named on…, or
// set custom properties when style is an object; children are elements or
// text.
B.el = function (tag, attrs, ...children) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k.startsWith("on") && typeof v === "function") e.addEventListener(k.slice(2), v);
    else if (k === "style" && typeof v === "object") for (const [p, x] of Object.entries(v)) e.style.setProperty(p, x);
    else if (v !== undefined && v !== null && v !== false) e.setAttribute(k, v === true ? "" : v);
  }
  for (const c of children.flat(Infinity)) {
    if (c === null || c === undefined || c === false || c === "") continue;
    e.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return e;
};
const el = B.el;
const enc = encodeURIComponent;

// The API. A 401 goes to the sign-in page; any other failure is an error
// with the server's words and the status.
B.api = async function (path, init) {
  const res = await fetch(path, init);
  if (res.status === 401) {
    location.assign("/signin");
    throw new Error("You are signed out.");
  }
  const text = await res.text();
  let body = {};
  try { body = text ? JSON.parse(text) : {}; } catch { body = {error: text}; }
  if (!res.ok) {
    const err = new Error(body.error || res.statusText || "The server answered " + res.status + ".");
    err.status = res.status;
    throw err;
  }
  return body;
};
B.send = function (method, path, body) {
  return B.api(path, {method, headers: {"Content-Type": "application/json"}, body: JSON.stringify(body || {})});
};

// act does what the one looking asked and shows the page again; what went
// wrong is said at the top. The buttons of the form it came from wait.
B.act = async function (promise, form) {
  const buttons = form ? [...form.querySelectorAll("button")] : [];
  for (const b of buttons) b.disabled = true;
  try {
    const out = await promise;
    B.trouble(null);
    return out;
  } catch (err) {
    B.trouble(err);
    return null;
  } finally {
    for (const b of buttons) b.disabled = false;
  }
};
B.trouble = function (err) { document.getElementById("trouble").textContent = err ? err.message : ""; };

// Times, in the office's zone: the time today, the day this week, the
// date before that.
const zone = () => B.me ? B.me.office.zone : undefined;
const day = (t) => new Date(t).toLocaleDateString("en-GB", {timeZone: zone()});
const clock = (t) => new Date(t).toLocaleTimeString("en-GB", {timeZone: zone(), hour: "2-digit", minute: "2-digit"});
B.when = function (t) {
  if (!t) return "";
  const d = new Date(t), now = new Date(), timeZone = zone();
  if (day(d) === day(now)) return clock(d);
  if (Math.abs(now - d) < 6 * 864e5) return d.toLocaleString("en-GB", {timeZone, weekday: "short", hour: "2-digit", minute: "2-digit"}).replace(",", "");
  return d.toLocaleDateString("en-GB", {timeZone, day: "numeric", month: "short", year: d.getFullYear() === now.getFullYear() ? undefined : "numeric"});
};
// time gives a time as an element, its whole date on hovering; text, when
// given, is what it shows.
B.time = function (t, text) {
  if (!t) return "";
  return el("time", {datetime: t, title: new Date(t).toLocaleString("en-GB", {timeZone: zone()})}, text || B.when(t));
};
// span gives from one time to another; the end leaves out the day when
// it is the start's.
B.span = function (from, to) {
  if (!to) return B.time(from);
  return [B.time(from), " – ", B.time(to, day(from) === day(to) ? clock(to) : null)];
};
// took gives how long from one time to another: 40s, 2m 05s, 3h 10m.
B.took = function (from, to) {
  const s = Math.max(0, Math.round((new Date(to || Date.now()) - new Date(from)) / 1000));
  if (s < 60) return s + "s";
  if (s < 3600) return Math.floor(s / 60) + "m " + String(s % 60).padStart(2, "0") + "s";
  return Math.floor(s / 3600) + "h " + String(Math.floor(s / 60) % 60).padStart(2, "0") + "m";
};
B.size = function (n) {
  if (typeof n !== "number") return "";
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return Math.round(n / 1024) + " KB";
  return (n / 1024 / 1024).toFixed(1) + " MB";
};
B.firstLine = function (text) { return String(text || "").split("\n")[0]; };

// The icons: static markup of the screens' own, never with data in it.
const ICONS = {
  clock: '<svg viewBox="0 0 16 16"><circle cx="8" cy="8" r="5.8"/><path d="M8 4.8V8l2.2 1.6"/></svg>',
  file: '<svg viewBox="0 0 16 16"><path d="M4 1.5h5.5l3 3v10H4z"/><path d="M9.5 1.5v3h3"/></svg>',
  door: '<svg viewBox="0 0 24 22" aria-hidden="true"><rect class="door-gap" x="7.5" y="4" width="3" height="14"/><path class="door-leaf" d="M9 18H23"/><path class="door-swing" d="M23 18A14 14 0 0 0 9 4"/></svg>',
};
B.icon = function (name) {
  const s = el("span", {class: "ic", "aria-hidden": "true"});
  s.innerHTML = ICONS[name];
  return s;
};
B.door = function () {
  const t = document.createElement("template");
  t.innerHTML = ICONS.door;
  return t.content.firstChild;
};

// Parties: a person, a guest, a team, a routine, a rule or a worker, each
// an icon beside its name. A team links to its page unless asked not to.
B.initials = function (name) {
  const parts = String(name || "?").split(/\s+/).filter(Boolean);
  return (parts.length > 1 ? parts[0][0] + parts[1][0] : String(name || "?").slice(0, 1)).toUpperCase();
};
B.ruleName = function (path) { return String(path).split("/").pop().replace(/\.[^.]+$/, ""); };
B.isMe = function (p) { return p && p.kind === "person" && B.me && p.id === B.me.person.id; };
B.party = function (p, link) {
  if (!p) return "";
  const name = p.name || p.id || p.kind;
  const who = (ic, text, title) => el("span", {class: "who", title}, ic, el("span", {}, text));
  switch (p.kind) {
    case "person": return who(el("span", {class: "ic person" + (B.isMe(p) ? " you" : ""), "aria-hidden": "true"}, B.initials(name)), B.isMe(p) ? name + " (you)" : name);
    case "guest": return who(el("span", {class: "ic guest", "aria-hidden": "true"}, "?"), name);
    case "routine": return who(B.icon("clock"), name);
    case "rule": return who(el("span", {class: "ic rule", "aria-hidden": "true"}), B.ruleName(p.id || p.name), p.id || p.name);
    case "worker": return who(el("span", {class: "ic worker", "aria-hidden": "true"}, String(name).slice(0, 2)), name);
    case "team": {
      const ic = el("span", {class: "ic team", "aria-hidden": "true"});
      return link === false ? who(ic, name) : el("a", {class: "who", href: "#/teams/" + enc(p.id)}, ic, el("span", {}, name));
    }
  }
  return who(null, name);
};
// chain gives deciders in the order they are asked, each after the one
// before; names gives a person's name by their id.
B.chain = function (deciders, names) {
  return el("span", {class: "chain"}, deciders.map((d, i) => el("span", {class: "step"},
    i ? el("span", {class: "then", "aria-hidden": "true"}, "›") : null,
    B.party({kind: d.kind, id: d.name, name: (d.kind === "person" && names[d.name]) || d.name}))));
};

// status gives a dot and a word.
const WORDS = {"timed-out": "timed out", "waiting-to-start": "not started", no: "said no"};
B.status = function (word) { return el("span", {class: "status " + word}, WORDS[word] || word); };

// Bits of a page
B.label = function (text, count) {
  return el("h2", {class: "label"}, text, count !== undefined ? el("span", {class: "count"}, count) : null);
};
B.sect = function (text, count, ...children) { return el("section", {class: "sect"}, B.label(text, count), ...children); };
B.empty = function (text) { return el("p", {class: "empty"}, text); };
B.field = function (label, input, cls) { return el("label", {class: "field " + (cls || "")}, el("span", {}, label), input); };
B.crumbs = function (...parts) {
  return el("nav", {class: "crumbs", "aria-label": "Where this is"}, parts.flatMap((c, i) => [i ? " / " : "", c]));
};
B.notThere = function () {
  return el("div", {class: "page"}, el("div", {class: "head"}, el("h1", {}, "Nothing is here"),
    el("p", {class: "muted"}, "It may be gone, or the address may be wrong. ", el("a", {href: "#/"}, "Back to the requests"))));
};

// The screens

B.route = function (pattern, view, nav) { B.routes.push({pattern, view, nav}); };

// current gives the address of the page shown, and its route.
function current() {
  const hash = location.hash.replace(/^#/, "") || "/";
  return [hash, B.routes.find((r) => r.pattern.test(hash))];
}

// header draws the office's name, the screen one is on, how much waits on
// the one looking, and who they are.
function header(nav, waiting) {
  document.getElementById("brand").textContent = B.me.office.name;
  for (const a of document.querySelectorAll("[data-nav]")) {
    if (a.dataset.nav === nav) a.setAttribute("aria-current", "page"); else a.removeAttribute("aria-current");
  }
  const owed = document.getElementById("owed");
  owed.hidden = !waiting;
  owed.replaceChildren(String(waiting), el("span", {class: "vh"}, " waiting on you"));
  const p = B.me.person;
  document.getElementById("me").replaceChildren(
    el("span", {class: "ic person you", "aria-hidden": "true"}, B.initials(p.name)),
    el("span", {}, p.name));
}

// writing says whether the one looking has written anything on the page:
// a field holds other than what it was given, which is kept in its
// data-was.
function writing() {
  return [...document.querySelectorAll("#main textarea, #main input, #main select")].some((f) =>
    f.type === "file" ? f.files.length > 0 : f.value !== (f.dataset.was || ""));
}

// keep and restore carry what the one looking opened, where they were and
// what had their focus across a drawing of the same page.
function keep(root) {
  const active = document.activeElement;
  return {
    open: new Map([...root.querySelectorAll("details[data-key]")].map((d) => [d.dataset.key, d.open])),
    focus: active && root.contains(active) ? active.dataset.key : null,
    y: scrollY,
  };
}
function restore(root, kept) {
  for (const d of root.querySelectorAll("details[data-key]")) {
    if (kept.open.has(d.dataset.key)) d.open = kept.open.get(d.dataset.key);
  }
  const f = kept.focus && root.querySelector('[data-key="' + CSS.escape(kept.focus) + '"]');
  if (f) f.focus({preventScroll: true});
  scrollTo(scrollX, kept.y);
}

let showing = 0;
let waiting = 0;   // how many cards waited on the one looking, when last counted

// show draws the page the address names. Each view is given what follows
// its pattern and the list of requests to come, which the header counts
// from. again keeps what was open, the focus and the place on the page.
B.show = async function (again) {
  const n = ++showing;
  const main = document.getElementById("main");
  const kept = again ? keep(main) : null;
  const [hash, route] = current();
  try {
    if (!B.me) B.me = await B.api("/api/me");
    const args = hash.match(route.pattern).slice(1).map(decodeURIComponent);
    const listed = B.api("/api/requests");
    const [list, page] = await Promise.all([listed, route.view(args, listed)]);
    if (n !== showing) return;
    waiting = list.waiting.length;
    header(route.nav, waiting);
    main.replaceChildren(page);
    for (const f of main.querySelectorAll("select")) f.dataset.was = f.value;
    document.getElementById("stale").hidden = true;
    if (kept) restore(main, kept);
  } catch (err) {
    if (n !== showing) return;
    if (B.me) header(route.nav, waiting);
    if (err.status === 404) main.replaceChildren(B.notThere());
    else B.trouble(err);
  }
};

// count counts again the cards waiting on the one looking.
async function count() {
  try {
    waiting = (await B.api("/api/requests")).waiting.length;
    header(current()[1].nav, waiting);
  } catch (err) {
    B.trouble(err);
  }
}

// The stream of changes: each change fetches again what is shown, once
// for a burst of them, unless something is being written on the page,
// when the one looking is told and shows it when they choose. The office,
// its teams and their docs change only with the office: a request's
// change counts again only what waits on the one looking.
let later = 0;
let office = false;
B.changed = function (what) {
  if (what.office) { B.me = null; office = true; }
  clearTimeout(later);
  later = setTimeout(() => {
    const all = office || current()[1].nav !== "office";
    office = false;
    if (!all) count();
    else if (writing()) document.getElementById("stale").hidden = false;
    else B.show(true);
  }, 250);
};

B.start = function () {
  addEventListener("hashchange", async () => {
    B.trouble(null);
    await B.show();
    scrollTo(0, 0);
    document.getElementById("main").focus({preventScroll: true});
  });
  B.narrow.addEventListener("change", () => B.show(true));
  document.getElementById("stale-show").addEventListener("click", () => B.show(true));
  document.getElementById("signout").addEventListener("click", async () => {
    await fetch("/signout", {method: "POST"}).catch(() => null);
    location.assign("/signin");
  });
  // A stream that comes back after a break fetches again what is shown,
  // for what changed meanwhile.
  const changes = new EventSource("/api/events");
  let opened = false;
  changes.addEventListener("open", () => { if (opened) B.changed({office: true}); opened = true; });
  changes.addEventListener("changed", (e) => {
    let what = {};
    try { what = JSON.parse(e.data); } catch { what = {}; }
    B.changed(what);
  });
  B.show();
};
