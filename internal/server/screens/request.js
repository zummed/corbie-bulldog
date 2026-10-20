// One request: its thread, what waits on the one looking in it, and its
// way in bands: In, where it was asked, by whom, and its heed; Team, its
// team's lead and runs; Out, the teams and people it asked, its
// decisions, its disclosure and where its answer goes; then what its team
// read. The desk draws a decision and an answer to a question with the
// same parts.
(function () {
  "use strict";

  const KIND = {action: "Authorisation", disclosure: "Disclosure", change: "Change"};
  const ENDED = {yes: "approved", refused: "refused", withdrawn: "withdrawn"};
  const WHAT = {answer: "Answer", question: "Question", tell: "Tell", sub: "Sub-request"};
  const FEW = 3; // of a long list of people, how many are drawn before +N

  // clip gives a text on one line, no longer than n.
  function clip(text, n) {
    const s = String(text === undefined || text === null ? "" : text).replace(/\s+/g, " ").trim();
    return s.length > n ? s.slice(0, n - 1) + "…" : s;
  }

  function firstLine(text) { return String(text || "").split("\n")[0]; }

  function arrow() { return B.el("span", {class: "rq-arrow", role: "img", "aria-label": "to"}, "→"); }

  // kept is what the one looking did on a request's page, so that drawing
  // it again at a change keeps it: the thread as far back as Earlier
  // reached, how far each More went, the sub-requests opened and the
  // records opened.
  let kept = keep(0);
  function keep(n) { return {n, entries: [], before: 0, more: {}, open: new Set(), records: new Set()}; }

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
      B.el("div", {class: "line"}, B.el("span", {class: "rq-kind"}, KIND[d.kind] || d.kind), B.teamEl(d.team)),
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
    return B.el("span", {class: "rq-carried" + (bad(c) ? " bad" : "")},
      c.bridge + (to ? " → " + to : "") + ": " + c.state + (c.why ? " (" + c.why + ")" : ""));
  }

  // foot gives where an entry was carried: a line for each, or, when it
  // went to many, a count for each bridge and state, and each that failed.
  function foot(cs) {
    if (cs.length <= FEW + 1) return cs.map(carried);
    const counts = new Map();
    for (const c of cs.filter((c) => !bad(c))) {
      const k = c.bridge + ": " + c.state;
      counts.set(k, (counts.get(k) || 0) + 1);
    }
    return [[...counts].map(([k, n]) => B.el("span", {class: "rq-carried"}, k + " ×" + n)), few(cs.filter(bad), carried)];
  }

  // waitsOn gives the key of someone an entry is to, as a request names
  // whom it waits on.
  function waitsOn(w) { return w.kind === "person" ? "p:" + w.name : w.kind === "guest" ? "g:" + w.id : ""; }

  // entry draws one entry of a thread. A question or a decision put to
  // people is open while nothing answers it and the request waits on one
  // of those it was put to.
  function entry(v, e, state) {
    const mine = state.waits.has(e.id);
    const heed = state.noted.get(e.id);
    const putTo = (e.to || []).some((w) => state.on.has(waitsOn(w)));
    const open = (e.kind === "question" || e.kind === "decide") && (mine || (!state.answered.has(e.id) && v.state === "waiting" && putTo));
    const cls = ["entry", e.kind,
      e.kind === "told" ? "quiet" : "",
      e.read ? "read" : "",
      open ? "open" : "",
      mine ? "waits" : ""].filter(Boolean).join(" ");
    const to = e.to || [];
    const div = B.el("article", {class: cls},
      B.el("div", {class: "hd"},
        B.el("span", {class: "k"}, e.kind === "sub" ? "sub-request" : e.kind),
        B.party(e.by),
        to.length ? [arrow(), few(to, B.party, true)] : null,
        where(e.in),
        e.read ? B.el("span", {class: "rq-read"}, B.icon("lock"), "Read · " + B.teamCalled(e.read)) : null,
        e.noted ? B.el("span", {class: "tag"}, "Noted") : null,
        B.el("span", {class: "at"}, B.whenEl(e.at))),
      e.kind === "sub" && e.sub ? B.el("div", {class: "rq-branch"}, B.icon("branch"), B.el("a", {class: "mono", href: "#/r/" + e.sub}, "#" + e.sub)) : null,
      e.text ? B.el("div", {class: "body"}, e.text) : null,
      e.files && e.files.length ? B.el("div", {class: "files"}, e.files.map((f) =>
        B.el("a", {class: "who rq-file", href: f.url}, B.icon("file"), f.name, B.el("span", {class: "faint"}, B.size(f.size))))) : null,
      e.carried && e.carried.length ? B.el("div", {class: "foot"}, foot(e.carried)) : null,
      heed ? B.el("div", {class: "rq-heed"}, B.icon("lock"), "Noted", B.el("span", {class: "faint"}, "·"), B.party(heed.by),
        B.el("span", {class: "faint"}, "·"), B.el("span", {class: "rq-note"}, heed.text)) : null,
      e.kind === "question" && mine ? B.replyForm(v, e) : null);
    div.dataset.id = e.id;
    return div;
  }

  // union gives the entries of both, each once, the later given winning,
  // in the order they were written.
  function union(a, b) {
    const byId = new Map();
    for (const e of a.concat(b)) byId.set(e.id, e);
    return [...byId.values()].sort((x, y) => x.id - y.id);
  }

  // fill draws the thread's entries, with Earlier where some are not
  // given: after the first entry and any earlier one still open.
  function fill(box, v) {
    const state = {
      waits: new Set(v.waits_on_me || []),
      answered: new Set(v.entries.map((e) => e.answers).filter(Boolean)),
      on: new Set((v.on || []).map(waitsOn).filter(Boolean)),
      noted: new Map(v.entries.filter((e) => e.kind === "noted").map((e) => [e.answers, e])),
    };
    const shown = v.entries.filter((e) => e.kind !== "noted" || !v.entries.some((m) => m.id === e.answers));
    const drawn = shown.map((e) => entry(v, e, state));
    if (v.before) {
      const button = B.el("button", {class: "btn ghost sm rq-earlier", type: "button"}, "Earlier");
      button.addEventListener("click", () => earlier(box, v, button));
      const at = shown.findIndex((e) => e.id >= v.before);
      drawn.splice(at < 0 ? drawn.length : at, 0, button);
    }
    box.replaceChildren(...drawn);
  }

  // earlier fetches the entries before the gap, and draws the thread again.
  async function earlier(box, v, button) {
    button.disabled = true;
    try {
      const back = await B.api("/api/requests/" + v.number + "?before=" + v.before);
      v.entries = union(back.entries || [], v.entries);
      v.before = back.before || 0;
      if (kept.n === v.number) { kept.entries = v.entries; kept.before = v.before; }
      fill(box, v);
    } catch (err) {
      button.disabled = false;
      B.trouble(err);
    }
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
    const box = B.el("div", {class: "thread"});
    fill(box, v);
    return box;
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

  // The side

  // subTree draws sub-requests beneath each other; those deeper than the
  // first two levels open on asking, and are drawn then.
  function subTree(list, depth) {
    const ul = B.el("ul", {class: "tree rq-subs"});
    const draw = (s) => {
      const has = Boolean(s.subs && s.subs.length);
      const toggle = has && depth > 0
        ? B.el("button", {class: "toggle", type: "button", "aria-expanded": "false", "aria-label": "Sub-requests of #" + s.number}, "▸") : null;
      const li = B.el("li", {},
        B.el("div", {class: "rq-sub"},
          toggle,
          B.el("a", {class: "mono", href: "#/r/" + s.number}, "#" + s.number),
          B.teamEl(s.team),
          B.status(s)),
        s.question ? B.el("div", {class: "rq-q", title: s.question}, clip(s.question, 140)) : null,
        s.decisions && s.decisions.length ? B.el("div", {class: "rq-decs"}, s.decisions.map(authorisation)) : null,
        s.answer ? B.el("div", {class: "rq-a"}, s.answer) : null);
      let kids = null;
      const show = (on) => {
        if (on && !kids) { kids = subTree(s.subs, depth + 1); li.append(kids); }
        if (kids) kids.hidden = !on;
        if (toggle) {
          toggle.setAttribute("aria-expanded", String(on));
          toggle.textContent = on ? "▾" : "▸";
        }
      };
      if (toggle) toggle.addEventListener("click", () => {
        const on = !kept.open.has(s.number);
        if (on) kept.open.add(s.number); else kept.open.delete(s.number);
        show(on);
      });
      if (has && (depth === 0 || kept.open.has(s.number))) show(true);
      return li;
    };
    return B.el("div", {}, paged(ul, list, 20, draw, depth === 0 ? "subs" : null));
  }

  // authorisation gives one decision: its kind, unless the row it is
  // under names it, what it is and how it stands, and its walk or who
  // ended it.
  function authorisation(d, kindless) {
    const why = d.why && !(d.by && d.by.note === d.why) ? d.why : null;
    return B.el("div", {class: "rq-auth"},
      B.el("div", {class: "line"}, kindless === true ? null : B.el("span", {class: "rq-kind"}, KIND[d.kind] || d.kind), B.el("span", {class: "rq-does"}, d.does), B.el("span", {class: "rq-end"}, ending(d))),
      chain(d),
      why ? B.el("div", {class: "mono small muted"}, why) : null);
  }

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

  // record is what one run read, drawn when it is opened.
  function record(rec, teams) {
    const name = rec.worker ? B.workerEl(rec.worker, teams > 1 ? rec.team : null) : B.el("span", {class: "mono"}, rec.run);
    if (rec.gone) return B.el("div", {class: "inner rq-record line"}, name, B.el("span", {class: "mono small faint"}, "Corbie keeps this record no longer"));
    const body = B.el("div", {class: "events rows"});
    const box = B.el("details", {class: "inner rq-record"}, B.el("summary", {}, name), body);
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

  // read gives what was read, sealed, in one block: the team's, or, when
  // the records are of several teams, each named with its team.
  function read(records, team) {
    const teamOf = (r) => r.team || team;
    const teams = [...new Set(records.map(teamOf))];
    const called = teams.length === 1 ? B.teamCalled(teams[0]) : "";
    const sorted = records.slice().sort((a, b) => teams.indexOf(teamOf(a)) - teams.indexOf(teamOf(b)));
    return B.el("section", {class: "sect"},
      B.label(B.el("span", {class: "who"}, B.icon("lock"), called ? "Read · " + called : "Read"),
        ...B.info("About Read", called ? "Seen only by " + called + "'s people." : "Seen only by each team's people.")),
      B.el("div", {class: "sealed rq-sealed"}, paged(B.el("div", {class: "rq-records"}), sorted, 20, (r) => record(r, teams.length), "records")));
  }

  const RUN = {going: "working live", failed: "failed", cancelled: "rq-gone", lost: "failed", unsent: "failed"};
  const ENDS = {going: "working", done: "done", failed: "failed", cancelled: "cancelled", lost: "lost", unsent: "unsent"};
  const HEED = {act: "act", note: "noted"};

  const REPEATS = /(\.[0-9]+)?(#[0-9]+)?(@[0-9]+)?$/;

  // colleagues gives the colleagues a run's record shows it calling, each
  // by the tool it was called as and its worker, with how its run ended
  // and the colleagues it called in turn. Bulldog's own machines, the ask
  // among them, are not colleagues.
  function colleagues(rec) {
    const under = {}, ended = {};
    for (const ev of rec && Array.isArray(rec.events) ? rec.events : []) {
      if (ev.name !== undefined) continue;
      if (ev.status && ev.status !== "running") ended[ev.id] = ev.status;
      for (const a of ev.asks || []) {
        const machine = a.job && a.job.machine || "";
        if (a.kind !== "child" || !a.child || /(^|\/)\.bulldog\//.test(machine)) continue;
        const worker = /\.ya?ml$/.test(machine) ? machine.split("/").pop().replace(/\.ya?ml$/, "") : "";
        (under[ev.id] = under[ev.id] || []).push({id: a.child, tool: a.name.replace(REPEATS, ""), worker});
      }
    }
    const tree = (id) => (under[id] || []).map((c) => ({...c, status: ended[c.id] || "going", below: tree(c.id)}));
    return tree(rec ? rec.run : "");
  }

  // callsEl gives colleague calls as a tree: the tool, the worker it
  // reached, and how its run ended.
  function callsEl(calls, team) {
    if (!calls.length) return null;
    return B.el("ul", {class: "rq-calls"}, calls.map((c) => B.el("li", {},
      B.el("div", {class: "line"},
        c.worker !== c.tool ? B.el("span", {class: "mono"}, c.tool) : null,
        c.worker && c.worker !== c.tool ? B.el("span", {class: "faint", "aria-hidden": "true"}, "›") : null,
        c.worker ? B.workerEl(c.worker, team, {bare: true}) : null,
        B.el("span", {class: "rq-end status " + (RUN[c.status] || "")}, ENDS[c.status] || c.status)),
      callsEl(c.below, team))));
  }

  // runRow gives one run: its kind, its worker or rule, when it started
  // and ended, how it ended, a heed's act or note, and its word on it;
  // and beneath it, from its record, the colleagues it called.
  function runRow(r, rec) {
    const ends = r.says || ENDS[r.status] || r.status;
    return B.el("div", {class: "rq-run"},
      B.el("div", {class: "line"},
        B.el("span", {class: "kind"}, r.kind),
        r.kind === "heed" ? B.ruleEl(r.worker) : r.worker ? B.workerEl(r.worker, r.team, {bare: true}) : B.el("span", {class: "mono"}, r.name),
        B.el("span", {class: "rq-end status " + (HEED[r.says] || RUN[r.status] || "")}, ends)),
      B.el("div", {class: "line"},
        B.el("span", {class: "mono small faint"}, B.when(r.started) + "–" + (r.ended ? B.when(r.ended) : "")),
        r.tokens ? B.el("span", {class: "mono small faint"}, r.tokens + " tok") : null,
        B.el("a", {class: "rq-end rq-jump", href: "#/run/" + encodeURIComponent(r.name), "aria-label": "Watch " + (r.worker || r.name)}, "Watch")),
      r.how ? B.el("div", {class: "rq-note small"}, r.how) : null,
      callsEl(colleagues(rec), r.team));
  }

  // The way, in bands

  // inBand gives where a request was asked, by whom, and, to its team's
  // people, what its heed last said.
  function inBand(v) {
    const heeds = (v.runs || []).filter((r) => r.kind === "heed");
    const heed = heeds[heeds.length - 1];
    const way = v.parent ? ["Beneath", B.el("a", {class: "mono", href: "#/r/" + v.parent}, "#" + v.parent)]
      : v.origin && v.origin.bridge ? ["Place", B.way(v.origin)]
      : v.origin && v.origin.kind === "routine" ? ["Routine", B.way(v.origin)]
      : ["Chat", B.el("span", {class: "mono"}, "screens")];
    const asker = v.asker ? B.el("span", {class: "line"}, B.party(v.asker), (v.asker.teams || []).map((t) => B.el("span", {class: "tag"}, B.teamCalled(t)))) : null;
    const routineBy = v.origin && v.origin.by ? ["Run now", B.party(v.origin.by)] : null;
    return B.band("In", [way, ["Asker", asker], routineBy,
      heed ? ["Heed", B.el("span", {class: "line"}, B.el("span", {class: "status " + (HEED[heed.says] || RUN[heed.status] || "")}, heed.says || ENDS[heed.status] || heed.status),
        B.ruleEl(heed.worker))] : null]);
  }

  // teamBand gives the team a request went to and its lead, with its
  // runs beneath, to its people.
  function teamBand(v) {
    const t = B.team(v.team);
    const lead = t && t.lead ? (B.person(t.lead) ? B.personEl(t.lead) : B.workerEl(t.lead, v.team, {bare: true})) : null;
    const runs = v.runs || [];
    return B.band("Team", [["Team", B.el("span", {class: "line"}, B.teamEl(v.team), lead ? [B.el("span", {class: "faint", "aria-hidden": "true"}, "›"), lead] : null),
      runs.length ? paged(B.el("div", {class: "rq-runs"}), runs, 8, (r) => runRow(r, (v.records || []).find((x) => x.run === r.name)), "runs") : null]]);
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

  // outBand gives what a request asked of other teams and of people, its
  // decisions, to its team's people its disclosure, and where its answer
  // goes and how it stands.
  function outBand(v) {
    if (v.label === "noted") return null;
    const ds = v.decisions || [];
    const held = ds.filter((d) => d.kind === "disclosure" && d.what === "sub" && !d.ended);
    const subs = v.subs || [];
    const teams = subs.length || held.length ? B.el("div", {},
      held.map((d) => B.el("div", {class: "rq-auth"}, B.el("div", {class: "line"}, B.el("span", {class: "tag"}, "Held · Disclosure"), B.el("span", {class: "rq-does"}, d.does), B.el("span", {class: "rq-end"}, ending(d))), chain(d))),
      subs.length ? subTree(subs, 0) : null) : null;
    const questions = v.questions || [];
    const people = questions.length ? B.el("div", {class: "rq-asks"}, questions.map((q) => B.el("div", {class: "line"},
      B.party(q.by), arrow(), few(q.to, B.party, true),
      B.el("a", {class: "rq-end status" + (q.answered ? "" : " waiting"), href: "#/r/" + v.number, onclick: (ev) => { ev.preventDefault(); scrollTo(q.entry); }}, q.answered ? "answered" : "open")))) : null;
    const decs = ds.filter((d) => d.kind !== "disclosure");
    const disclosure = v.disclosure ? disclosureOf(v, ds.filter((d) => d.kind === "disclosure" && d.what !== "sub")) : null;
    const a = v.answer;
    const answer = a ? B.el("span", {class: "line"}, answerTo(v),
      a.entry ? B.el("a", {class: "status " + ANSWER[a.state], href: "#/r/" + v.number, onclick: (ev) => { ev.preventDefault(); scrollTo(a.entry); }}, a.state)
        : B.el("span", {class: "status " + ANSWER[a.state]}, a.state),
      a.why ? B.el("span", {class: "mono small muted"}, a.why) : null) : null;
    return B.band("Out", [
      ["Teams", teams ? B.el("span", {class: "count"}, subs.length + held.length) : B.el("span", {class: "faint"}, "None"), teams],
      ["People", people ? B.el("span", {class: "count"}, questions.length) : B.el("span", {class: "faint"}, "None"), people],
      decs.length ? ["Authorisation", B.el("span", {class: "count"}, decs.length), paged(B.el("div", {class: "rq-list"}), decs, 6, (d) => authorisation(d, true), "auths")] : null,
      disclosure,
      answer ? ["Answer", answer] : null]);
  }

  // disclosureOf gives a request's Disclosure row: None when its team sets
  // none, Not needed when only the team reads the answer, or who decides
  // it; with the disclosures of what it said beneath.
  function disclosureOf(v, ds) {
    const d = v.disclosure;
    const value = !d.set ? B.el("span", {class: "faint"}, "None")
      : !d.needed ? B.el("span", {class: "line"}, "Not needed", B.info("About Not needed", "Read only by people of " + B.teamCalled(v.team) + "."))
      : B.chain(d.deciders || [], v.team);
    return ["Disclosure", value, ds.length ? B.el("div", {class: "rq-list"}, ds.map((d) => authorisation(d, true))) : null];
  }

  // scrollTo brings an entry of the thread into view, when it is drawn.
  function scrollTo(id) {
    const at = document.querySelector('.thread [data-id="' + id + '"]');
    if (at) at.scrollIntoView({behavior: "smooth", block: "center"});
  }

  // The view

  B.route(/^#\/r\/(\d+)$/, async function (m) {
    const v = await B.api("/api/requests/" + m[1]);
    const n = v.number;
    if (kept.n !== n) kept = keep(n);
    const threadEl = thread(v);
    const asked = v.entries.find((e) => e.kind === "asked");
    const first = asked ? firstLine(asked.text) : "";

    const mine = (v.decisions || []).filter((d) => d.mine && !d.ended);
    const records = v.records || [];

    const crumbs = [B.el("a", {href: "#/requests"}, "Requests"), v.parent ? B.el("a", {class: "mono", href: "#/r/" + v.parent}, "#" + v.parent) : null].filter(Boolean);
    const title = [B.el("span", {class: "rq-num"}, "#" + n), first ? B.el("span", {title: first.length > 90 ? first : null}, clip(first, 90)) : null];
    const stop = v.whole && v.state !== "answered" ? B.el("button", {class: "btn ghost sm", type: "button"}, "Stop") : null;
    if (stop) stop.addEventListener("click", () => act(() => B.post("/api/requests/" + n + "/stop"), [stop], []));
    const source = v.source ? B.el("span", {class: "line"},
      v.source.link ? B.el("a", {class: "mono", href: v.source.link, rel: "noopener noreferrer", target: "_blank"}, v.source.status) : B.el("span", {class: "mono"}, v.source.status),
      B.whenEl(v.source.at)) : null;

    return B.el("div", {class: "rq"},
      B.head(crumbs, title, v.whole && v.state ? B.status(v) : null, stop),
      v.whole ? B.el("div", {class: "rq-meta"}, B.kv([
        ["Made", B.whenEl(v.made)],
        ["Source", source],
      ])) : null,
      B.el("div", {class: "cols"},
        B.el("div", {class: "main"},
          B.el("section", {class: "sect"},
            B.label(v.whole ? "Thread" : "Put to you"),
            v.entries.length || v.before ? threadEl : B.empty()),
          compose(v)),
        B.el("div", {class: "side"},
          mine.length ? B.sect("Waiting on you", B.el("div", {class: "stack"}, mine.map((d) => B.decisionCard(v, d)))) : null,
          v.whole ? B.el("section", {class: "bands rq-way", "aria-label": "Way"}, inBand(v), teamBand(v), outBand(v)) : null,
          records.length ? read(records, v.team) : null)));
  }, "requests", (what) => what === "office" || what.startsWith("request:"));
})();
