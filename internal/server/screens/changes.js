// Changes: the office's files, a change of one's own drafted and checked
// and put to be decided, and the history. An
// office may have hundreds of files and thousands of changes, so a folder
// is drawn only when it is opened, lists are paged, and the lists are
// fetched again only when the office changed.
(function () {
  "use strict";

  const PAGE = 50;
  const empty = () => ({files: {}, set: [], move: {}, remove: []});

  // The draft of a change of one's own, as it is made on the screens: whole
  // files, or null for one removed; fields of files set; and workers moved
  // or taken away. It is kept while the screens are open.
  let draft = empty();

  // The office's files and history, as fetched for the office the
  // screens last read; they are fetched again when it is read again.
  let lists = null;

  // What the view keeps while it is drawn again: the folders open, what is
  // typed and not yet in the draft, the draft's last check and how many
  // times the draft changed, what is searched for, and how much of each
  // list is shown.
  const state = {
    open: new Set(), sel: null, typed: {}, worker: "", to: "", path: "", why: "",
    check: null, drafts: 0, fileQ: "", histQ: "", pages: new Map(),
  };
  addEventListener("hashchange", () => {
    if (/^#\/(changes|history\/)/.test(location.hash)) return;
    lists = null;
    state.fileQ = ""; state.histQ = ""; state.pages = new Map();
  });

  function draftBody() {
    const d = {};
    if (Object.keys(draft.files).length) d.files = draft.files;
    if (draft.set.length) d.set = draft.set;
    if (Object.keys(draft.move).length) d.move = draft.move;
    if (draft.remove.length) d.remove = draft.remove;
    return d;
  }

  // drafted changes the draft, and its last check no longer holds.
  function drafted(change, update) {
    change();
    state.check = null;
    state.drafts++;
    update();
  }

  // B.drafting adds a draft another screen made, as Use and Copy on
  // Shipped do, to the draft, a field set again taking the place of the
  // one before, and opens Changes on it.
  B.drafting = function (edit) {
    const same = (a, b) => a.path === b.path && a.field.join("\n") === b.field.join("\n");
    Object.assign(draft.files, edit.files || {});
    for (const s of edit.set || []) draft.set = draft.set.filter((x) => !same(x, s)).concat([s]);
    state.check = null;
    state.drafts++;
    location.hash = "#/changes";
  };

  // cleared empties the draft, once it is put.
  function cleared() {
    draft = empty();
    state.typed = {};
    state.why = "";
    state.check = null;
    state.drafts++;
    lists = null;
  }

  const fileHref = (path) => "#/changes?file=" + encodeURIComponent(path);

  // actor gives who made or decided a change, by the kind of thing it is;
  // any other name is taken for a worker's.
  function actor(name) {
    if (!name) return "";
    if (name === "bulldog") return B.bulldogEl();
    if (B.person(name)) return B.personEl(name);
    if (name.startsWith("./")) return B.ruleEl(name);
    return B.workerEl(name);
  }

  // paged draws the items a page at a time, with More for the next. How
  // many are shown is kept by key, and reaches the item at when there is one.
  function paged(items, draw, key, at) {
    const box = B.el("div", {});
    let shown = Math.max(state.pages.get(key) || PAGE, at >= 0 ? (Math.floor(at / PAGE) + 1) * PAGE : 0);
    const fill = () => box.replaceChildren(...items.slice(0, shown).map(draw),
      items.length > shown ? B.el("div", {class: "ch-pager"},
        B.el("button", {class: "btn sm ghost ch-more", type: "button", onclick: () => { shown += PAGE; state.pages.set(key, shown); fill(); }},
          "More", B.el("span", {class: "mono faint"}, String(items.length - shown)))) : "");
    fill();
    return box;
  }

  function textRows(list) {
    return B.el("div", {class: "rows"}, list.map((s) => B.el("div", {class: "row"}, B.el("span", {class: "grow"}, s))));
  }

  // A file's mark: what the draft does to it, or else the one other thing
  // so of it. Marks are set again from the draft when it changes.
  function mark(path, rest) {
    const m = B.el("span", {"data-mark": path, "data-rest": rest || ""});
    setMark(m);
    return m;
  }
  function setMark(m) {
    const path = m.dataset.mark, d = path in draft.files;
    m.className = d ? "ch-in" : m.dataset.rest ? "tag" : "";
    m.textContent = d ? (draft.files[path] === null ? "removed" : "drafted") : m.dataset.rest;
    m.hidden = m.textContent === "";
  }
  function remark() {
    for (const m of document.querySelectorAll("#main .ch [data-mark]")) setMark(m);
  }

  // Files

  // folders makes a tree of the paths: each folder its folders and files.
  function folders(files) {
    const root = {name: "", path: "", dirs: new Map(), files: [], n: 0};
    for (const f of files) {
      let node = root;
      node.n++;
      for (const part of f.path.split("/").slice(0, -1)) {
        if (!node.dirs.has(part)) node.dirs.set(part, {name: part, path: (node.path ? node.path + "/" : "") + part, dirs: new Map(), files: [], n: 0});
        node = node.dirs.get(part);
        node.n++;
      }
      node.files.push(f);
    }
    return root;
  }

  function fileLine(f, selected, label) {
    return B.el("div", {class: "ch-file"},
      B.el("a", {href: fileHref(f.path), "aria-current": f.path === selected ? "true" : null, title: f.path}, label),
      mark(f.path, f.by_hand ? "by hand" : ""));
  }

  // folder draws a folder's folders, each drawn when it is opened, and its
  // files.
  function folder(node, selected) {
    const dirs = [...node.dirs.values()].sort((a, b) => a.name.localeCompare(b.name)).map((d) => {
      const sub = B.el("div", {class: "sub"});
      let built = false;
      const build = () => { if (!built) { built = true; sub.replaceChildren(...folder(d, selected)); } };
      const det = B.el("details", {open: state.open.has(d.path) || null},
        B.el("summary", {class: "dir"}, d.name + "/", B.el("span", {class: "ch-n"}, String(d.n))), sub);
      if (det.open) build();
      det.addEventListener("toggle", () => {
        if (det.open) { state.open.add(d.path); build(); } else state.open.delete(d.path);
      });
      return det;
    });
    const at = node.path ? node.path.length + 1 : 0;
    return [...dirs, paged(node.files, (f) => fileLine(f, selected, f.path.slice(at)), "dir:" + node.path, node.files.findIndex((f) => f.path === selected))];
  }

  // filesTree draws the files as folders, or flat as a search finds them.
  function filesTree(files, selected) {
    const s = B.search("Search files", files, (f) => f.path, (list, q) => {
      if (!list.length) return B.empty();
      return B.el("div", {class: "files-tree"}, q
        ? paged(list, (f) => fileLine(f, selected, f.path), "found", list.findIndex((f) => f.path === selected))
        : folder(folders(list), selected));
    });
    if (state.fileQ) { s.box.value = state.fileQ; s.box.dispatchEvent(new Event("input")); }
    s.box.addEventListener("input", () => { state.fileQ = s.box.value; });
    return B.sect(B.label("Files", B.el("span", {class: "count"}, String(files.length))), s.box, s.out);
  }

  // The draft

  // editor draws the file open: its text as typed, as drafted, or as it is.
  // text is null for a file that is not there.
  function editor(path, listed, text, update) {
    if (!path) return "";
    const box = B.el("textarea", {class: "in code", spellcheck: "false", "aria-label": path});
    const own = draft.files[path];
    box.value = state.typed[path] !== undefined ? state.typed[path] : typeof own === "string" ? own : text || "";
    box.dataset.was = typeof own === "string" ? own : text || "";
    box.addEventListener("input", () => { state.typed[path] = box.value; });
    const put = (t) => drafted(() => { draft.files[path] = t; delete state.typed[path]; }, update);
    return B.el("div", {class: "stack ch-editor"},
      B.el("div", {class: "line"}, B.el("span", {class: "who"}, B.icon("file"), B.el("span", {class: "mono"}, path)),
        mark(path, text !== null ? "" : listed ? "gone" : "new")),
      box,
      B.el("div", {class: "line"},
        B.el("button", {class: "btn sm", type: "button", onclick: () => put(box.value)}, "Add to draft"),
        text !== null ? B.el("button", {class: "btn sm no", type: "button", onclick: () => put(null)}, "Remove file") : ""));
  }

  // kept gives an input that keeps what is typed in it in state[key].
  function kept(key, attrs) {
    const input = B.el("input", {class: "in", type: "text", ...attrs});
    input.value = state[key];
    input.addEventListener("input", () => { state[key] = input.value; });
    return input;
  }

  function newFile() {
    const path = kept("path", {class: "in mono"});
    return B.el("form", {class: "ch-form", onsubmit: (e) => {
      e.preventDefault();
      const p = path.value.trim().replace(/^\/+/, "");
      if (p) { path.value = state.path = ""; location.hash = fileHref(p); }
    }}, B.field("New file", path), B.el("button", {class: "btn sm ghost", type: "submit"}, "Open"));
  }

  // workers draws a worker moved to a team, or taken away. The team is
  // found as it is typed; the one looking's own teams come first.
  function workers(update) {
    const worker = kept("worker", {class: "in mono"});
    const mine = new Set(B.me.me.teams || []);
    const teams = (B.me.teams || []).slice().sort((a, b) => (mine.has(b.name) - mine.has(a.name)) || a.called.localeCompare(b.called));
    const to = kept("to", {list: "ch-teams", autocomplete: "off"});
    const team = () => { const v = to.value.trim(); const t = teams.find((x) => x.name === v || x.called === v); return t ? t.name : v; };
    const move = () => {
      const w = worker.value.trim(), t = team();
      if (!w || !t) return;
      worker.value = state.worker = to.value = state.to = "";
      drafted(() => { draft.move[w] = t; draft.remove = draft.remove.filter((x) => x !== w); }, update);
    };
    const away = () => {
      const w = worker.value.trim();
      if (!w) return;
      worker.value = state.worker = "";
      drafted(() => { delete draft.move[w]; if (!draft.remove.includes(w)) draft.remove.push(w); }, update);
    };
    return B.el("div", {class: "ch-form"},
      B.field("Worker", worker), B.field("To", to),
      B.el("datalist", {id: "ch-teams"}, teams.map((t) => B.el("option", {value: t.name}, t.called))),
      B.el("button", {class: "btn sm ghost", type: "button", onclick: move}, "Move"),
      B.el("button", {class: "btn sm no", type: "button", onclick: away}, "Remove"));
  }

  function partRow(kind, what, drop, update) {
    return B.el("div", {class: "row"},
      B.el("span", {class: "ch-kind"}, kind),
      B.el("span", {class: "grow line"}, what),
      B.el("button", {class: "btn sm ghost", type: "button", onclick: () => drafted(drop, update)}, "Take out"));
  }

  // parts draws what is in the draft, each part with a way to take it out.
  function parts(update) {
    const rows = [
      ...Object.entries(draft.files).map(([p, t]) => partRow(t === null ? "remove" : "write",
        B.el("a", {class: "mono", href: fileHref(p)}, p), () => { delete draft.files[p]; }, update)),
      ...draft.set.map((s) => partRow("set",
        [B.el("a", {class: "mono", href: fileHref(s.path)}, s.path), B.el("span", {class: "mono faint"}, s.field.join("."))],
        () => { draft.set = draft.set.filter((x) => x !== s); }, update)),
      ...Object.entries(draft.move).map(([w, t]) => partRow("move",
        [B.workerEl(w), B.el("span", {class: "faint mono"}, "→"), B.teamEl(t)], () => { delete draft.move[w]; }, update)),
      ...draft.remove.map((w) => partRow("remove", B.workerEl(w), () => { draft.remove = draft.remove.filter((x) => x !== w); }, update)),
    ];
    return rows.length ? B.el("div", {class: "rows"}, rows) : "";
  }

  function problemRow(p) {
    return B.el("div", {class: "row ch-bad"},
      p.file ? B.el("span", {class: "mono small"}, p.file + (p.line ? ":" + p.line : "")) : "",
      B.el("span", {class: "grow"}, p.said));
  }

  // result draws a draft's check: whether it can be put, what it says and
  // lets more by, what keeps it from being put, and the files it writes.
  function result(c, update) {
    if (!c) return "";
    const why = kept("why", {});
    const put = B.el("button", {class: "btn go", type: "button", disabled: !state.why.trim() || null}, "Put");
    why.addEventListener("input", () => { put.disabled = !why.value.trim(); });
    put.addEventListener("click", async () => {
      B.trouble(null);
      put.disabled = true;
      try {
        const out = await B.post("/api/office/put", {...draftBody(), why: state.why, mark: c.mark});
        cleared();
        B.said(out.said);
        await B.show();
      } catch (err) {
        // A change put whose writing failed is put all the same, and its
        // draft is not to be put again; any other refusal wants a new check.
        if (/put to be decided, as change \d+/.test(err.message)) { cleared(); await B.show(); } else { state.check = null; update(); }
        B.trouble(err);
      }
    });
    const tag = (f) => f.gone ? B.el("span", {class: "tag"}, "removed") : f.had ? "" : B.el("span", {class: "tag"}, "new");
    return B.el("div", {class: "stack ch-check"},
      B.el("div", {}, B.el("span", {class: "status " + (c.fit ? "ch-fit" : "failed")}, c.fit ? "can be put" : "cannot be put")),
      c.told.length ? textRows(c.told) : "",
      c.says.length ? B.sect("Says", textRows(c.says)) : "",
      c.lets_more_by.length ? B.sect(B.label("Lets more by", ...B.info("About lets more by", "Where someone may see or do more than before, or something that waited for a yes no longer does.")),
        B.el("div", {class: "lets"}, textRows(c.lets_more_by))) : "",
      c.problems.length || c.refused.length ? B.sect("Problems", B.el("div", {class: "rows"}, c.problems.map(problemRow),
        c.refused.map((r) => B.el("div", {class: "row ch-bad"}, B.el("span", {class: "grow"}, r))))) : "",
      c.standing.length ? B.sect("Standing problems", B.el("div", {class: "rows"}, c.standing.map(problemRow))) : "",
      c.files.length ? B.sect("Files", B.el("div", {class: "rows"}, c.files.map((f) =>
        B.el("div", {class: "row"}, B.el("a", {class: "mono grow", href: fileHref(f.path)}, f.path), tag(f))))) : "",
      c.fit ? B.el("div", {class: "ch-form"}, B.field("Why", why), put) : "");
  }

  // History

  function history(list) {
    const row = (c) => B.el("div", {class: "row ch-hist"},
      B.el("a", {class: "clip", href: "#/history/" + encodeURIComponent(c.id), title: c.why}, c.why),
      B.el("span", {class: "ch-cell"}, actor(c.by)),
      B.el("span", {class: "ch-cell"}, actor(c.decided_by)),
      B.el("span", {class: "ch-at"}, B.whenEl(c.at)));
    const s = B.search("Search history", list, (c) => c.why + " " + c.by + " " + (c.decided_by || ""), (found) => found.length
      ? B.el("div", {class: "rows"},
        B.el("div", {class: "row ch-hist ch-cols"}, B.el("span", {}, "Change"), B.el("span", {}, "By"), B.el("span", {}, "Decided by"), B.el("span", {class: "ch-at"}, "When")),
        paged(found, row, "history"))
      : B.empty());
    if (state.histQ) { s.box.value = state.histQ; s.box.dispatchEvent(new Event("input")); }
    s.box.addEventListener("input", () => { state.histQ = s.box.value; });
    return B.sect(B.label("History", B.el("span", {class: "count"}, String(list.length))), s.box, s.out);
  }

  // The view

  B.route(/^#\/changes(\?.*)?$/, async function () {
    const selected = (B.query().get("file") || "").trim();
    if (!lists || lists.me !== B.me) {
      const me = B.me;
      const [files, past] = await Promise.all([B.api("/api/office/files"), B.api("/api/history")]);
      lists = {me, files: files.files, history: past.history};
    }
    const listed = lists.files.find((f) => f.path === selected);
    let text = null;
    if (listed) {
      try { text = (await B.api("/api/office/file?path=" + encodeURIComponent(selected))).text || ""; } catch (err) {
        // A file removed by hand is listed, and is not there.
        if (!listed.by_hand) B.trouble(err);
      }
    }
    // The way to a file opened is opened once; it may be closed after.
    if (selected !== state.sel) {
      state.sel = selected;
      const steps = selected.split("/");
      for (let i = 1; i < steps.length; i++) state.open.add(steps.slice(0, i).join("/"));
    }

    const editorBox = B.el("div", {});
    const partsBox = B.el("div", {});
    const resultBox = B.el("div", {});
    const check = B.el("button", {class: "btn go", type: "button"}, "Check");

    // update draws again what the draft shows: the files' marks, the file
    // open, the draft's parts and its check.
    const update = () => {
      remark();
      editorBox.replaceChildren(editor(selected, Boolean(listed), text, update));
      partsBox.replaceChildren(parts(update));
      check.disabled = Object.keys(draftBody()).length === 0;
      resultBox.replaceChildren(result(state.check, update));
    };

    // A check is shown only for the draft it was asked of.
    check.addEventListener("click", async () => {
      B.trouble(null);
      check.disabled = true;
      const n = state.drafts;
      try {
        const c = await B.post("/api/office/check", draftBody());
        if (n === state.drafts) state.check = c;
      } catch (err) { if (n === state.drafts) { state.check = null; B.trouble(err); } }
      update();
    });
    update();

    return B.el("div", {class: "ch"},
      B.head(null, "Changes"),
      B.el("div", {class: "cols"},
        B.el("div", {class: "main"},
          B.sect(B.label("Draft"), B.el("div", {class: "stack"}, editorBox, newFile(), workers(update), partsBox, B.el("div", {}, check), resultBox)),
          history(lists.history)),
        B.el("div", {class: "side ch-files"}, filesTree(lists.files, selected))));
  }, "changes", (what) => what === "office");

  // One change of the history, each file it touched before and after. A
  // file's texts are drawn when it is opened; a change of a few files has
  // them open.
  B.route(/^#\/history\/(.+)$/, async function (m) {
    const out = await B.api("/api/history/" + encodeURIComponent(decodeURIComponent(m[1])));
    const c = out.commit;
    const few = out.files.length <= 3;
    const side = (label, text) => B.el("div", {class: "ch-side"}, B.el("div", {class: "kind"}, label), text ? B.el("pre", {}, text) : "");
    const tag = (f) => f.new ? B.el("span", {class: "tag"}, "new") : f.gone ? B.el("span", {class: "tag"}, "removed") : "";
    const version = (f) => {
      const det = B.el("details", {class: "ch-version", open: few || null}, B.el("summary", {}, B.el("span", {class: "mono"}, f.path), tag(f)));
      let built = false;
      const build = () => {
        if (built || !det.open) return;
        built = true;
        det.append(f.binary ? B.el("div", {class: "empty"}, "not text") : B.el("div", {class: "versions"}, side("Was", f.was), side("Now", f.now)));
      };
      build();
      det.addEventListener("toggle", build);
      return det;
    };
    return B.el("div", {class: "ch"},
      B.head([B.el("a", {href: "#/changes"}, "Changes")], c.why),
      B.el("div", {class: "stack ch-commit"},
        B.kv([
          ["By", actor(c.by)],
          ["Decided by", actor(c.decided_by)],
          ["When", B.whenEl(c.at)],
          ["Request", c.request ? B.el("a", {class: "mono", href: "#/r/" + c.request}, "#" + c.request) : null],
          ["Note", c.note],
        ]),
        c.more ? B.sect("More", B.el("pre", {}, c.more)) : "",
        c.does && c.does.length ? B.sect("Does", textRows(c.does)) : "",
        c.lets_more_by && c.lets_more_by.length ? B.sect("Lets more by", B.el("div", {class: "lets"}, textRows(c.lets_more_by))) : "",
        out.files.length ? B.sect(B.label("Files", B.el("span", {class: "count"}, String(out.files.length))),
          B.el("div", {class: "rows"}, paged(out.files, version, "change:" + c.id))) : ""));
  }, "changes", (what) => what === "office");
})();
