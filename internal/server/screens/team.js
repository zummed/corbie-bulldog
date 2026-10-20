// One team, as a room in bands: In, where it is asked and, before its
// lead, its heed; Team, its people, its lead and, to its people, the tree
// of what the lead calls and what is at work; Out, whom it may ask, who
// authorises and who discloses, and where its answer is said. Beside it,
// to its people, its routines, models, rules, notes and problems, and its
// docs to those who may open them. A team may have hundreds of people and
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

  // The tree of workers: the lead at its root, each tool beneath the
  // worker that holds it, and a colleague's tools beneath it in turn;
  // workers the lead does not reach stand beside it as roots of their own.

  const kinds = {script: "Script", machine: "Machine", worker: "Colleague", bulldog: "Bulldog"};

  function byName(t, name) { return (t.workers || []).find((x) => x.name === name); }

  // problemEls gives a worker's problems, said on its row.
  function problemEls(t, name) {
    return (t.problems || []).filter((p) => p.worker === name).map((p) => B.el("div", {class: "mono te-problem lt-problem"}, p.text));
  }

  // workerRow gives a worker on one line: its name, its type, its tags,
  // and its one status, cannot run or at work.
  function workerRow(t, w, tags) {
    const busy = (t.at_work || []).some((a) => a.worker === w.name);
    return B.el("span", {class: "line lt-row"},
      B.workerEl(w.name, t.name, {bare: true}),
      B.el("span", {class: "who muted small"}, B.icon(w.type, "faint"), w.type),
      tags,
      w.runs === false ? B.el("span", {class: "status failed"}, "cannot run") : busy ? B.status({state: "working"}) : null);
  }

  // lockEl gives who decides a tool that waits for a yes, in order, or
  // that nobody does.
  function lockEl(t, tl) {
    if (!tl.waits) return null;
    return B.el("span", {class: "line lt-lock"}, B.icon("lock", "faint"),
      tl.deciders && tl.deciders.length ? B.chain(tl.deciders, t.name) : B.el("span", {class: "mono muted"}, "refused"));
  }

  // toolRow gives a tool on one line: its name, what it is, what it does
  // behind its (i), where it is, and who decides it when it waits.
  function toolRow(t, tl, name) {
    return B.el("span", {class: "line lt-row"},
      name || B.el("span", {class: "mono"}, tl.listed ? tl.name + "_*" : tl.name),
      B.el("span", {class: "tag"}, kinds[tl.kind] || tl.kind),
      tl.does ? B.info("About " + tl.name, tl.does) : null,
      tl.kind === "bulldog" ? B.el("a", {class: "small", href: "#/shipped"}, "Shipped") : null,
      tl.file && tl.kind !== "worker" ? B.el("span", {class: "mono faint small"}, tl.file) : null,
      lockEl(t, tl));
  }

  // toolNode gives one tool beneath its worker; a colleague holds its own
  // tools, folded, unless it is already above it.
  function toolNode(t, w, tl, above) {
    const c = tl.kind === "worker" ? byName(t, tl.worker) : null;
    if (!c) return B.el("li", {}, toolRow(t, tl));
    const row = toolRow(t, tl, workerRow(t, c, null));
    if (above.has(c.name)) return B.el("li", {}, row);
    return B.el("li", {}, opened("tree:" + w.name + "/" + tl.name, "lt-node",
      B.el("summary", {}, row),
      B.el("div", {}, problemEls(t, c.name), toolsOf(t, c, new Set(above).add(c.name)))));
  }

  function toolsOf(t, w, above) {
    const tools = w.tools || [];
    if (!tools.length) return null;
    return paged("tools of " + w.name, tools, (tl) => toolNode(t, w, tl, above), PAGE, B.el("ul", {class: "lt"}), (b) => B.el("li", {}, b));
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
  // each worker it does not reach, tagged Routine when a routine starts it.
  function tree(t) {
    const workers = t.workers || [];
    if (!workers.length) return null;
    const lead = byName(t, t.lead);
    const near = lead ? reached(t, lead, new Set([lead.name])) : new Set();
    const roots = workers.filter((w) => !near.has(w.name));
    const root = (w) => {
      const routine = (t.routines || []).some((r) => r.to === w.name);
      const tag = B.el("span", {class: "tag"}, routine ? "Routine" : "Unreached");
      return B.el("li", {}, opened("tree:" + w.name, "lt-node", B.el("summary", {}, workerRow(t, w, tag)),
        B.el("div", {}, problemEls(t, w.name), toolsOf(t, w, new Set([w.name])))));
    };
    return B.el("div", {class: "lt-tree"},
      lead ? B.el("ul", {class: "lt lt-roots"}, B.el("li", {}, workerRow(t, lead, B.el("span", {class: "tag"}, "Lead")), problemEls(t, lead.name), toolsOf(t, lead, new Set([lead.name])))) : null,
      roots.length ? paged("roots", roots, root, ROOTS, B.el("ul", {class: "lt lt-roots"}), (b) => B.el("li", {}, b)) : null);
  }

  function runsEl(t) {
    if (!t.in || !t.at_work || !t.at_work.length) return null;
    return B.el("div", {class: "stack band-list"}, t.at_work.map((a) => B.el("span", {class: "line"},
      worker(t, a.worker), B.el("a", {class: "mono", href: "#/r/" + a.request}, "#" + a.request),
      B.el("a", {class: "btn sm ghost", href: "#/run/" + encodeURIComponent(a.run), "aria-label": "Watch " + a.worker + " on #" + a.request}, "Watch"))));
  }

  // Out: whom the lead may ask, who authorises what waits, who discloses
  // what leaves, and where the answer is said.

  function asksEl(t) {
    const lead = byName(t, t.lead);
    if (!lead) return null;
    return lead.may_ask.length ? B.el("span", {class: "line"}, lead.may_ask.map(B.teamEl)) : B.el("span", {class: "faint"}, "None");
  }

  function authorisationEl(t) {
    if (!t.decide || !t.decide.length) return null;
    return B.el("div", {class: "stack band-list"},
      rows("authorisation", t.decide, (l) => B.el("div", {class: "row te-list"}, B.el("span", {class: "mono te-key"}, l.key), B.chain(l.deciders || [], t.name)), "te-lists"));
  }

  function disclosureEl(t) {
    if (t.disclosure && t.disclosure.length) return B.chain(t.disclosure, t.name);
    return B.el("span", {class: "faint"}, "None");
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

  function routines(t) {
    if (!t.routines || !t.routines.length) return null;
    return B.sect("Routines", rows("routines", t.routines, (r) => {
      const button = t.in ? B.el("button", {class: "btn sm ghost", type: "button", "aria-label": "Run now: " + r.to + ", " + r.at}, "Run now") : null;
      if (button) button.addEventListener("click", () => runNow(t, r, button));
      return B.el("div", {class: "row te-rt"},
        B.el("span", {class: "line te-at"}, B.el("span", {class: "mono"}, r.at), B.el("span", {class: "faint small"}, "(" + r.zone + ")")),
        B.el("span", {class: "te-to"}, worker(t, r.to)),
        button || B.el("span", {}),
        B.el("span", {class: "te-say clip", title: r.say}, r.say),
        B.el("span", {class: "te-rt-foot line"},
          routineStatus(r),
          r.next ? B.el("span", {class: "line"}, B.el("span", {class: "faint small"}, "Next"), B.whenEl(r.next)) : null,
          B.el("span", {class: "line"}, B.el("span", {class: "faint small"}, "Answer"), "People of " + B.teamCalled(t.name))));
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

  const ruleWords = {heed: "Heed", disclosure: "Disclosure"};

  function rules(t) {
    if (!t.rules || !t.rules.length) return null;
    return B.sect("Rules", rows("rules", t.rules, (r) => B.el("div", {class: "row te-rule"},
      B.el("span", {class: "line"}, B.ruleEl(r.path), r.shipped ? B.el("a", {class: "tag", href: "#/shipped"}, "Shipped") : null,
        r.runs ? null : B.el("span", {class: "status failed"}, "stopped")),
      B.el("span", {class: "line"}, r.as.map((a) => ruleWords[a] ? B.el("span", {class: "tag"}, ruleWords[a]) : B.el("span", {class: "tag mono"}, a))),
      r.script ? B.el("span", {class: "faint small"}, "Script") : B.reachEl(r.reach)), "te-rules"));
  }

  function notes(t) {
    if (!t.notes || !t.notes.length) return null;
    return B.sect(B.label("Notes", B.el("span", {class: "count"}, String(t.notes.length)),
      ...B.notesInfo()),
    rows("notes", t.notes, (n) => {
      const d = B.el("details", {open: state.open.has(n.name)},
        B.el("summary", {}, B.el("span", {class: "mono"}, n.name),
          B.el("span", {class: "mono faint te-size"}, n.size.toLocaleString("en-GB") + " characters"), B.givenEl(n)),
        B.el("pre", {}, n.text));
      d.addEventListener("toggle", () => { if (d.open) state.open.add(n.name); else state.open.delete(n.name); });
      return B.el("div", {class: "row te-note"}, d);
    }, "te-notes"));
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
    const crumbs = [B.el("a", {href: "#/teams"}, "Teams"), ...within(t).map((u) => B.el("a", {href: B.teamHref(u.name)}, u.called))];
    const head = B.head(crumbs, t.called,
      B.el("a", {class: "te-reqs", href: "#/requests?team=" + encodeURIComponent(t.name)}, "Requests"),
      t.about ? B.el("p", {class: "te-about"}, t.about) : null);

    const whole = Array.isArray(t.workers);
    const channel = B.placesOf(t.name).some((x) => x.place.name !== "direct");
    const team = B.el("section", {class: "room te-room", "aria-label": t.within ? "Team" : "Office"},
      B.el("div", {class: "plate"}, t.within ? B.el("b", {}, "Team")
        : [B.el("b", {}, "Office"), ...B.info("About the office", "The top team, written in office.yaml. A decision nobody else takes ends at its everything.")]),
      B.el("div", {class: "bands"},
        B.band("In", [
          ["Asked", B.el("div", {class: "stack band-list"}, B.asksIn(t.name))],
          channel || t.heed ? [B.heedLabel(t), B.heedEl(t), B.settingsEl(t.heed && t.heed.settings)] : null,
        ]),
        B.band("Team", [
          ["People", people(t) || (t.reached && t.reached.length ? null : B.empty())],
          !(t.people && t.people.length) && t.reached && t.reached.length
            ? [B.el("span", {class: "line"}, "Reached", ...B.info("About reached", "The people this team's decisions reach. They stand as its people.")),
              B.el("span", {class: "line te-people te-reached"}, t.reached.map((n) => B.personEl(n)))] : null,
          [B.leadLabel(t), B.el("span", {class: "line"}, leadEl(t), t.lead_runs ? null : B.el("span", {class: "status failed"}, "cannot run"))],
          whole ? ["Workers", tree(t)] : null,
          whole ? ["Runs", runsEl(t)] : null,
          ["Within", t.within ? B.teamEl(t.within) : null],
          ["Teams inside", inside(t)],
          ["File", whole && t.file ? B.el("span", {class: "mono"}, t.file) : null],
        ]),
        B.band("Out", [
          whole ? ["Asks", asksEl(t)] : null,
          whole ? [B.el("span", {class: "line"}, "Authorisation", ...B.info("About authorisation", "Asked in order. A yes or a refusal ends it; a refer goes on, with its note, to whoever is next.")), authorisationEl(t)] : null,
          whole ? [B.disclosureLabel(t), disclosureEl(t)] : null,
          ["Answer", B.answersTo(t.name)],
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
