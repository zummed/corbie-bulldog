// A request: the team's room with the request's way through it, its forks,
// what the team read for its people, the thread it is in, and saying more.
"use strict";

// argLine gives a call's input on one line.
function argLine(args) {
  if (args === undefined || args === null) return "";
  if (typeof args !== "object") return String(args);
  return Object.values(args).map((v) => typeof v === "string" ? v : JSON.stringify(v)).join(", ");
}

// calledList gives calls as a small tree: each call, what it was given,
// how it ended, and under a colleague the tools it called.
function calledList(calls) {
  return el("ul", {class: "calls"}, calls.map((c) => el("li", {},
    el("span", {class: "tool"}, c.tool),
    el("span", {class: "arg"}, argLine(c.args)),
    c.error ? el("span", {class: "bad", title: c.error}, "✗ failed")
      : c.result !== undefined && c.result !== null ? el("span", {class: "ok"}, "✓")
        : el("span", {class: "on"}, "…"),
    c.calls ? calledList(c.calls) : null)));
}

// used gives the tools and colleagues the lead used.
function used(calls, live) {
  if (!calls.length) return B.empty(live ? "Nothing used yet." : "No tools used.");
  return el("div", {}, el("div", {class: "k"}, "Used"), calledList(calls));
}

// readPanel gives what the team read, folded: each decider's word and
// note, the lead's conversation, and every call in full.
function readPanel(v) {
  const r = v.read;
  const decided = v.nodes.filter((n) => (n.stage === "heed" || n.stage === "check") && n.deciders && n.deciders.length);
  const show = (x) => typeof x === "string" ? x : JSON.stringify(x, null, 2);
  const parts = [
    ...decided.map((n) => el("section", {}, el("div", {class: "k"}, STAGE[n.stage]), B.decided(n.deciders))),
    r.turns && r.turns.length ? el("section", {}, el("div", {class: "k"}, "The lead's conversation"),
      el("ol", {class: "turns"}, r.turns.map((t) => el("li", {class: t.role}, el("span", {class: "role"}, t.role), el("p", {}, t.text))))) : null,
    r.calls && r.calls.length ? el("section", {}, el("div", {class: "k"}, "Calls"),
      el("ol", {class: "turns"}, r.calls.map((c) => el("li", {class: "tool"},
        el("span", {class: "role"}, B.when(c.at)),
        el("div", {}, el("strong", {class: "mono"}, c.tool), el("pre", {}, show(c.args)),
          c.error ? el("pre", {class: "error"}, c.error) : c.result !== undefined && c.result !== null ? el("pre", {class: "muted"}, "→ " + show(c.result)) : null))))) : null,
  ].filter(Boolean);
  return el("details", {class: "read", "data-key": "read"},
    el("summary", {}, "What the team read"),
    el("div", {class: "read-body"}, parts.length ? parts : B.empty("Nothing read yet.")));
}

function answerOf(text) {
  if (!text) return null;
  return el("div", {class: "answer"}, el("span", {class: "k"}, "Answer"), el("p", {class: "answer-text"}, text));
}

function saidOf(src) {
  const files = src.files && src.files.length ? el("div", {class: "files"}, src.files.map((f) =>
    el("a", {class: "file", href: f.href, download: f.name}, B.icon("file"), f.name, el("span", {class: "faint"}, B.size(f.size))))) : null;
  return [el("p", {class: "said-text"}, src.said || "(nothing said)"), files];
}

// threadOf gives the requests of the thread in order, each linking to
// its own, this one marked, and a box to say more.
function threadOf(v) {
  const thread = v.thread || [];
  const others = thread.filter((t) => t.id !== v.id);
  if (!others.length && !v.canSay) return null;
  const say = el("textarea", {class: "in", name: "said", rows: 2, required: true, "data-key": "say"});
  const form = v.canSay ? el("form", {class: "say-more", onsubmit: async (e) => {
    e.preventDefault();
    const out = await B.act(B.send("POST", "/api/requests/" + enc(v.id) + "/say", {said: say.value.trim()}), form);
    if (out) { B.starting.add(out.id); location.hash = "#/requests/" + enc(out.id); }
  }}, B.field("Say more", say), el("button", {class: "btn go", type: "submit"}, "Say")) : null;
  if (!others.length) return el("section", {class: "sect"}, form);
  return B.sect("Thread", thread.length,
    el("ol", {class: "thread"}, thread.map((t) => {
      const here = t.id === v.id;
      return el("li", {class: "entry" + (here ? " here" : ""), "aria-current": here ? "page" : null},
        el("div", {class: "entry-said"},
          here ? el("strong", {}, B.firstLine(t.said)) : el("a", {href: "#/requests/" + enc(t.id)}, B.firstLine(t.said)),
          B.status(t.status), B.time(t.at)),
        t.answer ? el("p", {class: "entry-answer"}, t.answer) : null);
    })),
    form);
}

// A request asked here is not there until Corbie has started its run, and
// shows when its first change comes.
const starting = () => el("div", {class: "page"}, el("div", {class: "head"},
  el("h1", {}, "Starting the request"), el("p", {class: "muted"}, "It shows here once Corbie has started it.")));

B.requestView = async function ([id], listed) {
  const [v, list] = await Promise.all([B.api("/api/requests/" + enc(id)).catch((err) => {
    if (err.status === 404 && B.starting.has(id)) return null;
    throw err;
  }), listed]);
  if (!v) return starting();
  const src = v.source;
  const sealed = v.read.sealed;
  const first = B.firstLine(src.said) || "(nothing said)";
  document.title = first + " · " + v.teamName;

  const stop = v.canStop
    ? el("button", {class: "btn no sm", type: "button", onclick: async (e) => {
      if (!confirm("Stop this request, and everything it asked?")) return;
      const out = await B.act(B.send("POST", "/api/requests/" + enc(v.id) + "/stop"), e.target.parentElement);
      if (out) B.show(true);
    }}, "Stop") : null;

  // Under WORK: what the team used and read, for its people, hatched for
  // others. With no WORK, as after a heed's no, only what the team read.
  const worked = v.nodes.some((n) => n.stage === "work" && n.state !== "skipped");
  const work = sealed
    ? (worked ? el("div", {class: "sealed sealed-note sealed-work"}, el("span", {}, "Seen only by the team's people.")) : null)
    : [v.read.error ? el("p", {class: "error"}, v.read.error) : null,
      worked ? used(v.read.calls || [], v.status === "working" || v.status === "waiting") : null,
      readPanel(v)];

  // What waits on the one looking here that no fork holds: a question put
  // by a heed or a check.
  const forks = v.forks || [];
  const forked = new Set(forks.filter((f) => f.card).map((f) => f.card.ref));
  const others = list.waiting.filter((c) => c.request === v.id && !forked.has(c.ref));

  const drawing = B.drawing({
    label: v.teamName,
    name: el("a", {href: "#/teams/" + enc(v.team)}, v.teamName),
    nodes: v.nodes.map((n) => ({stage: n.stage, state: n.state, what: n.line || null})),
    said: saidOf(src),
    work,
    answer: answerOf(v.answer),
    forks: forks.map(B.forkEl),
  });

  return el("div", {class: "page"},
    el("div", {class: "head"},
      B.crumbs(el("a", {href: "#/"}, "Requests"), el("a", {href: "#/teams/" + enc(v.team)}, v.teamName)),
      el("h1", {class: "vh"}, first),
      el("div", {class: "meta"},
        B.status(v.status),
        el("span", {}, B.span(v.at, v.ended), v.ended ? " · " + B.took(v.at, v.ended) : null),
        src.request ? el("a", {href: "#/requests/" + enc(src.request)}, "the request that asked this ›") : null,
        stop)),
    drawing,
    others.length ? B.sect("Waiting on you", others.length, el("div", {class: "cards"}, others.map((c) => B.card(c)))) : null,
    threadOf(v));
};
