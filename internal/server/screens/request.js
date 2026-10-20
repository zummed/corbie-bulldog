// One request: its thread, what waits on the one looking in it, its
// sub-requests, its authorisations, what its team read and its runs. The
// desk draws a decision and an answer to a question with the same parts.
(function () {
  "use strict";

  const KIND = {action: "Authorisation · action", disclosure: "Disclosure", change: "Change"};
  const ENDED = {yes: "approved", refused: "refused", withdrawn: "withdrawn"};
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

  // chain gives a decision's deciders in order: those who referred it,
  // then whoever ended it, or those it waits on, the one looking first.
  function chain(d) {
    const links = (d.referrals || []).map((w) => word(w, d.team, "past"));
    if (d.ended && d.by) links.push(word(d.by, d.team, "rq-by"));
    else if (!d.ended && d.asked && d.asked.length) {
      const asked = d.asked.filter(B.isMe).concat(d.asked.filter((a) => !B.isMe(a)));
      links.push(B.el("span", {class: "rq-asked"}, few(asked, B.personEl)));
    }
    if (!links.length) return null;
    return B.el("span", {class: "chain"}, links.map((l, i) => [i ? B.el("span", {class: "then"}, "→") : null, l]));
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
      chain(d),
      B.el("div", {class: "act"}, B.field("Note", note), yes, no));
  };

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
        B.el("span", {class: "at"}, B.whenEl(e.at))),
      e.kind === "sub" && e.sub ? B.el("div", {class: "rq-branch"}, B.icon("branch"), B.el("a", {class: "mono", href: "#/r/" + e.sub}, "#" + e.sub)) : null,
      e.text ? B.el("div", {class: "body"}, e.text) : null,
      e.files && e.files.length ? B.el("div", {class: "files"}, e.files.map((f) =>
        B.el("a", {class: "who rq-file", href: f.url}, B.icon("file"), f.name, B.el("span", {class: "faint"}, B.size(f.size))))) : null,
      e.carried && e.carried.length ? B.el("div", {class: "foot"}, foot(e.carried)) : null,
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
    };
    const drawn = v.entries.map((e) => entry(v, e, state));
    if (v.before) {
      const button = B.el("button", {class: "btn ghost sm rq-earlier", type: "button"}, "Earlier");
      button.addEventListener("click", () => earlier(box, v, button));
      const at = v.entries.findIndex((e) => e.id >= v.before);
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

  function authorisation(d) {
    const why = d.why && !(d.by && d.by.note === d.why) ? d.why : null;
    return B.el("div", {class: "rq-auth"},
      B.el("div", {class: "line"}, B.el("span", {class: "rq-kind"}, KIND[d.kind] || d.kind), B.teamEl(d.team), B.el("span", {class: "rq-end"}, ending(d))),
      B.el("div", {class: "rq-does"}, d.does),
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

  const RUN = {going: "working live", failed: "failed", cancelled: "rq-gone"};

  function runRow(r) {
    return B.el("div", {class: "row rq-run"},
      B.el("span", {class: "grow"}, r.worker ? B.workerEl(r.worker, r.team, {bare: true}) : B.el("span", {class: "mono"}, r.name)),
      B.el("span", {class: "kind"}, r.kind),
      B.el("span", {class: "status " + (RUN[r.status] || ""), title: r.how || null}, r.status),
      B.whenEl(r.started),
      r.tokens ? B.el("span", {class: "mono small faint"}, r.tokens + " tok") : null,
      B.el("a", {class: "btn ghost sm", href: "#/run/" + encodeURIComponent(r.name), "aria-label": "Watch " + (r.worker || r.name)}, "Watch"));
  }

  function counted(label, n) { return B.label(label, B.el("span", {class: "count"}, n)); }

  // The view

  B.route(/^#\/r\/(\d+)$/, async function (m) {
    const v = await B.api("/api/requests/" + m[1]);
    const n = v.number;
    if (kept.n !== n) kept = keep(n);
    const threadEl = thread(v);
    const asked = v.entries.find((e) => e.kind === "asked");
    const first = asked ? firstLine(asked.text) : "";

    const decisions = v.decisions || [];
    const mine = decisions.filter((d) => d.mine && !d.ended);
    const subs = v.subs || [];
    const runs = v.runs || [];
    const records = v.records || [];

    const crumbs = [B.el("a", {href: "#/requests"}, "Requests"), v.parent ? B.el("a", {class: "mono", href: "#/r/" + v.parent}, "#" + v.parent) : null].filter(Boolean);
    const title = [B.el("span", {class: "rq-num"}, "#" + n), first ? B.el("span", {title: first.length > 90 ? first : null}, clip(first, 90)) : null];
    const stop = v.whole && v.state !== "answered" ? B.el("button", {class: "btn ghost sm", type: "button"}, "Stop") : null;
    if (stop) stop.addEventListener("click", () => act(() => B.post("/api/requests/" + n + "/stop"), [stop], []));
    const source = v.source ? B.el("span", {class: "line"},
      v.source.link ? B.el("a", {class: "mono", href: v.source.link, rel: "noopener noreferrer", target: "_blank"}, v.source.status) : B.el("span", {class: "mono"}, v.source.status),
      B.whenEl(v.source.at)) : null;

    return B.el("div", {class: "rq"},
      B.head(crumbs, title, v.whole && v.state ? B.status({state: v.state, on: v.on}) : null, stop),
      v.whole ? B.el("div", {class: "rq-meta"}, B.kv([
        ["Team", v.team ? B.teamEl(v.team) : null],
        ["Asker", v.asker ? B.party(v.asker) : null],
        v.parent ? ["Beneath", B.el("a", {class: "mono", href: "#/r/" + v.parent}, "#" + v.parent)] : ["Origin", B.origin(v.origin)],
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
          subs.length ? B.sect(counted("Sub-requests", subs.length), subTree(subs, 0)) : null,
          decisions.length ? B.sect(counted("Authorisations", decisions.length), paged(B.el("div", {}), decisions, 6, authorisation, "auths")) : null,
          records.length ? read(records, v.team) : null,
          runs.length ? B.sect(counted("Runs", runs.length), paged(B.el("div", {class: "rows"}), runs, 8, runRow, "runs")) : null)));
  }, "requests", (what) => what === "office" || what.startsWith("request:"));
})();
