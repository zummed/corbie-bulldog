// Run: one run as it happens, each thing it does as a line, the runs of
// its colleagues indented beneath the calls that started them.
(function () {
  "use strict";

  const KINDS = ["words", "call", "result", "log", "refused", "failed", "emit", "ended"];
  const CLIP = 100;

  let current = null; // stops watching the run being watched

  // clip gives the first line of a text, cut short.
  function clip(text) {
    const line = String(text).split("\n")[0];
    return line.length > CLIP ? line.slice(0, CLIP) + "…" : line;
  }

  // parsed gives a text read as JSON, or undefined when it is not.
  function parsed(text) {
    try { return JSON.parse(text); } catch (err) { return undefined; }
  }

  // firstString gives the first string with something in it in what was
  // parsed.
  function firstString(v) {
    if (typeof v === "string") return v || undefined;
    if (v && typeof v === "object") {
      for (const x of Object.values(v)) {
        const s = firstString(x);
        if (s !== undefined) return s;
      }
    }
    return undefined;
  }

  // more gives a line with all of it behind a <details>, drawn only when
  // it is first opened.
  function more(line, all) {
    const pre = B.el("pre", {});
    return B.el("details", {ontoggle: () => { if (!pre.firstChild) pre.textContent = all(); }},
      B.el("summary", {}, line), pre);
  }

  // textEl gives a text as one line, with all of it behind a <details>
  // when there is more: JSON pretty, anything else as it is.
  function textEl(text) {
    if (!text) return "";
    const v = parsed(text);
    if (v && typeof v === "object") {
      const first = firstString(v);
      return more(clip(first === undefined ? JSON.stringify(v) : first), () => JSON.stringify(v, null, 2));
    }
    const plain = typeof v === "string" ? v : text;
    if (clip(plain) === plain) return plain;
    return more(clip(plain), () => plain);
  }

  // The jobs a held call asks authorisation for.
  function jobsOf(text) {
    const v = parsed(text);
    return v && Array.isArray(v.jobs) ? v.jobs : [];
  }

  // askEl gives a held call: the does line of each job it asks for.
  function askEl(text) {
    const jobs = jobsOf(text);
    return more(jobs.length ? jobs.map((j) => (j.asked && j.asked.does) || j.name).join(" · ") : clip(text),
      () => { const v = parsed(text); return v === undefined ? text : JSON.stringify(v, null, 2); });
  }

  // ending gives the one word a run's end is shown by, and its class.
  function ending(s) {
    if (!s.at && !s.machine) return ["gone", ""];
    if (s.text === "done") return ["done", ""];
    if (/^it was cancelled/.test(s.text || "")) return ["cancelled", ""];
    return ["failed", "failed"];
  }

  function refusal(s) {
    if (!s.refused && !s.layer && !s.fix) return null;
    return B.el("div", {class: "why"}, B.kv([["Refused", s.refused], ["Layer", s.layer], ["Fix", s.fix]]));
  }

  // view draws the page. The run is watched only once the page is the one
  // shown: a view may be drawn and thrown away when the office changes
  // while it is being fetched.
  function view(m) {
    const name = decodeURIComponent(m[1].split("?")[0]);

    const machineSlot = B.el("span", {class: "run-machine"});
    const statusEl = B.el("span", {class: "status working live"}, "working");
    const meta = B.el("div", {});
    const rows = B.el("div", {class: "events rows"});
    const root = B.el("div", {class: "run-view"},
      B.head([B.el("a", {href: "#/requests"}, "Requests")],
        B.el("span", {class: "run-title"}, "Run", machineSlot), statusEl),
      B.el("div", {class: "cols"},
        B.el("div", {class: "main"}, B.sect("Events", rows)),
        B.el("div", {class: "side"}, meta)));

    const drawMeta = (at) => meta.replaceChildren(B.kv([
      ["Run", B.el("span", {class: "mono small run-name"}, name)],
      ["Started", at ? B.whenEl(at) : null],
    ]));
    drawMeta(null);

    setTimeout(() => { if (root.isConnected) watch(name, root, rows, machineSlot, statusEl, drawMeta); }, 0);
    return root;
  }

  function watch(name, root, rows, machineSlot, statusEl, drawMeta) {
    if (current) current();

    const depth = {};     // run → how far below the run watched it is
    const below = {};     // tool called → the depth its run will have, when it is a colleague
    const machines = {};  // machines seen
    const called = {};    // tool → the cell of its last call, made a worker if its machine is seen
    const held = {};      // run → the authorisation it waits on, while it waits
    depth[name] = 0;
    let first = null;
    let started = false;
    let words = null;     // the words being joined: {run, text}

    // A run's id stands in for its machine when Corbie could not say it.
    const machineOf = (s) => s.machine && s.machine !== s.run ? s.machine : "";
    const machineEl = (s) => machineOf(s) ? B.workerEl(s.machine) : "";

    const setStatus = (word, cls) => {
      statusEl.className = "status" + (cls ? " " + cls : "");
      statusEl.textContent = word;
    };

    const depthOf = (s) => {
      if (!s.run || s.run === name) return 0;
      if (depth[s.run] === undefined) depth[s.run] = below[s.machine] !== undefined ? below[s.machine] : 1;
      return depth[s.run];
    };

    const seeMachine = (machine) => {
      if (!machine || machines[machine]) return;
      machines[machine] = true;
      if (called[machine]) called[machine].replaceChildren(B.workerEl(machine));
      delete called[machine];
    };

    const nameCell = (s) => {
      const cell = B.el("span", {class: "n"});
      if (!s.name) return cell;
      if (s.kind === "call" && machines[s.name]) { cell.append(B.workerEl(s.name)); return cell; }
      cell.append(s.name);
      if (s.kind === "call") called[s.name] = cell;
      return cell;
    };

    // settle makes a held call's authorisation plain once it is answered
    // or its run ends.
    const settle = (run) => {
      if (held[run]) held[run].replaceWith(held[run].textContent);
      delete held[run];
    };

    // The newest is kept in sight while the reader is at it, scrolled to
    // once a frame.
    let stick = true;
    let frame = 0;
    const onScroll = () => { stick = innerHeight + scrollY >= document.documentElement.scrollHeight - 80; };
    const keep = () => {
      if (frame || !stick) return;
      frame = requestAnimationFrame(() => {
        frame = 0;
        if (stick && root.isConnected) scrollTo(0, document.documentElement.scrollHeight);
      });
    };

    const source = new EventSource("/api/runs/" + encodeURIComponent(name) + "/watch");
    const stop = () => {
      source.close();
      removeEventListener("scroll", onScroll);
      if (frame) cancelAnimationFrame(frame);
      frame = 0;
    };
    current = stop;

    const seen = (e) => {
      const s = JSON.parse(e.data);

      // The model's words come as they are made, and are shown together.
      if (s.kind === "words" && words && words.run === s.run) {
        words.text.appendData(s.text || "");
        keep();
        return;
      }
      words = null;

      if (!first && machineOf(s) && (!s.run || s.run === name)) {
        first = s.machine;
        machineSlot.replaceChildren(" · ", B.workerEl(first));
      }
      if (s.at && !started) { started = true; drawMeta(s.at); }
      seeMachine(machineOf(s));

      const d = depthOf(s);
      if (s.kind === "call" && s.name && s.name !== "ask") below[s.name] = d + 1;

      const ask = s.kind === "call" && s.name === "ask";
      let n, x;
      if (ask) {
        const jobs = jobsOf(s.text).map((j) => j.name).filter(Boolean).join(", ");
        const word = B.el("span", {class: "status waiting"}, "authorisation" + (jobs ? " · " + jobs : ""));
        settle(s.run);
        held[s.run] = word;
        n = B.el("span", {class: "n"}, word);
        x = askEl(s.text);
      } else if (s.kind === "result" && s.name === "ask") {
        settle(s.run);
        n = B.el("span", {class: "n"}, "authorisation");
        x = textEl(s.text);
      } else {
        n = nameCell(s);
        x = s.kind === "ended" || s.kind === "failed" || s.kind === "refused" ? (s.text || "") : s.kind === "words" ? null : textEl(s.text);
      }
      const to = s.kind === "call" ? "→" : s.kind === "result" ? "←" : "";

      const xEl = B.el("span", {class: "x"}, x, s.kind === "refused" ? refusal(s) : null);
      if (s.kind === "words") {
        words = {run: s.run, text: document.createTextNode(s.text || "")};
        xEl.append(words.text);
      }

      const end = s.kind === "ended" ? ending(s) : null;
      rows.append(B.el("div", {class: "row " + s.kind + (end && end[1] ? " " + end[1] : "") + (ask ? " ask" : "")},
        B.el("span", {class: "t"}, B.whenEl(s.at)),
        B.el("span", {class: "ev" + (d ? " nest d" + Math.min(d, 4) : "")},
          B.el("span", {class: "k"}, s.kind),
          B.el("span", {class: "m"}, machineEl(s)),
          B.el("span", {class: "to", "aria-hidden": "true"}, to),
          n, xEl)));
      keep();

      if (end) {
        stop();
        for (const run of Object.keys(held)) settle(run);
        setStatus(end[0], end[1]);
      } else if (s.kind === "failed" && !s.machine) {
        // Bulldog itself failed in following the run.
        stop();
        setStatus("failed", "failed");
      }
    };

    for (const kind of KINDS) source.addEventListener(kind, seen);
    source.onerror = () => {
      if (source.readyState !== EventSource.CLOSED) return;
      stop();
      setStatus("not there", "");
      if (!rows.firstChild) rows.append(B.empty());
    };
    addEventListener("scroll", onScroll, {passive: true});
    addEventListener("hashchange", stop, {once: true});
    // B.streaming stays set after the run ends, until the address changes:
    // drawing the view again would replay the whole run.
    B.streaming = source;
  }

  B.route(/^#\/run\/(.+)$/, view, "requests");
})();
