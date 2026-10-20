// Teams: the teams as they sit in each other, the top team the room around
// them, and the people. An office may have hundreds of each, so the tree is
// folded but for the way to the one looking's own teams, lists are paged,
// and a search shows what it finds flat, each team with the teams around it.
(function () {
  "use strict";

  const PAGE = 50;

  // What the view keeps while it is drawn again: the teams opened in the
  // tree, how many of each list are shown, and what is searched for, which
  // is forgotten when the one looking goes elsewhere.
  const state = {open: null, shown: new Map(), teamQ: "", peopleQ: ""};

  // The view last drawn, and the office it was drawn from. The view shows
  // nothing of requests, so a change that leaves the teams and people as
  // they were gives back the same view, with what was typed in it.
  let drawn = null;
  addEventListener("hashchange", () => {
    if (/^#\/teams$/.test(location.hash)) return;
    state.teamQ = ""; state.peopleQ = "";
    drawn = null;
  });

  // office reads the teams and people once for each office drawn: which
  // teams sit in which, the teams around each, the one looking's teams with
  // those around them, which come first wherever teams are listed, and the
  // text each team and person is searched by.
  function office() {
    const teams = B.me.teams || [];
    const byName = new Map(teams.map((t) => [t.name, t]));
    const folk = new Map((B.me.people || []).map((p) => [p.name, p]));
    const called = (n) => { const p = folk.get(n); return p ? p.called : n; };
    const around = (t) => {
      const out = [], seen = new Set([t.name]);
      for (let u = byName.get(t.within); u && !seen.has(u.name); u = byName.get(u.within)) { seen.add(u.name); out.unshift(u); }
      return out;
    };
    const mine = new Set(B.me.me.teams || []);
    const near = new Set();
    for (const n of mine) {
      near.add(n);
      const t = byName.get(n);
      if (t) for (const u of around(t)) near.add(u.name);
    }
    const order = (a, b) => (near.has(b.name) - near.has(a.name)) || a.called.localeCompare(b.called);
    const inside = new Map();
    for (const t of teams) {
      const up = t.within && byName.has(t.within) ? t.within : "";
      if (!inside.has(up)) inside.set(up, []);
      inside.get(up).push(t);
    }
    for (const list of inside.values()) list.sort(order);
    if (!state.open) {
      state.open = new Set();
      for (const n of mine) { const t = byName.get(n); if (t) for (const u of around(t)) state.open.add(u.name); }
    }

    const teamText = new Map(teams.map((t) => [t, [t.called, t.name, t.about, ...(t.people || []).map(called)].join(" ")]));
    const pathOf = new Map(teams.map((t) => [t, around(t).concat(t).map((u) => u.called).join(" / ")]));
    const teamsOf = new Map();
    for (const t of teams) for (const p of t.people || []) {
      if (!teamsOf.has(p)) teamsOf.set(p, []);
      teamsOf.get(p).push(t);
    }
    const mineFirst = (a, b) => (mine.has(b.name) - mine.has(a.name)) || a.called.localeCompare(b.called);
    for (const list of teamsOf.values()) list.sort(mineFirst);
    const all = (B.me.people || []).slice().sort((a, b) => (B.isMe(b.name) - B.isMe(a.name)) || a.called.localeCompare(b.called));
    const personText = new Map(all.map((p) => [p, [p.called, p.name, p.title, ...(p.bridges || []), ...(teamsOf.get(p.name) || []).map((t) => t.called)].join(" ")]));
    return {teams, inside, around, mine, folk, called, teamText, pathOf, teamsOf, all, personText};
  }

  function more(key, left, onclick) {
    return B.el("button", {class: "btn sm ghost tm-page", type: "button", "data-k": "more " + key, onclick}, "More", B.el("span", {class: "mono faint"}, String(left)));
  }

  // paged fills box with the first of the items, and More the next. How
  // many are shown is kept by key while the same is searched for; after
  // More, the first of those it shows has the focus.
  function paged(box, key, q, items, draw, tag) {
    const fill = (from) => {
      const s = state.shown.get(key);
      const n = s && s.q === q ? s.n : PAGE;
      const rows = items.slice(0, n).map(draw);
      box.replaceChildren(...rows,
        ...(items.length > n ? [B.el(tag, {class: "row"}, more(key, items.length - n, () => { state.shown.set(key, {q, n: n + PAGE}); fill(n); }))] : []));
      if (from !== undefined && rows[from]) {
        let f = rows[from].querySelector("a[href], button");
        if (!f) { f = rows[from]; f.tabIndex = -1; }
        f.focus();
      }
    };
    fill();
    return box;
  }

  // people gives a team's people as up to four faces and how many more,
  // the one looking first; for a team with none, the people its decisions
  // reach, drawn apart.
  function people(o, names, reached) {
    const list = names && names.length ? names : reached || [];
    if (!list.length) return "";
    const me = B.me.me.name, shown = list.includes(me) ? [me] : [];
    for (const n of list) { if (shown.length >= 4) break; if (n !== me) shown.push(n); }
    const left = list.length - shown.length;
    return B.el("span", {class: "tm-ppl" + (list === names ? "" : " tm-reached"), title: shown.map(o.called).join(", ") + (left ? ", +" + left : "")},
      shown.map((n) => { const e = B.personEl(n); e.classList.add("tm-face"); return e; }),
      left ? B.el("span", {class: "mono faint tm-more"}, "+" + left) : "");
  }

  function leadEl(o, t) {
    if (!t.lead) return "";
    return o.folk.has(t.lead) ? B.personEl(t.lead) : B.workerEl(t.lead, t.name, {bare: true});
  }

  // placesCell gives the places that ask a team, one under another.
  function placesCell(t) {
    const list = B.placesOf(t.name);
    if (!list.length) return "";
    return B.el("span", {class: "tm-places"}, list.map((x) => B.placeEl(x.bridge, x.place.name)));
  }

  function yours(o, t) {
    return o.mine.has(t.name) ? B.el("span", {class: "tag"}, "yours") : "";
  }

  // teamRow is one team on one line, with its toggle in the tree; a team
  // found by a search is shown with the teams around it.
  function teamRow(o, t, toggle) {
    const around = toggle ? [] : o.around(t);
    return B.el("div", {class: "row tm-team" + (toggle ? "" : " tm-flat")},
      toggle || "",
      B.el("span", {class: "tm-name tm-cut"},
        around.length ? B.el("span", {class: "mono faint small tm-path"}, around.map((u) => u.called).join(" / ") + " /") : "",
        B.teamEl(t.name)),
      B.el("span", {class: "tm-about muted clip small", title: t.about || null}, t.about || ""),
      B.el("span", {class: "tm-cut"}, placesCell(t)),
      B.el("span", {}, people(o, t.people, t.reached)),
      B.el("span", {class: "tm-cut"}, leadEl(o, t)),
      B.el("span", {class: "tm-mine"}, yours(o, t)));
  }

  // list draws the teams inside a team, paged, each with its toggle.
  function list(o, up) {
    return paged(B.el("ul", {class: "tree"}), "in " + up, "", o.inside.get(up) || [], (t) => node(o, t), "li");
  }

  function node(o, t) {
    const li = B.el("li", {});
    const draw = (focus) => {
      const kids = o.inside.get(t.name) || [];
      const open = kids.length > 0 && state.open.has(t.name);
      const toggle = kids.length
        ? B.el("button", {class: "toggle", type: "button", "data-k": "open " + t.name, "aria-expanded": open ? "true" : "false",
          "aria-label": "Teams in " + t.called + ", " + kids.length, title: String(kids.length),
          onclick: () => { if (open) state.open.delete(t.name); else state.open.add(t.name); draw(true); }},
          open ? "▾" : "▸")
        : B.el("span", {class: "toggle", "aria-hidden": "true"});
      li.replaceChildren(...[teamRow(o, t, toggle), open ? list(o, t.name) : null].filter(Boolean));
      if (focus) toggle.focus();
    };
    draw(false);
    return li;
  }

  // officeRows gives what the top team's room says of the whole office: its
  // zone; how many runs are at work out of as many as it lets work, and
  // how many wait their turn; who decides a change, in order; and each
  // bridge with its one status.
  function officeRows(t) {
    if (t.within) return [];
    const runs = B.me.office.runs;
    const change = B.me.office.change || [];
    const bridges = B.me.office.bridges || [];
    return [
      ["Zone", B.el("span", {class: "mono"}, B.me.office.zone)],
      ["Runs", B.el("span", {class: "line"}, B.el("span", {class: "mono"}, runs.working + " / " + runs.limit),
        runs.queued ? B.el("span", {class: "tag"}, "Queued " + runs.queued) : null)],
      ["Change", change.length ? B.chain(change, t.name) : ""],
      ["Bridges", bridges.length ? B.el("span", {class: "line"}, bridges.map((b) =>
        B.el("span", {class: "line tm-bridge"}, B.el("a", {class: "who", href: "#/bridges"}, B.icon("bridge"), b.name), B.bridgeStatus(b)))) : ""],
    ];
  }

  // room draws a top team as a room: what it says of the whole office,
  // then its bands, In, where it is asked and its heed; Team, its people
  // and its lead; and Out, its disclosure and where its answer is said;
  // and the tree of its teams.
  function room(o, t) {
    const own = officeRows(t).filter((p) => p[1] !== "");
    const channel = B.placesOf(t.name).some((x) => x.place.name !== "direct");
    return B.el("section", {class: "room tm-room", "aria-label": t.called},
      B.el("div", {class: "plate"}, B.el("a", {href: B.teamHref(t.name)}, t.called),
        t.about ? B.el("span", {class: "about"}, t.about) : null, yours(o, t) || null),
      own.length ? B.kv(own) : null,
      B.el("div", {class: "bands"},
        B.band("In", [
          ["Asked", B.el("div", {class: "stack band-list"}, B.asksIn(t.name))],
          channel || t.heed ? [B.heedLabel(t), B.heedEl(t), B.settingsEl(t.heed && t.heed.settings)] : null,
        ]),
        B.band("Team", [
          t.people && t.people.length ? ["People", people(o, t.people)] : null,
          !(t.people && t.people.length) && t.reached && t.reached.length
            ? [B.el("span", {class: "line"}, "Reached", ...B.info("About reached", "The people this team's decisions reach. They stand as its people.")), people(o, null, t.reached)] : null,
          [B.leadLabel(t), B.el("span", {class: "line"}, leadEl(o, t), t.lead_runs ? null : B.el("span", {class: "status failed"}, "cannot run"))],
        ]),
        B.band("Out", [
          t.discloses !== undefined ? [B.disclosureLabel(t), t.discloses ? "Set" : B.el("span", {class: "faint"}, "None")] : null,
          ["Answer", B.answersTo(t.name)],
        ])),
      (o.inside.get(t.name) || []).length ? B.el("div", {class: "rows tm-tree"}, list(o, t.name)) : null);
  }

  // Defined: the tools, models and docs the teams' files define, each
  // with the teams that use it.

  function teamsCell(names) {
    return names.length ? B.el("span", {class: "line"}, names.map(B.teamEl)) : B.el("span", {class: "faint"}, "None");
  }

  function table(head, rows) {
    return B.el("div", {class: "scroll"}, B.el("table", {class: "table tm-defined"},
      B.el("thead", {}, B.el("tr", {}, head.map((h) => B.el("th", {scope: "col"}, h)))),
      B.el("tbody", {}, rows)));
  }

  const kinds = {script: "Script", machine: "Machine", tools: "Tools"};

  function defined(d) {
    const groups = [
      d.tools.length ? B.el("div", {class: "group"}, B.label("Tools", B.el("span", {class: "count"}, String(d.tools.length))),
        table(["Name", "Kind", "Teams"], d.tools.map((x) => B.el("tr", {},
          B.el("td", {}, x.shipped ? B.el("span", {class: "line"}, B.el("a", {href: "#/shipped"}, x.shipped), B.el("span", {class: "faint"}, x.name)) : x.name),
          B.el("td", {}, B.el("span", {class: "tag"}, kinds[x.kind] || x.kind)),
          B.el("td", {}, teamsCell(x.teams)))))) : null,
      d.models.length ? B.el("div", {class: "group"}, B.label("Models", B.el("span", {class: "count"}, String(d.models.length))),
        table(["Model", "Teams"], d.models.map((x) => B.el("tr", {},
          B.el("td", {class: "clip", title: x.name}, B.modelText(x.name) || x.name),
          B.el("td", {}, teamsCell(x.teams)))))) : null,
      d.docs.length ? B.el("div", {class: "group"}, B.label("Docs", B.el("span", {class: "count"}, String(d.docs.length))),
        table(["Name", "Team", "Size", "Read by"], d.docs.map((x) => B.el("tr", {},
          B.el("td", {}, x.name),
          B.el("td", {}, B.teamEl(x.team)),
          B.el("td", {class: "faint"}, B.size(x.size)),
          B.el("td", {}, teamsCell(x.teams)))))) : null,
    ].filter(Boolean);
    return B.sect(B.label("Defined", ...B.info("About defined", "What the files of the teams you see define, with the teams that use each.")),
      groups.length ? B.el("div", {class: "stack tm-groups"}, groups) : B.empty());
  }

  // search gives a search box that puts back what was searched for when
  // the view is drawn again, drawing the list once.
  function search(key, placeholder, items, text, draw) {
    let held = !!state[key];
    const s = B.search(placeholder, items, text, (found, q) => held ? "" : draw(found, q));
    s.box.dataset.k = key;
    s.box.addEventListener("input", () => { state[key] = s.box.value; });
    if (held) { held = false; s.box.value = state[key]; s.box.dispatchEvent(new Event("input")); }
    return s;
  }

  // focused is what has the focus in the view, to be given it again once
  // the view is in the page; again gives it back.
  function focused() {
    const a = document.activeElement;
    if (!a || !a.dataset || !a.dataset.k || !document.getElementById("main").contains(a)) return null;
    return {k: a.dataset.k, start: typeof a.selectionStart === "number" ? a.selectionStart : null, end: a.selectionEnd};
  }
  function again(root, f) {
    if (!f) return;
    setTimeout(() => {
      if (!root.isConnected) return;
      const e = [...root.querySelectorAll("[data-k]")].find((x) => x.dataset.k === f.k);
      if (!e) return;
      e.focus();
      if (f.start !== null && typeof e.setSelectionRange === "function") e.setSelectionRange(f.start, f.end);
    });
  }

  function view(d) {
    const o = office();

    const teams = search("teamQ", "Search teams", o.teams, (t) => o.teamText.get(t), (found, q) => {
      if (!q) return paged(B.el("div", {}), "rooms", "", o.inside.get("") || [], (t) => room(o, t), "div");
      if (!found.length) return B.empty();
      const sorted = found.slice().sort((a, b) => (o.mine.has(b.name) - o.mine.has(a.name)) || o.pathOf.get(a).localeCompare(o.pathOf.get(b)));
      return paged(B.el("div", {class: "rows tm-found"}), "found", q, sorted, (t) => teamRow(o, t, null), "div");
    });

    const personRow = (p) => {
      const in_ = o.teamsOf.get(p.name) || [];
      return B.el("div", {class: "row tm-person"},
        B.el("span", {class: "tm-cut"}, B.personEl(p.name)),
        B.el("span", {class: "muted clip small", title: p.title || null}, p.title || ""),
        B.el("span", {class: "clip", title: in_.map((t) => t.called).join(", ") || null}, in_.flatMap((t, i) => [i ? ", " : "", B.el("a", {href: B.teamHref(t.name)}, t.called)])),
        B.el("span", {class: "mono clip"}, (p.bridges || []).join(", ")));
    };
    const folk = search("peopleQ", "Search everyone", o.all, (p) => o.personText.get(p), (found, q) =>
      found.length ? paged(B.el("div", {class: "rows tm-people"}), "people", q, found, personRow, "div") : B.empty());

    const count = (n) => B.el("span", {class: "count"}, String(n));
    return B.el("div", {class: "tm"},
      B.head(null, "Teams"),
      B.sect(B.label("Teams", count(o.teams.length), ...B.info("About teams", "Teams are sealed: only people named in a team see what it read.")),
        teams.box, teams.out),
      defined(d),
      B.sect(B.label("Everyone", count(o.all.length), ...B.info("About everyone", "Every person written under everyone in office.yaml. A team's People are those its file names.")), folk.box, folk.out));
  }

  // The view is drawn again only when the office it shows has changed: the
  // office is fetched again on few changes, and compared only then.
  B.route(/^#\/teams$/, async function () {
    const f = focused();
    if (!drawn || drawn.me !== B.me) {
      const d = await B.api("/api/defined");
      const sig = JSON.stringify([B.me.me.name, B.me.me.teams, B.me.teams, B.me.people, B.me.office, d]);
      if (!drawn || drawn.sig !== sig) drawn = {sig, el: view(d)};
      drawn.me = B.me;
    }
    again(drawn.el, f);
    return drawn.el;
  }, "teams", (what) => what === "office" || what === "runs");
})();
