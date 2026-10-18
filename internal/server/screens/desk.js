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
    const names = (e.to || []).map(B.who);
    if (!names.length) return null;
    return [B.el("span", {class: "faint", "aria-hidden": "true"}, "→"), B.el("span", {class: "desk-vh"}, "to"), names];
  }

  // Waiting on you

  // waiting gives what is put to the one looking, by request: each thing
  // that waits on them a card.
  async function waiting() {
    if (!B.me.waiting) return null;
    try {
      const out = await B.api("/api/requests?waiting=me&limit=" + WAITING);
      const list = out.requests || [];
      const details = await Promise.all(list.map((r) => B.api("/api/requests/" + r.number + "?size=1").catch((err) => ({number: r.number, error: err}))));
      const reqs = list.map((r, i) => waitingOn(r, details[i]));
      const cards = reqs.reduce((n, r) => n + r.querySelectorAll(".ask-card").length, 0);
      return B.sect(B.label("Waiting on you", B.el("span", {class: "count"}, out.before ? B.me.waiting : cards)),
        reqs.length ? B.el("div", {class: "desk-waiting"}, reqs) : B.empty(),
        out.before ? more("#/requests?waiting=me") : null);
    } catch (err) { return failed("Waiting on you", err); }
  }

  // waitingOn gives one request's head, and what in it is put to the one
  // looking: its decisions, its questions, and the request itself when
  // they take it.
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
    if (sum.whole && onMe && (!cards.length || (team && team.takes === me))) cards.push(taken(v));
    return B.el("article", {class: "desk-req"}, head, cards);
  }

  function question(v, e) {
    return keep(B.el("div", {class: "ask-card"},
      B.el("div", {class: "line spread"}, B.label("Question"), B.whenEl(e.at)),
      B.el("div", {class: "line"}, B.who(e.by), to(e)),
      B.el("div", {class: "desk-said"}, e.text),
      typeof B.replyForm === "function" ? B.replyForm(v, e) : null), v.number + ":e" + e.id);
  }

  // taken is a request the one looking takes: what was said last in it,
  // and their answer.
  function taken(v) {
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
      B.el("div", {class: "line spread"}, B.label("Taker"), last ? B.whenEl(last.at) : null),
      last ? B.el("div", {class: "line"}, B.who(last.by), to(last)) : null,
      last && last.text ? B.el("div", {class: "desk-said"}, last.text) : null,
      form), v.number + ":t");
  }

  // Ask

  // ask gives the form that asks a team in the chat: picked from a list
  // when the office is small, typed when it is not; the one looking's own
  // teams first.
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
      go.disabled = false;
    }},
      B.el("div", {class: "desk-ask-team"}, B.field("Team", team), list),
      B.field("Request", text),
      B.el("div", {class: "desk-ask-foot"}, B.field("Files", files), go));
    return B.sect("Ask", keep(form, "ask", false));
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
          r.state ? B.status(r) : null,
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
          t && t.takes ? B.el("span", {class: "line desk-taker"}, B.el("span", {class: "small muted"}, "Taker"),
            B.person(t.takes) ? B.personEl(t.takes) : B.workerEl(t.takes, name, {bare: true})) : null,
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
      for (const w of (t && t.at_work) || []) rows.push(B.el("div", {class: "row"},
        B.el("span", {class: "grow"}, B.workerEl(w.worker, names[i])),
        reqLink(w.request),
        B.el("a", {href: "#/run/" + encodeURIComponent(w.run)}, "Watch")));
    });
    if (!rows.length) return null;
    return B.sect("Runs", B.el("div", {class: "rows"}, rows),
      all.length > TEAMS ? more("#/teams", all.length - TEAMS) : null);
  }

  function problems() {
    const all = B.me.problems || [];
    if (!all.length) return null;
    return B.sect("Problems", B.el("div", {class: "rows"}, all.slice(0, PROBLEMS).map((p) =>
      B.el("div", {class: "row"}, B.teamEl(p.team), B.el("span", {class: "grow"}, p.said)))),
      all.length > PROBLEMS ? more("#/me", all.length - PROBLEMS) : null);
  }

  function failing() {
    if (!B.me.failing || !B.me.failing.length) return null;
    return B.sect("Bridges failing", B.el("div", {class: "rows"}, B.me.failing.map((f) =>
      B.el("div", {class: "row"}, B.el("span", {class: "who"}, B.icon("bridge"), f.bridge),
        B.el("span", {class: "status failed"}, "failing"), B.el("span", {class: "mono faint"}, "×" + f.failures),
        B.el("span", {class: "grow mono small"}, f.error), B.whenEl(f.since)))));
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
