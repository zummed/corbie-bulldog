// One team, as the way a request goes through it, downward: where it is
// asked, its heed, the team, what its lead calls on, its disclosure and
// where its answer goes back; each stage unfolds its setup on asking, and
// beside it, to the team's people, what is in it now. Beneath the stages,
// to its people, its problems, and folded, its routines, models, what its
// rules reach and its notes, and its docs to those who may open them. A
// team may have hundreds of people and many workers, so long lists are
// paged and searched.
(function () {
  "use strict";

  const PEOPLE = 12;    // people shown before More
  const INSIDE = 12;    // teams inside shown before +N
  const ROOTS = 50;     // workers beside the lead shown before More
  const PAGE = 20;      // tools, routines, notes and lists shown before More
  const NOW = 3;        // things in a stage now shown before +N
  const FEW = 2;        // of the parts of what the lead calls on, how many are named before N more

  // What the view keeps while it is drawn again, for one team: what is
  // searched for, how many of each list are shown, which stages, notes,
  // docs and folders of docs are open; and what is in its stages now that
  // was drawn before, each with when it came in, so that only what is new
  // comes in, and none on the first drawing.
  const state = {team: "", q: new Map(), shown: new Map(), open: new Set(), seen: new Map(), drawn: false};
  function keepFor(team) {
    if (state.team !== team) Object.assign(state, {team, q: new Map(), shown: new Map(), open: new Set(), seen: new Map(), drawn: false});
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
  // by their kits, and the names of its own tools, each leading to it on
  // the worker's page; workers the lead does not reach stand beside it as
  // roots of their own. Each tool's kind, what it does and its file are
  // on the worker's own page.

  function byName(t, name) { return (t.workers || []).find((x) => x.name === name); }

  // problemEls gives a worker's problems, said on its row.
  function problemEls(t, name) {
    return (t.problems || []).filter((p) => p.worker === name).map((p) => B.el("div", {class: "mono te-problem lt-problem"}, p.text));
  }

  // roleEl gives how a worker stands in the tree, said faint after its
  // name: the lead, with its (i), a colleague, one a routine starts, or
  // one nothing starts.
  function roleEl(t, role) {
    if (!role) return null;
    if (role === "lead") return B.el("span", {class: "line lt-role"}, ...B.leadLabel(t, "lead").childNodes);
    if (role === "unused") return B.el("span", {class: "line lt-role"}, "unused", ...B.info("About unused",
      "Not the lead, called by no worker of " + t.called + " and started by no routine, so nothing runs it."));
    return B.el("span", {class: "lt-role"}, role);
  }

  // workerRow gives a worker on one line: its name, how it stands in the
  // tree, the lock of what waits for a yes in it, its one status, cannot
  // run or at work, with each request it is at work on, to watch.
  function workerRow(t, w, role, lock) {
    const busy = B.atWork(t).filter((a) => a.worker === w.name);
    return B.el("span", {class: "line lt-row"},
      B.workerEl(w.name, t.name, {bare: true}), roleEl(t, role), lock,
      w.runs === false ? B.el("span", {class: "status failed"}, "cannot run") : busy.length ? B.status({state: "working"}) : null,
      w.runs !== false && t.in ? busy.map((a) => B.el("a", {class: "mono small", href: "#/run/" + encodeURIComponent(a.run), title: "Watch " + w.name + " on #" + a.request, "aria-label": "Watch " + w.name + " on #" + a.request}, "#" + a.request)) : null);
  }

  // lockOf gives a small lock that says, when asked, what waits for a
  // yes: a tool, with who decides it, or that nobody does; or a colleague
  // that holds tools that wait.
  let locks = 0;
  function lockOf(said) {
    const id = "lt-lock-" + ++locks;
    return [B.el("button", {class: "lt-lock", type: "button", popovertarget: id, "aria-label": said}, B.icon("lock")),
      B.el("div", {class: "pop", id, popover: "auto"}, said)];
  }
  function lockEl(t, tl) {
    if (!tl.waits) return null;
    const who = (tl.deciders || []).map((d) => d.kind === "person" ? B.personCalled(d.name) : d.kind === "bulldog" ? "Bulldog" : d.name);
    return lockOf(tl.name + " waits for a yes: " + (who.length ? who.join(", then ") : "refused, since nobody decides it"));
  }

  // holdsWaiting says whether anything a worker runs, or its colleagues
  // run, as far down as they go, waits for a yes.
  function holdsWaiting(t, w, seen) {
    return (w.tools || []).some((tl) => {
      if (tl.waits) return true;
      const c = tl.kind === "worker" ? byName(t, tl.worker) : null;
      return c && !seen.has(c.name) && holdsWaiting(t, c, new Set(seen).add(c.name));
    });
  }

  function nameEl(t, w, tl) {
    return B.el("span", {class: "line lt-tool"},
      B.el("a", {class: "mono lt-name", href: B.workerHref(t.name, w.name) + "?tool=" + encodeURIComponent(tl.name)}, tl.listed ? tl.name + "_*" : tl.name),
      lockEl(t, tl));
  }

  // colleagueNode gives a colleague beneath the worker that calls it,
  // holding its own beneath it, folded, unless it is already above it or
  // holds nothing.
  function colleagueNode(t, w, tl, c, above) {
    const lock = lockEl(t, tl) || (holdsWaiting(t, c, new Set(above).add(c.name)) ? lockOf(c.name + " holds tools that wait for a yes") : null);
    const row = workerRow(t, c, "colleague", lock);
    const body = above.has(c.name) ? null : [...problemEls(t, c.name), under(t, c, new Set(above).add(c.name))].filter(Boolean);
    if (!body || !body.length) return B.el("li", {class: "lt-leaf"}, row);
    return B.el("li", {}, opened("tree:" + w.name + "/" + tl.name, "lt-node", B.el("summary", {}, row), B.el("div", {}, body)));
  }

  // colleaguesOf gives the colleagues a worker calls, each with the tool
  // that calls it. A worker Corbie refused has no tools to tell them by,
  // so they are the workers whose files it reads that none of the others
  // reads in turn.
  function colleaguesOf(t, w) {
    if ((w.tools || []).length || w.runs !== false) {
      return (w.tools || []).filter((tl) => tl.kind === "worker" && byName(t, tl.worker)).map((tl) => [tl, byName(t, tl.worker)]);
    }
    const read = (t.workers || []).filter((x) => x.name !== w.name && (w.uses || []).includes(x.file));
    return read.filter((x) => !read.some((y) => y !== x && (y.uses || []).includes(x.file)))
      .map((x) => [{name: x.name, kind: "worker", worker: x.name}, x]);
  }

  // under gives what is beneath a worker: its colleagues, a line of what
  // it does with Bulldog's tools, and a line of its own tools' names.
  function under(t, w, above) {
    const colleagues = colleaguesOf(t, w), kitted = new Map(), own = [];
    for (const tl of w.tools || []) {
      if (tl.kind === "worker" && byName(t, tl.worker)) continue;
      if (tl.kind === "bulldog" && B.KITS[tl.kit]) kitted.set(tl.kit, [...(kitted.get(tl.kit) || []), tl]);
      else own.push(tl);
    }
    if (!colleagues.length && !kitted.size && !own.length) return null;
    const kitEls = Object.keys(B.KITS).filter((k) => kitted.has(k)).map((k) => B.el("span", {class: "line lt-kit"},
      B.el("a", {class: "lt-name", href: B.workerHref(t.name, w.name) + "?tool=" + encodeURIComponent(kitted.get(k)[0].name), title: kitted.get(k).map((tl) => tl.name).join(", ")}, B.KITS[k]),
      kitted.get(k).map((tl) => lockEl(t, tl))));
    return B.el("div", {class: "lt-under"},
      colleagues.length ? paged("colleagues of " + w.name, colleagues, ([tl, c]) => colleagueNode(t, w, tl, c, above), PAGE, B.el("ul", {class: "lt"}), (b) => B.el("li", {}, b)) : null,
      kitEls.length ? B.el("span", {class: "line lt-kits"}, kitEls.flatMap((e, i) => [i ? B.el("span", {class: "faint", "aria-hidden": "true"}, "·") : null, e])) : null,
      own.length ? paged("tools of " + w.name, own, (tl) => nameEl(t, w, tl), PAGE, B.el("span", {class: "line lt-own"}), (b) => b) : null);
  }

  // reached gives the workers a worker calls, as far down as they go.
  function reached(t, w, into) {
    for (const [, c] of colleaguesOf(t, w)) {
      if (!into.has(c.name)) { into.add(c.name); reached(t, c, into); }
    }
    return into;
  }

  // tree gives the lead, open, with all it calls beneath it, and beside it
  // each worker it does not reach, as one a routine starts or unused.
  function tree(t) {
    const workers = t.workers || [];
    if (!workers.length) return null;
    const lead = byName(t, t.lead);
    const near = lead ? reached(t, lead, new Set([lead.name])) : new Set();
    const roots = workers.filter((w) => !near.has(w.name));
    const root = (w) => {
      const routine = (t.routines || []).some((r) => r.to === w.name);
      const body = [...problemEls(t, w.name), under(t, w, new Set([w.name]))].filter(Boolean);
      const row = workerRow(t, w, routine ? "routine" : "unused");
      return body.length ? B.el("li", {}, opened("tree:" + w.name, "lt-node", B.el("summary", {}, row), B.el("div", {}, body)))
        : B.el("li", {class: "lt-leaf"}, row);
    };
    return B.el("div", {class: "lt-tree"},
      lead ? B.el("ul", {class: "lt lt-roots"}, B.el("li", {}, workerRow(t, lead, "lead"),
        problemEls(t, lead.name), under(t, lead, new Set([lead.name])))) : null,
      roots.length ? paged("roots", roots, root, ROOTS, B.el("ul", {class: "lt lt-roots"}), (b) => B.el("li", {}, b)) : null);
  }

  // What the lead calls on, in full: whom it may ask and who authorises
  // what waits, each when there is any.

  // asked gives the teams the lead may ask.
  function asked(t) {
    const lead = byName(t, t.lead);
    return lead ? lead.may_ask : [];
  }

  function authorisationEl(t) {
    if (!t.decide || !t.decide.length) return null;
    return B.el("div", {class: "stack band-list"},
      rows("authorisation", t.decide, (l) => B.el("div", {class: "row te-list"}, B.el("span", {class: "mono te-key"}, l.key), B.chain(l.deciders || [], t.name)), "te-lists"));
  }

  // The stages a request to the team may go through, downward: where it
  // is asked, its heed, the team, what its lead calls on, its disclosure,
  // and where its answer goes back. A request need not go through each in
  // turn: a heed weighs only what is said at a channel place, and a run
  // calls on what it needs, asks and waits any number of times, and runs
  // again when more is said. Each stage unfolds its setup on asking, and
  // beside it, to the team's people, is what is in it now.

  // named gives names on one line, the first few, then how many more.
  function named(list, few) {
    return list.length > few ? list.slice(0, few).join(" · ") + " · " + (list.length - few) + " more" : list.join(" · ");
  }

  // shortRule gives a rule by the last part of its path, without its kind.
  function shortRule(path) { return String(path).split("/").pop().replace(/\.[^.]+$/, ""); }

  // deciderWord gives a decider in a word or two.
  function deciderWord(d) {
    if (d.kind === "person") return B.partyWord(d);
    if (d.kind === "rule") return shortRule(d.name);
    return d.kind === "bulldog" ? "Bulldog" : d.name;
  }

  // infoOf gives the (i) of one of a team's labels, without its word.
  function infoOf(label) { return [...label.querySelectorAll(".info, .pop")]; }

  // block gives a part of a stage's setup too big for a row: its word
  // over it.
  function block(word, body) {
    return body ? B.el("div", {class: "pp-block"}, B.el("div", {class: "pp-k"}, word), body) : null;
  }

  // ways gives where the team is asked, in a line: the places of each
  // bridge after it, then the chat and another team's request.
  function ways(places) {
    const by = new Map();
    for (const x of places) by.set(x.bridge, [...(by.get(x.bridge) || []), x.place.name]);
    const way = (text) => B.el("span", {class: "pp-way"}, text);
    return [...[...by].map(([bridge, names]) => way(bridge + " › " + names.join(", "))), way("Chat"), way("Sub-request")];
  }

  // waysOf gives where the team is asked: each place of a bridge, with
  // what is said of it, then the chat and another team's request.
  function waysOf(t, place, chat, sub) {
    return B.kv([...B.placesOf(t.name).map((x) => [B.placeEl(x.bridge, x.place.name), place(x)]),
      [B.el("span", {class: "who"}, B.icon("chat"), "Chat"), chat],
      ["Sub-request", sub]]);
  }

  // stages gives the stages of the team as the one looking may see them,
  // each its id, its word, its name and a little more in its box, its
  // parts, always shown, and its setup.
  function stages(t, whole) {
    const places = B.placesOf(t.name);
    const channel = places.filter((x) => x.place.name !== "direct");
    const down = places.find((x) => x.status !== "running");
    const out = [{id: "source", word: "Source", name: "Where it is asked", sub: ways(places),
      also: down ? B.bridgeStatus({status: down.status}) : null,
      setup: waysOf(t, (x) => B.el("span", {class: "line"}, x.place.guests ? "Guests may ask" : "People of the office",
        x.status !== "running" ? B.bridgeStatus({status: x.status}) : null), "On the screens", "By another team, through its ask_team")}];

    if (channel.length || t.heed) {
      const h = t.heed || {};
      const rule = h.rule ? (t.rules || []).find((r) => r.path === h.rule) : null;
      const short = (h.settings || []).filter((x) => x.value != null && typeof x.value !== "object" && String(x.value).length <= 24).map((x) => String(x.value));
      out.push({id: "heed", word: "Heed", check: true, name: !t.heed ? "None" : h.rule ? shortRule(h.rule) : "Set", sub: short.join(" · "),
        also: h.stopped ? B.el("span", {class: "status failed"}, "stopped") : null,
        info: infoOf(B.heedLabel(t)),
        setup: [B.kv([
          h.rule ? ["Rule", B.el("span", {class: "line"}, B.ruleEl(h.rule), h.shipped ? B.el("a", {class: "faint small", href: "#/office/bulldog"}, "From Bulldog") : null)] : null,
          channel.length ? ["Only at", B.el("span", {class: "line"}, channel.map((x) => B.placeEl(x.bridge, x.place.name)))] : null,
        ]), B.settingsEl(h.settings), rule && !rule.script ? B.reachEl(rule.reach) : null]});
    }

    const folk = t.people && t.people.length ? t.people : t.reached || [];
    out.push({id: "team", word: "Team", name: B.leadCalled(t), sub: named(folk.map((n) => B.partyWord({kind: "person", name: n})), 3),
      also: t.lead_runs ? null : B.el("span", {class: "status failed"}, "cannot run"),
      info: infoOf(B.leadLabel(t)),
      setup: B.kv([
        t.people && t.people.length ? ["People", people(t)] : null,
        !(t.people && t.people.length) && t.reached && t.reached.length
          ? [B.el("span", {class: "line"}, "Reached", ...B.info("About reached", "The people this team's decisions reach. They stand as its people.")),
            B.el("span", {class: "line te-people te-reached"}, t.reached.map((n) => B.personEl(n)))] : null,
        ["Lead", leadEl(t)],
        ["Teams inside", inside(t)],
        whole && (t.notes.length || t.writers.length) ? ["Notes", B.el("span", {class: "line"}, String(t.notes.length),
          t.writers.length ? [B.el("span", {class: "faint small"}, "written by"), t.writers.map((w) => worker(t, w))] : null)] : null,
        t.docs && t.docs.length ? ["Docs", String(t.docs.length)] : null,
        whole && t.file ? ["File", B.el("span", {class: "mono"}, t.file)] : null,
      ])});

    const lead = whole ? byName(t, t.lead) : null;
    if (whole && (lead || (t.decide || []).length || (t.workers || []).length)) {
      const near = lead ? [...reached(t, lead, new Set([lead.name]))].map((n) => byName(t, n)) : [];
      const holding = (name) => near.filter((w) => (w.tools || []).some((tl) => tl.kind === "bulldog" && tl.name === name));
      const tools = lead ? lead.tools || [] : [];
      const own = tools.filter((tl) => tl.kind !== "bulldog" && !(tl.kind === "worker" && byName(t, tl.worker))).map((tl) => tl.listed ? tl.name + "_*" : tl.name);
      const kits = Object.keys(B.KITS).filter((k) => tools.some((tl) => tl.kind === "bulldog" && tl.kit === k)).map((k) => B.KITS[k]);
      const keys = (t.decide || []).map((l) => l.key + ": " + (l.deciders || []).map(deciderWord).join(", then "));
      const parts = [
        lead ? ["Colleagues", named(colleaguesOf(t, lead).map(([, c]) => c.name), 3)] : null,
        ["Tools", named(own, FEW)],
        ["Bulldog's", named(kits, FEW)],
        ["Authorisation", keys.length ? B.el("span", {class: "pp-keys"}, keys.slice(0, FEW + 1).map((k) => B.el("span", {}, k)),
          keys.length > FEW + 1 ? B.el("span", {class: "faint"}, keys.length - FEW - 1 + " more") : null) : null, true],
        holding("ask_person").length ? ["People", named([...folk.map((n) => B.partyWord({kind: "person", name: n})), "whoever asked"], 3)] : null,
        ["Other teams", named(asked(t).map(B.teamCalled), 3)],
      ].filter((p) => p && p[1]);
      out.push({id: "handoff", word: "Handoff", name: lead ? "What " + lead.name + " calls on" : "What its workers call on", parts,
        setup: [block("Workers", tree(t)),
          block(B.el("span", {class: "line"}, "Authorisation", ...B.info("About authorisation", "Asked in order. A yes or a refusal ends it; a refer goes on, with its note, to whoever is next.")), authorisationEl(t)),
          block("Other teams", asked(t).length ? B.el("span", {class: "line"}, asked(t).map(B.teamEl)) : null)]});
    }

    if ((t.disclosure && t.disclosure.length) || t.discloses) {
      const ds = t.disclosure || [];
      out.push({id: "disclosure", word: "Disclosure", check: true, name: ds.length ? "Asked of " + deciderWord(ds[0]) : "Set",
        sub: ds.length > 1 ? "then " + ds.slice(1).map(deciderWord).join(", then ") : "",
        info: infoOf(B.disclosureLabel(t)),
        setup: ds.length ? B.kv([["Asked", B.chain(ds, t.name)], ["Not needed for", "People of " + t.called]]) : null});
    }

    out.push({id: "answer", word: "Answer", name: "Back where it was asked", sub: ways(places),
      setup: waysOf(t, () => "Where it was asked", "On the screens", "To the team that asked")});
    return out;
  }

  // What a disclosure would let out, in a word or two.
  const LEAVING = {answer: "its answer", question: "a question", tell: "a tell", sub: "a sub-request"};

  // nowItems gives what is in the team's stages now, each with the stage
  // it is beside, its key, kept across drawings, how it goes, working,
  // waiting on someone, pending or done, its request, whom it waits on,
  // what it is, and the request's first line. A run held while its
  // request waits is not at work: what it waits on is beside its stage.
  function nowItems(t) {
    const firstOf = new Map((t.now || []).map((x) => [x.request, x.first]));
    const who = (ps) => (ps || []).map(B.partyWord).join(", ");
    const asked = (names) => (names || []).map((n) => B.partyWord({kind: "person", name: n})).join(", ");
    const out = B.atWork(t).map((a) => ({stage: "team", key: "work:" + a.request + ":" + a.run, cls: "working", request: a.request, text: a.worker, first: firstOf.get(a.request)}));
    for (const x of t.now || []) {
      const it = {key: x.kind + ":" + x.request + ":" + (x.entry || x.decision || x.sub || ""), request: x.request, first: x.first};
      switch (x.kind) {
        case "queued": Object.assign(it, {stage: "source", cls: "pending", text: "queued " + x.queued}); break;
        case "heed": Object.assign(it, {stage: "heed", cls: "working", text: shortRule(x.rule)}); break;
        case "lead": Object.assign(it, {stage: "team", cls: "waiting", who: who(x.on)}); break;
        case "question": Object.assign(it, {stage: "handoff", cls: "waiting", who: who(x.to), text: B.partyWord(x.by) + " asks"}); break;
        case "sub": Object.assign(it, {stage: "handoff", cls: x.state === "waiting" ? "waiting" : "working", who: x.state === "waiting" ? who(x.on) : "", text: "→ " + B.teamCalled(x.team) + " #" + x.sub}); break;
        case "disclosure": Object.assign(it, {stage: "disclosure", cls: "waiting", who: asked(x.asked), text: LEAVING[x.what] || x.what}); break;
        case "answered": Object.assign(it, {stage: "answer", cls: "done", text: x.label || ""}); break;
        default: Object.assign(it, {stage: "handoff", cls: "waiting", who: asked(x.asked), text: x.does});
      }
      out.push(it);
    }
    return out;
  }

  // itemEl draws one thing in a stage now, leading to its request: whom
  // it waits on whole, what it is and the request's first line cut short;
  // one coming in, as far as it came.
  function itemEl(it, now) {
    return B.comeIn(B.el("a", {class: "pp-item " + it.cls, href: "#/r/" + it.request, title: it.first || null},
      B.el("i", {class: "dot", "aria-hidden": "true"}),
      B.el("span", {class: "mono pp-n"}, "#" + it.request),
      it.who ? B.el("span", {class: "pp-who"}, it.who) : null,
      it.text ? B.el("span", {class: "pp-what"}, it.text) : null,
      it.first ? B.el("span", {class: "pp-first"}, it.first) : null), state.seen, it.key, state.drawn, now);
  }

  // stageEl draws one stage: its word beside the line, its node on it, its
  // box, which unfolds its setup when it has one, and what is in it now,
  // the first few, then how many more, leading to the team's requests.
  function stageEl(t, s, items, now) {
    const head = B.el("span", {class: "pp-head"},
      B.el("span", {class: "pp-name"}, s.name),
      s.sub ? B.el("span", {class: "mono pp-sub"}, s.sub) : null,
      s.also, s.info);
    const parts = s.parts && s.parts.length ? B.el("span", {class: "pp-parts"}, s.parts.map(([kind, name, check]) =>
      B.el("span", {class: "pp-part"}, B.el("span", {class: "chip" + (check ? " check" : "")}, kind), B.el("span", {class: "pp-pname"}, name)))) : null;
    const setup = [s.setup].flat(Infinity).filter(Boolean);
    const cls = "pp-box" + (s.check ? " check" : "");
    const box = setup.length ? opened("stage:" + s.id, cls, B.el("summary", {}, head, parts), B.el("div", {class: "pp-setup"}, setup))
      : B.el("div", {class: cls}, B.el("div", {class: "pp-top"}, head, parts));
    return B.el("section", {class: "stage" + (items.some((it) => it.cls !== "done") ? " busy" : ""), "aria-label": s.word},
      B.el("span", {class: "pp-step", "aria-hidden": "true"}, s.word),
      B.el("span", {class: "line", "aria-hidden": "true"}),
      box,
      items.length ? B.el("div", {class: "pp-items"}, items.slice(0, NOW).map((it) => itemEl(it, now)),
        items.length > NOW ? B.el("a", {class: "mono pp-left", href: "#/requests?team=" + encodeURIComponent(t.name)}, "+" + (items.length - NOW)) : null) : null);
  }

  // pipeEl draws the stages on their line, which trickles on while
  // anything in them is at work; what is in a stage the team has not, as
  // a decision of a routine's worker, is beside the team.
  function pipeEl(t, whole) {
    const now = Date.now();
    const items = t.in ? nowItems(t) : [];
    const list = stages(t, whole);
    const ids = new Set(list.map((s) => s.id));
    const els = list.map((s) => stageEl(t, s, items.filter((it) => (ids.has(it.stage) ? it.stage : "team") === s.id), now));
    const shown = new Set(items.map((it) => it.key));
    for (const key of state.seen.keys()) if (!shown.has(key)) state.seen.delete(key);
    state.drawn = true;
    const pipe = B.keepBeat(B.el("div", {class: "flow pipe" + (items.some((it) => it.cls === "working") ? " live" : "")}, els), now);
    return B.el("section", {class: "pp-wrap", "aria-label": "How a request goes through " + t.called}, pipe);
  }

  // Beneath the stages: its problems, and folded, how else it is written

  // folded gives a part beneath the stages, folded until it is opened,
  // kept open by key while the view is drawn again.
  function folded(key, label, body) {
    return opened("part:" + key, "te-part", B.el("summary", {}, typeof label === "string" ? B.label(label) : label), body);
  }

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
    return folded("routines", B.label("Routines", B.el("span", {class: "count"}, String(t.routines.length))), rows("routines", t.routines, (r) => {
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
    return folded("models", B.label("Models", B.el("span", {class: "count"}, String(t.models.length))), B.el("div", {class: "scroll"}, B.el("table", {class: "table te-models"},
      B.el("thead", {}, B.el("tr", {}, ["Name", "Model", "Secret"].map((h) => B.el("th", {scope: "col"}, h)))),
      B.el("tbody", {}, t.models.map((m) => B.el("tr", {},
        B.el("td", {}, m.name),
        B.el("td", {}, B.modelText(m.text) || m.text),
        B.el("td", {}, m.secret || B.el("span", {class: "faint"}, "None"))))))));
  }

  // rules gives what the team's rules say that its stages do not: of
  // each it names in a list or as its disclosure, by its path, whether it
  // comes with Bulldog, whether it is stopped, and that it is a bare
  // script, or what it reaches. The heed's rule is said on the heed.
  function rules(t) {
    const list = (t.rules || []).filter((r) => r.as.some((a) => a !== "heed"));
    if (!list.length) return null;
    return folded("rules", B.label("Rules", B.el("span", {class: "count"}, String(list.length))), rows("rules", list, (r) => B.el("div", {class: "row te-rule"},
      B.el("span", {class: "line"}, B.ruleEl(r.path),
        r.shipped ? B.el("a", {class: "faint small", href: "#/office/bulldog"}, "From Bulldog") : null,
        r.script ? B.el("span", {class: "faint small"}, "script") : null,
        r.runs ? null : B.el("span", {class: "status failed"}, "stopped")),
      r.script ? null : B.reachEl(r.reach)), "te-rules"));
  }

  // notes gives the team's notes, and the workers that may write them.
  function notes(t) {
    const writers = t.writers || [];
    if (!(t.notes && t.notes.length) && !writers.length) return null;
    return folded("notes", B.label("Notes", B.el("span", {class: "count"}, String(t.notes.length)),
      ...B.notesInfo()), B.el("div", {},
    writers.length ? B.el("span", {class: "line te-writers"}, B.el("span", {class: "faint small"}, "Written by"),
      writers.map((w) => worker(t, w)), ...B.info("About written by", "The workers of this team whose definitions name the notes tool. Each may write any of its notes, at once, with no decision.")) : null,
    t.notes.length ? rows("notes", t.notes, (n) => {
      const d = B.el("details", {open: state.open.has(n.name)},
        B.el("summary", {}, B.el("span", {class: "mono"}, n.name),
          B.el("span", {class: "mono faint te-size"}, n.size.toLocaleString("en-GB") + " characters"), B.givenEl(n)),
        B.el("pre", {}, n.text));
      d.addEventListener("toggle", () => { if (d.open) state.open.add(n.name); else state.open.delete(n.name); });
      return B.el("div", {class: "row te-note"}, d);
    }, "te-notes") : null));
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
    return folded("docs", B.label("Docs", B.el("span", {class: "count"}, String(t.docs.length)),
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
      whole && t.file ? B.el("span", {class: "mono faint small"}, t.file) : null,
      t.about ? B.el("p", {class: "te-about"}, t.about) : null);
    const beneath = (whole ? [problems(t), routines(t), models(t), rules(t), notes(t), docs(t)] : [docs(t)]).filter(Boolean);
    const out = B.el("div", {class: "te"}, head, pipeEl(t, whole),
      beneath.length ? B.el("div", {class: "te-beneath"}, beneath) : null);
    refocus(out, at);
    return out;
  }, "teams");
})();
