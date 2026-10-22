// The office and its teams: the office's room with its teams inside as
// rooms; a team's room with no run, each node showing what is written for
// it; and a team's doc.
"use strict";

const docHref = (team, name) => "#/teams/" + enc(team) + "/docs/" + enc(name);
const docPath = (team, name) => "/api/teams/" + enc(team) + "/docs/" + enc(name);

// sentence gives the first sentence of a text.
const sentence = (text) => text.split(/(?<=\.) /)[0];

// usesOf gives what a worker uses: its colleagues, each with what it uses
// in turn, and its tools, each with the first sentence of what it does.
function usesOf(colleagues, tools) {
  return [
    ...colleagues.map((c) => el("li", {}, B.party({kind: "worker", name: c.name}), el("span", {class: "does"}, "colleague"),
      c.colleagues.length || c.tools.length ? el("ul", {class: "calls"}, usesOf(c.colleagues, c.tools)) : null)),
    ...tools.map((x) => el("li", {}, el("span", {class: "tool"}, x.name),
      x.does ? el("span", {class: "does", title: x.does}, sentence(x.does)) : null,
      x.held ? el("span", {class: "tag held"}, "held for approval") : null))];
}

// roomOf gives a team as a room: its name on the wall, what it looks after,
// its lead and people, its problems, and the teams inside it.
function roomOf(t, names, top) {
  const person = (id) => B.party({kind: "person", id, name: names[id] || id});
  const lead = t.lead.kind === "person" ? person(t.lead.name) : B.party({kind: "worker", name: t.lead.name});
  return el("section", {class: "room " + (top ? "office-room" : "team-room"), "aria-label": t.name},
    el(top ? "h2" : "h3", {class: "room-name"}, el("a", {href: "#/teams/" + enc(t.id)}, t.name)),
    el("div", {class: "room-body"},
      t.about && !top ? el("p", {class: "about"}, t.about) : null,
      el("div", {class: "line"}, el("span", {class: "k"}, "Lead"), lead),
      el("div", {class: "line"}, el("span", {class: "k"}, "People"),
        t.people.length ? el("span", {class: "faces"}, t.people.map(person)) : el("span", {class: "faint small"}, "none; the office's people see its requests")),
      t.problems ? el("a", {class: "status failed", href: "#/teams/" + enc(t.id)}, t.problems === 1 ? "1 problem" : t.problems + " problems") : null),
    t.teams.length ? el("div", {class: "rooms"}, t.teams.map((x) => roomOf(x, names, false))) : null);
}

function teamNames(room, out) {
  out[room.id] = room.name;
  for (const t of room.teams) teamNames(t, out);
  return out;
}

function problemsOf(problems) {
  return problems.length
    ? el("ul", {class: "rows"}, problems.map((p) => el("li", {class: "problem"}, p.file + (p.line ? ":" + p.line : "") + "  " + p.text)))
    : B.empty("None.");
}

B.officeView = async function () {
  const o = await B.api("/api/office");
  const names = Object.fromEntries(o.people.map((p) => [p.id, p.name]));
  const teams = teamNames(o.team, {});
  document.title = "Office · " + o.name;
  const teamLink = (id) => el("a", {href: "#/teams/" + enc(id)}, teams[id] || id);
  return el("div", {class: "page"},
    el("div", {class: "head"},
      el("h1", {}, o.name),
      o.about ? el("p", {class: "about"}, o.about) : null,
      el("p", {class: "about faint"}, "Times are in " + o.zone + ".")),
    el("div", {class: "drawing"}, roomOf(o.team, names, true)),
    el("div", {class: "cols"},
      el("div", {class: "main"},
        B.sect("Bridges", o.bridges.length, o.bridges.length ? el("ul", {class: "rows"}, o.bridges.map((b) => el("li", {class: "bridge"},
          el("div", {class: "line"}, el("strong", {}, b.name), B.status(b.in.status), el("span", {class: "mono faint"}, b.from)),
          b.in.error ? el("p", {class: "error"}, b.in.error) : null,
          el("ul", {class: "bridge-places", "aria-label": "Places"}, b.places.map((p) => el("li", {},
            p.place, " → ", teamLink(p.team), p.guests ? [" ", el("span", {class: "tag"}, "guests")] : null)))))) : B.empty("No bridges."))),
      el("div", {class: "side"},
        B.sect("People", o.people.length, el("ul", {class: "rows"}, o.people.map((p) => el("li", {class: "person"},
          B.party({kind: "person", id: p.id, name: p.name}),
          el("span", {class: "line small"}, p.teams.map(teamLink), p.bridges.map((b) => el("span", {class: "tag"}, b))))))),
        B.sect("Problems", o.problems.length || undefined, problemsOf(o.problems)))));
};

// The team's room with no run: at SOURCE the places that ask it as doors,
// then the screens and other teams, which ask every team; the heed's
// deciders, the lead and what it uses, the check's deciders, and decide
// as a fork from WORK.
function teamDrawing(t) {
  const f = t.flow;
  const names = Object.fromEntries([B.me.person, ...t.people].map((p) => [p.id, p.name]));
  const doors = el("ul", {class: "doors", "aria-label": "Places that ask " + t.name},
    f.source.map((p) => el("li", {class: "door"},
      B.door(), el("span", {}, p.bridge + " › " + p.place), p.guests ? el("span", {class: "tag"}, "guests") : null)),
    el("li", {class: "door faint"}, "the screens and other teams"));
  const lead = f.work.lead.kind === "person"
    ? B.party({kind: "person", id: f.work.lead.name, name: names[f.work.lead.name] || f.work.lead.name})
    : B.party({kind: "worker", name: f.work.lead.name});
  const uses = usesOf(f.work.colleagues, f.work.tools);
  const decide = f.decide.length ? [el("li", {class: "fork"},
    el("div", {class: "fork-head"}, el("span", {class: "chip"}, "decision"), B.chain(f.decide, names)),
    el("p", {class: "muted small"}, "Says yes or no to each job Corbie holds for approval."))] : [];
  return B.drawing({
    label: t.name,
    name: t.name,
    nodes: [
      {stage: "source", state: "passed", what: doors},
      f.heed.length ? {stage: "heed", state: "passed", what: B.chain(f.heed, names)} : null,
      {stage: "work", state: "passed", what: lead},
      f.check.length ? {stage: "check", state: "passed", what: B.chain(f.check, names)} : null,
      {stage: "reply", state: "passed", what: el("span", {class: "faint"}, "back where it was asked")},
    ].filter(Boolean),
    work: uses.length ? el("div", {}, el("div", {class: "k"}, "Uses"), el("ul", {class: "calls"}, uses)) : null,
    forks: decide,
  });
}

B.teamView = async function ([id]) {
  const t = await B.api("/api/teams/" + enc(id));
  document.title = t.name + " · " + B.me.office.name;
  return el("div", {class: "page"},
    el("div", {class: "head"},
      B.crumbs(el("a", {href: "#/office"}, "Office"), t.name),
      el("h1", {}, t.name),
      t.about ? el("p", {class: "about"}, t.about) : null),
    teamDrawing(t),
    el("div", {class: "cols"},
      el("div", {class: "main"},
        B.sect("Workers", t.workers.length, t.workers.length ? el("div", {class: "defs"}, t.workers.map((w) =>
          el("details", {"data-key": "worker-" + w.name},
            el("summary", {}, B.party({kind: "worker", name: w.name}), el("span", {class: "tag"}, w.type)),
            el("pre", {}, w.definition)))) : B.empty("No workers.")),
        B.sect("Docs", t.docs ? t.docs.length : undefined, t.docs === null
          ? el("div", {class: "sealed sealed-note"}, el("span", {}, "Seen only by the team's people."))
          : t.docs.length ? el("ul", {class: "rows"}, t.docs.map((d) => el("li", {}, el("a", {href: docHref(t.id, d)}, d)))) : B.empty("No docs yet.")),
        el("section", {class: "sect machine"},
          el("details", {"data-key": "machine"},
            el("summary", {}, el("h2", {class: "label"}, "The request machine")),
            el("p", {class: "muted small"}, "As it is sent to Corbie: one run of it for each request."),
            el("pre", {}, t.machine)))),
      el("div", {class: "side"},
        B.sect("People", t.people.length, t.people.length
          ? el("ul", {class: "rows"}, t.people.map((p) => el("li", {}, B.party({kind: "person", id: p.id, name: p.name}))))
          : B.empty("None; the office's people see its requests.")),
        B.sect("Routines", t.routines.length, t.routines.length ? el("ul", {class: "rows"}, t.routines.map((r) => el("li", {class: "routine"},
          el("span", {class: "who"}, B.icon("clock"), el("span", {class: "mono"}, r.at)),
          r.next ? el("span", {class: "small faint"}, "next ", B.time(r.next)) : null,
          el("span", {class: "say"}, r.say)))) : B.empty("None.")),
        B.sect("Problems", t.problems.length || undefined, problemsOf(t.problems)))));
};

B.docView = async function ([team, name]) {
  const [t, d] = await Promise.all([B.api("/api/teams/" + enc(team)), B.api(docPath(team, name))]);
  document.title = d.name + " · " + t.name;
  const text = el("textarea", {class: "in code", name: "text", "data-was": d.text, "data-key": "doc-text"}, d.text);
  const form = el("form", {onsubmit: async (e) => {
    e.preventDefault();
    const out = await B.act(B.send("PUT", docPath(team, name), {text: text.value}), form);
    if (out) { text.dataset.was = text.value; B.show(true); }
  }}, B.field("Text", text), el("button", {class: "btn go", type: "submit"}, "Save"));
  return el("div", {class: "page"},
    el("div", {class: "head"},
      B.crumbs(el("a", {href: "#/office"}, "Office"), el("a", {href: "#/teams/" + enc(t.id)}, t.name), "docs"),
      el("h1", {}, d.name)),
    d.text ? el("div", {class: "doc-text"}, d.text) : B.empty("This doc is empty."),
    el("details", {class: "doc-edit", "data-key": "doc-edit"}, el("summary", {}, "Edit"), form));
};
