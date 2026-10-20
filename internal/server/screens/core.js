// What every view shares: making elements, reaching the API, the names of
// people, teams and workers, times, statuses, and the views by address.
// What is shown is put in as text, never as markup; the one markup put in
// is the icons below, which hold nothing of the office's.
"use strict";

var B = {
  me: null,       // /api/me, fetched when the screens load and when the office changes
  views: [],      // [pattern, view]: the view of an address after #
};

// el makes an element: attrs set attributes, or listen when named on…;
// children are elements or text.
B.el = function (tag, attrs, ...children) {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k.startsWith("on") && typeof v === "function") e.addEventListener(k.slice(2), v);
    else if (v !== undefined && v !== null && v !== false) e.setAttribute(k, v === true ? "" : v);
  }
  for (const c of children.flat(Infinity)) {
    if (c === null || c === undefined || c === false || c === "") continue;
    e.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
  return e;
};

// route adds a view: a pattern for the address after #, a function of its
// match that gives the view's element, or a promise of one, the screen it
// is under, and, when given, which changes it is drawn again for: a
// function of what changed and its match.
B.route = function (pattern, view, nav, cares) {
  B.views.push([pattern, view, nav, cares]);
};

// The API

B.api = async function (path, options) {
  const res = await fetch(path, options);
  const out = await res.json().catch(() => ({error: res.statusText}));
  if (!res.ok) throw new Error(out.error || res.statusText);
  return out;
};

B.post = function (path, body) {
  return B.api(path, {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify(body || {})});
};

// send sends what is said with its files, as a form of several parts, or
// as JSON when there are none.
B.send = function (path, fields, files) {
  if (!files || !files.files.length) return B.post(path, fields);
  const body = new FormData();
  for (const [k, v] of Object.entries(fields)) body.append(k, v);
  for (const f of files.files) body.append("files", f);
  return B.api(path, {method: "POST", body});
};

// act does what the one looking asked, says what came of it, and shows the
// view again; the fields of the form it came from, when given, are emptied.
B.act = async function (promise, form) {
  try {
    const out = await promise;
    if (form) for (const f of form.querySelectorAll("textarea, input")) if (f.type !== "search") f.value = "";
    B.said(out && out.said);
    await B.show();
    return out;
  } catch (err) { B.trouble(err); }
};

B.said = function (text) { document.getElementById("said").textContent = text || ""; };
B.trouble = function (err) { document.getElementById("trouble").textContent = err ? err.message : ""; };

// Names, as the office calls them

B.person = function (name) { return (B.me.people || []).find((p) => p.name === name); };
B.team = function (name) { return (B.me.teams || []).find((t) => t.name === name); };
B.personCalled = function (name) { const p = B.person(name); return p ? p.called : name; };
B.teamCalled = function (name) { const t = B.team(name); return t ? t.called : name; };
B.isMe = function (name) { return B.me && B.me.me.name === name; };
B.teamHref = function (name) { return "#/t/" + encodeURIComponent(name); };
B.personHref = function (name) { return B.isMe(name) ? "#/me" : "#/p/" + encodeURIComponent(name); };
B.workerHref = function (team, name) { return "#/t/" + encodeURIComponent(team) + "/w/" + encodeURIComponent(name); };

function initials(called) {
  const parts = String(called || "?").split(/\s+/).filter(Boolean);
  return (parts.length > 1 ? parts[0][0] + parts[1][0] : String(called || "?").slice(0, 1)).toUpperCase();
}

// The icons: static markup of the screens' own, never with data in it.
const ICONS = {
  chat: '<svg viewBox="0 0 16 16"><path d="M2.5 3.5h11v7h-6l-3 2.5v-2.5h-2z"/></svg>',
  bridge: '<svg viewBox="0 0 16 16"><path d="M3 14V3h10v11"/><path d="M1.5 14h13"/><path d="M6 14V7h4v7"/></svg>',
  clock: '<svg viewBox="0 0 16 16"><circle cx="8" cy="8" r="5.8"/><path d="M8 4.8V8l2.2 1.6"/></svg>',
  branch: '<svg viewBox="0 0 16 16"><path d="M4 2.5v11"/><path d="M4 7c0 2 2 3 6 3h2.5"/><path d="M10.5 8l2 2-2 2"/></svg>',
  agent: '<svg viewBox="0 0 16 16"><path d="M8 2l1.6 4.1L14 8l-4.4 1.9L8 14l-1.6-4.1L2 8l4.4-1.9z"/></svg>',
  flow: '<svg viewBox="0 0 16 16"><rect x="1.5" y="5.5" width="4" height="5"/><rect x="10.5" y="5.5" width="4" height="5"/><path d="M5.5 8h5"/></svg>',
  statechart: '<svg viewBox="0 0 16 16"><rect x="1.5" y="2.5" width="6" height="4.5" rx="2"/><rect x="8.5" y="9" width="6" height="4.5" rx="2"/><path d="M4.5 7v4.2h4"/></svg>',
  lock: '<svg viewBox="0 0 16 16"><rect x="3" y="7" width="10" height="7"/><path d="M5.2 7V5a2.8 2.8 0 0 1 5.6 0v2"/></svg>',
  file: '<svg viewBox="0 0 16 16"><path d="M4 1.5h5.5l3 3v10H4z"/><path d="M9.5 1.5v3h3"/></svg>',
  script: '<svg viewBox="0 0 16 16"><path d="M5.5 5L2.5 8l3 3"/><path d="M10.5 5l3 3-3 3"/></svg>',
  search: '<svg viewBox="0 0 16 16"><circle cx="7" cy="7" r="4.5"/><path d="M10.5 10.5l3.5 3.5"/></svg>',
};

// icon gives an icon by its name, with the classes given.
B.icon = function (name, cls) {
  const s = B.el("span", {class: "ic " + (cls || ""), "aria-hidden": "true"});
  s.innerHTML = ICONS[name] || "";
  return s;
};

// The kinds of thing, each an icon beside its name. A name links to where
// the thing is shown, when the one looking may see it there.
// A person as a page's title is large, and links nowhere.
B.personEl = function (name, opts) {
  const called = B.personCalled(name);
  const title = opts && opts.title;
  const ic = B.el("span", {class: "ic person" + (B.isMe(name) ? " you" : "") + (title ? " lg" : ""), "aria-hidden": "true"}, initials(called));
  return B.person(name) && !title ? B.el("a", {class: "who", href: B.personHref(name)}, ic, B.el("span", {}, called)) : B.el("span", {class: "who"}, ic, B.el("span", {}, called));
};
B.workerEl = function (name, team, opts) {
  const ic = B.el("span", {class: "ic worker", "aria-hidden": "true"}, String(name).slice(0, 2));
  const label = team && !(opts && opts.bare) ? [name, " ", B.el("span", {class: "faint"}, "· " + B.teamCalled(team))] : [name];
  return team ? B.el("a", {class: "who", href: B.workerHref(team, name)}, ic, B.el("span", {}, label)) : B.el("span", {class: "who"}, ic, B.el("span", {}, label));
};
B.teamEl = function (name) {
  return B.el("a", {class: "who", href: B.teamHref(name)}, B.el("span", {class: "ic team", "aria-hidden": "true"}), B.el("span", {}, B.teamCalled(name)));
};
B.ruleEl = function (name) {
  return B.el("span", {class: "who"}, B.el("span", {class: "ic rule", "aria-hidden": "true"}), B.el("span", {class: "mono"}, name));
};
// ruleShortEl gives a rule by its short name, the last part of its path
// without its kind, its path when asked.
B.ruleShortEl = function (path) {
  return B.el("span", {class: "who", title: path}, B.el("span", {class: "ic rule", "aria-hidden": "true"}),
    B.el("span", {class: "mono"}, path.split("/").pop().replace(/\.[^.]+$/, "")));
};

// The kits Bulldog's tools come in, by what they let a worker do.
B.KITS = {talk: "Talks to people and teams", docs: "Reads docs", notes: "Writes notes", build: "Drafts changes to the office"};
// deciderEl gives one decider: a person, a worker with the team it is of,
// a rule by its path, or Bulldog. A worker of another team than here is
// given with its team.
B.deciderEl = function (d, here) {
  switch (d.kind) {
    case "person": return B.personEl(d.name);
    case "worker": return B.workerEl(d.name, d.team, {bare: d.team === here});
    case "rule": return B.ruleEl(d.name);
    case "bulldog": return B.bulldogEl();
  }
  return B.el("span", {class: "mono"}, d.called || d.name || d.kind);
};
B.bulldogEl = function () { return B.el("span", {class: "ic bulldog"}, "Bulldog"); };

// chain gives who decides, in the order the walk asks them.
B.chain = function (deciders, here) {
  return B.el("span", {class: "chain"}, deciders.flatMap((d, i) => [i ? B.el("span", {class: "then", "aria-hidden": "true"}, "›") : null, B.deciderEl(d, here)]));
};

// placeEl gives a place a bridge listens in as the way to it: the bridge,
// then the place.
B.placeEl = function (bridge, place) {
  return B.el("span", {class: "who"}, B.icon("bridge"), B.el("a", {href: "#/office/bridges"}, bridge), B.el("span", {class: "faint"}, "›"), B.el("span", {class: "mono"}, place));
};

// placesOf gives the places of every bridge that ask a team.
B.placesOf = function (team) {
  return (B.me.office.bridges || []).flatMap((b) => b.places.filter((p) => p.team === team).map((p) => ({bridge: b.name, place: p, status: b.status})));
};

// band gives one band: its label beside the rows it holds, each a label
// and what it says, nested beneath when it holds more; a row with no label
// says it across the band.
B.band = function (label, rows, ...more) {
  const shown = rows.filter((r) => r && r[1] !== null && r[1] !== undefined && r[1] !== "");
  if (!shown.length) return null;
  return B.el("section", {class: "band", "aria-label": label},
    B.el("div", {class: "band-label"}, label, ...more),
    B.el("div", {class: "band-rows"}, shown.map(([k, v, under]) => k ? B.el("div", {class: "band-row"},
      B.el("div", {class: "band-k"}, k), B.el("div", {class: "band-v"}, v),
      under ? B.el("div", {class: "band-under"}, under) : null)
      : B.el("div", {class: "band-row band-across"}, B.el("div", {class: "band-v"}, v)))));
};

// notesInfo gives the (i) of a team's notes.
B.notesInfo = function () {
  return B.info("About notes", "Given whole to every run of this team, up to 50,000 characters, in name order.");
};

// givenEl gives how much of a note a run is given, when not all of it.
B.givenEl = function (n) {
  if (n.given === n.size) return null;
  return n.given ? B.el("span", {class: "status cut"}, "cut")
    : B.el("span", {class: "status not-given"}, "not given");
};

// The words a thread's entries are read by, for their kinds; Bulldog's
// own have none, and an entry of a decision is "Decision" where the
// decision is not at hand.
const ENTRY = {asked: "Asked", said: "Said", question: "Question", told: "Told", replied: "Reply", sub: "Sub-request",
  answer: "Answer", late: "Late answer", held: "Held", decide: "Asked to decide", decided: "Verdict", referred: "Referred",
  notice: "Notice", noted: "Noted", decision: "Decision", bulldog: ""};
B.entryWord = function (kind) { return kind in ENTRY ? ENTRY[kind] : kind; };

// The words decisions are read by, for their kinds.
const DECISION = {action: "Authorisation", disclosure: "Disclosure", change: "Change"};
B.decisionWord = function (kind) { return DECISION[kind] || kind; };

// entryLabel gives the word an entry is read by: an entry of a decision
// by the decision's kind, of those given.
B.entryLabel = function (decisions, e) {
  const d = e.kind === "decision" && decisions.find((x) => x.id === e.decision);
  return d ? B.decisionWord(d.kind) : B.entryWord(e.kind);
};

// leadCalled gives a team's lead as it is called, or "the lead".
B.leadCalled = function (t) {
  if (!t || !t.lead) return "the lead";
  return B.person(t.lead) ? B.personCalled(t.lead) : t.lead;
};

// The labels of a team's Heed, Lead and Disclosure rows, each with what
// it means for this team behind its (i); the lead's may be said in other
// words, as where it stands in a tree.
B.heedLabel = function (t) {
  return B.el("span", {class: "line"}, "Heed", ...B.info("About heed",
    "Asked first about what is said at a channel place. Act: " + B.leadCalled(t) + " starts. Note: kept in the request; nothing runs or is said."));
};
B.leadLabel = function (t, word) {
  return B.el("span", {class: "line"}, word || "Lead", ...B.info("About lead", B.person(t.lead)
    ? "Each request to " + t.called + " waits on " + B.leadCalled(t) + ", and what they say in it is its answer. A routine's request goes to the worker the routine names. Being lead decides nothing."
    : "Started on each request to " + t.called + ", and again on anything said to " + t.called + " in it, answered or not. A routine's request starts the worker the routine names. Being lead decides nothing."));
};
B.disclosureLabel = function (t) {
  return B.el("span", {class: "line"}, "Disclosure", ...B.info("About disclosure",
    "Asked before anything leaves " + t.called + ": answers, questions and tells to people outside it, requests to other teams. Yes: disclosed as it was. Refused: back to " + B.leadCalled(t) + ", with the note."));
};

// settingsEl gives a rule's settings, each its value, marked default when
// the team file leaves it out, or None when nothing gives it one, and what
// it is for behind its (i).
B.settingsEl = function (settings) {
  if (!settings || !settings.length) return null;
  const value = (x) => x.value == null ? B.el("span", {class: "faint"}, "None")
    : B.el("span", {class: "mono"}, typeof x.value === "object" ? JSON.stringify(x.value) : String(x.value));
  return B.kv(settings.map((x) => [B.el("span", {class: "line settings-k"}, B.el("span", {class: "mono"}, x.name), x.about ? B.info("About " + x.name, x.about) : ""),
    B.el("span", {class: "line"}, value(x), x.default ? B.el("span", {class: "faint small"}, "default") : null)]));
};
B.guestEl = function (id, bridge) {
  return B.el("span", {class: "who"}, B.el("span", {class: "ic guest", "aria-hidden": "true"}, "?"), B.el("span", {}, "guest ", B.el("span", {class: "mono"}, id), " · " + bridge));
};

// party gives someone as the office names them now: who asked, whom
// something waits on, and whom an entry is by or to.
B.party = function (p) {
  if (!p) return "";
  switch (p.kind) {
    case "person": return B.personEl(p.name);
    case "guest": return B.guestEl(p.id, p.bridge);
    case "gone": return B.el("span", {class: "who faint"}, p.name + " (gone)");
    case "worker": return B.workerEl(p.name, p.team);
    case "rule": return B.ruleEl(p.name);
    case "team": return B.teamEl(p.name);
    case "bulldog": return B.bulldogEl();
  }
  return p.called || p.name || p.kind;
};

// partyWord gives a party in a word or two, for a status.
B.partyWord = function (p) {
  if (!p) return "";
  if (p.kind === "person") return B.isMe(p.name) ? "you" : B.personCalled(p.name);
  if (p.kind === "guest") return "guest";
  return p.called || p.name || p.kind;
};

// origin gives where a request was asked: the chat, a routine's time, or
// a bridge and the place it lists.
B.origin = function (o) {
  if (!o) return "";
  switch (o.kind) {
    case "chat": return B.el("span", {class: "who muted"}, B.icon("chat"), "chat");
    case "routine": return B.el("span", {class: "who muted"}, B.icon("clock"), B.el("span", {class: "mono"}, o.at));
  }
  if (o.bridge) return B.placeEl(o.bridge, o.heard || o.place);
  return o.kind;
};

// way gives where a request was asked, as the way to it: a bridge, the
// place listed, then the place itself where it is narrower; the chat; a
// routine's time and worker.
B.way = function (o) {
  if (!o) return "";
  const sep = () => B.el("span", {class: "faint", "aria-hidden": "true"}, "›");
  switch (o.kind) {
    case "chat": return B.el("span", {class: "who"}, B.icon("chat"), "Chat");
    case "routine": return B.el("span", {class: "line"}, B.icon("clock"), B.el("span", {class: "mono"}, o.at), sep(), o.team ? B.workerEl(o.to, o.team, {bare: true}) : o.to);
  }
  if (!o.bridge) return o.kind;
  const heard = o.heard || o.place;
  return B.el("span", {class: "line"}, B.placeEl(o.bridge, heard), o.place && o.place !== heard ? [sep(), B.el("span", {class: "mono"}, o.place)] : null);
};

// status gives a dot and a word: queued with its place in the queue,
// working, waiting · whom, answered, or what labels an answered request,
// stopped, noted or quiet; and, for a request from an issue, its status
// at its source.
B.status = function (r) {
  if (r.source) return B.el("span", {class: "status", title: "at its source, " + B.when(r.source.at)}, r.source.status);
  if (r.state === "working" && r.queued) return B.el("span", {class: "status queued"}, "queued " + r.queued);
  const on = r.on && r.on.length ? " · " + r.on.map(B.partyWord).join(", ") : "";
  const word = r.state === "answered" && r.label ? r.label : r.state;
  return B.el("span", {class: "status " + word + (r.state === "working" ? " live" : "")},
    word + (r.state === "waiting" ? on + (r.in ? " in #" + r.in : "") : ""));
};

// standing gives a request as its one status says it: one at work whose
// sub-request, as far down as they go, waits on someone stands waiting on
// them, in that sub-request.
B.standing = function (r) {
  if (r.state !== "working" || r.queued) return r;
  for (const s of r.subs || []) {
    const deep = B.standing(s);
    if (deep.state === "waiting") return Object.assign({}, r, {state: "waiting", on: deep.on, in: deep.in || s.number});
  }
  return r;
};

// atWork gives a team's workers at work, but for a run held while its
// request waits: on someone, or on a sub-request it asked.
const UNHELD = new Set(["queued", "heed", "answered"]);
B.atWork = function (t) {
  const held = new Set((t.now || []).filter((x) => !UNHELD.has(x.kind)).map((x) => x.request));
  return (t.at_work || []).filter((a) => !held.has(a.request));
};

// What flows comes in only when it is new since a view was first drawn,
// and a view drawn again finds it as far in as it came; what breathes and
// rings keeps its beat across drawings. Both times are the tokens of the
// stylesheet.
function token(name) { return parseFloat(getComputedStyle(document.documentElement).getPropertyValue(name)) * 1000 || 0; }

// comeIn marks el as coming in, as far as it came, when key is new since
// the view was first drawn; seen holds when each key came, 0 for those of
// the first drawing.
B.comeIn = function (el, seen, key, drawn, now) {
  if (!seen.has(key)) seen.set(key, drawn ? now : 0);
  const came = seen.get(key);
  if (came && now - came < token("--enter")) {
    el.classList.add("new");
    if (now > came) el.style.animationDelay = -(now - came) + "ms";
  }
  return el;
};

// keepBeat sets the beat of what breathes and rings in el where it was.
B.keepBeat = function (el, now) {
  el.style.setProperty("--phase", -(now % token("--breathe")) + "ms");
  return el;
};

// bridgeStatus gives a bridge's one status: running, stopped, or, to the
// top team's people, failing.
B.bridgeStatus = function (b) { return B.el("span", {class: "status " + b.status}, b.status); };

// modelText gives a model as it is sent, written in a row, in a few
// words: its name and service, or "" when it gives neither.
B.modelText = function (text) {
  const field = (k) => {
    const m = String(text || "").match(new RegExp("(?:^|[{,\\s])" + k + "\\s*:\\s*(\"[^\"]*\"|'[^']*'|[^,}\\s]+)"));
    return m ? m[1].replace(/^["']|["']$/g, "") : "";
  };
  return [field("name"), field("service")].filter(Boolean).join(" · ");
};

// size gives a size in bytes as a person reads one.
B.size = function (n) {
  if (typeof n !== "number") return "";
  if (n < 1024) return n + " B";
  if (n < 1024 * 1024) return Math.round(n / 1024) + " KB";
  return (n / 1024 / 1024).toFixed(1) + " MB";
};

// Times, in the office's zone: the time today, the day this week, the
// date before that.
B.when = function (t) {
  if (!t) return "";
  const d = new Date(t), now = new Date(), zone = B.me ? B.me.office.zone : undefined;
  const day = (x) => x.toLocaleDateString("en-GB", {timeZone: zone});
  if (day(d) === day(now)) return d.toLocaleTimeString("en-GB", {timeZone: zone, hour: "2-digit", minute: "2-digit"});
  if (now - d < 6 * 864e5) return d.toLocaleString("en-GB", {timeZone: zone, weekday: "short", hour: "2-digit", minute: "2-digit"}).replace(",", "");
  return d.toLocaleDateString("en-GB", {timeZone: zone, day: "numeric", month: "short", year: d.getFullYear() === now.getFullYear() ? undefined : "numeric"});
};
B.whenEl = function (t) {
  return t ? B.el("time", {class: "mono faint", datetime: t, title: new Date(t).toLocaleString("en-GB", {timeZone: B.me.office.zone})}, B.when(t)) : "";
};

// info gives an (i) that shows a little more on asking.
let pops = 0;
B.info = function (label, ...text) {
  const id = "pop-" + ++pops;
  return [
    B.el("button", {class: "info", type: "button", "aria-label": label, popovertarget: id}, "i"),
    B.el("div", {class: "pop", id, popover: "auto"}, ...text),
  ];
};

// Bits of a page

B.label = function (text, ...more) { return B.el("div", {class: "label"}, text, ...more); };
B.sect = function (label, ...children) { return B.el("section", {class: "sect"}, typeof label === "string" ? B.label(label) : label, ...children); };
B.kv = function (pairs) {
  return B.el("dl", {class: "kv"}, pairs.filter((p) => p && p[1] !== undefined && p[1] !== null && p[1] !== "").map(([k, v]) => [B.el("dt", {}, k), B.el("dd", {}, v)]));
};
B.empty = function (text) { return B.el("div", {class: "empty"}, text || "none"); };
B.head = function (crumbs, title, ...right) {
  return B.el("div", {},
    crumbs && crumbs.length ? B.el("div", {class: "crumbs"}, crumbs.flatMap((c, i) => [i ? " / " : "", c])) : null,
    B.el("div", {class: "head"}, B.el("h1", {}, title), ...right));
};
B.field = function (label, input) { return B.el("label", {class: "field"}, B.el("span", {}, label), input); };

// search gives a search box that keeps, of the items given, those whose
// text holds what is typed, and draws them with draw.
B.search = function (placeholder, items, text, draw) {
  const out = B.el("div", {});
  const box = B.el("input", {class: "in search", type: "search", placeholder, "aria-label": placeholder});
  const redraw = () => {
    const q = box.value.trim().toLowerCase();
    out.replaceChildren(draw(q ? items.filter((it) => text(it).toLowerCase().includes(q)) : items, q));
  };
  box.addEventListener("input", redraw);
  redraw();
  return {box, out};
};

// The views

// writing says whether the one looking has written anything in the view: a
// field holds other than what it was given, which a view that fills one
// keeps in its data-was.
function writing() {
  return [...document.querySelectorAll("#main textarea, #main input:not([type=search])")].some((f) =>
    f.type === "file" ? f.files.length > 0 : f.value !== (f.dataset.was || ""));
}

// header draws the office's name, the screens one may go to, and the one
// looking with how much waits on them.
function header(nav) {
  document.getElementById("office").textContent = B.me.office.name;
  document.title = B.me.office.name;
  for (const a of document.querySelectorAll("[data-nav]")) {
    if (a.dataset.nav === nav) a.setAttribute("aria-current", "page"); else a.removeAttribute("aria-current");
  }
  const me = document.getElementById("me");
  me.setAttribute("aria-label", B.me.me.called + (B.me.waiting ? ", " + B.me.waiting + " waiting on you" : ""));
  me.replaceChildren(...[
    B.me.waiting ? B.el("span", {class: "owed"}, B.me.waiting) : null,
    B.el("span", {class: "mono"}, B.me.me.called),
    B.el("span", {class: "ic person you", "aria-hidden": "true"}, initials(B.me.me.called))].filter(Boolean));
  for (const runs of document.querySelectorAll(".of-runs")) runs.replaceWith(B.runsEl());
}

// runsEl gives how many runs of the office are at work of as many as it
// lets work, and how many wait their turn; it is drawn again in place as
// they change.
B.runsEl = function () {
  const runs = B.me.office.runs;
  return B.el("span", {class: "line of-runs"}, B.el("span", {class: "mono"}, runs.working + " / " + runs.limit),
    runs.queued ? B.el("span", {class: "tag"}, "Queued " + runs.queued) : null);
};

let showing = 0;

// show draws the view the address names. The office as the one looking
// sees it is fetched first when it is not there yet.
B.show = async function () {
  const n = ++showing;
  document.getElementById("stale").hidden = true;
  B.trouble(null);
  try {
    if (!B.me) B.me = await B.api("/api/me");
    for (const [pattern, view, nav] of B.views) {
      const m = location.hash.match(pattern);
      if (!m) continue;
      const out = await view(m);
      if (n !== showing) return;
      header(nav);
      document.getElementById("main").replaceChildren(out);
      break;
    }
  } catch (err) { B.trouble(err); }
};

// changed is what the stream of changes names: the office read again, or
// the runs at work, fetches /api/me again; the view is drawn again, once
// for a burst of changes, unless it streams itself or something is being
// written in it, when the one looking is told and shows it when they
// choose.
const REDRAW = 250; // ms, how long a burst of changes is gathered
let redraw = 0;
B.changed = async function (what) {
  if (what === "office" || what === "runs" || what === "waiting" || what === "" || what === undefined) {
    try { B.me = await B.api("/api/me"); } catch (err) { B.trouble(err); return; }
  }
  if (B.streaming || !caresFor(what)) { header(B.navOf()); return; }
  if (writing()) {
    header(B.navOf());
    document.getElementById("stale").hidden = false;
    return;
  }
  if (!redraw) redraw = setTimeout(() => { redraw = 0; B.show(); }, REDRAW);
};

// caresFor says whether the view shown is drawn again for what changed.
function caresFor(what) {
  for (const [pattern, , , cares] of B.views) {
    const m = location.hash.match(pattern);
    if (m) return !cares || !what || cares(what, m);
  }
  return true;
}

B.navOf = function () {
  for (const [pattern, , nav] of B.views) if (location.hash.match(pattern)) return nav;
  return "";
};

// query gives the address's query, after its ?.
B.query = function () { return new URLSearchParams(location.hash.split("?")[1] || ""); };
