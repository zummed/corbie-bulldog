// Office: the office's own page, reached by its name, with a part for its
// bridges, one for what comes with Bulldog, and, for the people who decide
// a change, one for its changes. Each part is its own view; the addresses
// each had before lead to it.
(function () {
  "use strict";

  const parts = [["bridges", "Bridges"], ["bulldog", "From Bulldog"], ["changes", "Changes"]];

  // B.officeHead gives the head of a part of the office's page: the
  // office's name, what is given beside it, and the parts the one looking
  // may go to, the one shown marked.
  B.officeHead = function (at, ...right) {
    const shown = parts.filter(([key]) => key !== "changes" || B.me.me.decides_changes);
    return B.el("div", {class: "of-head"},
      B.head(null, B.me.office.name, ...right),
      B.el("nav", {class: "tabs", "aria-label": "Office"}, shown.map(([key, label]) =>
        B.el("a", {href: "#/office/" + key, "aria-current": key === at ? "page" : null}, label))));
  };

  const moved = {bridges: "bridges", shipped: "bulldog", changes: "changes"};
  B.route(/^#\/(bridges|shipped|changes)(\?.*)?$/, function (m) {
    location.replace("#/office/" + moved[m[1]] + (m[2] || ""));
    return B.el("div", {});
  }, "office");
})();
