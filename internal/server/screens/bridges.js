// Bridges: each bridge of the office with the folder it runs, whether it
// listens or hears, its one status, and beneath it the places it listens
// in, each read as the way something said there goes: to its team, past
// the team's heed, to its lead, and past its disclosure. The people who
// decide a change also see its settings and what its machines reach.
(function () {
  "use strict";

  // hostEl gives where something may reach: a host, a setting that names
  // one, or anywhere.
  function hostEl(host) {
    if (host === "*") return "Anywhere";
    const setting = /^\$\{input\.settings\.(\w+)\}$/.exec(host);
    if (setting) return B.el("span", {class: "line"}, B.el("span", {class: "mono"}, setting[1]), B.el("span", {class: "faint"}, "Setting"));
    return B.el("span", {class: "mono"}, host);
  }

  // The places a secret may go that are no host reached over an encrypted
  // connection, by how they are written.
  const destinations = {"net.unencrypted": "Unencrypted", "exec": "Program", "fs.write": "File"};

  // toEl gives where a secret may go: a host or anywhere, or, labelled, a
  // host over any connection, a program or a path.
  function toEl(to) {
    const i = to.indexOf(":");
    const label = i < 0 ? undefined : destinations[to.slice(0, i)];
    if (!label) return hostEl(to);
    return B.el("span", {class: "line"}, B.el("span", {class: "faint"}, label), B.el("span", {class: "mono"}, to.slice(i + 1)));
  }

  // B.reachEl gives what something reaches: the hosts it may connect to,
  // and each secret it may send, with where, nested beside its name.
  B.reachEl = function (r) {
    const none = () => B.el("span", {class: "faint"}, "None");
    return B.kv([
      ["Hosts", r.hosts.length ? B.el("span", {class: "line"}, r.hosts.map(hostEl)) : none()],
      ["Secrets", r.secrets.length ? B.el("div", {class: "br-secrets"}, r.secrets.map((x) =>
        B.el("div", {class: "line"}, x.name === "*" ? "Every" : B.el("span", {class: "mono"}, x.name), B.el("span", {class: "faint"}, "→"), ...x.to.map(toEl)))) : none()],
    ]);
  };

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
      [B.el("span", {class: "line br-th"}, "Guests", ...B.info("About guests", "People the office does not name. They may ask only where a place lets them, are answered there, and nothing more.")), "Requests"]);
    const rows = b.places.map((p, i) => B.el("tr", {},
      B.el("td", {}, p.name),
      B.el("td", {}, B.teamEl(p.team)),
      B.el("td", {}, heedEl(p, teams[i])),
      B.el("td", {}, leadEl(teams[i])),
      discloses ? B.el("td", {}, disclosureEl(teams[i])) : null,
      B.el("td", {}, p.guests ? "Yes" : B.el("span", {class: "faint"}, "No")),
      B.el("td", {}, B.el("a", {href: "#/requests?" + new URLSearchParams({bridge: b.name, place: p.name})}, "Requests"))));
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

  // machineEl gives one of a bridge's machines, to those who see it: its
  // file, then, once Corbie has checked it, what it reaches nested
  // beneath.
  function machineEl(m) {
    if (!m.hosts) return B.el("span", {class: "mono"}, m.file);
    return B.el("div", {class: "br-machine"}, B.el("span", {class: "mono"}, m.file), B.reachEl(m));
  }

  // valueEl gives a setting as written: a list or a mapping nested, each
  // key beside its value, and anything else as text.
  function valueEl(v) {
    if (Array.isArray(v)) return B.el("div", {class: "br-secrets"}, v.map(valueEl));
    if (v && typeof v === "object") return B.kv(Object.keys(v).map((k) => [B.el("span", {class: "mono"}, k), valueEl(v[k])]));
    return B.el("span", {class: "mono"}, String(v));
  }

  // settingsEl gives a bridge's settings as written but its from and its
  // places, which the folder and the places table show; none when that
  // leaves nothing.
  function settingsEl(b) {
    const rest = Object.keys(b.settings || {}).filter((k) => k !== "from" && k !== "places");
    if (!rest.length) return null;
    return ["Settings", valueEl(Object.fromEntries(rest.map((k) => [k, b.settings[k]])))];
  }

  // bridge draws one bridge: its head, what it runs, with its settings and
  // its machines to those who see them, what is wrong with it to those
  // who see that, and its places nested beneath.
  function bridge(b) {
    const problems = (B.me.problems || []).filter((p) => p.bridge === b.name);
    return B.el("section", {class: "br", id: "bridge-" + b.name, "aria-label": b.name},
      B.el("div", {class: "br-head"},
        B.el("h2", {class: "who"}, B.icon("bridge"), b.name),
        B.bridgeStatus(b)),
      B.el("div", {class: "br-body"},
        B.kv([
          ["Folder", b.shipped ? B.el("span", {class: "line"}, B.el("a", {href: "#/shipped"}, "Shipped"), B.el("span", {class: "mono faint"}, b.folder)) : B.el("span", {class: "mono"}, b.folder)],
          ["In", b.in ? B.el("div", {class: "br-in"}, inEl(b), machineEl(b.in)) : inEl(b)],
          b.out ? ["Out", machineEl(b.out)] : null,
          settingsEl(b),
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
