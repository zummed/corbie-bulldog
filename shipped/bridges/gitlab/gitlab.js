// GitLab's script, for both its machines: input.do is hear, for the in, or
// say, for the out. It speaks the REST API of the GitLab at the settings'
// site with a token, which it holds only as a handle.
const api = "https://" + input.settings.site + "/api/v4"
const token = secrets.get("GITLAB_TOKEN")

// The most things one job hands over; the rest are heard by the next.
const mostHeard = 20

// call calls GitLab, and gives its status and its answer. No error made
// here holds the address called.
function call(method, path, options) {
  const reply = http.request(api + path, Object.assign({ method: method, headers: { "PRIVATE-TOKEN": token }, as: "text", timeout: 30 }, options))
  let body = null
  try {
    body = JSON.parse(reply.body)
  } catch (e) {}
  return { status: reply.status, body: body }
}

// refused gives the error of a call GitLab refused, with its words.
function refused(method, path, reply) {
  const why = reply.body && (typeof reply.body.message === "string" ? reply.body.message : JSON.stringify(reply.body.message || reply.body.error || ""))
  return new Error("GitLab answered " + method + " " + path + " with " + reply.status + (why ? ": " + why : ""))
}

// must gives GitLab's answer to a call, and throws what GitLab refuses.
function must(method, path, options) {
  const reply = call(method, path, options)
  if (reply.status >= 200 && reply.status < 300) return reply.body
  throw refused(method, path, reply)
}

if (input.do === "hear") {
  const me = must("GET", "/user")
  const cursor = Object.assign({}, input.cursor || {})
  const things = []
  // mentions says whether a text has an @mention of the bridge's account.
  const mentions = text => (text.match(/@[\w.-]+/g) || []).map(m => m.replace(/\.+$/, "")).includes("@" + me.username)
  // bot says whether a user is a bot, as a project's or a group's token
  // is, asked of GitLab once a job. A user GitLab does not show is not.
  const bots = {}
  const bot = user => {
    if (!(user in bots)) {
      const path = "/users/" + user
      const shown = call("GET", path)
      if (shown.status !== 200 && shown.status !== 404) throw refused("GET", path, shown)
      bots[user] = shown.status === 200 && shown.body.bot === true
    }
    return bots[user]
  }
  for (const project of Object.keys(input.settings.places || {})) {
    const id = "/projects/" + encodeURIComponent(project)
    // A project first asked about gives its latest event as the cursor,
    // and nothing before it is heard.
    if (cursor[project] === undefined) {
      const latest = must("GET", id + "/events", { query: { per_page: "1" } })
      cursor[project] = latest.length ? latest[0].id : 0
      continue
    }
    // Events come newest first: read back to the cursor, then take them in
    // the order they happened.
    const events = []
    for (let page = 1; page <= 10; page++) {
      const some = must("GET", id + "/events", { query: { per_page: "100", page: String(page) } })
      const newer = some.filter(e => e.id > cursor[project])
      events.push(...newer)
      if (newer.length < some.length || some.length < 100) break
    }
    events.reverse()
    // The opening of an issue or a merge request is heard, and each
    // comment on one. Left out is what the bridge's own account or another
    // bot did, GitLab's system notes, an internal note, and anything else,
    // each logged with its id, its project and why, never its words.
    const leave = (e, why) => log("left out event:" + e.id + " in " + project + ": " + why)
    for (const e of events) {
      if (things.length >= mostHeard) break
      cursor[project] = Math.max(cursor[project], e.id)
      const target = e.note ? e.note.noteable_type : e.target_type
      if (e.author_id === me.id) leave(e, "own")
      else if (e.note && e.note.system) leave(e, "a system note")
      else if (e.note && (e.note.internal || e.note.confidential)) leave(e, "an internal note")
      else if (target !== "Issue" && target !== "MergeRequest") leave(e, target || e.action_name)
      else if (!e.note && e.action_name !== "opened") leave(e, e.action_name)
      else if (bot(e.author_id)) leave(e, "a bot did it")
      else {
        const merge = target === "MergeRequest"
        const iid = e.note ? e.note.noteable_iid : e.target_iid
        let text = e.note ? e.note.body : null
        if (!e.note) {
          // What was opened is read whole; one GitLab no longer gives, as an
          // issue deleted since, is heard by its title alone.
          const path = id + (merge ? "/merge_requests/" : "/issues/") + iid
          const opened = call("GET", path)
          if (opened.status !== 200 && opened.status !== 404) throw refused("GET", path, opened)
          const whole = opened.status === 200 ? opened.body : {}
          text = (whole.title || e.target_title || "") + (whole.description ? "\n\n" + whole.description : "")
        }
        things.push({
          id: e.note ? "note:" + e.note.id : "event:" + e.id, at: new Date(e.created_at).toISOString(), kind: "said", from: String(e.author_id),
          place: project, where: project + (merge ? "!" : "#") + iid, text: text || "", files: [], mentioned: mentions(text || ""),
          answers: null, ref: null, choice: null,
        })
      }
    }
  }
  if (JSON.stringify(cursor) !== JSON.stringify(input.cursor)) emit("heard", { things: things, cursor: cursor })
  return { cursor: cursor }
}

// say: the text as one comment. GitLab takes up to 1,000,000 characters
// in a comment, more than a Corbie job's input holds.
const to = String(input.to)
const cut = Math.max(to.lastIndexOf("#"), to.lastIndexOf("!"))
if (cut < 0) throw new Error("GitLab has no issue or merge request " + to)
if (!input.text) throw new Error("there is nothing to say")
const notes = "/projects/" + encodeURIComponent(to.slice(0, cut)) + (to[cut] === "!" ? "/merge_requests/" : "/issues/") + to.slice(cut + 1) + "/notes"
return { sent: "note:" + must("POST", notes, { json: { body: input.text } }).id }
