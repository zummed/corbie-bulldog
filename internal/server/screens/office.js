// Office: the office's own page, reached by its name, with a part for its
// bridges, one for how people sign in, one for what comes with Bulldog,
// and, for the people who decide a change, one for its changes. Each part is its own view; the addresses
// each had before lead to it.
(function () {
  "use strict";

  const parts = [["bridges", "Bridges"], ["signin", "Sign-in"], ["built-in", "Built-in"], ["changes", "Changes"]];

  // B.officeHead gives the head of a part of the office's page: the
  // office's name, what is given beside it, the parts the one looking may
  // go to, the one shown marked, and beneath them what holds for the whole
  // office: its zone, how many runs are at work of as many as it lets
  // work, and how many wait their turn, drawn again in place as they
  // change, and who decides a change.
  B.officeHead = function (at, ...right) {
    const shown = parts.filter(([key]) => key !== "changes" || B.me.me.decides_changes);
    const o = B.me.office, change = o.office_put || [];
    const said = (word, ...what) => B.el("span", {class: "line of-said"}, B.el("span", {class: "of-word"}, word), ...what);
    return B.el("div", {class: "of-head"},
      B.head(null, o.name, ...right),
      B.el("nav", {class: "tabs", "aria-label": "Office"}, shown.map(([key, label]) =>
        B.el("a", {href: "#/office/" + key, "aria-current": key === at ? "page" : null}, label))),
      B.el("div", {class: "line of-strip"},
        said("Zone", B.el("span", {class: "mono"}, o.zone)),
        said("Runs", B.runsEl()),
        change.length ? said("office_put", B.chain(change)) : null));
  };

  // signIn draws one sign-in method: what the sign-in page calls it, the
  // folder it runs, its one status, the address its provider sends the
  // browser back to, and what is wrong with it to those who see that.
  function signIn(m) {
    const problems = (B.me.problems || []).filter((p) => p.signin === m.name);
    return B.el("section", {class: "br", id: "signin-" + m.name, "aria-label": m.name},
      B.el("div", {class: "br-head"},
        B.el("h2", {class: "mono"}, m.name),
        B.el("span", {class: "status " + m.status}, m.status)),
      B.el("div", {class: "br-body"},
        B.kv([
          ["Button", "Sign in with " + m.called],
          ["Folder", m.shipped ? B.el("span", {class: "line"}, B.el("span", {class: "mono"}, m.folder), B.el("a", {class: "faint", href: "#/office/built-in"}, "Built-in")) : B.el("span", {class: "mono"}, m.folder)],
          [B.el("span", {class: "line"}, "Redirect URI", ...B.info("About the redirect URI", "Registered at the provider, exactly.")), B.el("span", {class: "mono"}, m.back)],
          problems.length ? ["Problems", B.el("div", {class: "stack br-problems"}, problems.map((p) => B.el("span", {class: "mono br-bad"}, p.said)))] : null,
        ])));
  }

  B.route(/^#\/office\/signin$/, function () {
    const o = B.me.office;
    return B.el("div", {class: "brs"},
      B.officeHead("signin"),
      (o.signin || []).map(signIn),
      B.sect("Others", B.kv([
        ["One-time link", "Everyone"],
        ["Password", o.password ? "password: true" : B.el("span", {class: "faint"}, "Nobody")],
      ])));
  }, "office", (what) => what === "office");

  const moved = {bridges: "bridges", shipped: "built-in", "office/bulldog": "built-in", changes: "changes"};
  B.route(/^#\/(bridges|shipped|office\/bulldog|changes)(\?.*)?$/, function (m) {
    location.replace("#/office/" + moved[m[1]] + (m[2] || ""));
    return B.el("div", {});
  }, "office");
})();
