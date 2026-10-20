// One team, as a room in bands: In, where it is asked and, before its
// lead, its heed; Team, its people and, to its people, the tree of what
// the lead calls, each worker at work with what on; Out, whom it may
// ask, who authorises and who discloses. Beside it, to its people, its
// routines, models, rules, notes and problems, and its docs to those who
// may open them. A team may have hundreds of people and
// many workers, so long lists are paged and searched.
(function () {
  "use strict";

  const PEOPLE = 12;    // people shown before More
  const INSIDE = 12;    // teams inside shown before +N
  const ROOTS = 50;     // workers beside the lead shown before More
  const PAGE = 20;      // tools, routines, notes and lists shown before More

  // What the view keeps while it is drawn again, for one team: what is
  // searched for, how many of each list are shown, and which notes, docs
  // and folders of docs are open.
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

  // worker gives a worker by its name, linked when it is one of the team's
  // workers the one looking is shown.
  function worker(t, name) {
    return (t.workers || []).some((x) => x.name === name) ? B.workerEl(name, t.name, {bare: true}) : B.workerEl(name);
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

  // The tree of workers: the lead at its root, beneath each worker the
  // colleagues it calls, folded, then what it does with Bulldog's tools,
  // by their kits, and the names of its own tools; workers the lead does
  // not reach stand beside it as roots of their own. Each tool's kind,
  // what it does and its file are on the worker's own page.

  const kits = {talk: "Talks to people and teams", docs: "Reads docs", notes: "Writes notes", build: "Drafts changes to the office"};

  function byName(t, name) { return (t.workers || []).find((x) => x.name === name); }

  // problemEls gives a worker's problems, said on its row.
  function problemEls(t, name) {
    return (t.problems || []).filter((p) => p.worker === name).map((p) => B.el("div", {class: "mono te-problem lt-problem"}, p.text));
  }

  // workerRow gives a worker on one line: its name, the lock of a
  // colleague that waits for a yes, its one status, cannot run or at
  // work, with each request it is at work on, to watch, and how it stands
  // in the tree.
  function workerRow(t, w, role, lock) {
    const busy = (t.at_work || []).filter((a) => a.worker === w.name);
    return B.el("span", {class: "line lt-row"},
      B.workerEl(w.name, t.name, {bare: true}), lock,
      w.runs === false ? B.el("span", {class: "status failed"}, "cannot run") : busy.length ? B.status({state: "working"}) : null,
      w.runs !== false && t.in ? busy.map((a) => B.el("a", {class: "mono small", href: "#/run/" + encodeURIComponent(a.run), title: "Watch " + w.name + " on #" + a.request, "aria-label": "Watch " + w.name + " on #" + a.request}, "#" + a.request)) : null,
      role ? B.el("span", {class: "lt-role"}, role) : null);
  }

  // lockEl gives a small lock on a tool that waits for a yes, saying who
  // decides it, or that nobody does, when asked.
  function lockEl(t, tl) {
    if (!tl.waits) return null;
    const who = (tl.deciders || []).map((d) => d.kind === "person" ? B.personCalled(d.name) : d.kind === "bulldog" ? "Bulldog" : d.name);
    const said = "Waits for a yes: " + (who.length ? who.join(", then ") : "refused, since nobody decides it");
    return B.el("span", {class: "lt-lock", title: said, role: "img", "aria-label": said}, B.icon("lock"));
  }

  function nameEl(t, tl) {
    return B.el("span", {class: "line lt-tool"}, B.el("span", {class: "mono"}, tl.listed ? tl.name + "_*" : tl.name), lockEl(t, tl));
  }

  // colleagueNode gives a colleague beneath the worker that calls it,
  // holding its own beneath it, folded, unless it is already above it or
  // holds nothing.
  function colleagueNode(t, w, tl, c, above) {
    const row = workerRow(t, c, "colleague", lockEl(t, tl));
    const body = above.has(c.name) ? null : [...problemEls(t, c.name), under(t, c, new Set(above).add(c.name))].filter(Boolean);
    if (!body || !body.length) return B.el("li", {class: "lt-leaf"}, row);
    return B.el("li", {}, opened("tree:" + w.name + "/" + tl.name, "lt-node", B.el("summary", {}, row), B.el("div", {}, body)));
  }

  // under gives what is beneath a worker: its colleagues, a line of what
  // it does with Bulldog's tools, and a line of its own tools' names.
  function under(t, w, above) {
    const colleagues = [], kitted = new Map(), own = [];
    for (const tl of w.tools || []) {
      const c = tl.kind === "worker" ? byName(t, tl.worker) : null;
      if (c) colleagues.push([tl, c]);
      else if (tl.kind === "bulldog" && kits[tl.kit]) kitted.set(tl.kit, [...(kitted.get(tl.kit) || []), tl]);
      else own.push(tl);
    }
    if (!colleagues.length && !kitted.size && !own.length) return null;
    const kitEls = Object.keys(kits).filter((k) => kitted.has(k)).map((k) => B.el("span", {class: "line lt-kit", title: kitted.get(k).map((tl) => tl.name).join(", ")},
      kits[k], kitted.get(k).map((tl) => lockEl(t, tl))));
    return B.el("div", {class: "lt-under"},
      colleagues.length ? paged("colleagues of " + w.name, colleagues, ([tl, c]) => colleagueNode(t, w, tl, c, above), PAGE, B.el("ul", {class: "lt"}), (b) => B.el("li", {}, b)) : null,
      kitEls.length ? B.el("span", {class: "line lt-kits"}, kitEls.flatMap((e, i) => [i ? B.el("span", {class: "faint", "aria-hidden": "true"}, "·") : null, e])) : null,
      own.length ? paged("tools of " + w.name, own, (tl) => nameEl(t, tl), PAGE, B.el("span", {class: "line lt-own"}), (b) => b) : null);
  }

  // reached gives the workers a worker calls, as far down as they go.
  function reached(t, w, into) {
    for (const tl of w.tools || []) {
      const c = tl.kind === "worker" ? byName(t, tl.worker) : null;
      if (c && !into.has(c.name)) { into.add(c.name); reached(t, c, into); }
    }
    return into;
  }

  // tree gives the lead, open, with all it calls beneath it, and beside it
  // each worker it does not reach, as one a routine starts or unreached.
  function tree(t) {
    const workers = t.workers || [];
    if (!workers.length) return null;
    const lead = byName(t, t.lead);
    const near = lead ? reached(t, lead, new Set([lead.name])) : new Set();
    const roots = workers.filter((w) => !near.has(w.name));
    const root = (w) => {
      const routine = (t.routines || []).some((r) => r.to === w.name);
      const body = [...problemEls(t, w.name), under(t, w, new Set([w.name]))].filter(Boolean);
      const row = workerRow(t, w, routine ? "routine" : "unreached");
      return body.length ? B.el("li", {}, opened("tree:" + w.name, "lt-node", B.el("summary", {}, row), B.el("div", {}, body)))
        : B.el("li", {class: "lt-leaf"}, row);
    };
    return B.el("div", {class: "lt-tree"},
      lead ? B.el("ul", {class: "lt lt-roots"}, B.el("li", {}, workerRow(t, lead, B.leadLabel(t, "lead")),
        problemEls(t, lead.name), under(t, lead, new Set([lead.name])))) : null,
      roots.length ? paged("roots", roots, root, ROOTS, B.el("ul", {class: "lt lt-roots"}), (b) => B.el("li", {}, b)) : null);
  }

  // Out: whom the lead may ask, who authorises what waits and who
  // discloses what leaves, each when there is any.

  function asksEl(t) {
    const lead = byName(t, t.lead);
    if (!lead || !lead.may_ask.length) return null;
    return B.el("span", {class: "line"}, lead.may_ask.map(B.teamEl));
  }

  function authorisationEl(t) {
    if (!t.decide || !t.decide.length) return null;
    return B.el("div", {class: "stack band-list"},
      rows("authorisation", t.decide, (l) => B.el("div", {class: "row te-list"}, B.el("span", {class: "mono te-key"}, l.key), B.chain(l.deciders || [], t.name)), "te-lists"));
  }

  function disclosureEl(t) {
    return t.disclosure && t.disclosure.length ? B.chain(t.disclosure, t.name) : null;
  }

  // The side

  function problems(t) {
    const list = (t.problems || []).filter((p) => !p.worker || !byName(t, p.worker));
    if (!list.length) return null;
    return B.sect(B.label("Problems", B.el("span", {class: "count"}, String(list.length))),
      rows("problems", list, (p) => B.el("div", {class: "row te-problem"}, B.el("span", {class: "grow"}, p.said)), "te-problems"));
  }

  async function runNow(t, r, button) {
    button.disabled = true;
    B.trouble(null);
    try {
      const out = await B.post("/api/teams/" + encodeURIComponent(t.name) + "/routines/now", {at: r.at, to: r.to, say: r.say});
      location.hash = "#/r/" + out.number;
    } catch (err) {
      button.disabled = false;
      B.trouble(err);
    }
  }

  // routineStatus gives a routine's one status: due, skipped while the
  // request it last opened is open, or cannot run.
  function routineStatus(r) {
    if (r.status === "skipped") return B.el("span", {class: "line"}, B.el("span", {class: "status waiting"}, "skipped"), B.el("a", {class: "mono", href: "#/r/" + r.open}, "#" + r.open + " open"));
    if (r.status === "cannot run") return B.el("span", {class: "status failed"}, "cannot run");
    return B.el("span", {class: "status due"}, "due");
  }

  // routines gives each routine: when it comes round, in its zone where
  // that is not the office's, the worker it starts, what it says, and its
  // one status with when it comes next.
  function routines(t) {
    if (!t.routines || !t.routines.length) return null;
    return B.sect("Routines", rows("routines", t.routines, (r) => {
      const button = t.in ? B.el("button", {class: "btn sm ghost", type: "button", "aria-label": "Run now: " + r.to + ", " + r.at}, "Run now") : null;
      if (button) button.addEventListener("click", () => runNow(t, r, button));
      return B.el("div", {class: "row te-rt"},
        B.el("span", {class: "line te-at"}, B.el("span", {class: "mono"}, r.at),
          r.zone !== B.me.office.zone ? B.el("span", {class: "faint small"}, r.zone) : null,
          B.el("span", {class: "te-to"}, worker(t, r.to))),
        button,
        B.el("span", {class: "te-say clip", title: r.say}, r.say),
        B.el("span", {class: "te-rt-foot line"},
          routineStatus(r),
          r.next ? B.el("span", {class: "line"}, B.el("span", {class: "faint small"}, "next"), B.whenEl(r.next)) : null));
    }, "te-routines"));
  }

  function models(t) {
    if (!t.models || !t.models.length) return null;
    return B.sect("Models", B.el("div", {class: "scroll"}, B.el("table", {class: "table te-models"},
      B.el("thead", {}, B.el("tr", {}, ["Name", "Model", "Secret"].map((h) => B.el("th", {scope: "col"}, h)))),
      B.el("tbody", {}, t.models.map((m) => B.el("tr", {},
        B.el("td", {}, m.name),
        B.el("td", {}, B.modelText(m.text) || m.text),
        B.el("td", {}, m.secret || B.el("span", {class: "faint"}, "None"))))))));
  }

  const ruleWords = {heed: "heed", disclosure: "disclosure"};

  // rules gives each rule the team names: its path, how it is named, as
  // the heed, the disclosure or a list's key, whether it is stopped, and
  // what it reaches, or that it is a bare script.
  function rules(t) {
    if (!t.rules || !t.rules.length) return null;
    return B.sect("Rules", rows("rules", t.rules, (r) => B.el("div", {class: "row te-rule"},
      B.el("span", {class: "line"}, B.ruleEl(r.path),
        B.el("span", {class: "mono faint small"}, r.as.map((a) => ruleWords[a] || a).join(", ")),
        r.runs ? null : B.el("span", {class: "status failed"}, "stopped")),
      r.script ? B.el("span", {class: "faint small"}, "script") : B.reachEl(r.reach)), "te-rules"));
  }

  // notes gives the team's notes, and the workers that may write them.
  function notes(t) {
    const writers = t.writers || [];
    if (!(t.notes && t.notes.length) && !writers.length) return null;
    return B.sect(B.label("Notes", B.el("span", {class: "count"}, String(t.notes.length)),
      ...B.notesInfo()),
    writers.length ? B.el("span", {class: "line te-writers"}, B.el("span", {class: "faint small"}, "Written by"),
      writers.map((w) => worker(t, w)), ...B.info("About written by", "The workers of this team whose definitions name the notes tool. Each may write any of its notes, at once, with no decision.")) : null,
    t.notes.length ? rows("notes", t.notes, (n) => {
      const d = B.el("details", {open: state.open.has(n.name)},
        B.el("summary", {}, B.el("span", {class: "mono"}, n.name),
          B.el("span", {class: "mono faint te-size"}, n.size.toLocaleString("en-GB") + " characters"), B.givenEl(n)),
        B.el("pre", {}, n.text));
      d.addEventListener("toggle", () => { if (d.open) state.open.add(n.name); else state.open.delete(n.name); });
      return B.el("div", {class: "row te-note"}, d);
    }, "te-notes") : null);
  }

  // Docs, nested by their folders, each opened in place.

  // folders gives docs as a tree: each folder its folders by name, and
  // its docs with the last part of their names.
  function folders(docs) {
    const root = {folders: new Map(), docs: []};
    for (const d of docs) {
      const parts = d.name.split("/");
      let at = root;
      for (const part of parts.slice(0, -1)) {
        if (!at.folders.has(part)) at.folders.set(part, {folders: new Map(), docs: []});
        at = at.folders.get(part);
      }
      at.docs.push({doc: d, part: parts[parts.length - 1]});
    }
    return root;
  }

  // opened gives a details kept open, by key, while the view is drawn
  // again, which calls first the first time it opens.
  function opened(key, cls, summary, body, first) {
    const d = B.el("details", {class: cls, open: state.open.has(key)}, summary, body);
    let done = false;
    d.addEventListener("toggle", () => {
      if (!d.open) { state.open.delete(key); return; }
      state.open.add(key);
      if (first && !done) { done = true; first(); }
    });
    return d;
  }

  function docEl(t, d, part) {
    const text = B.el("pre", {});
    return opened("doc:" + d.name, "te-doc",
      B.el("summary", {}, B.el("span", {class: "mono"}, part), B.el("span", {class: "mono faint te-size"}, B.size(d.size))),
      text,
      async () => {
        try {
          const out = await B.api("/api/teams/" + encodeURIComponent(t.name) + "/docs/" + d.name.split("/").map(encodeURIComponent).join("/"));
          text.textContent = out.text;
        } catch (err) {
          B.trouble(err);
        }
      });
  }

  function folderEls(t, node, path) {
    return [
      ...[...node.folders.keys()].sort().map((name) => opened("folder:" + path + name, "te-folder",
        B.el("summary", {class: "mono"}, name + "/"),
        B.el("div", {class: "te-docs"}, folderEls(t, node.folders.get(name), path + name + "/")))),
      ...node.docs.map((x) => docEl(t, x.doc, x.part)),
    ];
  }

  function docs(t) {
    if (!t.docs || !t.docs.length) return null;
    return B.sect(B.label("Docs", B.el("span", {class: "count"}, String(t.docs.length)),
      ...B.info("About docs", "Read by the workers of this team and of the teams within, when they ask, through the docs tools. Written through a change.")),
    B.el("div", {class: "te-docs"}, folderEls(t, folders(t.docs), "")));
  }

  // The view

  B.route(/^#\/t\/([^/]+)$/, async function (m) {
    const name = decodeURIComponent(m[1]);
    const t = await B.api("/api/teams/" + encodeURIComponent(name));
    if (location.hash !== m[0]) return B.el("div");
    keepFor(name);
    const at = focusOf();
    const whole = Array.isArray(t.workers);
    const crumbs = [B.el("a", {href: "#/teams"}, "Teams"), ...within(t).map((u) => B.el("a", {href: B.teamHref(u.name)}, u.called))];
    const head = B.head(crumbs, t.called,
      B.el("a", {class: "te-reqs", href: "#/requests?team=" + encodeURIComponent(t.name)}, "Requests"),
      whole && t.file ? B.el("span", {class: "mono faint small te-file"}, t.file) : null,
      t.about ? B.el("p", {class: "te-about"}, t.about) : null);
    const treed = whole && byName(t, t.lead);
    const channel = B.placesOf(t.name).some((x) => x.place.name !== "direct");
    const team = B.el("section", {class: "room te-room", "aria-label": t.within ? "Team" : "Office"},
      B.el("div", {class: "plate"}, t.within ? B.el("b", {}, "Team")
        : [B.el("b", {}, "Office"), ...B.info("About the office", "The top team, written in office.yaml. A decision nobody else takes ends at its everything.")]),
      B.el("div", {class: "bands"},
        B.band("In", [
          ["", B.asksIn(t.name)],
          channel || t.heed ? [B.heedLabel(t), B.heedEl(t)] : null,
        ]),
        B.band("Team", [
          ["", people(t)],
          !(t.people && t.people.length) && t.reached && t.reached.length
            ? [B.el("span", {class: "line"}, "Reached", ...B.info("About reached", "The people this team's decisions reach. They stand as its people.")),
              B.el("span", {class: "line te-people te-reached"}, t.reached.map((n) => B.personEl(n)))] : null,
          treed ? null : [B.leadLabel(t), B.el("span", {class: "line"}, leadEl(t), t.lead_runs ? null : B.el("span", {class: "status failed"}, "cannot run"))],
          whole ? ["", tree(t)] : null,
          ["Teams inside", inside(t)],
        ]),
        B.band("Out", [
          whole ? ["Asks", asksEl(t)] : null,
          whole ? [B.el("span", {class: "line"}, "Authorisation", ...B.info("About authorisation", "Asked in order. A yes or a refusal ends it; a refer goes on, with its note, to whoever is next.")), authorisationEl(t)] : null,
          whole ? [B.disclosureLabel(t), disclosureEl(t)] : null,
        ])));

    const side = (whole ? [routines(t), models(t), rules(t), notes(t), docs(t), problems(t)] : [docs(t)]).filter(Boolean);
    const out = B.el("div", {class: "te"}, head,
      B.el("div", {class: "cols"},
        B.el("div", {class: "main"}, team),
        side.length ? B.el("div", {class: "side"}, side) : null));
    refocus(out, at);
    return out;
  }, "teams");
})();
