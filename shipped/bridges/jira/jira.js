// Jira's script, for both its machines: input.do is hear, for the in, or
// say, for the out. It speaks the REST API of the Jira Cloud at the
// settings' site as the settings' email, with an API token it holds only
// as a handle.
const site = "https://" + input.settings.site
const api = site + "/rest/api/2"

// Jira takes up to 32,767 characters in a comment. A string's length here
// counts UTF-16 code units, never fewer than Jira counts.
const longest = 32767

// The most things one job hands over; the rest are heard by the next.
const mostHeard = 20

function auth() {
  const email = input.settings.email
  if (!email) throw new Error("the Jira bridge's settings give no email, which it signs in as with JIRA_TOKEN")
  return secrets.text`Basic ${secrets.encoded(secrets.text`${email}:${secrets.get("JIRA_TOKEN")}`, { as: "base64" })}`
}

// must calls Jira and gives its answer, and throws what Jira refuses, with
// its words. No error made here holds the address called.
function must(method, path, options) {
  const headers = { Authorization: auth(), Accept: "application/json" }
  const reply = http.request(api + path, Object.assign({ as: "text", timeout: 30 }, options, { method: method, headers: headers }))
  let body = null
  try {
    body = JSON.parse(reply.body)
  } catch (e) {}
  if (reply.status >= 200 && reply.status < 300) return body || {}
  const why = body ? (body.errorMessages || []).concat(Object.values(body.errors || {})).join("; ") : ""
  throw new Error("Jira answered " + method + " " + path + " with " + reply.status + (why ? ": " + why : ""))
}

// when reads one of Jira's times, as 2026-10-05T09:00:00.000+0000.
const when = text => Date.parse(String(text).replace(/([+-]\d\d)(\d\d)$/, "$1:$2"))

// The most issues one search gives, and the most searches one job makes;
// what is past them is heard by the next job, from where this one ended.
const perSearch = 50
const mostSearches = 10

if (input.do === "hear") {
  // A new bridge hears from now on.
  if (!input.cursor) {
    const cursor = { since: new Date().toISOString(), seen: [] }
    emit("heard", { things: [], cursor: cursor })
    return { cursor: cursor }
  }
  const since = Date.parse(input.cursor.since)
  const seen = input.cursor.seen || []
  const projects = Object.keys(input.settings.places || {})
  if (!projects.length) return { cursor: input.cursor }
  const me = must("GET", "/myself")
  const minutes = Math.ceil((Date.now() - since) / 60000) + 2
  const jql = "project in (" + projects.map(p => JSON.stringify(p)).join(", ") + ") AND updated >= -" + minutes + "m ORDER BY updated ASC"
  // Each new issue and comment, in the order it came: what came at the
  // cursor's own moment is heard unless the job before heard it. Left out
  // is what the bridge's own account or an app wrote, which the cursor
  // passes as it passes what is heard.
  const found = []
  const add = (at, author, thing) => {
    if (!author || !(at > since || (at === since && !seen.includes(thing.id)))) return
    const left = author.accountId === me.accountId ? "own" : author.accountType === "app" ? "an app wrote it" : ""
    found.push(Object.assign({ at: at, left: left, from: author.accountId }, thing))
  }
  let token
  for (let page = 0; page < mostSearches; page++) {
    const query = { jql: jql, fields: "summary,description,reporter,created,comment", maxResults: String(perSearch) }
    if (token) query.nextPageToken = token
    const result = must("GET", "/search/jql", { query: query })
    for (const issue of result.issues || []) {
      const f = issue.fields || {}
      const at = { place: issue.key.slice(0, issue.key.lastIndexOf("-")), where: issue.key }
      add(when(f.created), f.reporter, Object.assign({ id: "issue:" + issue.id, text: (f.summary || "") + (f.description ? "\n\n" + f.description : "") }, at))
      for (const c of (f.comment && f.comment.comments) || []) {
        add(when(c.created), c.author, Object.assign({ id: "comment:" + c.id, text: c.body || "" }, at))
      }
    }
    token = result.nextPageToken
    if (!token || result.isLast) break
  }
  // Up to the most one job hands over, and what is left out among them.
  found.sort((a, b) => a.at - b.at)
  const taken = []
  let counted = 0
  for (const t of found) {
    if (!t.left && counted === mostHeard) break
    if (!t.left) counted++
    taken.push(t)
  }
  if (!taken.length) return { cursor: input.cursor }
  // The cursor is the moment of the last thing taken, with every thing
  // taken at that moment, so the next job neither drops nor repeats them.
  const last = taken[taken.length - 1].at
  const at = taken.filter(t => t.at === last).map(t => t.id)
  const things = []
  for (const t of taken) {
    if (t.left) {
      log("left out " + t.id + " in " + t.where + ": " + t.left)
      continue
    }
    things.push({
      id: t.id, at: new Date(t.at).toISOString(), kind: "said", from: t.from, place: t.place, where: t.where,
      text: t.text, files: [], mentioned: t.text.includes("[~accountid:" + me.accountId + "]"), answers: null, ref: null, choice: null,
    })
  }
  const cursor = { since: new Date(last).toISOString(), seen: last === since ? seen.concat(at) : at }
  emit("heard", { things: things, cursor: cursor })
  return { cursor: cursor }
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

// say: the text in as many comments as it takes.
const issue = "/issue/" + encodeURIComponent(String(input.to))
const parts = split(input.text, longest)
if (!parts.length) throw new Error("there is nothing to say")
let sent
for (const part of parts) sent = "comment:" + must("POST", issue + "/comment", { json: { body: part } }).id
return { sent: sent }
