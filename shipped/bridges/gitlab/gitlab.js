// The GitLab bridge's script, for both its machines: input.do is hear, for
// the in machine, or say, for the out machine. It speaks the REST API of
// the GitLab at the settings' site with a token, which it holds only as a
// handle.
const web = "https://" + input.settings.site
const api = web + "/api/v4"
const token = secrets.get("GITLAB_TOKEN")

// GitLab takes up to 1,000,000 characters in a comment. A string's length
// here counts UTF-16 code units, never fewer than GitLab counts.
const longest = 1000000

// call calls GitLab, and gives its answer, or { refused } when GitLab
// refuses the call for good. GitLab says how a call went by its status:
// 400, 403, 404 and 422 are a call it will never take, or a thing that is
// not there for this token; anything else, 401, 429 or a failure of its
// own, is thrown, so the run fails and is tried again. No error made here
// holds the address called.
function call(method, path, options) {
  const reply = http.request(api + path, Object.assign({ method: method, headers: { "PRIVATE-TOKEN": token }, as: "text", timeout: 30 }, options))
  if (reply.status >= 200 && reply.status < 300) return JSON.parse(reply.body)
  let why = ""
  try {
    const body = JSON.parse(reply.body)
    why = typeof body.message === "string" ? body.message : JSON.stringify(body.message || body.error || "")
  } catch (e) {}
  if ([400, 403, 404, 422].includes(reply.status)) return { refused: "GitLab refused it with " + reply.status + (why ? ": " + why : "") }
  throw new Error("GitLab answered " + method + " " + path.split("?")[0] + " with " + reply.status + (why ? ": " + why : ""))
}

function must(path, query) {
  const answer = call("GET", path, { query: query })
  if (answer && answer.refused) throw new Error(answer.refused)
  return answer
}

// thing gives where in a project an issue or a merge request is, as GitLab
// writes a reference to it, and its address.
function thing(project, type, iid) {
  const merge = type === "MergeRequest"
  return { place: project + (merge ? "!" : "#") + iid, link: web + "/" + project + "/-/" + (merge ? "merge_requests/" : "issues/") + iid }
}

// big gives a long text as a blob, so that what a run ends with stays
// within what Corbie carries.
const big = text => (text.length > 16384 ? blobs.make(text) : text)

// The most things one run hears, for the same reason: the rest are heard
// by the next.
const mostHeard = 20

if (input.do === "hear") {
  const cursor = Object.assign({}, input.cursor || {})
  const me = must("/user").id
  const heard = []
  for (const project of Object.keys((input.settings && input.settings.places) || {})) {
    const id = "/projects/" + encodeURIComponent(project)
    // The first ask of a project marks where it stands, and hears nothing.
    if (cursor[project] === undefined) {
      const latest = must(id + "/events", { per_page: "1" })
      cursor[project] = latest.length ? latest[0].id : 0
      continue
    }
    // Events come newest first: read back to the cursor, then take them
    // in the order they happened.
    const events = []
    for (let page = 1; page <= 10; page++) {
      const some = must(id + "/events", { per_page: "100", page: String(page) })
      const newer = some.filter(e => e.id > cursor[project])
      events.push(...newer)
      if (newer.length < some.length || some.length < 100) break
    }
    // Every comment and opening of an issue or a merge request is heard,
    // and every change of its state. Left out is only what the bridge's
    // own account did, GitLab's system notes, and what happened to
    // anything else, each logged with its id, its project and why, and
    // never its words.
    events.reverse()
    const leave = (e, why) => log("left out event:" + e.id + " in " + project + ": " + why)
    for (const e of events) {
      if (heard.length >= mostHeard) break
      cursor[project] = Math.max(cursor[project], e.id)
      if (e.author_id === me) {
        leave(e, "own")
        continue
      }
      const from = String(e.author_id)
      if (e.note && e.note.system) {
        leave(e, "system")
        continue
      }
      if (e.note) {
        if (e.note.noteable_type === "Issue" || e.note.noteable_type === "MergeRequest") {
          const t = thing(project, e.note.noteable_type, e.note.noteable_iid)
          heard.push({ id: "note:" + e.note.id, kind: "said", from: from, heard_in: project, place: t.place, about: { link: t.link }, text: big(e.note.body || "") })
        } else {
          leave(e, e.note.noteable_type)
        }
        continue
      }
      if (e.target_type !== "Issue" && e.target_type !== "MergeRequest") {
        leave(e, e.target_type || e.action_name)
        continue
      }
      const t = thing(project, e.target_type, e.target_iid)
      if (e.action_name === "opened") {
        // What was opened is read whole; one GitLab no longer gives, such
        // as an issue deleted since, is heard by its title alone, so it
        // never stops the bridge hearing what comes after it.
        const path = id + (e.target_type === "MergeRequest" ? "/merge_requests/" : "/issues/") + e.target_iid
        let opened = call("GET", path)
        if (opened.refused) opened = {}
        const text = (opened.title || e.target_title || "") + (opened.description ? "\n\n" + opened.description : "")
        heard.push({ id: "event:" + e.id, kind: "said", from: from, heard_in: project, place: t.place, about: { link: opened.web_url || t.link }, text: big(text) })
      } else if (["closed", "reopened", "accepted", "merged"].includes(e.action_name)) {
        const status = e.action_name === "accepted" ? "merged" : e.action_name
        heard.push({ id: "event:" + e.id, kind: "status", from: from, heard_in: project, place: t.place, about: { link: t.link },
          status: status, closed: status !== "reopened" })
      } else {
        leave(e, e.action_name)
      }
    }
  }
  return { heard: heard, cursor: cursor }
}

// split cuts text into parts GitLab takes, in order, never inside a
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

// say: each file uploaded to the project and linked at the end, and the
// text in as many comments as it takes.
const to = String(input.to)
const cut = Math.max(to.lastIndexOf("#"), to.lastIndexOf("!"))
if (cut < 0) return { refused: "GitLab has no issue or merge request " + to }
const project = "/projects/" + encodeURIComponent(to.slice(0, cut))
let text = blobs.read(input.text)
for (const f of input.files || []) {
  const uploaded = call("POST", project + "/uploads", { upload: { file: { data: f.data, name: f.name } } })
  if (uploaded.refused) return { refused: uploaded.refused }
  text += (text ? "\n\n" : "") + uploaded.markdown
}
const notes = project + (to[cut] === "!" ? "/merge_requests/" : "/issues/") + to.slice(cut + 1) + "/notes"
const parts = split(text, longest)
if (!parts.length) return { refused: "there is nothing to say" }
let sent
for (const part of parts) {
  sent = call("POST", notes, { json: { body: part } })
  if (sent.refused) return { refused: sent.refused }
}
return { sent: "note:" + sent.id }
