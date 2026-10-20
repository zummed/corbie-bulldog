// The Jira bridge's script, for both its machines: input.do is hear, for
// the in machine, or say, for the out machine. It speaks the REST API of
// the Jira Cloud at the settings' site as the settings' email, with an API
// token it holds only as a handle.
const site = "https://" + input.settings.site
const api = site + "/rest/api/2"

// Jira takes up to 32,767 characters in a comment. A string's length here
// counts UTF-16 code units, never fewer than Jira counts.
const longest = 32767

// The most things one run hears, and the length of text past which it is
// given as a blob, so that what a run ends with stays within what Corbie
// carries; the rest are heard by the next run.
const mostHeard = 20
const big = text => (text.length > 16384 ? blobs.make(text) : text)

function auth() {
  const email = input.settings && input.settings.email
  if (!email) throw new Error("the Jira bridge's settings give no email, which it signs in as with JIRA_TOKEN")
  return secrets.text`Basic ${secrets.encoded(secrets.text`${email}:${secrets.get("JIRA_TOKEN")}`, { as: "base64" })}`
}

// call calls Jira, and gives its answer, or { refused } when Jira refuses
// the call for good. Jira says how a call went by its status: 400, 403 and
// 404 are a call it will never take, or an issue not there for this
// account; anything else, 401, 429 or a failure of its own, is thrown, so
// the run fails and is tried again. No error made here holds the address
// called.
function call(method, path, options) {
  const headers = Object.assign({ Authorization: auth(), Accept: "application/json" }, (options && options.headers) || {})
  const reply = http.request(api + path, Object.assign({ as: "text", timeout: 30 }, options, { method: method, headers: headers }))
  if (reply.status >= 200 && reply.status < 300) return reply.body ? JSON.parse(reply.body) : {}
  let why = ""
  try {
    const body = JSON.parse(reply.body)
    why = (body.errorMessages || []).concat(Object.values(body.errors || {})).join("; ")
  } catch (e) {}
  if ([400, 403, 404].includes(reply.status)) return { refused: "Jira refused it with " + reply.status + (why ? ": " + why : "") }
  throw new Error("Jira answered " + method + " " + path.split("?")[0] + " with " + reply.status + (why ? ": " + why : ""))
}

// when reads one of Jira's times, as 2026-10-05T09:00:00.000+0000.
const when = text => Date.parse(String(text).replace(/([+-]\d\d)(\d\d)$/, "$1:$2"))

// The most issues one search gives, and the most searches one run makes;
// what is past them is heard by the next run, from where this one ended.
const perSearch = 50
const mostSearches = 10

if (input.do === "hear") {
  const now = Date.now()
  if (!input.cursor) return { heard: [], cursor: { since: new Date(now).toISOString(), seen: [] } }
  const since = Date.parse(input.cursor.since)
  const seen = input.cursor.seen || []
  const projects = Object.keys((input.settings && input.settings.places) || {})
  if (!projects.length) return { heard: [], cursor: input.cursor }
  // What the bridge's own account says is what the out machine said, and
  // is not heard back.
  const me = call("GET", "/myself")
  if (me.refused) throw new Error(me.refused)
  const minutes = Math.ceil((now - since) / 60000) + 2
  const jql = "project in (" + projects.map(p => JSON.stringify(p)).join(", ") + ") AND updated >= -" + minutes + "m ORDER BY updated ASC"
  // Each new issue, comment and change of status, in the order it came:
  // what came at the cursor's own moment is heard unless the run before
  // heard it. Left out is only what the bridge's own account wrote, which
  // is logged with its id, its issue and why, and never its words, and is
  // passed by the cursor as what is heard is.
  const things = []
  const add = (at, thing) => {
    if (at > since || (at === since && !seen.includes(thing.id))) things.push(Object.assign({ at: at }, thing))
  }
  let token
  for (let page = 0; page < mostSearches; page++) {
    const query = { jql: jql, fields: "summary,description,status,reporter,created,comment", expand: "changelog", maxResults: String(perSearch) }
    if (token) query.nextPageToken = token
    const found = call("GET", "/search/jql", { query: query })
    if (found.refused) throw new Error(found.refused)
    for (const issue of found.issues || []) {
      const f = issue.fields || {}
      const project = issue.key.slice(0, issue.key.lastIndexOf("-"))
      const at = { heard_in: project, place: issue.key, about: { link: site + "/browse/" + issue.key } }
      if (f.reporter && f.reporter.accountId !== me.accountId) {
        add(when(f.created), Object.assign({ id: "issue:" + issue.id, kind: "said", from: f.reporter.accountId,
          text: big((f.summary || "") + (f.description ? "\n\n" + f.description : "")) }, at))
      } else if (f.reporter) {
        add(when(f.created), Object.assign({ id: "issue:" + issue.id, left: "own" }, at))
      }
      for (const c of (f.comment && f.comment.comments) || []) {
        if (c.author && c.author.accountId !== me.accountId) {
          add(when(c.created), Object.assign({ id: "comment:" + c.id, kind: "said", from: c.author.accountId, text: big(c.body || "") }, at))
        } else if (c.author) {
          add(when(c.created), Object.assign({ id: "comment:" + c.id, left: "own" }, at))
        }
      }
      for (const h of (issue.changelog && issue.changelog.histories) || []) {
        const change = (h.items || []).find(i => i.field === "status")
        if (change && h.author) {
          const done = f.status && f.status.statusCategory && f.status.statusCategory.key === "done" && f.status.name === change.toString
          add(when(h.created), Object.assign({ id: "change:" + h.id, kind: "status", from: h.author.accountId, status: change.toString, closed: !!done }, at))
        }
      }
    }
    token = found.nextPageToken
    if (!token || found.isLast) break
  }
  // Up to the most one run hears, and what is left out among them.
  things.sort((a, b) => a.at - b.at)
  const taken = []
  let counted = 0
  for (const t of things) {
    if (!t.left && counted === mostHeard) break
    if (!t.left) counted++
    taken.push(t)
  }
  if (!taken.length) return { heard: [], cursor: input.cursor }
  // The cursor is the moment of the last thing taken, with every thing
  // taken at that moment, so the next run neither drops nor repeats them.
  const last = taken[taken.length - 1].at
  const at = taken.filter(t => t.at === last).map(t => t.id)
  const heard = []
  for (const t of taken) {
    if (t.left) {
      log("left out " + t.id + " in " + t.place + ": " + t.left)
      continue
    }
    delete t.at
    heard.push(t)
  }
  return { heard: heard, cursor: { since: new Date(last).toISOString(), seen: last === since ? seen.concat(at) : at } }
}

// split cuts text into parts Jira takes, in order, never inside a
// character, at a line's end or a space where one is near; joined again
// they are the text whole.
function split(text, most) {
  const parts = []
  while (text.length > most) {
    let at = text.lastIndexOf("\n", most - 1) + 1
    if (at < most / 2) at = text.lastIndexOf(" ", most - 1) + 1
    if (at < most / 2) at = most
    const c = text.charCodeAt(at - 1)
    if (c >= 0xd800 && c < 0xdc00) at--
    parts.push(text.slice(0, at))
    text = text.slice(at)
  }
  if (text.length) parts.push(text)
  return parts
}

// say: the text in as many comments as it takes, then each file attached
// to the issue.
const issue = "/issue/" + encodeURIComponent(String(input.to))
const parts = split(blobs.read(input.text), longest)
const files = input.files || []
if (!parts.length && !files.length) return { refused: "there is nothing to say" }
let sent
for (const part of parts) {
  const comment = call("POST", issue + "/comment", { json: { body: part } })
  if (comment.refused) return { refused: comment.refused }
  sent = "comment:" + comment.id
}
for (const f of files) {
  const attached = call("POST", issue + "/attachments", { headers: { "X-Atlassian-Token": "no-check" }, upload: { file: { data: f.data, name: f.name } } })
  if (attached.refused) return { refused: attached.refused }
  if (!sent) sent = "attachment:" + attached[0].id
}
return { sent: sent }
