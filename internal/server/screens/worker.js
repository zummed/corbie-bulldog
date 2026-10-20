// Worker: one worker of a team, what it is, how it is started and who
// calls it, its definition folded, and a drawing of what it can do, with
// what each tool reaches and who decides it, the colleagues it calls,
// what it does with Bulldog's tools, by their tools files, and the colleagues
// whose held jobs come to its ask. A worker may have hundreds of tools
// and callers, so each list shows a few and More the rest.
(function () {
  "use strict";

  const FACES = 4;   // people drawn before "+n"
  const PAGE = 12;   // rows of a group drawn before More
  const INLINE = 6;  // callers, runs and teams drawn on a line before More
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

  // toolName gives a tool by its name, which shows what it does and, of
  // one of Bulldog's, the tools file it comes in; a listed one stands for each
  // tool its server lists, named after it.
  function toolName(tool) {
    if (tool.listed) return pop(["Each tool the server of " + tool.name + " lists, as " + tool.name + "_<tool>"], B.el("span", {class: "mono"}, tool.name + "_*"));
    const from = tool.kind === "bulldog" ? B.el("div", {class: "wk-does faint"}, "Built-in" + (tool.kit ? ", in tools/" + tool.kit + ".yaml" : "")) : null;
    return tool.does || from ? pop([tool.does, from], tool.name) : B.el("span", {class: "mono wk-name"}, tool.name);
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
    const rows = B.reachEl(r) || B.kv([]);
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
      ? B.el("a", {class: "mono small wk-file", href: "#/office/changes?file=" + encodeURIComponent(path)}, path)
      : B.el("span", {class: "mono small muted wk-file"}, path);
  }

  // row gives one line of the drawing; tools names the tools it is of, so
  // that a link to one of them is brought to it.
  function row(tools, ...cells) { return B.el("div", {class: "row wk-tool", "data-tools": tools.map((x) => x.name).join(" ")}, ...cells); }
  function group(key, label, rows) {
    if (!rows.length) return null;
    return B.el("div", {class: "group"}, B.label(...[label].flat(), rows.length > PAGE ? B.el("span", {class: "count"}, String(rows.length)) : null),
      paged(key, rows, PAGE, B.el("div", {class: "rows"}), (b) => B.el("div", {class: "row"}, b)));
  }

  // parts gives a flow's stages or a statechart's jobs, each with what it
  // runs: a worker of the team, one of Bulldog's tools, or a file.
  function parts(w, t) {
    return [group("stages", w.type === "flow" ? "Stages" : "Jobs", (w.tools || []).map((tool) => row([tool],
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
      return [group("tools", "Tools", tools.map((tool) => row([tool], B.el("span", {class: "wk-c1 mono"}, tool.name),
        B.el("span", {class: "wk-c2"}, authorisation(tool, t)))))];
    }
    const can = [], colleagues = [], kitted = new Map();
    for (const tool of tools) {
      if (tool.kind === "worker") {
        const name = tool.worker || tool.name;
        const face = B.el("span", {class: "who"}, B.el("span", {class: "ic worker", "aria-hidden": "true"}, String(name).slice(0, 2)), name);
        colleagues.push(row([tool], B.el("span", {class: "wk-c1"}, tool.does
          ? pop([B.workerEl(name, t.name, {bare: true}), B.el("div", {class: "wk-does"}, tool.does)], face)
          : B.workerEl(name, t.name, {bare: true})), B.el("span", {class: "wk-c2"}, authorisation(tool, t))));
      } else if (tool.kind === "bulldog" && B.KITS[tool.kit]) {
        kitted.set(tool.kit, [...(kitted.get(tool.kit) || []), tool]);
      } else {
        can.push(row([tool], B.el("span", {class: "wk-c1"}, toolName(tool)),
          B.el("span", {class: "wk-c2 line"}, B.el("span", {class: "mono faint small"}, tool.kind === "bulldog" ? "Built-in" : tool.kind), authorisation(tool, t)),
          fileEl(tool.file), reachEl(tool)));
      }
    }
    const kits = Object.keys(B.KITS).filter((k) => kitted.has(k)).map((k) => {
      const list = kitted.get(k);
      return row(list, B.el("span", {class: "wk-c1"}, B.KITS[k]),
        B.el("span", {class: "wk-c2 wk-kit"},
          B.el("span", {class: "line"}, list.map((tool) => B.el("span", {class: "line wk-kit-tool"}, toolName(tool), authorisation(tool, t)))),
          kitReach(k, list, w, t)));
    });
    return [
      group("can do", "Can do", can),
      group("colleagues", "Colleagues", colleagues),
      group("built-in", "Built-in", kits),
      group("held here", [B.el("span", {}, "Held here"), ...B.info("About held here", "Colleagues whose actions that wait for a yes are asked about through " + w.name + "'s ask.")],
        w.held_here.map((name) => row([], B.el("span", {class: "wk-c1"}, B.workerEl(name, t.name, {bare: true}))))),
    ];
  }

  // kitReach gives whom or what a tools file of Bulldog's reaches: the
  // talk tools the team's people and the teams it may ask, the notes tools
  // the team's notes, and the docs tools the team's docs, when it has any.
  function kitReach(kit, list, w, t) {
    const has = (name) => list.some((tool) => tool.name === name);
    if (kit === "talk/talk") return B.el("span", {class: "line"},
      PEOPLE_TOOLS.some(has) && (t.people || []).length ? faces(t.people, t) : null,
      has(TEAMS_TOOL) && w.may_ask.length ? inline("teams", w.may_ask.map(B.teamEl)) : null);
    if (kit === "notes/notes") return B.el("a", {href: B.teamHref(t.name)}, "Notes");
    if (kit.startsWith("docs/") && (t.docs || []).length) return B.el("a", {href: B.teamHref(t.name)}, "Docs");
    return null;
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

  // start says in words how the worker is started: as its team's lead,
  // on each of its requests; by its routines, at their times; and by the
  // colleagues that call it as a tool; or that nothing starts it.
  function start(w, t) {
    const callers = (t.workers || []).filter((x) => x.name !== w.name && (x.tools || []).some((tool) => tool.kind === "worker" && tool.worker === w.name));
    const at = (t.routines || []).filter((r) => r.to === w.name).map((r) => r.at);
    const sep = () => B.el("span", {class: "faint", "aria-hidden": "true"}, "·");
    const out = [
      t.lead === w.name ? B.el("span", {class: "line"}, "On each", B.el("a", {href: "#/requests?team=" + encodeURIComponent(t.name)}, "request"), "to " + B.teamCalled(t.name), ...B.leadLabel(t, "").querySelectorAll(".info, .pop")) : null,
      at.length ? B.el("span", {class: "line"}, "By its routine" + (at.length > 1 ? "s" : "") + " at", at.map((x) => B.el("span", {class: "mono"}, x))) : null,
      callers.length ? B.el("span", {class: "line"}, "Called by", callers.map((x) => B.workerEl(x.name, t.name, {bare: true}))) : null,
    ].filter(Boolean);
    return out.length ? B.el("span", {class: "line"}, out.flatMap((x, i) => [i ? sep() : null, x])) : B.el("span", {class: "faint"}, "Nothing");
  }

  // askEl gives where what it runs that waits for a yes is asked about:
  // the ask Bulldog ships, when it names it, or else the colleagues whose
  // ask its held jobs come to; none when there is neither.
  function askEl(w, t) {
    if (w.definition.asks) return "Bulldog's ask";
    const via = (t.workers || []).filter((x) => (x.held_here || []).includes(w.name));
    return via.length ? B.el("span", {class: "line"}, via.map((x) => B.workerEl(x.name, t.name, {bare: true}))) : null;
  }

  // inOffice gives a path the worker's file writes, from its own folder,
  // as the path in the office it names.
  function inOffice(w, path) {
    const parts = w.file.split("/").slice(0, -1);
    for (const step of path.split("/")) {
      if (step === "" || step === ".") continue;
      if (step === "..") parts.pop(); else parts.push(step);
    }
    return parts.join("/");
  }

  // definition gives what the worker's file writes of how it runs, folded.
  function definition(w) {
    const d = w.definition;
    const list = (xs) => xs.length ? B.el("span", {class: "line"}, xs.map((x) => B.el("span", {class: "mono"}, x))) : null;
    const rows = B.kv([
      ["System", d.system ? B.el("pre", {class: "wk-text"}, d.system) : null],
      ["Prompt", d.prompt ? B.el("pre", {class: "wk-text"}, d.prompt) : null],
      ["Inputs", list(d.inputs)],
      ["Tools files", d.tools_files.length ? B.el("span", {class: "line"}, d.tools_files.map((path) => fileEl(inOffice(w, path)))) : null],
      ["Uses", w.uses.length ? B.el("span", {class: "line"}, w.uses.map(fileEl)) : null],
    ]);
    const box = B.el("details", {class: "wk-def"}, B.el("summary", {}, B.label("Definition")), rows);
    return box;
  }

  // statusEl gives the worker's one status, cannot run or at work, with
  // each request it is at work on, to watch.
  function statusEl(w, t) {
    if (!w.runs) return B.el("span", {class: "status failed"}, "cannot run");
    const busy = B.atWork(t).filter((a) => a.worker === w.name);
    if (!busy.length) return null;
    return B.el("span", {class: "line"}, B.status({state: "working"}), inline("runs", busy.map((a) =>
      B.el("a", {class: "mono small", href: "#/run/" + encodeURIComponent(a.run), title: "Watch " + w.name + " on #" + a.request, "aria-label": "Watch " + w.name + " on #" + a.request}, "#" + a.request))));
  }

  // bringTo brings the row of a tool into view, and marks it, when the
  // address names one.
  function bringTo(root, tool) {
    if (!tool) return;
    setTimeout(() => {
      const at = [...root.querySelectorAll(".wk-tool[data-tools]")].find((r) => r.dataset.tools.split(" ").includes(tool));
      if (!at || !at.isConnected) return;
      at.classList.add("wk-hit");
      at.scrollIntoView({block: "center"});
    }, 0);
  }

  B.route(/^#\/t\/([^/]+)\/w\/([^/?]+)(?:\?.*)?$/, async function (m) {
    const team = decodeURIComponent(m[1]), name = decodeURIComponent(m[2]);
    keepFor(team + "/" + name);
    const crumbs = [B.el("a", {href: "#/teams"}, "Teams"), B.el("a", {href: B.teamHref(team)}, B.teamCalled(team)), name];
    let t = null, missing = "There is nothing here by that name.";
    try { t = await B.api("/api/teams/" + encodeURIComponent(team)); } catch (err) { missing = err.message; }
    if (t && !t.workers) missing = undefined;
    const w = t && (t.workers || []).find((x) => x.name === name);
    if (!w) return B.el("div", {class: "wk"}, B.head(crumbs, name), B.empty(missing));

    const title = B.el("span", {class: "who wk-title"}, B.el("span", {class: "ic worker lg", "aria-hidden": "true"}, String(w.name).slice(0, 2)), w.name);
    const out = B.el("div", {class: "wk"},
      B.head(crumbs, title, statusEl(w, t)),
      B.el("div", {class: "wk-meta"},
        B.kv([
          ["Started", start(w, t)],
          [B.el("span", {class: "line"}, "Asks through", ...B.info("About asks through",
            "Where a job it runs that waits for a yes goes to be decided: the ask Bulldog ships, which its file names, or the ask of the colleague that calls it.")), askEl(w, t)],
          ["Model", model(w)],
          ["Type", B.el("span", {class: "who"}, B.icon(w.type, "faint"), B.el("span", {class: "mono"}, w.type))],
          ["File", fileEl(w.file)],
        ])),
      definition(w),
      drawing(w, t));
    bringTo(out, B.query().get("tool"));
    return out;
  }, "teams");
})();
