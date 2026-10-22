// Requests, the home: a box to ask a team, the questions and decisions
// waiting on the one looking, and the requests they may see.
"use strict";

// notes gives what each decider said before, in order.
B.notes = function (notes) {
  return el("ol", {class: "notes", "aria-label": "Notes"}, notes.map((n) => el("li", {},
    B.party(n.by, false),
    el("span", {class: "says " + (n.says || "")}, n.says || "…"),
    n.note ? el("span", {class: "note"}, n.note) : null)));
};

// decided gives what each decider of a heed, a check or a decision said.
B.decided = function (deciders) {
  return B.notes(deciders.map((d) => ({by: {kind: d.kind, id: d.name, name: d.name}, says: d.says, note: d.note})));
};

// card gives a question or a decision put to the one looking, with its
// choices as buttons; a question may also be answered in words. A
// decision's words hold its held jobs and the notes of the deciders
// before. Under a fork's row it leaves out its head, which the row says.
B.card = function (c, within) {
  const answer = (value, form) => B.act(B.send("POST", "/api/waits/" + enc(c.ref) + "/answer", {answer: value}), form)
    .then((out) => { if (out) B.show(true); });
  const decision = c.kind === "decision";
  const choices = (c.choices || []).map((ch) => el("button", {
    class: "btn " + (decision ? (ch === "yes" ? "go" : ch === "no" ? "no" : "ghost") : "ghost"),
    type: "button", "data-key": "choice-" + c.ref + "-" + ch,
    onclick: (e) => answer(ch, e.target.closest("form"))}, decision ? (ch === "yes" ? "Yes" : ch === "no" ? "No" : ch) : ch));
  const words = decision ? null : el("textarea", {class: "in", name: "answer", rows: 1, "data-key": "words-" + c.ref, required: true});
  const form = el("form", {class: "card-act", onsubmit: (e) => { e.preventDefault(); answer(words.value.trim(), form); }},
    choices,
    words ? [B.field(choices.length ? "Or answer in words" : "Answer", words), el("button", {class: "btn", type: "submit"}, "Answer")] : null);
  return el("article", {class: "card", "aria-label": (decision ? "Decision" : "Question") + " for you"},
    within ? null : el("div", {class: "card-head"},
      el("span", {class: "chip wait"}, decision ? "Decision" : "Question"),
      el("a", {href: "#/requests/" + enc(c.request)}, c.teamName),
      B.time(c.at)),
    el("p", {class: "card-said"}, c.said),
    form);
};

// row gives one request on a line: its flow, its team, who asked, the
// first line of what was asked, and when. Its words open it.
B.row = function (r) {
  return el("li", {class: "rq-row " + r.status},
    B.mini(r.nodes, r.status),
    el("span", {class: "rq-team"}, r.teamName),
    el("span", {class: "rq-by"}, B.party(r.by, false)),
    el("a", {class: "rq-said", href: "#/requests/" + enc(r.id)}, r.said || "(nothing said)"),
    B.time(r.at));
};

// teamsOf gives every team of the office, the office first, each after
// the team it is in.
function teamsOf(room, depth) {
  return [{id: room.id, name: room.name, depth}, ...room.teams.flatMap((t) => teamsOf(t, depth + 1))];
}

// readFiles gives the files chosen, each its name and its bytes in base64.
function readFiles(input) {
  return Promise.all([...input.files].map((f) => new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve({name: f.name, data: String(r.result).replace(/^data:[^,]*,/, "")});
    r.onerror = () => reject(new Error("Could not read " + f.name + "."));
    r.readAsDataURL(f);
  })));
}

function askBox(office) {
  const teams = teamsOf(office.team, 0);
  const mine = B.me.person.teams || [];
  const chosen = (teams.find((t) => mine.includes(t.id)) || teams[0]).id;
  const team = el("select", {class: "in", name: "team", "data-key": "ask-team"},
    teams.map((t) => el("option", {value: t.id, selected: t.id === chosen}, " ".repeat(t.depth) + t.name)));
  const said = el("textarea", {class: "in", name: "said", rows: 2, required: true, "data-key": "ask-said"});
  const files = el("input", {class: "in", type: "file", name: "files", multiple: true, "data-key": "ask-files"});
  const form = el("form", {class: "ask", "aria-label": "Ask a team", onsubmit: async (e) => {
    e.preventDefault();
    const out = await B.act(readFiles(files).then((fs) =>
      B.send("POST", "/api/requests", {team: team.value, said: said.value.trim(), files: fs})), form);
    if (out) { B.starting.add(out.id); location.hash = "#/requests/" + enc(out.id); }
  }},
  B.field("Ask", team, "team"),
  B.field("Say", said, "say"),
  B.field("Files", files, "files"),
  el("button", {class: "btn go", type: "submit"}, "Ask"));
  return form;
}

B.home = async function (args, listed) {
  const [list, office] = await Promise.all([listed, B.api("/api/office")]);
  document.title = "Requests · " + B.me.office.name;
  return el("div", {class: "page home"},
    el("h1", {class: "vh"}, "Requests"),
    askBox(office),
    list.waiting.length ? B.sect("Waiting on you", list.waiting.length, el("div", {class: "cards"}, list.waiting.map((c) => B.card(c)))) : null,
    B.sect("Requests", list.requests.length,
      list.requests.length ? el("ol", {class: "rq-list"}, list.requests.map(B.row)) : B.empty("No requests yet.")));
};
