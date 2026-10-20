// Office: the office's own page, reached by its name, with a part for its
// bridges, one for what comes with Bulldog, and, for the people who decide
// a change, one for its changes. Each part is its own view; the addresses
// each had before lead to it.
(function () {
  "use strict";

  const parts = [["bridges", "Bridges"], ["bulldog", "From Bulldog"], ["changes", "Changes"]];

  // B.officeHead gives the head of a part of the office's page: the
  // office's name, what is given beside it, the parts the one looking may
  // go to, the one shown marked, and beneath them what holds for the whole
  // office: its zone, how many runs are at work of as many as it lets
  // work, and how many wait their turn, and who decides a change.
  B.officeHead = function (at, ...right) {
    const shown = parts.filter(([key]) => key !== "changes" || B.me.me.decides_changes);
    const o = B.me.office, change = o.change || [];
    const said = (word, ...what) => B.el("span", {class: "line of-said"}, B.el("span", {class: "of-word"}, word), ...what);
    return B.el("div", {class: "of-head"},
      B.head(null, o.name, ...right),
      B.el("nav", {class: "tabs", "aria-label": "Office"}, shown.map(([key, label]) =>
        B.el("a", {href: "#/office/" + key, "aria-current": key === at ? "page" : null}, label))),
      B.el("div", {class: "line of-strip"},
        said("Zone", B.el("span", {class: "mono"}, o.zone)),
        said("Runs", B.el("span", {class: "mono"}, o.runs.working + " / " + o.runs.limit),
          o.runs.queued ? B.el("span", {class: "tag"}, "Queued " + o.runs.queued) : null),
        change.length ? said("Change", B.chain(change)) : null));
  };

  const moved = {bridges: "bridges", shipped: "bulldog", changes: "changes"};
  B.route(/^#\/(bridges|shipped|changes)(\?.*)?$/, function (m) {
    location.replace("#/office/" + moved[m[1]] + (m[2] || ""));
    return B.el("div", {});
  }, "office");
})();
