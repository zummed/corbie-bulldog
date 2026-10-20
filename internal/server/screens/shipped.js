// Built-in, a part of the office's page: what Bulldog ships that the
// office uses, each by who uses it and the line that does, the office's
// own copies marked; and folded beneath it, the rest, each by what it is
// for and the line one writes to use it. Opened, a thing gives its page,
// what it reaches, its settings and what changed in each version. The
// people who decide a change also see what changed since the version that
// ran before, and Use and Copy, which open a draft on Changes.
(function () {
  "use strict";

  const kinds = {bridge: "Bridge", tools: "Tools", rule: "Rule"};

  // valueEl gives a setting's value as it is shipped: a mapping or a list
  // in a row, and anything else as text.
  function valueEl(v) {
    if (v === undefined || v === null) return B.el("span", {class: "faint"}, "None");
    return B.el("span", {class: "mono"}, typeof v === "object" ? JSON.stringify(v) : String(v));
  }

  // settingsEl gives a thing's settings, but for a bridge's from, which
  // is the line that uses it.
  function settingsEl(t) {
    const settings = t.settings.filter((s) => !(t.kind === "bridge" && s.name === "from"));
    if (!settings.length) return null;
    return B.kv(settings.map((s) => [B.el("span", {class: "line bi-setting"}, B.el("span", {class: "mono"}, s.name),
      s.about ? B.info("About " + s.name, s.about) : ""), valueEl(s.value)]));
  }

  // lineEl gives a line of a file, as written, broken in its value
  // rather than after its key when it does not fit.
  function lineEl(line) {
    return B.el("code", {class: "mono bi-written"}, line.replace(": ", ":\u00a0"));
  }

  // textEl gives a line of a page, what it quotes in mono.
  function textEl(text) {
    return B.el("span", {}, text.split("`").map((part, i) => i % 2 ? B.el("span", {class: "mono"}, part) : part));
  }

  // newestFirst gives changes by their versions, the newest first.
  function newestFirst(changes) {
    const parts = (v) => v.split(".").map(Number);
    return [...changes].sort((a, b) => {
      const x = parts(a.version), y = parts(b.version);
      for (let i = 0; i < Math.max(x.length, y.length); i++) if ((y[i] || 0) !== (x[i] || 0)) return (y[i] || 0) - (x[i] || 0);
      return 0;
    });
  }

  // whoEl gives who uses a thing at one place: the office's file, for a
  // bridge; a worker; a team.
  function whoEl(u) {
    if (u.bridge) return B.el("a", {class: "mono", href: "#/office/bridges"}, "office.yaml");
    if (u.worker) return B.workerEl(u.worker, u.team, {bare: true});
    return B.teamEl(u.team);
  }

  // usedEl gives who uses a thing, gathered by the line that does it,
  // each line once.
  function usedEl(t) {
    const byLine = new Map();
    for (const u of t.uses) byLine.set(u.line, [...(byLine.get(u.line) || []), u]);
    return [...byLine].map(([line, uses]) => B.el("div", {class: "bi-use"},
      B.el("span", {class: "line bi-who"}, uses.map(whoEl)),
      line ? lineEl(line) : B.el("span", {})));
  }

  // copiedEl marks a thing the office holds its own copy of, with where
  // each copy is and the version its page names.
  function copiedEl(t) {
    if (!t.copies || !t.copies.length) return null;
    return B.el("div", {class: "bi-use"},
      B.el("span", {class: "line"}, B.el("span", {class: "status bi-copied"}, "Copied"),
        ...B.info("About copied", "The office's own; a later Bulldog does not change it.")),
      B.el("span", {class: "line"}, t.copies.map((c) => B.el("span", {class: "line bi-copy"}, lineEl(c.folder),
        c.version ? B.el("span", {class: "mono faint"}, c.version) : null))));
  }

  // changedEl marks a thing that changed since the version that ran
  // before, by the newest version named.
  function changedEl(t) {
    if (!t.changed || !t.changed.length) return null;
    return B.el("span", {class: "line bi-mark"}, B.el("span", {class: "tag bi-changed"}, "Changed " + newestFirst(t.changed)[0].version),
      ...B.info("About changed", "Since the Bulldog that ran before."));
  }

  // reachEl gives what a thing reaches: as each bridge running it fills
  // it, to those who see that, or else as shipped.
  function reachEl(t) {
    const filled = t.uses.filter((u) => u.reach);
    if (!filled.length) return B.reachEl(t.needs);
    if (filled.length === 1) return B.reachEl(filled[0].reach);
    return B.el("div", {class: "stack"}, filled.map((u) => B.el("div", {class: "bi-reach"}, B.el("span", {class: "mono faint"}, u.bridge), B.reachEl(u.reach))));
  }

  // versionsEl gives what changed in each version, those since the
  // version that ran before marked.
  function versionsEl(t) {
    if (!t.versions.length) return null;
    const since = new Set((t.changed || []).map((c) => c.version));
    return B.el("div", {class: "bi-versions"}, newestFirst(t.versions).map((c) => [
      B.el("span", {class: "mono" + (since.has(c.version) ? " bi-new" : " faint")}, c.version), textEl(c.text)]));
  }

  // detailEl gives what a thing is, shown when its row is opened.
  function detailEl(t) {
    const actions = [
      t.use ? B.el("button", {class: "btn sm", type: "button", onclick: () => B.drafting(t.use)}, "Use") : "",
      t.copy ? B.el("button", {class: "btn sm ghost", type: "button", onclick: () => B.drafting(t.copy)}, "Copy") : "",
    ].filter(Boolean);
    const differs = (t.copies || []).filter((c) => c.differs.length);
    return B.el("div", {class: "bi-detail"},
      B.kv([
        ["Page", B.el("a", {href: "#/manual/" + t.path}, t.summary)],
        ["Reaches", reachEl(t)],
        ["Settings", settingsEl(t)],
        ["Changed", versionsEl(t)],
        differs.length ? ["Differs", B.el("div", {class: "stack bi-differs"}, differs.map((c) =>
          B.el("span", {class: "line"}, differs.length > 1 ? B.el("span", {class: "mono faint"}, c.folder) : null, B.el("span", {class: "mono"}, c.differs.join(" · ")))))] : null,
      ]),
      actions.length ? B.el("div", {class: "line"}, actions) : null);
  }

  // nameEl gives a thing's name and what it is.
  function nameEl(t) {
    return B.el("span", {class: "bi-name"}, B.el("span", {class: "mono"}, t.name), B.el("span", {class: "faint small"}, kinds[t.kind]));
  }

  // usedRow gives a thing the office uses: who uses it and the line that
  // does, its copies, and what it is when opened.
  function usedRow(t) {
    return B.el("details", {class: "bi", id: "built-in-" + t.path.replace("/", "-")},
      B.el("summary", {class: "bi-row"}, nameEl(t),
        B.el("span", {class: "bi-uses"}, usedEl(t), copiedEl(t)),
        changedEl(t)),
      detailEl(t));
  }

  // availableRow gives a thing the office does not use: what it is for,
  // the line that would use it, and what it is when opened.
  function availableRow(t) {
    return B.el("details", {class: "bi", id: "built-in-" + t.path.replace("/", "-")},
      B.el("summary", {class: "bi-row"}, nameEl(t),
        B.el("span", {class: "bi-for muted"}, t.summary.replace(/^[^:]*:\s*/, "")),
        lineEl(t.line),
        changedEl(t)),
      detailEl(t));
  }

  B.route(/^#\/office\/built-in$/, async function () {
    const out = await B.api("/api/shipped");
    const used = out.things.filter((t) => t.status !== "unused" || (t.copies && t.copies.length));
    const rest = out.things.filter((t) => !used.includes(t));
    return B.el("div", {class: "bis"},
      B.officeHead("built-in", B.el("span", {class: "mono faint"}, "Bulldog " + out.version)),
      B.el("section", {class: "bi-group", "aria-label": "Used"},
        B.label("Used by " + B.me.office.name, B.el("span", {class: "count"}, String(used.length))),
        used.length ? used.map(usedRow) : B.empty()),
      rest.length ? B.el("details", {class: "bi-group bi-available"},
        B.el("summary", {}, B.label("Available", B.el("span", {class: "count"}, String(rest.length)))),
        rest.map(availableRow)) : null);
  }, "office", (what) => what === "office");
})();
