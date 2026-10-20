// Manual: Bulldog's pages of the manual, the office page, the page of each
// thing Bulldog ships and the program's: the page open, drawn by its
// headings, code and paragraphs, and beside it every page, listed by what
// it is.
(function () {
  "use strict";

  const groups = [["bridges/", "Bridges"], ["tools/", "Tools"], ["rules/", "Rules"]];

  function groupOf(name) {
    const g = groups.find(([prefix]) => name.startsWith(prefix));
    return g ? g[1] : "Bulldog";
  }

  function href(name) { return "#/manual/" + name.split("/").map(encodeURIComponent).join("/"); }

  // listEl gives the pages, grouped, the one open marked.
  function listEl(pages, open) {
    return ["Bulldog", ...groups.map((g) => g[1])].map((label) => {
      const of = pages.filter((p) => groupOf(p.name) === label);
      if (!of.length) return null;
      return B.el("section", {class: "mn-group", "aria-label": label},
        B.label(label),
        B.el("ul", {class: "mn-list"}, of.map((p) => B.el("li", {},
          B.el("a", {href: href(p.name), "aria-current": p.name === open ? "page" : null},
            B.el("span", {class: "mono"}, p.name), B.el("span", {class: "small muted"}, p.summary))))));
    });
  }

  // textEl gives a page's text in its parts: a heading, a block of code
  // between fences, or a paragraph, each as text.
  function textEl(text) {
    const out = [];
    let para = [], code = null;
    const flush = () => {
      if (para.length) out.push(B.el("p", {class: "mn-p"}, para.join("\n")));
      para = [];
    };
    for (const line of String(text).split("\n")) {
      if (code) {
        if (/^\s*```/.test(line)) { out.push(B.el("pre", {class: "mn-code"}, code.join("\n"))); code = null; } else code.push(line);
        continue;
      }
      const heading = line.match(/^(#{1,6})\s+(.*)$/);
      if (/^\s*```/.test(line)) { flush(); code = []; }
      else if (heading) { flush(); out.push(B.el(heading[1].length <= 2 ? "h2" : "h3", {class: "mn-h"}, heading[2])); }
      else if (line.trim() === "") flush();
      else para.push(line);
    }
    flush();
    if (code) out.push(B.el("pre", {class: "mn-code"}, code.join("\n")));
    return out;
  }

  B.route(/^#\/manual(?:\/(.+))?$/, async function (m) {
    const name = m[1] ? decodeURIComponent(m[1]) : "office";
    const [list, page] = await Promise.all([
      B.api("/api/manual"),
      B.api("/api/manual/" + name.split("/").map(encodeURIComponent).join("/")),
    ]);
    return B.el("div", {class: "mn"},
      B.head(null, "Manual"),
      B.el("div", {class: "cols"},
        B.el("article", {class: "main mn-page"},
          B.el("div", {class: "mn-head"}, B.el("h2", {class: "mono"}, page.name), B.el("span", {class: "muted"}, page.summary)),
          B.el("div", {class: "mn-text"}, textEl(page.text))),
        B.el("nav", {class: "side mn-side", "aria-label": "Pages"}, listEl(list.pages, page.name))));
  }, "manual", () => false);
})();
