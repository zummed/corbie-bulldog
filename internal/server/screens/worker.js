// Worker: one worker of a team, what it is, how it is started and who
// calls it, its definition folded, and a drawing of what it can do, with
// what each tool reaches, whom it can ask, and the colleagues whose held
// jobs come to its ask. A worker may have hundreds of tools and callers,
// so each list shows a few and More the rest.
(function () {
  "use strict";

  const FACES = 4;   // people drawn before "+n"
  const PAGE = 12;   // rows of a group drawn before More
  const INLINE = 6;  // callers, runs and teams drawn on a line before More
  const NOTES_TOOL = "write_notes";
  const PEOPLE_TOOLS = ["ask_person", "tell"];
  const TEAMS_TOOL = "ask_team";
  const THREAD_TOOL = "read_thread";

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
    return B.el("span", {class: "line wk-lock"}, B.icon("lock", "faint"), B.chain(list, t.name));
  }

  // reachEl gives what a script's or a machine's job reaches, what it is
  // refused and what of it is asked about, nested beneath its tool; none
  // when it reaches nothing.
  function reachEl(tool) {
    const r = tool.reach;
    if (!r || !r.hosts.length && !r.secrets.length && !r.deny.length && !r.ask_about.length) return null;
    const list = (xs) => B.el("span", {class: "line"}, xs.map((x) => B.el("span", {class: "mono"}, x)));
    const rows = B.reachEl(r);
    rows.append(...[
      r.deny.length ? [B.el("dt", {}, "Deny"), B.el("dd", {}, list(r.deny))] : [],
      r.ask_about.length ? [B.el("dt", {}, "Ask about"), B.el("dd", {}, list(r.ask_about))] : [],
    ].flat());
    return B.el("div", {class: "wk-reach"}, rows);
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
    return B.el("div", {class: "group"}, B.label(...[label].flat(), rows.length > PAGE ? B.el("span", {class: "count"}, String(rows.length)) : null),
      paged(key, rows, PAGE, B.el("div", {class: "rows"}), (b) => B.el("div", {class: "row"}, b)));
  }

  // parts gives a flow's stages or a statechart's jobs, each with what it
  // runs: a worker of the team, one of Bulldog's tools, or a file.
  function parts(w, t) {
    return [group("stages", w.type === "flow" ? "Stages" : "Jobs", (w.tools || []).map((tool) => row(
      B.el("span", {class: "wk-c1"}, toolName(tool)),
      B.el("span", {class: "wk-c2 line"},
        tool.kind === "worker" ? B.workerEl(tool.worker, t.name, {bare: true}) : null,
        tool.kind === "bulldog" ? B.el("span", {class: "tag"}, "Bulldog") : null,
        authorisation(tool, t)),
      tool.kind === "worker" ? null : fileEl(tool.file), reachEl(tool))))];
  }

  // groups sorts the worker's tools by what they reach: what it can do
  // itself, whom it can ask, and the changes it can draft. Tools that do
  // not say what they reach are listed as they are.
  function groups(w, t) {
    if (w.type === "flow" || w.type === "statechart") return parts(w, t);
    const tools = w.tools || [];
    if (!tools.some((tool) => tool.kind)) {
      return [group("tools", "Tools", tools.map((tool) => row(B.el("span", {class: "wk-c1 mono"}, tool.name),
        B.el("span", {class: "wk-c2"}, authorisation(tool, t)))))];
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
          B.el("span", {class: "wk-c2"}, w.may_ask.length ? inline("teams", w.may_ask.map(B.teamEl)) : B.el("span", {class: "muted"}, "none"))));
      } else if (tool.kind === "bulldog" && tool.name === THREAD_TOOL) {
        can.push(row(B.el("span", {class: "wk-c1"}, toolName(tool)), B.el("span", {class: "wk-c2"}, "Thread")));
      } else if (tool.kind === "bulldog" && tool.name === NOTES_TOOL) {
        others.push(row(B.el("span", {class: "wk-c1"}, toolName(tool)),
          B.el("span", {class: "wk-c2"}, B.el("a", {href: B.teamHref(t.name)}, "Notes"))));
      } else if (tool.kind === "bulldog") {
        changes.push(B.el("span", {class: "tag", title: tool.does || null}, tool.name));
      } else {
        can.push(row(B.el("span", {class: "wk-c1"}, toolName(tool)),
          B.el("span", {class: "wk-c2"}, authorisation(tool, t)),
          fileEl(tool.file), reachEl(tool)));
      }
    }
    return [
      group("can do", "Can do", can),
      group("can ask", "Can ask", [...colleagues, ...others]),
      group("held here", [B.el("span", {}, "Held here"), ...B.info("About held here", "Colleagues whose actions that wait for a yes are asked about through " + w.name + "'s ask.")],
        w.held_here.map((name) => row(B.el("span", {class: "wk-c1"}, B.workerEl(name, t.name, {bare: true}))))),
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
  // service, the word it is named by with the team that gives it, and the
  // secret it names as the service's key.
  function model(w) {
    if (!w.model && !w.model_name) return null;
    const sent = B.modelText(w.model);
    return B.el("span", {class: "line wk-model"},
      sent ? B.el("span", {class: "mono"}, sent) : null,
      w.model_name ? B.el("span", {class: "mono faint"}, w.model_name + (w.model_from ? " · " + B.teamCalled(w.model_from) : "")) : null,
      w.secret ? B.el("span", {class: "line"}, B.el("span", {class: "faint small"}, "Secret"), B.el("span", {class: "mono"}, w.secret)) : null);
  }

  // start gives how the worker is started: as its team's lead, on its
  // requests, by a routine, or as a tool of a colleague.
  function start(w, t) {
    const callers = (t.workers || []).filter((x) => x.name !== w.name && (x.tools || []).some((tool) => tool.kind === "worker" && tool.worker === w.name));
    const out = [
      t.lead === w.name ? B.el("a", {class: "tag", href: "#/requests?team=" + encodeURIComponent(t.name)}, "Lead") : null,
      (t.routines || []).some((r) => r.to === w.name) ? B.el("span", {class: "tag"}, "Routine") : null,
      ...callers.map((x) => B.el("span", {class: "line"}, B.el("span", {class: "tag"}, "Tool of"), B.workerEl(x.name, t.name, {bare: true}))),
    ].filter(Boolean);
    return out.length ? B.el("span", {class: "line"}, out) : B.el("span", {class: "faint"}, "None");
  }

  // askEl gives where what it runs that waits for a yes is asked about:
  // the ask Bulldog ships, when it names it, or else the nearest above.
  function askEl(w) {
    return w.definition.asks ? B.bulldogEl() : B.el("span", {class: "faint"}, "Above");
  }

  // definition gives what the worker's file writes of how it runs, folded.
  function definition(w) {
    const d = w.definition;
    const list = (xs) => xs.length ? B.el("span", {class: "line"}, xs.map((x) => B.el("span", {class: "mono"}, x))) : null;
    const rows = B.kv([
      ["System", d.system ? B.el("pre", {class: "wk-text"}, d.system) : null],
      ["Prompt", d.prompt ? B.el("pre", {class: "wk-text"}, d.prompt) : null],
      ["Inputs", list(d.inputs)],
      ["Tools files", d.tools_files.length ? B.el("span", {class: "line"}, d.tools_files.map(fileEl)) : null],
      ["Uses", w.uses.length ? B.el("span", {class: "line"}, w.uses.map(fileEl)) : null],
    ]);
    const box = B.el("details", {class: "wk-def"}, B.el("summary", {}, B.label("Definition")), rows);
    return box;
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
    if (t && !t.workers) missing = undefined;
    const w = t && (t.workers || []).find((x) => x.name === name);
    if (!w) return B.el("div", {class: "wk"}, B.head(crumbs, name), B.empty(missing));

    const title = B.el("span", {class: "who wk-title"}, B.el("span", {class: "ic worker lg", "aria-hidden": "true"}, String(w.name).slice(0, 2)), w.name);
    const busy = (t.at_work || []).some((a) => a.worker === w.name);
    return B.el("div", {class: "wk"},
      B.head(crumbs, title,
        t.lead === w.name ? B.el("span", {class: "tag"}, "Lead of " + B.teamCalled(t.name)) : null,
        !w.runs ? B.el("span", {class: "status failed"}, "cannot run") : busy ? B.status({state: "working"}) : null),
      B.el("div", {class: "wk-meta"},
        B.kv([
          ["Start", start(w, t)],
          ["Ask", askEl(w)],
          ["Model", model(w)],
          ["Type", B.el("span", {class: "who"}, B.icon(w.type, "faint"), B.el("span", {class: "mono"}, w.type))],
          ["File", fileEl(w.file)],
        ]),
        B.kv([
          ["Team", B.teamEl(t.name)],
          ["Runs", runs(w, t)],
        ])),
      definition(w),
      drawing(w, t));
  }, "teams");
})();
