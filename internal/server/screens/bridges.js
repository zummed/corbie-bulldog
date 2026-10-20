// Bridges: each bridge of the office with the folder it runs, whether it
// listens or hears, its one status, and beneath it the places it listens
// in, each read as the way something said there goes: to its team, past
// the team's heed, to its lead, and past its disclosure.
(function () {
  "use strict";

  // heedEl gives a place's heed: none at a direct place, which is never
  // weighed; the rule, to those who see it; else whether the team sets
  // one.
  function heedEl(place, t) {
    if (place.name === "direct") return "";
    if (!t || !t.heed) return B.el("span", {class: "faint"}, "None");
    return t.heed.rule ? B.ruleEl(t.heed.rule) : "Set";
  }

  function disclosureEl(t) {
    if (!t || t.discloses === undefined) return "";
    return t.discloses ? "Set" : B.el("span", {class: "faint"}, "None");
  }

  function leadEl(t) {
    if (!t || !t.lead) return "";
    return B.person(t.lead) ? B.personEl(t.lead) : B.workerEl(t.lead, t.name, {bare: true});
  }

  // places draws the places a bridge listens in, then what it leaves out.
  function places(b) {
    const teams = b.places.map((p) => B.team(p.team));
    const discloses = teams.some((t) => t && t.discloses !== undefined);
    const head = ["Place", "Team", "Heed", "Lead"].concat(discloses ? ["Disclosure"] : [],
      [B.el("span", {class: "line br-th"}, "Guests", ...B.info("About guests", "People the office does not name. They may ask only where a place lets them, are answered there, and nothing more."))]);
    const rows = b.places.map((p, i) => B.el("tr", {},
      B.el("td", {}, p.name),
      B.el("td", {}, B.teamEl(p.team)),
      B.el("td", {}, heedEl(p, teams[i])),
      B.el("td", {}, leadEl(teams[i])),
      discloses ? B.el("td", {}, disclosureEl(teams[i])) : null,
      B.el("td", {}, p.guests ? "Yes" : B.el("span", {class: "faint"}, "No"))));
    const rest = B.el("tr", {class: "br-rest"}, B.el("td", {colspan: head.length},
      B.el("span", {class: "line"}, "Everything else",
        ...B.info("About everything else", "Left out: an unlisted place, a guest where guests may not ask, a choice on nothing waiting."))));
    return B.el("div", {class: "br-places"},
      B.label("Places", B.el("span", {class: "count"}, String(b.places.length)),
        ...B.info("About places", "Where on a bridge something said begins a request, to the team named, or the office, and is answered there.")),
      B.el("div", {class: "scroll"}, B.el("table", {class: "table br-table"},
        B.el("thead", {}, B.el("tr", {}, head.map((h) => B.el("th", {}, h)))),
        B.el("tbody", {}, rows, rest))));
  }

  // inEl gives how the bridge's in machine is started: it listens, or it
  // hears at its address.
  function inEl(b) {
    if (b.hears) return B.el("span", {class: "line"}, "Hears",
      ...B.info("About hearing", "The service calls this address with each thing said or pressed; it must be able to reach it."),
      B.el("span", {class: "mono"}, b.address));
    if (b.listens) return B.el("span", {class: "line"}, "Listens",
      ...B.info("About listening", "Asks the service for what is new since last time; nothing is missed between."));
    return "";
  }

  // bridge draws one bridge: its head, what it runs, what is wrong with it
  // to those who see that, and its places nested beneath.
  function bridge(b) {
    const problems = (B.me.problems || []).filter((p) => p.bridge === b.name);
    return B.el("section", {class: "br", id: "bridge-" + b.name, "aria-label": b.name},
      B.el("div", {class: "br-head"},
        B.el("h2", {class: "who"}, B.icon("bridge"), b.name),
        B.bridgeStatus(b)),
      B.el("div", {class: "br-body"},
        B.kv([
          ["Folder", b.shipped ? B.el("span", {class: "line"}, "Shipped", B.el("span", {class: "mono faint"}, b.folder)) : B.el("span", {class: "mono"}, b.folder)],
          ["In", inEl(b)],
          b.failing ? ["Failing", B.el("span", {class: "line"},
            B.el("span", {class: "mono br-bad"}, b.failing.error),
            B.el("span", {class: "mono faint"}, "×" + b.failing.failures), B.whenEl(b.failing.since))] : null,
          problems.length ? ["Problems", B.el("div", {class: "stack br-problems"}, problems.map((p) => B.el("span", {class: "mono br-bad"}, p.said)))] : null,
        ]),
        places(b)));
  }

  B.route(/^#\/bridges$/, function () {
    const all = B.me.office.bridges || [];
    return B.el("div", {class: "brs"},
      B.head(null, "Bridges"),
      all.length ? all.map(bridge) : B.empty());
  }, "bridges", (what) => what === "office");
})();
