// One request as it went, a trace that builds downward: where it was
// asked, its heed's word, each run of its lead, what the team called on,
// the questions, decisions and sub-requests, each sub-request's own trace
// inside it, and where its answer went. What goes on now breathes, what
// waits on someone rings, and what is new since the page was drawn comes
// in. The desk draws a decision and an answer to a question with the same
// parts.
(function () {
  "use strict";

  const ENDED = {yes: "approved", refused: "refused", withdrawn: "withdrawn"};
  const WHAT = {answer: "Answer", question: "Question", tell: "Tell", sub: "Sub-request"};
  const FEW = 3; // of a long list of people, how many are drawn before +N
  const DEEP = 2; // how many levels of sub-requests are drawn before they open on asking
  const BREATHE = 1800; // ms, the beat of what breathes and rings, kept across drawings
  const ENTER = 600; // ms, how long a new step takes to come in

  // clip gives a text on one line, no longer than n.
  function clip(text, n) {
    const s = String(text === undefined || text === null ? "" : text).replace(/\s+/g, " ").trim();
    return s.length > n ? s.slice(0, n - 1) + "…" : s;
  }

  function firstLine(text) { return String(text || "").split("\n")[0]; }

  function arrow() { return B.el("span", {class: "rq-arrow", role: "img", "aria-label": "to"}, "→"); }

  // kept is what the one looking did on a request's page, so that drawing
  // it again at a change keeps it: the thread as far back as Earlier
  // reached, how far each More went, the sub-requests opened and folded,
  // the records opened; and what the page has drawn: the steps already
  // shown, each with when it came in, so that only a new one comes in and
  // a drawing again finds it as far in as it came; the newest time among
  // them; and the requests drawn in it, whose changes draw it again.
  let kept = keep(0);
  function keep(n) {
    return {n, entries: [], before: 0, more: {}, open: new Set(), shut: new Set(), records: new Set(),
      seen: new Map(), drawn: false, latest: 0, tree: new Set()};
  }

  // fields are what is written on a page, by where it is written, so that
  // drawing the page again puts back the same field, with its draft and
  // its files; a field is let go once what it held is sent.
  const fields = new Map();
  function field(key, make) {
    if (!fields.has(key)) fields.set(key, make());
    return fields.get(key);
  }

  // act sends what one form holds, holding its buttons while it goes; on
  // success it lets go of that form's fields only, says what came of it,
  // and shows the view again.
  async function act(send, buttons, keys) {
    for (const b of buttons) b.disabled = true;
    try {
      const out = await send();
      for (const k of keys) fields.delete(k);
      B.said(out && out.said);
      await B.show();
    } catch (err) { B.trouble(err); }
    finally { for (const b of buttons) b.disabled = false; }
  }

  // paged draws items into box, n at a time, with More for the rest; with
  // a key, how far More went is kept.
  function paged(box, items, n, draw, key) {
    let shown = 0;
    const left = B.el("span", {class: "faint mono"});
    const more = B.el("button", {class: "btn ghost sm rq-more", type: "button"}, "More", left);
    const next = (upto) => {
      for (const it of items.slice(shown, upto)) box.append(draw(it));
      shown = Math.min(upto, items.length);
      if (key) kept.more[key] = shown;
      more.hidden = shown >= items.length;
      left.textContent = String(items.length - shown);
    };
    more.addEventListener("click", () => next(shown + n));
    next(Math.max(n, key ? kept.more[key] || 0 : 0));
    return [box, more];
  }

  // few draws the first of a long list, and +N for the rest, drawn when
  // asked.
  function few(list, draw, comma) {
    const one = (x, i) => [i && comma ? B.el("span", {class: "faint"}, ",") : null, draw(x)];
    if (list.length <= FEW + 1) return list.map(one);
    const rest = list.slice(FEW);
    const more = B.el("button", {class: "btn ghost sm rq-few", type: "button", "aria-label": rest.length + " more"}, "+" + rest.length);
    more.addEventListener("click", () => more.replaceWith(...rest.map((x, i) => one(x, FEW + i)).flat().filter(Boolean)));
    return [list.slice(0, FEW).map(one), more];
  }

  function word(w, team, cls) {
    return B.el("span", {class: "rq-word " + (cls || "")},
      B.deciderEl(w, team),
      w.note ? B.el("span", {class: "rq-note"}, w.note) : null,
      B.whenEl(w.at));
  }

  // chain gives a decision's deciders in order: to its team's people and
  // those asked it, its walk, each who referred it with their note, whoever
  // ended it, and those it waits on marked; to anyone else, whoever ended
  // it, or those it waits on, the one looking first.
  function chain(d) {
    if (d.stages && d.stages.length) return walked(d);
    const links = (d.referrals || []).map((w) => word(w, d.team, "past"));
    if (d.ended && d.by) links.push(word(d.by, d.team, "rq-by"));
    else if (!d.ended && d.asked && d.asked.length) {
      const asked = d.asked.filter(B.isMe).concat(d.asked.filter((a) => !B.isMe(a)));
      links.push(B.el("span", {class: "rq-asked"}, few(asked, B.personEl)));
    }
    if (!links.length) return null;
    return B.el("span", {class: "chain"}, links.map((l, i) => [i ? B.el("span", {class: "then"}, "→") : null, l]));
  }

  // walked gives a decision's walk: each decider that referred it faint,
  // with its note; whoever ended it, with theirs; each asked now marked.
  function walked(d) {
    const same = (w, s) => w && w.kind === s.kind && w.name === s.name;
    return B.el("span", {class: "chain"}, d.stages.map((s, i) => {
      const referred = (d.referrals || []).find((w) => same(w, s));
      const by = d.ended && same(d.by, s);
      const now = !d.ended && s.kind === "person" && (d.asked || []).includes(s.name);
      const shown = referred || by ? word(referred || d.by, d.team, by ? "rq-by" : "past")
        : B.el("span", {class: "rq-word"}, B.deciderEl(s, d.team));
      return [i ? B.el("span", {class: "then", "aria-hidden": "true"}, "›") : null, shown,
        now ? B.el("span", {class: "rq-now", "aria-label": "asked now"}, "◂") : null];
    }));
  }

  function ending(d) {
    if (!d.ended) return B.el("span", {class: "status waiting"}, "waiting");
    const cls = d.ended === "refused" ? " failed" : d.ended === "withdrawn" ? " rq-gone" : "";
    return B.el("span", {class: "status" + cls}, ENDED[d.ended] || d.ended);
  }

  // decisionCard is a decision put to the one looking, to approve or
  // refuse; request needs only its number.
  B.decisionCard = function (request, d) {
    const n = request.number;
    const key = n + ":note:" + d.id;
    const note = field(key, () => B.el("input", {class: "in", type: "text", name: "note"}));
    const yes = B.el("button", {class: "btn go", type: "button"}, "Approve");
    const no = B.el("button", {class: "btn no", type: "button"}, "Refuse");
    const say = (says) => act(() => B.post("/api/requests/" + n + "/decisions/" + d.id, {says, note: note.value}), [yes, no], [key]);
    yes.addEventListener("click", () => say("yes"));
    no.addEventListener("click", () => say("refuse"));
    return B.el("section", {class: "ask-card rq-card"},
      B.el("div", {class: "line"}, B.el("span", {class: "rq-kind"}, B.decisionWord(d.kind)), B.teamEl(d.team)),
      B.el("div", {class: "what"}, d.does),
      d.leaving ? leaving(d.leaving) : null,
      d.kind === "action" && args(d.args) ? B.el("div", {class: "mono small"}, args(d.args)) : null,
      chain(d),
      B.el("div", {class: "act"}, B.field("Note", note), yes, no));
  };

  // leaving gives what a disclosure would let out: its text, whom it is
  // to, where it may in the end be read, and the requests it goes through
  // from the top, the one a sub-request would make last.
  function leaving(l) {
    const through = B.el("span", {class: "chain rq-through"}, (l.through || []).map((t, i) => [
      i ? B.el("span", {class: "then", "aria-hidden": "true"}, "›") : null,
      B.el("span", {class: "rq-pill"}, B.teamCalled(t.team), t.number ? B.el("span", {class: "mono faint"}, "#" + t.number) : B.el("span", {class: "tag"}, "New"))]));
    return [
      B.el("div", {class: "rq-leaving"}, l.text),
      B.kv([
        [WHAT[l.what] || l.what, through],
        ["To", B.party(l.to)],
        [B.el("span", {class: "line"}, "Read", l.what === "sub" ? B.info("About Read", "What leaves may in the end be read here: the team asked is not bound by this team's disclosure.") : null), B.way(l.read_at)],
      ]),
    ];
  }

  // replyForm answers a question put to the one looking.
  B.replyForm = function (request, entry) {
    const key = request.number + ":reply:" + entry.id;
    const text = field(key, () => B.el("textarea", {class: "in rq-reply", name: "text", rows: 2, required: true}));
    const button = B.el("button", {class: "btn go", type: "submit"}, "Answer");
    return B.el("form", {class: "reply", onsubmit: (ev) => {
      ev.preventDefault();
      if (!text.value.trim()) return;
      act(() => B.post("/api/requests/" + request.number + "/entries", {text: text.value, re: entry.id}), [button], [key]);
    }}, B.field("Answer", text), button);
  };

  // The thread

  function where(i) {
    if (!i) return null;
    if (i.bridge) return B.el("span", {class: "who rq-where"}, B.icon("bridge"), i.bridge, i.place ? " " + i.place : "");
    if (i.screens) return B.el("span", {class: "rq-where"}, "screens");
    return null;
  }

  const bad = (c) => /fail|refus|gone/.test(c.state || "");

  function carried(c) {
    const to = c.person ? B.personCalled(c.person) : c.place || "";
    return B.el("span", {class: "rq-carried bad"},
      c.bridge + (to ? " → " + to : "") + ": " + c.state + (c.why ? " (" + c.why + ")" : ""));
  }

  // who gives someone an entry is by or to, a worker of the request's own
  // team by its name alone.
  function who(p, team) {
    return p && p.kind === "worker" && p.team === team ? B.workerEl(p.name, team, {bare: true}) : B.party(p);
  }

  // waitsOn gives the key of someone an entry is to, as a request names
  // whom it waits on.
  function waitsOn(w) { return w.kind === "person" ? "p:" + w.name : w.kind === "guest" ? "g:" + w.id : ""; }

  // union gives the entries of both, each once, the later given winning,
  // in the order they were written.
  function union(a, b) {
    const byId = new Map();
    for (const e of a.concat(b)) byId.set(e.id, e);
    return [...byId.values()].sort((x, y) => x.id - y.id);
  }

  // thread takes what was fetched of the thread with what was kept of it:
  // when the latest page reaches what was kept, the two are one, as far
  // back as either went.
  function thread(v) {
    const fresh = v.entries || [];
    const before = v.before || 0;
    if (before && kept.entries.some((e) => e.id === before)) {
      v.entries = union(kept.entries, fresh);
      v.before = kept.before ? Math.min(kept.before, before) : 0;
    } else {
      v.entries = fresh;
      v.before = before;
    }
    kept.entries = v.entries;
    kept.before = v.before;
  }

  // earlier fetches the entries before the gap, and draws the page again.
  async function earlier(v, button) {
    button.disabled = true;
    try {
      const back = await B.api("/api/requests/" + v.number + "?before=" + v.before);
      kept.entries = union(back.entries || [], kept.entries);
      kept.before = back.before || 0;
      await B.show();
    } catch (err) {
      button.disabled = false;
      B.trouble(err);
    }
  }

  function compose(v) {
    const tkey = v.number + ":say", fkey = v.number + ":files";
    const text = field(tkey, () => B.el("textarea", {class: "in", name: "text"}));
    const files = field(fkey, () => B.el("input", {class: "in", type: "file", name: "files", multiple: true}));
    const button = B.el("button", {class: "btn go", type: "submit"}, "Send");
    return B.el("form", {class: "rq-compose", onsubmit: (ev) => {
      ev.preventDefault();
      if (!text.value.trim() && !files.files.length) return;
      act(() => B.send("/api/requests/" + v.number + "/entries", {text: text.value}, files), [button], [tkey, fkey]);
    }},
      B.field("Say more", text),
      B.el("div", {class: "rq-send"}, B.field("Files", files), button));
  }

  // What was read

  // args gives what a call was given, as name: value, of its plain values.
  function args(a) {
    if (a === undefined || a === null) return "";
    if (typeof a !== "object") return String(a);
    return Object.entries(a).filter(([, x]) => x === null || typeof x !== "object").map(([k, x]) => k + ": " + x).join(", ");
  }

  // gist gives what a record's event holds, in a line.
  function gist(x) {
    if (x === undefined || x === null) return "";
    if (typeof x !== "object") return String(x);
    if (typeof x.said === "string" && x.said) return x.said;
    if (typeof x.text === "string" && x.text) return x.text;
    if (Array.isArray(x.calls) && x.calls.length) return x.calls.map((c) => c.name + " " + args(c.args)).join(" · ");
    if (Array.isArray(x.jobs) && x.jobs.length) return x.jobs.map((j) => (j.name || "") + (j.asked && j.asked.does ? " · " + j.asked.does : "")).join(", ");
    if (x.error) return String(x.error.message || x.error);
    return Object.values(x).find((y) => typeof y === "string" && y) || "";
  }

  // lines gives what a record holds, a line for each thing done: the steps
  // of the run and of those it started, and the model's words joined.
  function lines(rec) {
    const out = [];
    let words = null;
    for (const ev of Array.isArray(rec.events) ? rec.events : []) {
      const nest = Boolean(ev.id && rec.run && ev.id !== rec.run);
      if (ev.step === undefined && ev.name !== undefined) {
        if (ev.name === "text" && words && words.id === ev.id && words.name === (ev.job || "")) { words.text += gist(ev.data); continue; }
        const l = {id: ev.id, at: ev.time, kind: ev.name === "text" ? "words" : ev.name, name: ev.job || "", text: gist(ev.data), nest};
        words = ev.name === "text" ? l : null;
        out.push(l);
        continue;
      }
      words = null;
      for (const e of ev.events || []) out.push({at: ev.time, kind: e.name, name: e.from || "", text: gist(e.data), nest});
      for (const a of ev.asks || []) {
        if (a.kind === "receipt" || a.kind === "wake") continue;
        const job = a.job || {};
        const text = a.kind === "hold" && job.shown ? job.shown.does + " · " + args(job.shown.args) : a.kind === "script" ? "" : gist(job.input);
        out.push({at: ev.time, kind: a.kind, name: a.name || a.what || "", text, nest});
      }
      if (ev.status === "failed" || ev.status === "cancelled") out.push({at: ev.time, kind: ev.status, name: "", text: "", nest});
    }
    return out;
  }

  function line(l) {
    return B.el("div", {class: "row" + (l.nest ? " nest" : "") + (l.kind === "words" ? " words" : "") + (l.kind === "failed" ? " failed" : "")},
      B.el("span", {class: "t"}, B.when(l.at)),
      B.el("span", {class: "k"}, l.kind),
      B.el("span", {class: "m"}, l.name),
      B.el("span", {class: "x", title: l.text.length > 160 ? clip(l.text, 2000) : null}, clip(l.text, 160)));
  }

  // record is what one run read, sealed and folded, drawn when it is
  // opened.
  function record(rec) {
    const label = [B.icon("lock"), "Read · " + B.teamCalled(rec.team)];
    if (rec.gone) return B.el("div", {class: "rq-read"}, label, B.el("span", {class: "faint"}, "· Corbie keeps this record no longer"));
    const body = B.el("div", {class: "events rows"});
    const box = B.el("details", {class: "rq-record"}, B.el("summary", {class: "rq-read"}, label), body);
    let built = false;
    const build = () => {
      if (built) return;
      built = true;
      const all = lines(rec);
      if (!all.length) { body.append(B.empty()); return; }
      const [, more] = paged(body, all, 100, line);
      box.append(more);
    };
    box.addEventListener("toggle", () => {
      if (box.open) { kept.records.add(rec.run); build(); } else kept.records.delete(rec.run);
    });
    if (kept.records.has(rec.run)) { build(); box.open = true; }
    return box;
  }

  const RUN = {going: "working live", failed: "failed", cancelled: "rq-gone", lost: "failed", unsent: "failed"};
  const ENDS = {going: "working", done: "done", failed: "failed", cancelled: "cancelled", lost: "lost", unsent: "unsent"};
  const HEED = {act: "act", note: "noted"};

  const REPEATS = /(\.[0-9]+)?(#[0-9]+)?(@[0-9]+)?$/;

  // calls gives what a run's record shows it calling, each when it was
  // called: its colleagues, each by the tool it was called as and, when it
  // is a worker of the team, its worker, with how its run ended; and its
  // own tool scripts; each colleague with what it called in turn. Bulldog's
  // own machines, the ask and the calls among them, are not colleagues: what
  // they do is in the thread. Corbie's own scripts are not tools. A
  // machine's path is written from its caller's folder, and it is a worker
  // when it lies in the team's folder itself, where a worker's run starts;
  // of any other run the folder is not known.
  function calls(rec, folder) {
    const under = {}, ended = {};
    for (const ev of rec && Array.isArray(rec.events) ? rec.events : []) {
      if (ev.name !== undefined) continue;
      if (ev.status && ev.status !== "running") ended[ev.id] = ev.status;
      for (const a of ev.asks || []) {
        const job = a.job || {};
        const machine = job.machine || "";
        const at = Date.parse(ev.time) || 0;
        if (a.kind === "child" && a.child && !/(^|\/)\.bulldog\//.test(machine)) {
          (under[ev.id] = under[ev.id] || []).push({id: a.child, at, tool: a.name.replace(REPEATS, ""), machine});
        } else if (a.kind === "script" && job.script && !/^corbie:/.test(job.script)) {
          (under[ev.id] = under[ev.id] || []).push({at, tool: a.name.replace(REPEATS, ""), script: job.script, given: args(job.input)});
        }
      }
    }
    const tree = (id, from) => (under[id] || []).map(({machine, ...c}) => {
      if (c.script) return c;
      const at = from === null ? null : within(from, machine);
      const worker = at !== null && /^[^/]+\.ya?ml$/.test(at) ? at.replace(/\.ya?ml$/, "") : "";
      const next = at === null ? from : at.includes("/") ? at.slice(0, at.lastIndexOf("/")) : "";
      return {...c, worker, status: ended[c.id] || "going", below: tree(c.id, next)};
    });
    return tree(rec ? rec.run : "", folder);
  }

  // within gives a machine's path, as its caller names it from its own
  // folder, from the team's folder; or null when it is no path, as a
  // machine written in place is not.
  function within(from, machine) {
    if (!/^\.\.?\//.test(machine)) return null;
    const parts = [];
    for (const s of (from + "/" + machine).split("/")) {
      if (s === "" || s === ".") continue;
      if (s === ".." && parts.length && parts[parts.length - 1] !== "..") parts.pop();
      else parts.push(s);
    }
    return parts.join("/");
  }

  // callName gives a call by its tool and, when they differ, the worker it
  // reached.
  function callName(c, team) {
    if (c.script) return [B.el("span", {class: "mono"}, c.tool), c.given ? B.el("span", {class: "mono faint rq-given", title: c.given}, clip(c.given, 60)) : null];
    return [c.worker !== c.tool ? B.el("span", {class: "mono"}, c.tool) : null,
      c.worker && c.worker !== c.tool ? B.el("span", {class: "faint", "aria-hidden": "true"}, "›") : null,
      c.worker ? B.workerEl(c.worker, team, {bare: true}) : null];
  }

  function callEnd(c) {
    return c.script ? null : B.el("span", {class: "status " + (RUN[c.status] || "")}, ENDS[c.status] || c.status);
  }

  // callsEl gives what a colleague called in turn, as a tree.
  function callsEl(list, team) {
    if (!list || !list.length) return null;
    return B.el("ul", {class: "rq-calls"}, list.map((c) => B.el("li", {},
      B.el("div", {class: "line"}, B.el("span", {class: "rq-chip" + (c.script ? "" : " solid")}, c.script ? "Tool" : "Colleague"),
        callName(c, team), callEnd(c)),
      callsEl(c.below, team))));
  }

  // Steps

  // A step is one thing in the trace: its key, kept across drawings; when
  // it happened, in ms, and the entry it stands at, for its order; how far
  // in it is set; its kind's word, dashed when it is one of Bulldog's
  // checks; whether it goes on now or waits on someone; what it is, what
  // was said in it, how it stands and what is beneath it.
  function step(s) { return Object.assign({at: 0, entry: Number.MAX_SAFE_INTEGER, depth: 0, kind: "", check: false, now: "", what: null, said: null, meta: null, more: null}, s); }

  const DEPTH = {asked: 0, said: 0, answer: 0, held: 0, late: 0, bulldog: 0, notice: 0};
  const FOLDED = new Set(["decision", "decide", "decided", "referred"]);
  const KIND_ABOUT = {
    Asked: "What was asked, first in every request.",
    Said: "More said in the request. It makes an answered request working again; noted by the team's heed, it starts nothing.",
    Question: "A worker's question to a person. It waits on them until they reply.",
    Told: "A worker telling someone something. Nothing waits on it.",
    Reply: "A person's reply to what was put to them. It ends their wait.",
    "Sub-request": "A worker asking another team, which makes a request of its own. The worker waits on its answer.",
    "Late answer": "A sub-request's answer that came when no run waited for it. It starts the lead again.",
    Held: "The lead's answer, held by the team's disclosure. It is said once it is disclosed.",
    Authorisation: "A held action's decision. The action waits on it.",
    Disclosure: "Asked before anything leaves the team. What would leave waits on it.",
    Change: "A change to the office files. The change waits on it.",
    Heed: "Asked first about what is said at a channel place. Act: the lead starts. Note: kept in the request; nothing runs or is said.",
    Colleague: "A worker of the team, used as a tool.",
    Tool: "One of the team's own tools.",
  };

  // steps gives the steps of one request as the one looking sees it, in
  // the order they happened, each sub-request's own steps right after the
  // step that asked it, drawn from what was fetched of it, views.
  function steps(v, views, depth, prefix) {
    const at = (t) => Date.parse(t) || 0;
    const entries = v.entries || [];
    const decisions = v.decisions || [];
    const waits = new Set(v.waits_on_me || []);
    const answered = new Set(entries.map((e) => e.answers).filter(Boolean));
    const on = new Set((v.on || []).map(waitsOn).filter(Boolean));
    const noted = new Map(entries.filter((e) => e.kind === "noted").map((e) => [e.answers, e]));
    const working = v.state === "working";
    const out = [];
    const key = (k) => prefix + k;

    // One step for each decision, where it was first written in the thread,
    // holding the entries the thread wrote of it.
    const ofDecision = new Map(decisions.map((d) => [d.id, []]));
    for (const e of entries) if (FOLDED.has(e.kind) && ofDecision.has(e.decision)) ofDecision.get(e.decision).push(e);

    for (const e of entries) {
      if (e.kind === "noted" && entries.some((m) => m.id === e.answers)) continue;
      if (FOLDED.has(e.kind) && ofDecision.has(e.decision)) continue;
      if (depth > 0 && e.kind === "asked") continue; // the step that asked it says it
      const mine = waits.has(e.id);
      const open = (e.kind === "question" || e.kind === "decide") && (mine || (!answered.has(e.id) && v.state === "waiting" && (e.to || []).some((w) => on.has(waitsOn(w)))));
      const heed = noted.get(e.id);
      const isAnswer = v.answer && v.answer.entry === e.id;
      const to = e.to || [];
      const s = step({key: key("e" + e.id), at: at(e.at), entry: e.id, depth: depth + (e.kind in DEPTH ? DEPTH[e.kind] : 1),
        kind: B.entryLabel(decisions, e) || "Bulldog", check: e.kind === "held", now: open ? "waiting" : "",
        what: [who(e.by, v.team), isAnswer ? [arrow(), answerTo(v)] : to.length ? [arrow(), few(to, (p) => who(p, v.team), true)] : null, where(e.in),
          e.noted ? B.el("span", {class: "status noted"}, "noted") : null],
        said: e.text || null,
        meta: [isAnswer ? answerMeta(v) : null,
          heed ? B.el("span", {class: "rq-heed"}, B.icon("lock"), heed.by && heed.by.kind === "rule" ? B.ruleShortEl(heed.by.name) : B.party(heed.by), B.el("span", {class: "rq-note"}, heed.text)) : null,
          (e.carried || []).filter(bad).map(carried)],
        more: [
          e.files && e.files.length ? B.el("div", {class: "files"}, e.files.map((f) =>
            B.el("a", {class: "who rq-file", href: f.url}, B.icon("file"), f.name, B.el("span", {class: "faint"}, B.size(f.size))))) : null,
          e.kind === "question" && mine ? B.replyForm(v, e) : null]});
      if (e.kind === "sub" && e.sub) {
        out.push(subStep(s, e, v, views, prefix));
        continue;
      }
      out.push(s);
    }

    for (const d of decisions) out.push(decisionStep(v, d, ofDecision.get(d.id), depth, key));

    if (v.read) {
      const recs = new Map((v.records || []).map((r) => [r.run, r]));
      const leads = (v.runs || []).filter((r) => r.kind === "work");
      for (const r of v.runs || []) {
        if (r.kind === "decide") continue; // beneath its decision
        const rec = recs.get(r.name);
        const going = r.status === "going";
        if (r.kind === "heed") {
          out.push(step({key: key("run:" + r.name), at: at(r.started), depth, kind: "Heed", check: true, now: going && working ? "working" : "",
            what: [B.ruleShortEl(r.worker), B.el("span", {class: "status " + (HEED[r.says] || RUN[r.status] || "")}, r.says || ENDS[r.status] || r.status)],
            meta: r.how ? B.el("span", {class: "rq-note"}, r.how) : null,
            more: rec ? record(rec) : null}));
          continue;
        }
        const again = leads.indexOf(r) > 0;
        out.push(step({key: key("run:" + r.name), at: at(r.started), depth, kind: "Team", now: going && working ? "working" : "",
          what: [r.worker ? B.workerEl(r.worker, r.team, {bare: true}) : B.el("span", {class: "mono"}, r.name), again ? "again" : "starts on it"],
          meta: runMeta(r),
          more: rec ? record(rec) : null}));
        for (const c of calls(rec, "")) {
          out.push(step({key: key("call:" + (c.id || r.name + ":" + c.tool + ":" + c.at)), at: c.at, depth: depth + 1,
            kind: c.script ? "Tool" : "Colleague", now: !c.script && c.status === "going" && working ? "working" : "",
            what: callName(c, r.team), meta: callEnd(c), more: callsEl(c.below, r.team)}));
        }
      }
    }

    // What came from the thread and the runs, in the order it happened;
    // what has no time of its own, a decision not written in the part of
    // the thread given, last but the answer.
    const order = (s) => s.at || Infinity;
    out.sort((a, b) => order(a) - order(b) || a.entry - b.entry);

    const tail = tailStep(v, depth, key);
    if (tail) out.push(tail);

    // Each sub-request's steps right after the one that asked it.
    return out.flatMap((s) => [s, ...(s.inside || [])]);
  }

  // runMeta gives how a run went: when it started and ended, how it
  // stands, how much it said, and where it is watched.
  function runMeta(r) {
    return [B.el("span", {class: "status " + (RUN[r.status] || "")}, ENDS[r.status] || r.status),
      B.el("span", {class: "mono faint"}, B.when(r.started) + "–" + (r.ended ? B.when(r.ended) : "")),
      r.tokens ? B.el("span", {class: "mono faint"}, r.tokens + " tok") : null,
      B.el("a", {class: "rq-jump", href: "#/run/" + encodeURIComponent(r.name), "aria-label": "Watch " + (r.worker || r.name)}, "Watch")];
  }

  // decisionStep gives one decision where the thread first wrote it: what
  // it is and how it stands, who decides it and who did, the rule runs
  // that decided it, what the thread said of it, and its card when it is
  // put to the one looking.
  function decisionStep(v, d, said, depth, key) {
    const anchor = said[0];
    const runs = (v.runs || []).filter((r) => r.kind === "decide" && deciderOf(d, r.worker));
    const recs = new Map((v.records || []).map((r) => [r.run, r]));
    const why = d.why && !(d.by && d.by.note === d.why) ? d.why : null;
    return step({key: key("dec:" + d.id), at: anchor ? Date.parse(anchor.at) || 0 : 0, entry: anchor ? anchor.id : Infinity, depth: depth + 1,
      kind: B.decisionWord(d.kind), check: true, now: d.ended ? "" : "waiting",
      what: d.does, meta: [ending(d), why ? B.el("span", {class: "mono small muted"}, why) : null],
      more: [chain(d),
        runs.map((r) => B.el("div", {class: "rq-ruled"}, B.el("div", {class: "line"}, B.ruleEl(r.worker), runMeta(r)), recs.has(r.name) ? record(recs.get(r.name)) : null)),
        said.length > 1 ? B.el("details", {class: "rq-said"}, B.el("summary", {class: "mono small faint"}, said.length + " in the thread"),
          B.el("div", {class: "stack"}, said.map((e) => B.el("div", {class: "line rq-said-line"},
            B.el("span", {class: "mono faint"}, B.when(e.at)), B.el("span", {class: "rq-chip"}, B.entryLabel(v.decisions, e)),
            e.by && e.by.kind !== "bulldog" ? B.party(e.by) : null, B.el("span", {class: "small"}, e.text))))) : null,
        d.mine && !d.ended ? B.decisionCard(v, d) : null]});
  }

  // deciderOf says whether a rule is among a decision's deciders.
  function deciderOf(d, rule) {
    const is = (w) => w && w.kind === "rule" && w.name === rule;
    return (d.stages || []).some(is) || (d.referrals || []).some(is) || is(d.by);
  }

  // subStep gives a sub-request where it was asked, with its own steps
  // inside it when they are fetched, or how it stands and its answer when
  // it is folded.
  function subStep(s, e, v, views, prefix) {
    const n = e.sub;
    const sum = (v.subs || []).find((x) => x.number === n) || {number: n};
    const sv = views.get(n);
    const open = Boolean(sv && !sv.error);
    const toggle = B.el("button", {class: "rq-fold", type: "button", "aria-expanded": String(open), "aria-label": (open ? "Fold" : "Open") + " #" + n},
      open ? "▾" : "▸");
    toggle.addEventListener("click", () => {
      if (open) { kept.shut.add(n); kept.open.delete(n); } else { kept.open.add(n); kept.shut.delete(n); }
      B.show();
    });
    const state = sv && !sv.error ? sv : sum;
    s.what = [who(e.by, v.team), arrow(), B.teamEl(sum.team || (e.to[0] && e.to[0].name)),
      B.el("a", {class: "mono", href: "#/r/" + n}, "#" + n), state.state ? B.status(state) : null, toggle];
    s.now = s.now || (state.state === "waiting" && !open ? "waiting" : "");
    if (open) {
      const inner = steps(sv, views, s.depth + 1, prefix + n + ":");
      if (inner.length) inner[0].group = [B.teamCalled(sv.team), " · ", B.el("a", {href: "#/r/" + n}, "#" + n),
        sv.before ? B.el("a", {class: "faint", href: "#/r/" + n}, " · earlier") : null];
      s.inside = inner;
    } else {
      s.more = [s.more, sv && sv.error ? B.el("div", {class: "mono small bad"}, sv.error.message) : null,
        sum.answer ? B.el("div", {class: "said rq-answered"}, sum.answer) : null];
    }
    return s;
  }

  // tailStep gives what the request waits on at its end that no step of
  // it shows: its turn, its lead who is a person, or its answer, where it
  // goes and how it stands.
  function tailStep(v, depth, key) {
    if (v.state === "working" && v.queued) {
      return step({key: key("queued"), at: Infinity, entry: Infinity, depth, kind: "Queued", now: "pending",
        what: B.status(v)});
    }
    const t = B.team(v.team);
    if (v.state === "waiting" && t && B.person(t.lead) && (v.on || []).some((p) => p.kind === "person" && p.name === t.lead)) {
      return step({key: key("lead"), at: Infinity, entry: Infinity, depth, kind: "Team", now: "waiting",
        what: [B.personEl(t.lead), "answers"]});
    }
    const a = v.answer;
    if (!a || a.entry || v.label === "noted") return null;
    return step({key: key("answer:" + a.state), at: Infinity, entry: Infinity, depth, kind: "Answer", now: a.state === "waits" ? "pending" : "",
      what: answerTo(v), meta: answerMeta(v)});
  }

  // answerTo gives where a request's answer goes: the run that asked a
  // sub-request, the place or the chat it was asked in, or the people of a
  // routine's team.
  function answerTo(v) {
    if (v.parent) return B.el("span", {class: "line"}, B.el("a", {class: "mono", href: "#/r/" + v.parent}, "#" + v.parent), B.el("span", {class: "faint"}, "›"), B.el("span", {class: "mono"}, "ask_team"));
    if (v.origin && v.origin.kind === "routine") return B.el("span", {class: "line"}, "People of", B.teamEl(v.origin.team));
    if (v.origin && v.origin.kind === "chat") return B.el("span", {class: "line"}, B.way(v.origin), B.el("span", {class: "faint"}, "›"), "screens");
    return B.way(v.origin);
  }

  const ANSWER = {waits: "waiting", held: "held", said: "", quiet: "quiet", "not delivered": "failed"};

  // answerMeta gives where an answer stands and, to the team's people,
  // the disclosure it needs: not needed when only the team reads it, or
  // who decides it.
  function answerMeta(v) {
    const a = v.answer;
    if (!a) return null;
    const d = v.disclosure;
    return [B.el("span", {class: "status " + ANSWER[a.state]}, a.state),
      a.why ? B.el("span", {class: "mono small muted"}, a.why) : null,
      d && d.set ? B.el("span", {class: "line rq-disclosure"}, B.el("span", {class: "rq-chip check"}, "Disclosure"),
        d.needed ? B.chain(d.deciders || [], v.team) : B.el("span", {class: "line"}, "not needed", B.info("About Not needed", "Read only by people of " + B.teamCalled(v.team) + "."))) : null];
  }

  // stepEl draws one step: its time beside the line, its node on it, and
  // what it is beside them; one coming in, as far as it came.
  function stepEl(s, prev, coming) {
    const time = s.at && s.at !== Infinity ? B.when(new Date(s.at).toISOString()) : "";
    const shown = time && time !== prev ? time : "";
    const cls = ["step", "d" + Math.min(s.depth, 3), s.now, coming >= 0 ? "new" : ""].filter(Boolean).join(" ");
    return B.el("div", {class: cls, "data-key": s.key, style: coming > 0 ? "animation-delay: -" + coming + "ms" : null},
      B.el("span", {class: "t"}, shown),
      B.el("span", {class: "line", "aria-hidden": "true"}),
      B.el("div", {class: "body"},
        s.group ? B.el("div", {class: "group"}, s.group) : null,
        B.el("div", {class: "hd"},
          B.el("span", {class: "rq-chip" + (s.check ? " check" : ""), title: KIND_ABOUT[s.kind] || null}, s.kind),
          B.el("span", {class: "what"}, s.what),
          shown ? B.el("span", {class: "at"}, shown) : null),
        s.said ? B.el("div", {class: "said"}, s.said) : null,
        s.meta && [s.meta].flat(Infinity).some(Boolean) ? B.el("div", {class: "meta"}, s.meta) : null,
        s.more && [s.more].flat(Infinity).some(Boolean) ? B.el("div", {class: "more"}, s.more) : null));
  }

  // flowEl draws the steps; only those not drawn before come in, and none
  // on the page's first drawing. The line goes on below the last while
  // anything in the request is at work.
  function flowEl(all, live, v) {
    const now = Date.now();
    let prev = "";
    const els = [];
    all.forEach((s, i) => {
      if (!kept.seen.has(s.key)) kept.seen.set(s.key, kept.drawn ? now : 0);
      const came = kept.seen.get(s.key);
      els.push(stepEl(s, prev, came && now - came < ENTER ? now - came : -1));
      if (s.at && s.at !== Infinity) prev = B.when(new Date(s.at).toISOString());
      if (i === 0 && v.before) {
        const button = B.el("button", {class: "btn ghost sm rq-earlier", type: "button"}, "Earlier");
        button.addEventListener("click", () => earlier(v, button));
        els.push(B.el("div", {class: "step gap"}, B.el("span", {class: "t"}), B.el("span", {class: "line", "aria-hidden": "true"}), B.el("div", {class: "body"}, button)));
      }
    });
    kept.drawn = true;
    kept.latest = Math.max(kept.latest, ...all.map((s) => s.at === Infinity ? 0 : s.at));
    if (live) els.push(B.el("div", {class: "step tail", "aria-hidden": "true"}, B.el("span", {class: "t"}), B.el("span", {class: "line"})));
    const flow = B.el("section", {class: "flow rq-flow" + (live ? " live" : ""), "aria-label": "What happened"}, els);
    flow.style.setProperty("--phase", -(Date.now() % BREATHE) + "ms");
    return flow;
  }

  // Live

  // The runs at work in the requests drawn, each followed while it goes,
  // so that what a colleague does shows as it is done; the page is drawn
  // again for what is newer than all it shows, at most once a second, as
  // any change draws it.
  const following = new Map();
  let soon = 0;
  function follow(names, n) {
    for (const [name, source] of following) {
      if (!names.has(name)) { source.close(); following.delete(name); }
    }
    for (const name of names) {
      if (following.has(name)) continue;
      const source = new EventSource("/api/runs/" + encodeURIComponent(name) + "/watch");
      const seen = (ev) => {
        let s = {};
        try { s = JSON.parse(ev.data); } catch (err) { /* nothing to place */ }
        if (ev.type === "ended") { source.close(); following.delete(name); }
        if (ev.type !== "ended" && !(Date.parse(s.at) > kept.latest)) return;
        if (soon) return;
        soon = setTimeout(() => { soon = 0; B.changed("request:" + n); }, 1000);
      };
      for (const kind of ["call", "result", "ended"]) source.addEventListener(kind, seen);
      following.set(name, source);
    }
  }
  addEventListener("hashchange", () => {
    for (const source of following.values()) source.close();
    following.clear();
  });

  // views fetches the sub-requests drawn inside a request: the first
  // levels, and those deeper opened by hand, not those folded.
  async function fetchSubs(v, level, views) {
    await Promise.all((v.subs || []).map(async (s) => {
      if (kept.shut.has(s.number) || (level >= DEEP && !kept.open.has(s.number))) return;
      try {
        const sv = await B.api("/api/requests/" + s.number);
        views.set(s.number, sv);
        await fetchSubs(sv, level + 1, views);
      } catch (err) { views.set(s.number, {error: err}); }
    }));
  }

  // labelInfo says what a request's Quiet or Noted means, behind its (i).
  function labelInfo(v) {
    const lead = B.leadCalled(B.team(v.team));
    if (v.state !== "answered") return null;
    if (v.label === "quiet") return B.info("About quiet", lead + " ran and answered with nothing, so nothing was said.");
    if (v.label === "noted") return B.info("About noted", "Kept in the request; " + lead + " did not start on it, and is given it in the thread when next asked.");
    return null;
  }

  // The view

  B.route(/^#\/r\/(\d+)$/, async function (m) {
    const v = await B.api("/api/requests/" + m[1]);
    const n = v.number;
    if (kept.n !== n) kept = keep(n);
    thread(v);
    const views = new Map();
    if (v.whole) await fetchSubs(v, 0, views);
    kept.tree = new Set(["request:" + n, ...[...views.keys()].map((k) => "request:" + k)]);

    const asked = v.entries.find((e) => e.kind === "asked");
    const first = asked ? firstLine(asked.text) : "";
    const all = steps(v, views, 0, "");
    const drawn = [v, ...[...views.values()].filter((x) => !x.error)];
    const live = drawn.some((x) => x.state === "working");
    follow(new Set(drawn.filter((x) => x.read && x.state === "working").flatMap((x) => (x.runs || []).filter((r) => r.kind === "work" && r.status === "going").map((r) => r.name))), n);

    const crumbs = [B.el("a", {href: "#/requests"}, "Requests"), v.team ? B.el("a", {href: "#/requests?team=" + encodeURIComponent(v.team)}, B.teamCalled(v.team)) : null].filter(Boolean);
    const title = [B.el("span", {class: "rq-num"}, "#" + n), first ? B.el("span", {title: first.length > 90 ? first : null}, clip(first, 90)) : null];
    const stop = v.whole && v.state !== "answered" ? B.el("button", {class: "btn ghost sm", type: "button"}, "Stop") : null;
    if (stop) stop.addEventListener("click", () => act(() => B.post("/api/requests/" + n + "/stop"), [stop], []));
    const dot = () => B.el("span", {class: "faint", "aria-hidden": "true"}, "·");
    const source = v.source ? [v.source.link ? B.el("a", {class: "mono", href: v.source.link, rel: "noopener noreferrer", target: "_blank"}, v.source.status) : B.el("span", {class: "mono"}, v.source.status),
      B.whenEl(v.source.at)] : null;
    const meta = v.whole ? B.el("div", {class: "rq-meta"},
      v.parent ? B.el("span", {class: "who"}, B.icon("branch"), "inside ", B.el("a", {class: "mono", href: "#/r/" + v.parent}, "#" + v.parent))
        : [B.party(v.asker), dot(), B.way(v.origin)],
      dot(), B.teamEl(v.team),
      v.state ? [dot(), B.status(v), labelInfo(v)] : null,
      source ? [dot(), source] : null,
      dot(), B.whenEl(v.made),
      stop) : null;

    return B.el("div", {class: "rq"},
      B.head(crumbs, title),
      meta,
      v.whole ? null : B.label("Put to you"),
      all.length || v.before ? flowEl(all, live, v) : B.empty(),
      compose(v));
  }, "requests", (what, m) => what === "office" || what === "request:" + m[1] || kept.tree.has(what));
})();
