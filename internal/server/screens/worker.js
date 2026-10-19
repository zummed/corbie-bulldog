// Worker: one worker of a team, what it is, who calls it, and a drawing of
// what it can do and whom it can ask. A worker may have hundreds of tools
// and callers, so each list shows a few and More the rest.
(function () {
  "use strict";

  const FACES = 4;   // people drawn before "+n"
  const PAGE = 12;   // rows of a group drawn before More
  const INLINE = 6;  // callers, runs and teams drawn on a line before More
  const NOTES_TOOL = "write_notes";
  const PEOPLE_TOOLS = ["ask_person", "tell"];
  const TEAMS_TOOL = "ask_team";

  // What the view keeps while it is drawn again, for one worker: how many
  // of each list are shown.
  const state = {key: "", shown: new Map()};
  function keepFor(key) {
    if (state.key !== key) { state.key = key; state.shown = new Map(); }
  }

  function more(left, label, onclick) {
    return B.el("button", {class: "btn sm ghost", type: "button", "aria-label": label + ", " + left + " more", onclick},
      "More", B.el("span", {class: "mono faint"}, String(left)));
  }

  // paged draws the first of the items into box, and More the next; key
  // keeps how many are shown when the view is drawn again.
  function paged(key, items, page, box, wrap) {
    const fill = () => {
      const n = state.shown.get(key) || page;
      box.replaceChildren(...items.slice(0, n),
        ...(items.length > n ? [wrap(more(items.length - n, "More " + key, () => { state.shown.set(key, n + page); fill(); }))] : []));
    };
    fill();
    return box;
  }
  function inline(key, items) {
    return items.length ? paged(key, items, INLINE, B.el("span", {class: "line"}), (b) => b) : null;
  }

  // pop gives a button that shows what is given on asking.
  let pops = 0;
  function pop(shows, ...face) {
    const id = "wk-pop-" + ++pops;
    return [
      B.el("button", {class: "wk-name", type: "button", popovertarget: id}, ...face),
      B.el("div", {class: "pop", id, popover: "auto"}, ...shows),
    ];
  }
  // toolName gives a tool by its name; a listed one stands for each tool
  // its server lists, named after it.
  function toolName(tool) {
    if (tool.listed) return pop(["Each tool the server of " + tool.name + " lists, as " + tool.name + "_<tool>"], B.el("span", {class: "mono"}, tool.name + "_*"));
    return tool.does ? pop([tool.does], tool.name) : B.el("span", {class: "mono wk-name"}, tool.name);
  }

  // authorisation gives who decides a tool that waits, in order; one that
  // waits with nobody to decide it is refused.
  function authorisation(tool, t) {
    if (!tool.waits) return null;
    const list = tool.deciders || [];
    if (!list.length) return B.el("span", {class: "who"}, B.icon("lock", "faint"), B.el("span", {class: "mono muted"}, "refused"));
    return B.el("span", {class: "chain"},
      B.icon("lock", "faint"),
      B.el("span", {class: "small muted"}, "authorised by"),
      list.map((d, i) => [i ? B.el("span", {class: "then", "aria-hidden": "true"}, "→") : null, B.deciderEl(d, t.name)]));
  }

  // faces gives people as a few icons, the one looking first, and how many
  // more, which the team's page lists.
  function faces(names, t) {
    const list = names.slice().sort((a, b) => B.isMe(b) - B.isMe(a));
    const left = list.length - FACES;
    return B.el("span", {class: "wk-faces"},
      list.slice(0, FACES).map((n) => { const e = B.personEl(n); e.classList.add("wk-face"); return e; }),
      left > 0 ? B.el("a", {class: "mono faint wk-more", href: B.teamHref(t.name), "aria-label": left + " more people"}, "+" + left) : "");
  }

  function fileEl(path) {
    if (!path) return "";
    return B.me.me.decides_changes
      ? B.el("a", {class: "mono small wk-file", href: "#/changes?file=" + encodeURIComponent(path)}, path)
      : B.el("span", {class: "mono small muted wk-file"}, path);
  }

  function row(...cells) { return B.el("div", {class: "row wk-tool"}, ...cells); }
  function group(key, label, rows) {
    if (!rows.length) return null;
    return B.el("div", {class: "group"}, B.label(label, rows.length > PAGE ? B.el("span", {class: "count"}, String(rows.length)) : null),
      paged(key, rows, PAGE, B.el("div", {class: "rows"}), (b) => B.el("div", {class: "row"}, b)));
  }

  // groups sorts the worker's tools by what they reach: what it can do
  // itself, whom it can ask, and the changes it can draft. Tools that do
  // not say what they reach are listed as they are.
  function groups(w, t) {
    const tools = w.tools || [];
    if (!tools.some((tool) => tool.kind)) {
      return [group("tools", "Tools", tools.map((tool) => row(B.el("span", {class: "wk-c1 mono"}, tool.name),
        tool.waits ? B.el("span", {class: "who wk-c2"}, B.icon("lock", "faint"), B.el("span", {class: "mono"}, (tool.deciders || []).map((d) => d.name).join(" → "))) : null)))];
    }
    const can = [], colleagues = [], others = [], changes = [];
    const said = tools.filter((tool) => tool.kind === "bulldog" && PEOPLE_TOOLS.includes(tool.name));
    for (const tool of tools) {
      if (tool.kind === "worker") {
        const name = tool.worker || tool.name;
        const face = B.el("span", {class: "who"}, B.el("span", {class: "ic worker", "aria-hidden": "true"}, String(name).slice(0, 2)), name);
        colleagues.push(row(B.el("span", {class: "wk-c1"}, tool.does
          ? pop([B.workerEl(name, t.name, {bare: true}), B.el("div", {class: "wk-does"}, tool.does)], face)
          : B.workerEl(name, t.name, {bare: true}))));
      } else if (tool.kind === "bulldog" && PEOPLE_TOOLS.includes(tool.name)) {
        if (tool === said[0] && (t.people || []).length) others.push(row(B.el("span", {class: "wk-c1 line"}, said.map(toolName)),
          B.el("span", {class: "wk-c2"}, faces(t.people, t))));
      } else if (tool.kind === "bulldog" && tool.name === TEAMS_TOOL) {
        others.push(row(B.el("span", {class: "wk-c1"}, toolName(tool)),
          B.el("span", {class: "wk-c2"}, w.may_ask && w.may_ask.length ? inline("teams", w.may_ask.map(B.teamEl)) : B.el("span", {class: "muted"}, "other teams"))));
      } else if (tool.kind === "bulldog" && tool.name === NOTES_TOOL) {
        others.push(row(B.el("span", {class: "wk-c1"}, toolName(tool)),
          B.el("span", {class: "wk-c2"}, B.el("a", {href: B.teamHref(t.name)}, "Notes"))));
      } else if (tool.kind === "bulldog") {
        changes.push(B.el("span", {class: "tag", title: tool.does || null}, tool.name));
      } else {
        can.push(row(B.el("span", {class: "wk-c1"}, toolName(tool)),
          B.el("span", {class: "wk-c2"}, authorisation(tool, t)),
          fileEl(tool.file)));
      }
    }
    return [
      group("can do", "Can do", can),
      group("can ask", "Can ask", [...colleagues, ...others]),
      changes.length ? B.el("div", {class: "group"}, B.label("Changes"), B.el("div", {class: "line wk-tags"}, changes)) : null,
    ];
  }

  function drawing(w, t) {
    const gs = groups(w, t).filter(Boolean);
    const me = B.el("div", {class: "me wk-me"},
      B.el("span", {class: "who"}, B.el("span", {class: "ic worker", "aria-hidden": "true"}, String(w.name).slice(0, 2)), B.el("span", {class: "name"}, w.name)),
      B.el("span", {class: "kind"}, "worker · " + B.teamCalled(t.name)));
    if (!gs.length) return B.el("div", {class: "reach"}, me);
    return B.el("div", {class: "reach"}, me, B.el("div", {class: "wire", "aria-hidden": "true"}), B.el("div", {class: "groups"}, gs));
  }

  // model gives a model as it is sent in a few words, its name and
  // service, and the word it is named by with the team that gives it.
  function model(w) {
    if (!w.model && !w.model_name) return null;
    const field = (k) => {
      const m = String(w.model || "").match(new RegExp("(?:^|[{,\\s])" + k + "\\s*:\\s*(\"[^\"]*\"|'[^']*'|[^,}\\s]+)"));
      return m ? m[1].replace(/^["']|["']$/g, "") : "";
    };
    const sent = [field("name"), field("service")].filter(Boolean).join(" · ");
    return B.el("span", {class: "line wk-model"},
      sent ? B.el("span", {class: "mono"}, sent) : null,
      w.model_name ? B.el("span", {class: "mono faint"}, w.model_name + (w.model_from ? " · " + B.teamCalled(w.model_from) : "")) : null);
  }

  // calledBy gives what starts the worker: the team's requests when it is
  // the lead, colleagues that have it as a tool, and routines.
  function calledBy(w, t) {
    const callers = (t.workers || []).filter((x) => x.name !== w.name && (x.tools || []).some((tool) =>
      tool.kind ? tool.kind === "worker" && tool.worker === w.name : tool.name === w.name));
    return inline("callers", [
      t.lead === w.name ? B.el("a", {href: "#/requests?team=" + encodeURIComponent(t.name)}, B.teamCalled(t.name) + " requests") : null,
      ...callers.map((x) => B.workerEl(x.name, t.name, {bare: true})),
      ...(t.routines || []).filter((r) => r.to === w.name).map((r) =>
        B.el("span", {class: "who"}, B.icon("clock", "faint"), "routine", B.el("span", {class: "mono"}, r.at))),
    ].filter(Boolean));
  }

  function runs(w, t) {
    return inline("runs", (t.at_work || []).filter((a) => a.worker === w.name).map((a) => B.el("span", {class: "mono"},
      B.el("a", {href: "#/r/" + a.request}, "#" + a.request), B.el("span", {class: "faint"}, " · "),
      B.el("a", {href: "#/run/" + encodeURIComponent(a.run), "aria-label": "Watch #" + a.request}, "Watch"))));
  }

  B.route(/^#\/t\/([^/]+)\/w\/([^/]+)$/, async function (m) {
    const team = decodeURIComponent(m[1]), name = decodeURIComponent(m[2]);
    keepFor(team + "/" + name);
    const crumbs = [B.el("a", {href: "#/teams"}, "Teams"), B.el("a", {href: B.teamHref(team)}, B.teamCalled(team)), name];
    let t = null, missing = "There is nothing here by that name.";
    try { t = await B.api("/api/teams/" + encodeURIComponent(team)); } catch (err) { missing = err.message; }
    if (t && !t.workers) missing = "Shown to the people of " + B.teamCalled(team);
    const w = t && (t.workers || []).find((x) => x.name === name);
    if (!w) return B.el("div", {class: "wk"}, B.head(crumbs, name), B.empty(missing));

    const title = B.el("span", {class: "who wk-title"}, B.el("span", {class: "ic worker lg", "aria-hidden": "true"}, String(w.name).slice(0, 2)), w.name);
    return B.el("div", {class: "wk"},
      B.head(crumbs, title,
        t.lead === w.name ? B.el("span", {class: "tag"}, "Lead of " + B.teamCalled(t.name)) : null,
        w.runs ? null : B.el("span", {class: "status failed"}, "cannot run")),
      B.el("div", {class: "wk-meta"},
        B.kv([
          ["Type", B.el("span", {class: "who"}, B.icon(w.type, "faint"), B.el("span", {class: "mono"}, w.type))],
          ["File", B.el("span", {class: "mono"}, w.file)],
          ["Model", model(w)],
        ]),
        B.kv([
          ["Team", B.teamEl(t.name)],
          ["Called by", calledBy(w, t)],
          ["Runs", runs(w, t)],
        ])),
      drawing(w, t));
  }, "teams");
})();
