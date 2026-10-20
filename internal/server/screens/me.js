// People: a person of the office as everyone sees them, and the one
// looking as themselves: who they are on each bridge, what they decide,
// who decides a change, what they see that is wrong, and their key.
(function () {
  "use strict";

  const teamsLine = (names) => names.length ? B.el("span", {class: "line"}, names.map(B.teamEl)) : B.empty();

  // reachedEl gives who the one looking is on each bridge, in the order a
  // one-time link is tried, and whether each has a direct place it goes
  // to.
  function reachedEl(me) {
    if (!me.identities || !me.identities.length) return B.empty();
    return B.el("div", {class: "scroll"}, B.el("table", {class: "table me-reached"},
      B.el("thead", {}, B.el("tr", {}, ["Order", "Bridge", "Identity", "Direct"].map((h) => B.el("th", {scope: "col"}, h)))),
      B.el("tbody", {}, me.identities.map((id, i) => B.el("tr", {},
        B.el("td", {}, String(i + 1)),
        B.el("td", {}, B.el("span", {class: "who"}, B.icon("bridge"), id.bridge)),
        B.el("td", {}, id.id),
        B.el("td", {}, id.direct ? "Yes" : B.el("span", {class: "faint"}, "No")))))));
  }

  // decidesEl gives each list that names the one looking, with where on
  // it they are, and the teams whose decisions end at them.
  function decidesEl(me) {
    return B.kv([
      ["Lists", me.lists && me.lists.length ? B.el("div", {class: "stack band-list"}, me.lists.map((l) => B.el("span", {class: "line"},
        B.teamEl(l.team), B.el("span", {class: "faint"}, "›"), B.el("span", {class: "mono"}, l.key), B.el("span", {class: "tag"}, "#" + l.at)))) : B.empty()],
      [B.el("span", {class: "line"}, "Ends at you", ...B.info("About ends at you", "Teams whose decisions reach you when every rule and worker before you refers them.")),
        me.walks && me.walks.length ? teamsLine(me.walks) : B.empty()],
    ]);
  }

  function mine() {
    const me = B.me.me;
    const change = B.me.office.put_draft || [];
    return B.el("div", {},
      B.head(null, B.personEl(me.name, {title: true})),
      B.el("div", {class: "cols"},
        B.el("div", {class: "main"},
          B.sect("Person", B.kv([
            ["Title", me.title],
            ["Teams", teamsLine(me.teams || [])],
          ])),
          B.sect(B.label("Reached", ...B.info("About reached", "A one-time link goes to the first of these, in order, that has a direct place and sends it.")), reachedEl(me)),
          B.sect("Decides", decidesEl(me)),
          B.sect("put_draft · office", B.kv([
            ["Decided by", change.length ? B.chain(change, "office") : B.empty()],
            ["You", me.decides_changes ? B.el("a", {href: "#/office/changes"}, "Decide") : B.el("span", {class: "faint"}, "No")],
          ])),
          B.me.problems.length ? B.sect("Problems", B.el("div", {class: "rows"}, B.me.problems.map((p) =>
            B.el("div", {class: "row"}, B.teamEl(p.team), B.el("span", {class: "grow"}, p.said))))) : null),
        B.el("div", {class: "side"},
          B.sect(B.label("Key", ...B.info("About the key", "Replacing it signs out every browser that holds it.")),
            B.el("button", {class: "btn ghost", onclick: () => B.act(B.post("/api/me/key"))}, "Replace key")))));
  }

  B.route(/^#\/me$/, mine, "", (what) => what === "office");

  B.route(/^#\/p\/([^/]+)$/, async function (m) {
    const name = decodeURIComponent(m[1]);
    if (B.isMe(name)) return mine();
    const crumbs = [B.el("a", {href: "#/teams"}, "Teams")];
    const p = B.person(name);
    if (!p) return B.el("div", {}, B.head(crumbs, name), B.empty("There is nothing here by that name."));
    const teams = (B.me.teams || []).filter((t) => (t.people || []).includes(name)).map((t) => t.name);
    return B.el("div", {},
      B.head(crumbs, B.personEl(name, {title: true})),
      B.sect("Person", B.kv([
        ["Title", p.title],
        ["Teams", teamsLine(teams)],
        ["Bridges", (p.bridges || []).length ? B.el("span", {class: "mono"}, p.bridges.join(", ")) : B.empty()],
      ])));
  }, "teams", (what) => what === "office");
})();
