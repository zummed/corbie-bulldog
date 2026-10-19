// One team: who is in it and its lead, and, to its people,
// how it is written: its workers, its authorisation, its routines, what
// its workers are at, its models and its notes. A team may have hundreds
// of people and many workers, so long lists are paged and searched.
(function () {
  "use strict";

  const PEOPLE = 12;    // people shown before More
  const INSIDE = 12;    // teams inside shown before +N
  const TOOLS = 30;     // characters of tools shown on a worker's line
  const WORKERS = 50;   // workers shown before More
  const SEARCH = 12;    // more workers than this are searched
  const PAGE = 20;      // routines, runs, notes and lists shown before More

  // What the view keeps while it is drawn again, for one team: what is
  // searched for, how many of each list are shown, and which notes are open.
  const state = {team: "", q: new Map(), shown: new Map(), open: new Set()};
  function keepFor(team) {
    if (state.team !== team) { state.team = team; state.q = new Map(); state.shown = new Map(); state.open = new Set(); }
  }

  // focusOf gives which search box the one looking is in, and where in it;
  // refocus puts them back there once the view is drawn again.
  function focusOf() {
    const a = document.activeElement;
    if (!a || !a.dataset || !a.dataset.keep) return null;
    return {keep: a.dataset.keep, start: a.selectionStart, end: a.selectionEnd};
  }
  function refocus(root, at) {
    if (!at) return;
    setTimeout(() => {
      const box = root.querySelector("[data-keep=\"" + at.keep + "\"]");
      if (!box || !box.isConnected) return;
      box.focus();
      try { box.setSelectionRange(at.start, at.end); } catch (err) { /* a box that keeps no place */ }
    }, 0);
  }

  // searched gives a search box over list, kept by key while the view is
  // drawn again; a new search starts its list from the top.
  function searched(key, placeholder, list, text, draw) {
    let drawn = state.q.get(key) || "", kept = false;
    const words = new Map(list.map((x) => [x, text(x)]));
    const found = B.search(placeholder, list, (x) => words.get(x), (xs, q) => {
      if (kept && q !== drawn) state.shown.delete(key);
      drawn = q;
      return draw(xs);
    });
    found.box.dataset.keep = key;
    found.box.addEventListener("input", () => { state.q.set(key, found.box.value); });
    if (state.q.get(key)) { found.box.value = state.q.get(key); found.box.dispatchEvent(new Event("input")); }
    kept = true;
    return found;
  }

  function more(left, label, onclick) {
    return B.el("button", {class: "btn sm ghost te-more", type: "button", "aria-label": label + ", " + left + " more", onclick},
      "More", B.el("span", {class: "mono faint"}, String(left)));
  }

  // paged draws the first of the items into box, and More the next; key
  // keeps how many are shown when the view is drawn again.
  function paged(key, items, draw, page, box, moreRow, step) {
    const fill = () => {
      const n = state.shown.get(key) || page;
      box.replaceChildren(...items.slice(0, n).map(draw),
        ...(items.length > n ? [moreRow(more(items.length - n, "More " + key, () => { state.shown.set(key, n + (step || page)); fill(); }))] : []));
    };
    fill();
    return box;
  }

  function rows(key, items, draw, cls) {
    return paged(key, items, draw, PAGE, B.el("div", {class: "rows " + (cls || "")}), (b) => B.el("div", {class: "row"}, b));
  }

  // within gives the teams a team is within, the outermost first.
  function within(t) {
    const out = [], seen = new Set([t.name]);
    for (let u = B.team(t.within); u && !seen.has(u.name) && out.length < 32; u = B.team(u.within)) { seen.add(u.name); out.unshift(u); }
    return out;
  }

  // model gives a model as it is sent in a few words: its name and service.
  function model(text) {
    const field = (k) => {
      const m = String(text).match(new RegExp("(?:^|[{,\\s])" + k + "\\s*:\\s*(\"[^\"]*\"|'[^']*'|[^,}\\s]+)"));
      return m ? m[1].replace(/^["']|["']$/g, "") : "";
    };
    const name = field("name"), service = field("service");
    return name || service ? [name, service].filter(Boolean).join(" · ") : String(text);
  }

  // worker gives a worker by its name, linked when it is one of the team's
  // workers the one looking is shown.
  function worker(t, name) {
    return (t.workers || []).some((x) => x.name === name) ? B.workerEl(name, t.name, {bare: true}) : B.workerEl(name);
  }

  function chain(t, list) {
    return B.el("span", {class: "chain"}, list.flatMap((d, i) => [i ? B.el("span", {class: "then", "aria-hidden": "true"}, "→") : "", B.deciderEl(d, t.name)]));
  }

  function leadEl(t) {
    if (!t.lead) return null;
    return B.person(t.lead) ? B.personEl(t.lead) : worker(t, t.lead);
  }

  // people gives a team's people, the one looking first, a few at a time,
  // and searched when there are many.
  function people(t) {
    const list = (t.people || []).slice().sort((a, b) => B.isMe(b) - B.isMe(a));
    if (!list.length) return null;
    const line = (ps) => ps.length ? paged("people", ps, (n) => B.personEl(n), PEOPLE, B.el("span", {class: "line te-people"}), (b) => b, 60) : B.empty();
    if (list.length <= PEOPLE) return line(list);
    const found = searched("people", "Search people", list, (n) => n + " " + B.personCalled(n), line);
    return B.el("div", {class: "te-found"}, found.box, found.out);
  }

  // inside gives the teams inside a team, the one looking's first.
  function inside(t) {
    const mine = new Set(B.me.me.teams || []);
    const list = (B.me.teams || []).filter((u) => u.within === t.name)
      .sort((a, b) => (mine.has(b.name) - mine.has(a.name)) || a.called.localeCompare(b.called, undefined, {numeric: true}));
    if (!list.length) return null;
    return paged("teams inside", list, (u) => B.teamEl(u.name), INSIDE, B.el("span", {class: "line te-inside"}), (b) => b, 60);
  }

  // Workers

  // tool gives one tool by its name: a colleague's links to it, and one
  // that waits is locked; what it does, its file and its deciders show on
  // hovering.
  function tool(t, tl) {
    const deciders = (tl.deciders || []).map((d) => d.name).join(" → ");
    const colleague = tl.kind === "worker" ? tl.worker || tl.name : "";
    const name = colleague && t.workers.some((x) => x.name === colleague) ? B.el("a", {href: B.workerHref(t.name, colleague)}, tl.name) : tl.name;
    const title = [tl.does, tl.file, tl.waits ? "Authorisation: " + (deciders || "refused") : ""].filter(Boolean).join("\n");
    return B.el("span", {class: "te-tool", title: title || null},
      name, tl.waits ? [B.icon("lock", "te-lock"), B.el("span", {class: "te-sr"}, " waits")] : "");
  }

  // tools gives a worker's tools on one line, as many as fit, and +N for
  // the rest, which its own page shows.
  function tools(t, w) {
    const list = w.tools || [];
    if (!list.length) return "";
    let n = 0;
    for (let used = 0; n < list.length && (n === 0 || used + list[n].name.length + 2 <= TOOLS); n++) used += list[n].name.length + 2;
    return B.el("span", {class: "te-tools"},
      list.slice(0, n).flatMap((x, i) => [i ? ", " : "", tool(t, x)]),
      list.length > n ? [" ", B.el("a", {class: "te-plus", href: B.workerHref(t.name, w.name), title: list.slice(n).map((x) => x.name).join(", "),
        "aria-label": (list.length - n) + " more tools of " + w.name}, "+" + (list.length - n))] : "");
  }

  function workerRow(t, w, busy) {
    return B.el("tr", {},
      B.el("td", {}, B.el("span", {class: "line te-wname"},
        B.workerEl(w.name, t.name, {bare: true}),
        w.name === t.lead ? B.el("span", {class: "tag"}, "Lead") : null,
        w.runs === false ? B.el("span", {class: "status failed"}, "cannot run") : busy.has(w.name) ? B.status({state: "working"}) : null)),
      B.el("td", {}, B.el("span", {class: "who muted"}, B.icon(w.type), w.type)),
      B.el("td", {class: "muted"}, B.el("span", {class: "te-clip te-file", title: w.file}, w.file)),
      B.el("td", {},
        B.el("span", {class: "te-clip te-model", title: w.model || null}, w.model ? model(w.model) : ""),
        w.model_name ? B.el("span", {class: "faint te-from"}, w.model_name + (w.model_from ? " · " + B.teamCalled(w.model_from) : "")) : null),
      B.el("td", {class: "te-tcell"}, tools(t, w)));
  }

  function workerTable(t, list) {
    if (!list.length) return B.empty();
    const busy = new Set((t.at_work || []).map((a) => a.worker));
    const body = paged("workers", list, (w) => workerRow(t, w, busy), WORKERS, B.el("tbody", {}),
      (b) => B.el("tr", {}, B.el("td", {colspan: "5"}, b)));
    return B.el("div", {class: "scroll"}, B.el("table", {class: "table te-workers"},
      B.el("thead", {}, B.el("tr", {}, ["Name", "Type", "File", "Model", "Tools"].map((h) => B.el("th", {scope: "col"}, h)))),
      body));
  }

  function workers(t) {
    if (!t.workers.length) return null;
    const list = t.workers.slice().sort((a, b) => (b.name === t.lead) - (a.name === t.lead));
    const label = B.label("Workers", B.el("span", {class: "count"}, String(list.length)));
    if (list.length <= SEARCH) return B.sect(label, workerTable(t, list));
    const found = searched("workers", "Search workers", list,
      (w) => [w.name, w.type, w.file, w.model ? model(w.model) : "", ...(w.tools || []).map((x) => x.name)].join(" "), (ws) => workerTable(t, ws));
    return B.sect(label, B.el("div", {class: "te-search"}, found.box), found.out);
  }

  // Authorisation and release

  function authorisation(t) {
    if (!t.decide || !t.decide.length) return null;
    return B.sect("Authorisation",
      rows("authorisation", t.decide, (l) => B.el("div", {class: "row te-list"}, B.el("span", {class: "mono te-key"}, l.key), chain(t, l.deciders || [])), "te-lists"));
  }

  function release(t) {
    if (!t.release || !t.release.length) return null;
    return B.sect("Release",
      B.el("div", {class: "rows"}, B.el("div", {class: "row"}, chain(t, t.release))));
  }

  // The side

  function problems(t) {
    if (!t.problems || !t.problems.length) return null;
    return B.sect(B.label("Problems", B.el("span", {class: "count"}, String(t.problems.length))),
      rows("problems", t.problems, (p) => B.el("div", {class: "row te-problem"}, B.el("span", {class: "grow"}, p.said)), "te-problems"));
  }

  async function runNow(t, r, button) {
    button.disabled = true;
    B.trouble(null);
    try {
      const out = await B.post("/api/teams/" + encodeURIComponent(t.name) + "/routines/now", r);
      location.hash = "#/r/" + out.number;
    } catch (err) {
      button.disabled = false;
      B.trouble(err);
    }
  }

  function routines(t) {
    if (!t.routines || !t.routines.length) return null;
    return B.sect("Routines", rows("routines", t.routines, (r) => {
      const button = t.in ? B.el("button", {class: "btn sm ghost", type: "button", "aria-label": "Run now: " + r.to + ", " + r.at}, "Run now") : null;
      if (button) button.addEventListener("click", () => runNow(t, r, button));
      return B.el("div", {class: "row te-rt"},
        B.el("span", {class: "mono te-at"}, r.at),
        B.el("span", {class: "te-to"}, worker(t, r.to)),
        button || B.el("span", {}),
        B.el("span", {class: "te-say clip", title: r.say}, r.say));
    }, "te-routines"));
  }

  function runs(t) {
    if (!t.in || !t.at_work || !t.at_work.length) return null;
    return B.sect(B.label("Runs", B.el("span", {class: "count"}, String(t.at_work.length))),
      rows("runs", t.at_work, (a) => B.el("div", {class: "row te-run"},
        B.el("span", {class: "te-to"}, worker(t, a.worker)),
        B.el("a", {class: "mono", href: "#/r/" + a.request}, "#" + a.request),
        B.el("a", {class: "btn sm ghost", href: "#/run/" + encodeURIComponent(a.run), "aria-label": "Watch " + a.worker + " on #" + a.request}, "Watch")), "te-runs"));
  }

  function models(t) {
    if (!t.models || !t.models.length) return null;
    return B.sect("Models", rows("models", t.models, (m) => B.el("div", {class: "row te-mrow"},
      B.el("span", {class: "mono te-key"}, m.name),
      B.el("span", {class: "mono clip grow", title: m.text}, model(m.text))), "te-models"));
  }

  function notes(t) {
    if (!t.notes || !t.notes.length) return null;
    return B.sect("Notes", rows("notes", t.notes, (n) => {
      const d = B.el("details", {open: state.open.has(n.name)},
        B.el("summary", {class: "mono"}, n.name),
        B.el("pre", {}, n.text));
      d.addEventListener("toggle", () => { if (d.open) state.open.add(n.name); else state.open.delete(n.name); });
      return B.el("div", {class: "row te-note"}, d);
    }, "te-notes"));
  }

  // The view

  B.route(/^#\/t\/([^/]+)$/, async function (m) {
    const name = decodeURIComponent(m[1]);
    const t = await B.api("/api/teams/" + encodeURIComponent(name));
    if (location.hash !== m[0]) return B.el("div");
    keepFor(name);
    const at = focusOf();
    const crumbs = [B.el("a", {href: "#/teams"}, "Teams"), ...within(t).map((u) => B.el("a", {href: B.teamHref(u.name)}, u.called))];
    const head = B.head(crumbs, t.called,
      B.el("a", {class: "te-reqs", href: "#/requests?team=" + encodeURIComponent(t.name)}, "Requests"),
      t.about ? B.el("p", {class: "te-about"}, t.about) : null);

    const whole = Array.isArray(t.workers);
    const team = B.el("section", {class: "room te-room", "aria-label": "Team"},
      B.el("div", {class: "plate"}, B.el("b", {}, "Team")),
      B.kv([
        ["People", people(t) || B.empty()],
        ["Lead", leadEl(t)],
        ["Within", t.within ? B.teamEl(t.within) : null],
        ["Teams inside", inside(t)],
        ["File", whole && t.file ? B.el("span", {class: "mono"}, t.file) : null],
      ]),
      whole ? workers(t) : null,
      whole ? authorisation(t) : null,
      whole ? release(t) : null);

    const side = whole ? [routines(t), runs(t), models(t), notes(t), problems(t)].filter(Boolean) : [];
    const out = B.el("div", {class: "te"}, head,
      B.el("div", {class: "cols"},
        B.el("div", {class: "main"}, team),
        side.length ? B.el("div", {class: "side"}, side) : null));
    refocus(out, at);
    return out;
  }, "teams");
})();
