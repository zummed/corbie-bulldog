// From Bulldog, a part of the office's page: each thing Bulldog ships,
// grouped by what it is, with its one status, and beneath it its page,
// what it reaches, its settings and where the office uses it. The people
// who decide a change also see what changed in it since the version that
// ran before, the office's copies of it, and Use and Copy, which open a
// draft on Changes.
(function () {
  "use strict";

  const groups = [["bridge", "Bridges"], ["tools", "Tools"], ["rule", "Rules"]];
  const words = {used: "Used", unused: "Unused", copied: "Copied"};

  // statusEl gives a thing's one status, with where it is used or copied
  // behind its (i).
  function statusEl(t) {
    const word = B.el("span", {class: "status sh-" + t.status}, words[t.status]);
    if (t.status === "used") return B.el("span", {class: "line"}, word,
      ...B.info("About used", "Named by the office's files, as Bulldog ships it; it changes with Bulldog."));
    if (t.status === "copied") return B.el("span", {class: "line"}, word,
      ...B.info("About copied", "The office's own copy at " + t.copies.map((c) => c.folder).join(", ") + "; a later Bulldog does not change it."));
    return word;
  }

  // changedEl gives the mark of what changed since the version that ran
  // before, the newest version named, with each line behind its (i).
  function changedEl(t) {
    if (!t.changed || !t.changed.length) return "";
    const newest = t.changed[t.changed.length - 1].version;
    return B.el("span", {class: "line sh-mark"}, B.el("span", {class: "tag sh-changed"}, "Changed " + newest),
      ...B.info("About what changed", ...t.changed.map((c) => B.el("div", {}, B.el("span", {class: "mono"}, c.version), " " + c.text))));
  }

  // valueEl gives a setting's value as it is shipped: a mapping or a list
  // in a row, and anything else as text.
  function valueEl(v) {
    if (v === undefined || v === null) return B.el("span", {class: "faint"}, "None");
    return B.el("span", {class: "mono"}, typeof v === "object" ? JSON.stringify(v) : String(v));
  }

  function settingsEl(t) {
    if (!t.settings.length) return null;
    return B.kv(t.settings.map((s) => [B.el("span", {class: "line sh-setting"}, B.el("span", {class: "mono"}, s.name),
      s.about ? B.info("About " + s.name, s.about) : ""), valueEl(s.value)]));
  }

  // asWord gives how a team names a rule: its heed, its disclosure, or a
  // list of deciders by its key.
  function asWord(as) {
    if (as === "heed") return "Heed";
    if (as === "disclosure") return "Disclosure";
    return B.el("span", {class: "mono"}, as);
  }

  // useEl gives one place the office uses a thing: a bridge's entry, with
  // what it reaches as its settings fill it to those who see that; a
  // worker that holds its tools; a team's heed or list that names it.
  function useEl(u) {
    const sep = () => B.el("span", {class: "faint"}, "›");
    if (u.bridge) return B.el("div", {class: "sh-use"},
      B.el("a", {class: "line", href: "#/office/bridges"}, B.el("span", {class: "mono"}, "office.yaml"), sep(), B.el("span", {class: "mono"}, "bridges"), sep(), B.el("span", {class: "mono"}, u.bridge)),
      u.reach ? B.reachEl(u.reach) : "");
    if (u.worker) return B.el("div", {class: "line"}, B.teamEl(u.team), sep(), B.workerEl(u.worker, u.team, {bare: true}));
    return B.el("div", {class: "line"}, B.teamEl(u.team), sep(), asWord(u.as));
  }

  function copyEl(c) {
    return B.el("div", {class: "line"}, B.el("span", {class: "mono"}, c.folder),
      c.version ? B.el("span", {class: "mono faint"}, c.version) : "",
      c.differs.length ? B.el("span", {class: "line"}, B.el("span", {class: "faint"}, "Differs"), B.el("span", {class: "mono"}, c.differs.join(" · "))) : B.el("span", {class: "faint"}, "Same"));
  }

  // thing draws one shipped thing: its head with its one status, and
  // beneath it on a guide line what it is and where it is used.
  function thing(t) {
    const page = B.el("details", {class: "sh-page"}, B.el("summary", {}, t.summary), B.el("pre", {}, t.page));
    const actions = [
      t.use ? B.el("button", {class: "btn sm", type: "button", onclick: () => B.drafting(t.use)}, "Use") : "",
      t.copy ? B.el("button", {class: "btn sm ghost", type: "button", onclick: () => B.drafting(t.copy)}, "Copy") : "",
    ].filter(Boolean);
    return B.el("section", {class: "sh", id: "shipped-" + t.path.replace("/", "-"), "aria-label": t.path},
      B.el("div", {class: "sh-head"},
        B.el("h3", {class: "mono"}, t.name),
        B.el("span", {class: "line"}, changedEl(t), statusEl(t))),
      B.el("div", {class: "sh-body"},
        B.kv([
          ["Page", page],
          ["Needs", B.reachEl(t.needs)],
          ["Settings", settingsEl(t)],
          t.uses.length ? ["Used", B.el("div", {class: "stack sh-uses"}, t.uses.map(useEl))] : null,
          t.copies && t.copies.length ? ["Copies", B.el("div", {class: "stack sh-uses"}, t.copies.map(copyEl))] : null,
        ]),
        actions.length ? B.el("div", {class: "line"}, actions) : ""));
  }

  B.route(/^#\/office\/bulldog$/, async function () {
    const out = await B.api("/api/shipped");
    return B.el("div", {class: "shs"},
      B.officeHead("bulldog", B.el("span", {class: "mono faint"}, "Bulldog " + out.version)),
      groups.map(([kind, label]) => {
        const things = out.things.filter((t) => t.kind === kind);
        return B.el("section", {class: "sh-group", "aria-label": label},
          B.label(label, B.el("span", {class: "count"}, String(things.length))),
          things.map(thing));
      }));
  }, "office", (what) => what === "office" || what === "runs");
})();
