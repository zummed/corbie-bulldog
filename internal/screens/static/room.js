// A team drawn as a room: one line from SOURCE on the left wall through
// HEED, WORK and CHECK to REPLY on the right wall, and the forks that leave
// from WORK through a gap in the bottom wall. Under 640px the line runs
// downward, SOURCE on the top wall and REPLY on the bottom wall, and the
// forks are set in under WORK. The same nodes, drawn small, are a row's
// flow in the list of requests.
"use strict";

const REACHED = new Set(["passed", "working", "waiting", "no", "failed"]);
const STAGE = {source: "Source", heed: "Heed", work: "Work", check: "Check", reply: "Reply"};
const STATE = {"waiting-to-start": "not reached", passed: "passed", working: "working", waiting: "waiting", no: "said no", failed: "failed"};

// segs gives the line from each node to the next: plan when the next was
// reached, line when not, flow ahead of a node at work, bad (dashed) after
// a check's no or a failure, and none after a heed's no, where the line
// ends.
B.segs = function (nodes) {
  let ended = false;
  return nodes.slice(1).map((next, i) => {
    const n = nodes[i];
    if (ended) return "none";
    if (n.stage === "heed" && n.state === "no") { ended = true; return "none"; }
    if (n.state === "failed" || (n.stage === "check" && n.state === "no")) {
      ended = !REACHED.has(next.state);
      return ended ? "none" : "bad";
    }
    if (REACHED.has(next.state)) return "plan";
    return n.state === "working" ? "flow" : "line";
  });
};

function nodeClass(n, i, segs) {
  return ["node", n.stage, "st-" + n.state,
    n.stage === "heed" || n.stage === "check" ? "diamond" : "",
    i > 0 ? "in-" + segs[i - 1] : "",
    i < segs.length ? "out-" + segs[i] : ""].join(" ");
}

const shown = (nodes) => nodes.filter((n) => n.state !== "skipped");

// mini gives a request's flow drawn small, its status read aloud.
B.mini = function (nodes, status) {
  const drawn = shown(nodes);
  const segs = B.segs(drawn);
  return el("span", {class: "mini"},
    drawn.map((n, i) => el("i", {class: nodeClass(n, i, segs), "aria-hidden": "true"}, el("b", {class: "mark"}))),
    el("span", {class: "vh"}, WORDS[status] || status));
};

// drawing gives a team's room. d holds:
//   name     the room's name on its top wall, an element or text
//   nodes    [{ stage, state, what }]: what is shown under the node's word
//   said     what is under SOURCE: what was said there
//   work     what is under WORK: what the team used and read; under
//            SOURCE when there is no WORK, as after a heed's no
//   answer   what left the team, on its way to REPLY
//   forks    the fork rows that leave from WORK
B.drawing = function (d) {
  const nodes = shown(d.nodes);
  const segs = B.segs(nodes);
  const down = B.narrow.matches;
  const wi = nodes.findIndex((n) => n.stage === "work");
  const forks = d.forks && d.forks.length ? el("ul", {class: "forks", "aria-label": "Forks from work"}, d.forks) : null;
  const under = (...parts) => parts.some(Boolean) ? el("div", {class: "under"}, parts) : null;

  const items = nodes.map((n, i) => {
    const here = (stage) => down && n.stage === stage;
    const text = el("div", {class: "text"},
      here("reply") ? d.answer : null,
      el("span", {class: "word"}, STAGE[n.stage], el("span", {class: "vh"}, ", " + (STATE[n.state] || n.state))),
      n.what ? el("div", {class: "what"}, n.what) : null,
      here("source") ? under(d.said, wi < 0 ? d.work : null) : null,
      here("work") ? [under(d.work), forks] : null);
    return el("li", {class: nodeClass(n, i, segs)},
      el("span", {class: "mark-row", "aria-hidden": "true"}, el("span", {class: "mark"})),
      text,
      !down && n.stage === "work" && forks ? el("span", {class: "drop", "aria-hidden": "true"}) : null);
  });

  // Across, the parts sit under the line: what was said from the left
  // wall to WORK, and from WORK to the right wall what the team used and
  // read, then the answer.
  let below = null;
  if (!down && (d.said || d.work || d.answer)) {
    below = wi < 0
      ? el("div", {class: "below solo"}, el("div", {class: "under-source"}, d.said, d.work, d.answer))
      : el("div", {class: "below"},
        el("div", {class: "under-source"}, d.said),
        el("div", {class: "under-work" + (forks ? " forked" : "")}, d.work, d.answer));
  }
  const workX = wi < 0 ? 50 : (wi + 0.5) / nodes.length * 100;
  const room = el("section", {class: "room", "aria-label": d.label},
    el("h2", {class: "room-name"}, d.name),
    el("ol", {class: "stages"}, items),
    below,
    !down && forks ? el("span", {class: "gap", "aria-hidden": "true"}) : null);
  return el("div", {class: "drawing" + (down ? " down" : ""), style: {"--work-x": workX + "%"}},
    room,
    !down && forks ? el("div", {class: "forks-out"}, forks) : null);
};

// forkEl gives one fork of a request: its kind, the party, when, how it
// stands, and, to those who may read it, the question, or the held jobs
// a decision is on, the answer, and what each decider said; a card under
// it when it waits on the one looking, which says the same.
B.forkEl = function (f, i) {
  const party = f.kind === "team" && f.request
    ? el("a", {class: "who", href: "#/requests/" + enc(f.request), title: "Open its request"},
      el("span", {class: "ic team", "aria-hidden": "true"}), el("span", {}, f.party.name + " ›"))
    : B.party(f.party, false);
  return el("li", {class: "fork " + f.status},
    el("div", {class: "fork-head"},
      el("span", {class: "chip" + (f.status === "waiting" ? " wait" : "")}, f.kind),
      party,
      el("span", {class: "faint"}, B.span(f.at, f.ended)),
      B.status(f.status)),
    f.card ? B.card(f.card, true) : [
      f.jobs ? el("p", {class: "fork-q mono"}, f.jobs.join(", ")) : null,
      f.question ? el("p", {class: "fork-q"}, f.question) : null,
      f.answer ? el("p", {class: "fork-a"}, f.answer) : null,
      f.deciders ? el("details", {class: "fork-more", "data-key": "fork-" + i},
        el("summary", {}, "notes"), B.decided(f.deciders)) : null]);
};
