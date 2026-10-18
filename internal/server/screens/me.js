// People: a person of the office as everyone sees them, and the one
// looking as themselves, with what they see that is wrong and their key.
(function () {
  "use strict";

  const teamsLine = (names) => names.length ? B.el("span", {class: "line"}, names.map(B.teamEl)) : B.empty();

  function mine() {
    const me = B.me.me;
    return B.el("div", {},
      B.head(null, B.personEl(me.name, {title: true})),
      B.el("div", {class: "cols"},
        B.el("div", {class: "main"},
          B.sect("Person", B.kv([
            ["Title", me.title],
            ["Teams", teamsLine(me.teams || [])],
            ["Bridges", B.el("span", {class: "mono"}, ["screens"].concat(me.bridges || []).join(", "))],
            ["Changes", me.decides_changes ? B.el("a", {href: "#/changes"}, "decides") : null],
          ])),
          B.me.problems.length ? B.sect("Problems", B.el("div", {class: "rows"}, B.me.problems.map((p) =>
            B.el("div", {class: "row"}, B.teamEl(p.team), B.el("span", {class: "grow"}, p.said))))) : null,
          B.me.failing && B.me.failing.length ? B.sect("Bridges failing", B.el("div", {class: "rows"}, B.me.failing.map((f) =>
            B.el("div", {class: "row"}, B.el("span", {class: "who"}, B.icon("bridge"), f.bridge), B.el("span", {class: "status failed"}, f.failures + " in a row"),
              B.el("span", {class: "grow mono small"}, f.error), B.whenEl(f.since))))) : null),
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
