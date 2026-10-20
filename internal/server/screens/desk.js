// The desk: what waits on the one looking, asking a team, what they asked,
// their teams, and what is at work in them.
(function () {
  "use strict";

  const WAITING = 10;   // requests drawn whole among those waiting on the one looking
  const ASKED = 10;     // requests they asked, newest first
  const TEAMS = 8;      // their teams drawn, and looked in for runs
  const PICK = 20;      // more teams than this are typed, not picked
  const PROBLEMS = 8;   // problems drawn before More

  // drafts keeps what is typed in the desk's forms by what each answers,
  // so that sending one, which draws the desk again with every field
  // emptied, loses none of the others.
  const drafts = new Map();

  // keep fills the fields within el from what was typed in them before,
  // and keeps what is typed in them from now. Sending forgets it, unless
  // forget is false, when whoever sends forgets it once it is sent.
  function keep(el, key, forget) {
    const fields = [...el.querySelectorAll("textarea, select, input:not([type=file])")];
    const files = el.querySelector("input[type=file]");
    const was = drafts.get(key);
    if (was) {
      fields.forEach((f, i) => { if (was.values[i] !== undefined) f.value = was.values[i]; });
      if (files && was.files) try { files.files = was.files; } catch (err) { /* a browser that will not set files leaves them empty */ }
    }
    const save = () => drafts.set(key, {values: fields.map((f) => f.value), files: files ? files.files : null});
    el.addEventListener("input", save);
    el.addEventListener("change", save);
    if (forget !== false) {
      el.addEventListener("submit", () => drafts.delete(key));
      el.addEventListener("click", (ev) => { if (ev.target.closest("button[type=button]")) drafts.delete(key); });
    }
    return el;
  }

  // path gives a team by the teams it is within: The house › The garden.
  function path(name, byName) {
    const out = [];
    for (let t = byName.get(name), n = 0; t && n < 32; t = t.within ? byName.get(t.within) : null, n++) out.unshift(t.called);
    return out.length ? out.join(" › ") : name;
  }

  // reqLink gives a request's number, linking to it.
  function reqLink(n) { return B.el("a", {class: "mono", href: "#/r/" + n}, "#" + n); }

  function more(href, n) {
    return B.el("div", {class: "desk-more"}, B.el("a", {href}, "More"), n ? B.el("span", {class: "mono faint"}, n) : null);
  }

  function failed(label, err) {
    return B.sect(label, B.el("div", {class: "desk-error"}, err.message));
  }

  // to gives whom an entry is to, after an arrow.
  function to(e) {
    const names = (e.to || []).map(B.party);
    if (!names.length) return null;
    return [B.el("span", {class: "faint", "aria-hidden": "true"}, "→"), B.el("span", {class: "desk-vh"}, "to"), names];
  }

  // Waiting on you

  // waiting gives what is put to the one looking, by request: each thing
  // that waits on them a card.
  async function waiting() {
    if (!B.me.waiting) return null;
    try {
      const own = (B.me.changes || []).filter((d) => d.mine);
      const out = B.me.waiting > own.length ? await B.api("/api/requests?waiting=me&limit=" + WAITING) : {requests: []};
      // A request inside another that waits on them too is folded inside
      // it, and is a card of its own all the same.
      const all = (rs) => rs.flatMap((r) => [r, ...all(r.subs || [])]);
      const list = all(out.requests || []);
      const details = await Promise.all(list.map((r) => B.api("/api/requests/" + r.number + "?size=1").catch((err) => ({number: r.number, error: err}))));
      const reqs = [...own.map(ownChange), ...list.map((r, i) => waitingOn(r, details[i]))];
      const cards = reqs.reduce((n, r) => n + r.querySelectorAll(".ask-card").length, 0);
      return B.sect(B.label("Waiting on you", B.el("span", {class: "count"}, out.before ? B.me.waiting : cards)),
        reqs.length ? B.el("div", {class: "desk-waiting"}, reqs) : B.empty(),
        out.before ? more("#/requests?waiting=me") : null);
    } catch (err) { return failed("Waiting on you", err); }
  }

  // waitingOn gives one request's head, and what in it is put to the one
  // looking: its decisions, its questions, and the request itself when
  // they are its lead.
  function waitingOn(sum, v) {
    const head = B.el("div", {class: "desk-req-head"}, reqLink(sum.number),
      sum.whole ? [B.teamEl(sum.team), B.el("span", {class: "desk-first"}, sum.first)] : null);
    if (v.error) return B.el("article", {class: "desk-req"}, head, B.el("div", {class: "desk-error"}, v.error.message));
    const cards = [];
    if (typeof B.decisionCard === "function") {
      for (const d of v.decisions || []) if (d.mine) cards.push(keep(B.decisionCard(v, d), v.number + ":d" + d.id));
    }
    const byId = new Map((v.entries || []).map((e) => [e.id, e]));
    for (const id of v.waits_on_me || []) {
      const e = byId.get(id);
      if (e && e.kind === "question") cards.push(question(v, e));
    }
    const me = B.me.me.name;
    const team = B.team(sum.team);
    const onMe = (sum.on || []).some((p) => p.kind === "person" && p.name === me);
    if (sum.whole && onMe && (!cards.length || (team && team.lead === me))) cards.push(leadCard(v));
    return B.el("article", {class: "desk-req"}, head, cards);
  }

  // ownChange gives a person's own change from the screens put to the one
  // looking, which serves no request, as its card.
  function ownChange(d) {
    return B.el("article", {class: "desk-req"},
      B.el("div", {class: "desk-req-head"}, B.me.me.decides_changes ? B.el("a", {href: "#/office/changes"}, "Change") : B.el("span", {}, "Change"),
        B.el("span", {class: "desk-first"}, "from the screens")),
      typeof B.decisionCard === "function" ? keep(B.decisionCard({number: 0}, d), "own:d" + d.id) : null);
  }

  function question(v, e) {
    return keep(B.el("div", {class: "ask-card"},
      B.el("div", {class: "line spread"}, B.label("Question"), B.whenEl(e.at)),
      B.el("div", {class: "line"}, B.party(e.by), to(e)),
      B.el("div", {class: "desk-said"}, e.text),
      typeof B.replyForm === "function" ? B.replyForm(v, e) : null), v.number + ":e" + e.id);
  }

  // leadCard is a request the one looking is the lead of: what was said
  // last in it, and their answer.
  function leadCard(v) {
    const entries = v.entries || [];
    const last = entries[entries.length - 1];
    const text = B.el("textarea", {class: "in", rows: 2});
    const files = B.el("input", {class: "in", type: "file", multiple: true});
    const form = B.el("form", {class: "desk-answer", onsubmit: (ev) => {
      ev.preventDefault();
      B.act(B.send("/api/requests/" + v.number + "/entries", {text: text.value}, files), form);
    }},
      B.field("Answer", text),
      B.el("div", {class: "desk-ask-foot"}, B.field("Files", files), B.el("button", {class: "btn go", type: "submit"}, "Answer")));
    return keep(B.el("div", {class: "ask-card"},
      B.el("div", {class: "line spread"}, B.label("Lead of " + B.teamCalled(v.team)), last ? B.whenEl(last.at) : null),
      last ? B.el("div", {class: "line"}, B.party(last.by), to(last)) : null,
      last && last.text ? B.el("div", {class: "desk-said"}, last.text) : null,
      form), v.number + ":t");
  }

  // Ask

  // strip gives the way a request to a team goes: its lead, and whether it
  // can be started; and, to those who see it, that the team sets a
  // disclosure, when it does.
  function strip(t) {
    if (!t) return [];
    const lead = B.person(t.lead) ? B.personEl(t.lead) : B.workerEl(t.lead, t.name, {bare: true});
    return [
      ["Lead", B.el("span", {class: "line"}, lead, t.lead_runs ? null : B.el("span", {class: "status failed"}, "cannot run"))],
      t.discloses ? [B.disclosureLabel(t), "Set"] : null,
    ];
  }

  // ask gives the form that asks a team in the chat: picked from a list
  // when the office is small, typed when it is not; the one looking's own
  // teams first; beside it the way a request to the team picked goes, and
  // Ask only while its lead can be started.
  function ask() {
    const mine = new Set(B.me.me.teams || []);
    const all = B.me.teams || [];
    const byName = new Map(all.map((t) => [t.name, t]));
    const teams = all.map((t) => ({name: t.name, path: path(t.name, byName)}))
      .sort((a, b) => (mine.has(b.name) ? 1 : 0) - (mine.has(a.name) ? 1 : 0));
    let team, pick, list = null;
    if (teams.length <= PICK) {
      team = B.el("select", {class: "in"}, teams.map((t) => B.el("option", {value: t.name}, t.path)));
      pick = () => team.value;
    } else {
      list = B.el("datalist", {id: "desk-teams"}, teams.map((t) => B.el("option", {value: t.path})));
      team = B.el("input", {class: "in", type: "text", list: "desk-teams", autocomplete: "off"});
      pick = () => {
        const typed = team.value.trim().toLowerCase();
        const t = teams.find((t) => t.path.toLowerCase() === typed) || teams.find((t) => t.name.toLowerCase() === typed);
        if (!t) throw new Error("No team is " + team.value.trim() + ".");
        return t.name;
      };
    }
    const text = B.el("textarea", {class: "in"});
    const files = B.el("input", {class: "in", type: "file", multiple: true});
    const go = B.el("button", {class: "btn go", type: "submit"}, "Ask");
    const way = B.el("div", {class: "desk-strip"});
    const picked = () => { try { return B.team(pick()); } catch (err) { return null; } };
    const showWay = () => {
      const t = picked();
      way.hidden = !t;
      way.replaceChildren(B.kv(strip(t)));
      go.disabled = Boolean(t && !t.lead_runs);
    };
    team.addEventListener("change", showWay);
    team.addEventListener("input", showWay);
    const form = B.el("form", {class: "desk-ask", onsubmit: async (ev) => {
      ev.preventDefault();
      B.trouble(null);
      go.disabled = true;
      try {
        const out = await B.send("/api/requests", {team: pick(), text: text.value}, files);
        text.value = "";
        files.value = "";
        drafts.delete("ask");
        location.hash = "#/r/" + out.number;
      } catch (err) { B.trouble(err); }
      showWay();
    }},
      B.el("div", {class: "desk-ask-head"}, B.el("div", {class: "desk-ask-team"}, B.field("Team", team), list), way),
      B.field("Request", text),
      B.el("div", {class: "desk-ask-foot"}, B.field("Files", files), go));
    keep(form, "ask", false);
    showWay();
    return B.sect("Ask", form);
  }

  // Asked by you

  async function asked() {
    try {
      const out = await B.api("/api/requests?asked=me&limit=" + ASKED);
      const list = out.requests || [];
      return B.sect("Asked by you",
        list.length ? B.el("div", {class: "rows"}, list.map((r) => B.el("div", {class: "row"},
          reqLink(r.number),
          B.el("span", {class: "grow clip mono"}, r.first || ""),
          r.team ? B.teamEl(r.team) : null,
          r.state ? B.status(B.standing(r)) : null,
          B.whenEl(r.made)))) : B.empty(),
        out.before ? more("#/requests?asked=me") : null);
    } catch (err) { return failed("Asked by you", err); }
  }

  // Your teams

  function yours() {
    const names = B.me.me.teams || [];
    if (!names.length) return null;
    return B.sect("Your teams",
      B.el("div", {class: "rows"}, names.slice(0, TEAMS).map((name) => {
        const t = B.team(name);
        return B.el("div", {class: "row"},
          B.el("span", {class: "grow"}, B.teamEl(name)),
          t && t.lead ? B.el("span", {class: "line desk-lead"}, B.el("span", {class: "small muted"}, "Lead"),
            B.person(t.lead) ? B.personEl(t.lead) : B.workerEl(t.lead, name, {bare: true})) : null,
          B.el("a", {href: "#/requests?team=" + encodeURIComponent(name)}, "Requests"));
      })),
      names.length > TEAMS ? more("#/teams", names.length - TEAMS) : null);
  }

  // Side: runs, problems, failing bridges

  async function runs() {
    const all = B.me.me.teams || [];
    const names = all.slice(0, TEAMS);
    const teams = await Promise.all(names.map((name) => B.api("/api/teams/" + encodeURIComponent(name)).catch(() => null)));
    const rows = [];
    teams.forEach((t, i) => {
      for (const w of t ? B.atWork(t) : []) rows.push(B.el("div", {class: "row"},
        B.el("span", {class: "grow"}, B.workerEl(w.worker, names[i])),
        reqLink(w.request),
        B.el("a", {href: "#/run/" + encodeURIComponent(w.run)}, "Watch")));
    });
    if (!rows.length) return null;
    return B.sect("Runs", B.el("div", {class: "rows"}, rows),
      all.length > TEAMS ? more("#/teams", all.length - TEAMS) : null);
  }

  // stops gives what a problem stops, its worker, its rule or its bridge,
  // or else the team it is of; the key it is grouped by; and whether it is
  // stopped.
  function stops(p) {
    if (p.worker) return ["w:" + p.team + "/" + p.worker, B.workerEl(p.worker, p.team), true];
    if (p.rule) return ["r:" + p.rule, B.el("span", {class: "line"}, B.ruleEl(p.rule), B.teamEl(p.team)), true];
    if (p.bridge) return ["b:" + p.bridge, B.el("a", {class: "who", href: "#/office/bridges"}, B.icon("bridge"), p.bridge), true];
    return ["t:" + p.team, B.teamEl(p.team), false];
  }

  // problems gives the problems the one looking sees, each beneath what it
  // stops.
  function problems() {
    const all = B.me.problems || [];
    if (!all.length) return null;
    const groups = new Map();
    for (const p of all.slice(0, PROBLEMS)) {
      const [key, head, stopped] = stops(p);
      if (!groups.has(key)) groups.set(key, {head, stopped, said: []});
      groups.get(key).said.push(p);
    }
    return B.sect(B.label("Problems", B.el("span", {class: "count"}, String(all.length))),
      B.el("div", {class: "desk-problems"}, [...groups.values()].map((g) => B.el("div", {class: "desk-problem"},
        B.el("div", {class: "desk-problem-head"}, g.head, g.stopped ? B.el("span", {class: "status stopped"}, "stopped") : null),
        B.el("ul", {class: "desk-problem-list"}, g.said.map((p) => B.el("li", {},
          p.file ? B.el("span", {class: "mono small faint"}, p.file + (p.line ? ":" + p.line : "")) : null,
          B.el("span", {}, p.text || p.said))))))),
      all.length > PROBLEMS ? more("#/me", all.length - PROBLEMS) : null);
  }

  function failing() {
    const all = (B.me.office.bridges || []).filter((b) => b.failing);
    if (!all.length) return null;
    return B.sect("Bridges failing", B.el("div", {class: "rows"}, all.map((b) =>
      B.el("div", {class: "row"}, B.el("a", {class: "who", href: "#/office/bridges"}, B.icon("bridge"), b.name),
        B.bridgeStatus(b), B.el("span", {class: "mono faint"}, "×" + b.failing.failures),
        B.el("span", {class: "grow mono small"}, b.failing.error), B.whenEl(b.failing.since)))));
  }

  B.desk = async function () {
    const [w, a, r] = await Promise.all([waiting(), asked(), runs()]);
    return B.el("div", {class: "desk"},
      B.head(null, "Desk"),
      B.el("div", {class: "cols"},
        B.el("div", {class: "main"}, w, ask(), a, yours()),
        B.el("div", {class: "side"}, r, problems(), failing())));
  };
})();
