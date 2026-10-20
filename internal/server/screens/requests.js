// Requests: those the one looking may see, newest first, narrowed by
// whose they are, their state, their team and the place they were asked
// at, a page at a time; the noted only when picked, and the sub-requests
// of a request folded inside it when both are listed.
(function () {
  "use strict";

  // The address's query names these, and they go on to the API as given.
  const PASSED = ["team", "state", "asked", "waiting", "bridge", "place", "noted", "limit", "before"];
  const STATES = ["working", "queued", "waiting", "answered"];
  const PAGE = 100; // the server's page, when the address names none
  const MOST = 500; // the most the server gives in one page

  // kept is what the one looking did in the view at an address, so that
  // drawing it again at a change keeps it: what is typed in either box,
  // the oldest request More fetched and how many there were, and what was
  // drawn last, to be given again when nothing in it changed; and the
  // requests whose sub-requests the one looking opened or folded.
  let kept = fresh("");
  function fresh(hash) { return {hash, q: "", team: null, until: 0, count: 0, sig: "", root: null, open: new Map()}; }

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

  // placeFilter picks one place a bridge listens in, by the bridge and the
  // place as listed; one the address names that is listed no longer is
  // offered too, and there is none to pick in an office with no places.
  function placeFilter(q) {
    const all = (B.me.office.bridges || []).flatMap((b) => (b.places || []).map((p) => ({bridge: b.name, place: p.name})));
    const at = {bridge: q.get("bridge") || "", place: q.get("place") || ""};
    if (at.bridge && !all.some((x) => x.bridge === at.bridge && x.place === at.place)) all.push(at);
    if (!all.length) return null;
    const sel = B.el("select", {class: "in"},
      B.el("option", {value: ""}, "all"),
      all.map((x, i) => B.el("option", {value: String(i), selected: x.bridge === at.bridge && x.place === at.place},
        x.place ? x.bridge + " › " + x.place : x.bridge)));
    sel.addEventListener("change", () => {
      const x = all[Number(sel.value)];
      location.hash = hrefWith(x ? {bridge: x.bridge, place: x.place} : {bridge: null, place: null});
    });
    return B.field("Place", sel);
  }

  // notedChip picks whether the requests noted, whose every message the
  // team's heed noted, are listed; it is there while there are any.
  function notedChip(q, count) {
    const picked = q.get("noted") === "yes";
    if (!count && !picked) return null;
    return B.el("span", {class: "line rqs-noted"},
      B.el("a", {class: "rqs-chip", href: hrefWith({noted: picked ? null : "yes"}), "aria-pressed": String(picked)},
        "Noted", B.el("span", {class: "count"}, String(count))),
      ...B.info("About noted", "Kept in the request; its lead did not start on it, and is given it in the thread when next asked."));
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
    if (r.parent) return B.el("span", {class: "who muted"}, B.icon("branch"), "inside ", B.el("a", {class: "mono", href: "#/r/" + r.parent}, "#" + r.parent));
    return B.origin(r.origin);
  }

  // inside gives the requests beneath one, as far down as they go, each
  // with how far down it is.
  function inside(r, depth) {
    return (r.subs || []).flatMap((s) => [{r: s, depth: depth + 1}, ...inside(s, depth + 1)]);
  }

  // A row is a link by its number, which covers the whole row; the other
  // links in it lie above that. A request with sub-requests inside it opens
  // them by its toggle; one beneath it is set in by its depth, with no
  // origin of its own.
  function row(r, byTeam, depth, toggle) {
    const nest = depth ? " nest d" + Math.min(depth, 3) : "";
    const n = cell("n" + nest, B.el("a", {class: "mono rqs-open", href: "#/r/" + r.number,
      "aria-label": "#" + r.number + (r.whole && r.first ? " " + r.first : ""), title: r.whole ? r.first : null}, "#" + r.number));
    if (!r.whole) {
      return B.el("div", {class: "row link"}, n, cell("first muted", "put to you"),
        r.state ? cell("st", B.status(r)) : null);
    }
    const st = B.status(r);
    return B.el("div", {class: "row link" + (depth ? " sub" : "")},
      n,
      B.el("span", {class: "cell first" + nest, title: r.first}, B.el("span", {class: "text"}, r.first), toggle),
      byTeam ? null : cell("team", B.teamEl(r.team)),
      cell("asker", B.party(r.asker)),
      cell("origin", depth ? "" : where(r)),
      B.el("span", {class: "cell st", title: st.textContent}, st),
      cell("made", B.whenEl(r.made)));
  }

  // opened says whether the requests inside one are shown: as the one
  // looking last left them, or else when one of them waits on someone or
  // is of the team the list is narrowed to.
  function opened(r, below, q) {
    if (kept.open.has(r.number)) return kept.open.get(r.number);
    return below.some((x) => x.r.state === "waiting" || (q.has("team") && x.r.team === q.get("team")));
  }

  // rowsOf gives a request's row and, when it is opened, the rows of the
  // requests inside it.
  function rowsOf(r, q, rowOf, redraw) {
    const below = inside(r, 0);
    if (!below.length) return [rowOf(r, 0, null)];
    const open = opened(r, below, q);
    const toggle = B.el("button", {class: "rqs-inside", type: "button", "aria-expanded": String(open),
      "aria-label": below.length + " inside #" + r.number, onclick: () => {
        kept.open.set(r.number, !open);
        redraw();
      }}, open ? "▾" : "▸", " " + below.length + " inside");
    return [rowOf(r, 0, toggle), open ? below.map((x) => rowOf(x.r, x.depth, null)) : []];
  }

  function header(byTeam) {
    return B.el("div", {class: "row rqs-hd", "aria-hidden": "true"},
      B.el("span", {}, "#"), B.el("span", {}, "Request"), byTeam ? null : B.el("span", {}, "Team"),
      B.el("span", {}, "Asker"), B.el("span", {}, "Origin"), B.el("span", {}, "State"), B.el("span", {}, "Made"));
  }

  // words gives what a request is found by: its own and those of every
  // request inside it.
  function words(r) {
    if (!r.whole) return "#" + r.number;
    const a = r.asker || {};
    return ["#" + r.number, r.first, a.name, a.called, a.id, r.team, B.teamCalled(r.team), ...(r.subs || []).map(words)].filter(Boolean).join(" ");
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

    // Each row is made once for where it stands, and the search only
    // chooses among them.
    const made = new Map();
    const rowOf = (r, depth, toggle) => {
      const key = r.number + "/" + depth + "/" + (toggle ? toggle.getAttribute("aria-expanded") : "");
      if (!made.has(key) || made.get(key).r !== r) made.set(key, {r, el: row(r, byTeam, depth, toggle)});
      return made.get(key).el;
    };
    const hd = header(byTeam);
    const redraw = () => search.box.dispatchEvent(new Event("input"));
    const search = B.search("Search", items, words, (shown, typed) => shown.length
      ? B.el("div", {class: "rows rqs-list" + (byTeam ? " by-team" : "")}, hd, shown.map((r) => rowsOf(r, q, rowOf, redraw)))
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
      B.el("div", {class: "rqs-bar"}, tabs(q), B.el("div", {class: "rqs-filters"}, stateFilter(q), teamFilter(q), placeFilter(q), notedChip(q, first.noted || 0))),
      search.out,
      B.el("div", {class: "rqs-more"}, more));
    kept.sig = sig();
    kept.root = root;
    refocus(root, at);
    return root;
  }, "requests");
})();
