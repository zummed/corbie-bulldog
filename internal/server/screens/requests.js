// Requests: those the one looking may see, newest first, narrowed by
// whose they are, their state and their team, a page at a time.
(function () {
  "use strict";

  // The address's query names these, and they go on to the API as given.
  const PASSED = ["team", "state", "asked", "waiting", "limit", "before"];
  const STATES = ["working", "waiting", "answered"];
  const PAGE = 100; // the server's page, when the address names none
  const MOST = 500; // the most the server gives in one page

  // kept is what the one looking did in the view at an address, so that
  // drawing it again at a change keeps it: what is typed in either box,
  // the oldest request More fetched and how many there were, and what was
  // drawn last, to be given again when nothing in it changed.
  let kept = fresh("");
  function fresh(hash) { return {hash, q: "", team: null, until: 0, count: 0, sig: "", root: null}; }

  function apiQuery(q, before, limit) {
    const out = new URLSearchParams();
    for (const k of PASSED) if (q.has(k)) out.set(k, q.get(k));
    if (before) out.set("before", before);
    if (limit) out.set("limit", limit);
    return out;
  }

  // hrefWith gives the list's address with the query changed: a value of
  // null takes a key away. A new filter starts at the newest page.
  function hrefWith(changes) {
    const q = B.query();
    q.delete("before");
    for (const [k, v] of Object.entries(changes)) {
      if (v === null || v === "") q.delete(k); else q.set(k, v);
    }
    const s = q.toString();
    return "#/requests" + (s ? "?" + s : "");
  }

  function tabs(q) {
    const at = q.get("waiting") === "me" ? "waiting" : q.get("asked") === "me" ? "asked" : "all";
    const tab = (key, label, changes) => B.el("a", {href: hrefWith(changes), "aria-current": at === key ? "page" : null}, label);
    return B.el("nav", {class: "tabs", "aria-label": "Requests"},
      tab("all", "All", {asked: null, waiting: null}),
      tab("waiting", "Waiting on you", {waiting: "me", asked: null}),
      tab("asked", "Asked by you", {asked: "me", waiting: null}));
  }

  function stateFilter(q) {
    const sel = B.el("select", {class: "in"},
      B.el("option", {value: ""}, "all"),
      STATES.map((s) => B.el("option", {value: s, selected: q.get("state") === s}, s)));
    sel.addEventListener("change", () => { location.hash = hrefWith({state: sel.value}); });
    return B.field("State", sel);
  }

  // teamFilter is a box that offers the teams the one looking is in, and
  // takes a team's name or what it is called. It is a search box, so that
  // what it holds is not taken for something being written; what is typed
  // in it is kept for when the view is drawn again.
  function teamFilter(q) {
    const teams = B.me.teams || [];
    const list = B.el("datalist", {id: "rqs-teams"}, teams.map((t) => B.el("option", {value: t.name, label: t.called})));
    const box = B.el("input", {class: "in", type: "search", list: "rqs-teams", autocomplete: "off", "data-keep": "team",
      value: kept.team !== null ? kept.team : q.get("team") || ""});
    const pick = () => {
      const v = box.value.trim();
      const t = B.team(v) || teams.find((x) => String(x.called).toLowerCase() === v.toLowerCase());
      location.hash = hrefWith({team: t ? t.name : v});
    };
    box.addEventListener("change", pick);
    box.addEventListener("input", () => {
      kept.team = box.value;
      // Emptying the box, by its clear button too, takes the team away.
      if (box.value === "" && q.has("team")) pick();
    });
    return B.el("div", {class: "rqs-team"}, B.field("Team", box), list);
  }

  function cell(cls, ...children) { return B.el("span", {class: "cell " + cls}, ...children); }

  function where(r) {
    if (r.parent) return B.el("span", {class: "who muted"}, B.icon("branch"), "beneath ", B.el("a", {class: "mono", href: "#/r/" + r.parent}, "#" + r.parent));
    return B.origin(r.origin);
  }

  // A row is a link by its number, which covers the whole row; the other
  // links in it lie above that.
  function row(r, byTeam) {
    const n = cell("n", B.el("a", {class: "mono rqs-open", href: "#/r/" + r.number,
      "aria-label": "#" + r.number + (r.whole && r.first ? " " + r.first : ""), title: r.whole ? r.first : null}, "#" + r.number));
    if (!r.whole) {
      return B.el("div", {class: "row link"}, n, cell("first muted", "put to you"),
        r.state ? cell("st", B.status(r)) : null);
    }
    const st = B.status(r);
    return B.el("div", {class: "row link"},
      n,
      B.el("span", {class: "cell first", title: r.first}, r.first),
      byTeam ? null : cell("team", B.teamEl(r.team)),
      cell("asker", B.party(r.asker)),
      cell("origin", where(r)),
      B.el("span", {class: "cell st", title: st.textContent}, st),
      cell("made", B.whenEl(r.made)));
  }

  function header(byTeam) {
    return B.el("div", {class: "row rqs-hd", "aria-hidden": "true"},
      B.el("span", {}, "#"), B.el("span", {}, "Request"), byTeam ? null : B.el("span", {}, "Team"),
      B.el("span", {}, "Asker"), B.el("span", {}, "Origin"), B.el("span", {}, "State"), B.el("span", {}, "Made"));
  }

  function words(r) {
    if (!r.whole) return "#" + r.number;
    const a = r.asker || {};
    return ["#" + r.number, r.first, a.name, a.called, a.id, r.team, B.teamCalled(r.team)].filter(Boolean).join(" ");
  }

  // focusOf gives which of the view's boxes the one looking is in, and
  // where in it; refocus puts them back there once the view is drawn.
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

  B.route(/^#\/requests(\?.*)?$/, async function () {
    const q = B.query();
    const byTeam = q.has("team");
    if (kept.hash !== location.hash) kept = fresh(location.hash);
    const at = focusOf();
    const page = Math.min(Number(q.get("limit")) || PAGE, MOST);

    // Drawn again, the view fetches as far back as More had reached, in as
    // few pages as the server allows; a More that lands meanwhile reaches
    // further, and that is fetched too.
    const first = await B.api("/api/requests?" + apiQuery(q, null, Math.min(Math.max(page, kept.count), MOST)));
    const items = first.requests.slice();
    let before = first.before;
    while (before && kept.until && before > kept.until) {
      const next = await B.api("/api/requests?" + apiQuery(q, before, page));
      items.push(...next.requests);
      before = next.before;
    }

    const sig = () => JSON.stringify([items, before, B.me.teams, B.me.people]);
    if (kept.root && kept.sig === sig()) {
      refocus(kept.root, at);
      return kept.root;
    }

    // Each row is made once, and the search only chooses among them.
    const made = new Map();
    const rowOf = (r) => { if (!made.has(r)) made.set(r, row(r, byTeam)); return made.get(r); };
    const hd = header(byTeam);
    const search = B.search("Search", items, words, (shown, typed) => shown.length
      ? B.el("div", {class: "rows rqs-list" + (byTeam ? " by-team" : "")}, hd, shown.map(rowOf))
      : B.empty(typed && before ? "none loaded" : undefined));
    search.box.dataset.keep = "q";
    search.box.addEventListener("input", () => { kept.q = search.box.value; });
    if (kept.q) {
      search.box.value = kept.q;
      search.box.dispatchEvent(new Event("input"));
    }

    const more = B.el("button", {class: "btn ghost", type: "button", hidden: !before}, "More");
    more.addEventListener("click", async () => {
      more.disabled = true;
      try {
        const next = await B.api("/api/requests?" + apiQuery(q, before, page));
        // A view drawn again meanwhile has taken this one's place.
        if (!more.isConnected) return;
        items.push(...next.requests);
        before = next.before;
        if (items.length) kept.until = items[items.length - 1].number;
        kept.count = items.length;
        kept.sig = sig();
        search.box.dispatchEvent(new Event("input"));
        more.hidden = !before;
      } catch (err) { B.trouble(err); }
      more.disabled = false;
    });

    const root = B.el("div", {class: "rqs-view"},
      B.head(null, "Requests", search.box),
      B.el("div", {class: "rqs-bar"}, tabs(q), B.el("div", {class: "rqs-filters"}, stateFilter(q), teamFilter(q))),
      search.out,
      B.el("div", {class: "rqs-more"}, more));
    kept.sig = sig();
    kept.root = root;
    refocus(root, at);
    return root;
  }, "requests");
})();
