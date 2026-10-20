// The Slack bridge's script, for both its machines: input.do is hear, for
// the in machine, or say, for the out machine. It speaks Slack's Web API
// with the app's bot token, which it holds only as a handle.
const api = "https://slack.com/api"
const bearer = () => secrets.text`Bearer ${secrets.get("SLACK_TOKEN")}`

// Slack takes up to 40,000 characters in a message's text, and 3,000 in a
// section that a message with buttons holds. A string's length here counts
// UTF-16 code units, never fewer than Slack counts.
const longest = 40000
const longestSection = 3000

// What Slack says when a call is refused for good, by its error code: a
// place the app cannot write to, or a message it will never take. Any
// other error, such as ratelimited or one of Slack's own, is thrown, so
// the run fails and is tried again.
const refusedFor = ["channel_not_found", "not_in_channel", "is_archived", "msg_too_long", "no_text", "invalid_blocks",
  "restricted_action", "restricted_action_read_only_channel", "restricted_action_non_threadable_channel", "user_not_found",
  "user_disabled", "cannot_dm_bot", "messages_tab_disabled", "invalid_arguments"]

// What Slack says when a reply cannot go under the message it answers,
// which it no longer has: the text is then said in the place, not in reply.
const threadGone = ["thread_not_found", "cannot_reply_to_message"]

// call calls a method of the Web API, and gives Slack's answer, which says
// by ok whether it went and by error why not. A failure of the call
// itself is thrown. No error made here holds the address called.
function call(method, body, query) {
  const options = { headers: { Authorization: bearer() }, as: "text", timeout: 30 }
  if (body) options.json = body
  if (query) options.query = query
  const reply = body ? http.post(api + "/" + method, options) : http.get(api + "/" + method, options)
  if (reply.status === 429) throw new Error("Slack answered " + method + " with 429, too many calls")
  let answer
  try {
    answer = JSON.parse(reply.body)
  } catch (e) {
    throw new Error("Slack answered " + method + " with status " + reply.status + " and no answer it could read")
  }
  return answer
}

// must gives Slack's answer to a call that must go: while hearing, any
// error is a fault of the bridge.
function must(method, body, query) {
  const answer = call(method, body, query)
  if (!answer.ok) throw new Error("Slack refused " + method + ": " + answer.error)
  return answer
}

// big gives a long text as a blob, so that what a run ends with stays
// within what Corbie carries.
const big = text => (text.length > 16384 ? blobs.make(text) : text)

if (input.do === "hear") {
  const d = input.delivery
  const header = name => {
    for (const k of Object.keys(d.headers || {})) if (k.toLowerCase() === name.toLowerCase()) return d.headers[k]
    return ""
  }
  // Only what Slack signed in the last five minutes is taken: an older
  // delivery sent again is a replay. The signature is compared by hashes
  // of both, so how long the comparison takes says nothing of how much of
  // a forged signature was right.
  const at = Number(header("X-Slack-Request-Timestamp"))
  if (!at || Math.abs(Date.now() / 1000 - at) > 300) return { heard: [], answer: { status: 401, body: "not signed in the last five minutes" } }
  const signed = "v0=" + crypto.hmac("v0:" + header("X-Slack-Request-Timestamp") + ":" + d.body, { key: secrets.get("SLACK_SIGNING_SECRET"), algorithm: "sha256", as: "hex" })
  const hashed = text => crypto.hash(text, { algorithm: "sha256", as: "hex" })
  if (hashed(signed) !== hashed(header("X-Slack-Signature"))) return { heard: [], answer: { status: 401, body: "not signed by Slack" } }

  // A press of a button comes as a form holding payload.
  let body
  if (header("Content-Type").startsWith("application/x-www-form-urlencoded")) {
    const form = {}
    for (const pair of d.body.split("&")) {
      const [k, v] = pair.split("=")
      form[decodeURIComponent(k)] = decodeURIComponent((v || "").replace(/\+/g, " "))
    }
    body = JSON.parse(form.payload || "{}")
  } else {
    body = JSON.parse(d.body)
  }
  if (body.type === "url_verification") return { heard: [], answer: { status: 200, body: body.challenge } }

  // heardIn gives the listed place a channel is, by its name as a person
  // writes it, as #help, and who can read it, by who is in it.
  const channelOf = channel => {
    const info = must("conversations.info", null, { channel: channel })
    if (info.channel.is_im) return { direct: true }
    const readers = []
    let cursor = ""
    for (let page = 0; page < 10; page++) {
      const members = must("conversations.members", null, { channel: channel, limit: "1000", cursor: cursor })
      readers.push(...members.members)
      cursor = members.response_metadata && members.response_metadata.next_cursor
      if (!cursor) break
    }
    return { name: "#" + info.channel.name, readers: readers }
  }

  if (body.type === "block_actions") {
    const action = (body.actions || [])[0] || {}
    const value = String(action.value || "")
    const cut = value.indexOf(":")
    const channel = body.channel ? body.channel.id : ""
    const container = body.container || {}
    const where = channel ? channelOf(channel) : { direct: true }
    return {
      heard: [{
        id: "press:" + body.user.id + ":" + action.action_ts, kind: "choice", from: body.user.id,
        heard_in: where.direct ? "direct" : where.name,
        place: where.direct ? body.user.id : channel + "/" + (container.thread_ts || container.message_ts),
        answers: cut < 0 ? value : value.slice(0, cut), choice: cut < 0 ? "" : value.slice(cut + 1),
      }],
      answer: { status: 200 },
    }
  }

  // answered gives what a message said to the app directly answers: what
  // it replies under, or else the app's latest message in that
  // conversation before it, so a person answers a question by typing in
  // their conversation with the app. What the app sent is named as
  // channel/ts, as say names it.
  const answered = message => {
    if (message.thread_ts && message.thread_ts !== message.ts) return message.channel + "/" + message.thread_ts
    const history = must("conversations.history", null, { channel: message.channel, latest: message.ts, limit: "20" })
    const sent = (history.messages || []).find(m => m.bot_id)
    return sent ? message.channel + "/" + sent.ts : undefined
  }

  // mentions says whether a text names the app, as <@its id>, which Slack
  // says when asked who the app is; a text naming nobody asks nothing.
  const mentions = text => {
    if (!text.includes("<@")) return false
    const me = must("auth.test").user_id
    return text.includes("<@" + me + ">") || text.includes("<@" + me + "|")
  }

  // Every message said is heard. Left out is only what is no message, and
  // what a bot said, the app among them, each logged with its id, its
  // place and why, by Slack's own word for what it is, and never its
  // words.
  const e = body.event || {}
  const said = !e.subtype || e.subtype === "file_share" || e.subtype === "thread_broadcast" || e.subtype === "me_message"
  let left = ""
  if (body.type !== "event_callback" || e.type !== "message") left = e.type || body.type
  else if (!said) left = e.subtype
  else if (e.bot_id || !e.user) left = "bot"
  if (left) {
    log("left out " + (body.event_id || body.type) + " in " + (e.channel || "no channel") + ": " + left)
    return { heard: [], answer: { status: 200 } }
  }
  const direct = e.channel_type === "im"
  const where = direct ? { direct: true } : channelOf(e.channel)
  const files = []
  for (const f of e.files || []) {
    const got = http.get(f.url_private, { headers: { Authorization: bearer() }, as: "bytes", timeout: 60 })
    if (got.status !== 200) throw new Error("Slack answered status " + got.status + " for the file " + f.name)
    files.push({ name: f.name, data: got.body })
  }
  const thing = {
    id: body.event_id || e.client_msg_id || e.channel + "/" + e.ts, kind: "said", from: e.user,
    heard_in: direct ? "direct" : where.name,
    place: direct ? e.user : e.channel + "/" + (e.thread_ts || e.ts),
    readers: where.readers || [], text: big(e.text || ""), files: files,
  }
  if (direct) {
    const answers = answered(e)
    if (answers) thing.answers = answers
  }
  if (mentions(e.text || "")) thing.mentioned = true
  return { heard: [thing], answer: { status: 200 } }
}

// split cuts text into parts Slack takes, in order, never inside a
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

// say: to a person directly in their conversation with the app, or in a
// thread; the text in as many messages as it takes, the last with a button
// for each choice. Slack takes a file only at files.slack.com, which this
// machine does not reach, so each file is named, and not sent. What it
// sent is named by its last message, the one that holds the buttons.
let channel, thread
if (input.direct) {
  const opened = call("conversations.open", { users: input.to })
  if (!opened.ok) {
    if (refusedFor.includes(opened.error)) return { refused: "Slack refused to open a conversation: " + opened.error }
    throw new Error("Slack failed conversations.open: " + opened.error)
  }
  channel = opened.channel.id
} else {
  [channel, thread] = String(input.to).split("/")
}
const choices = input.ref && input.choices && input.choices.length ? input.choices : []
let text = blobs.read(input.text)
for (const f of input.files || []) text += (text ? "\n" : "") + "[a file that was not sent to Slack: " + f.name + "]"
const parts = split(text, choices.length ? longestSection : longest)
if (!parts.length) return { refused: "there is nothing to say" }
let sent
for (let i = 0; i < parts.length; i++) {
  const message = { channel: channel, text: parts[i] }
  if (thread) message.thread_ts = thread
  if (i === parts.length - 1 && choices.length) {
    message.blocks = [
      { type: "section", text: { type: "plain_text", text: parts[i] } },
      { type: "actions", elements: choices.map((c, n) => ({ type: "button", action_id: "choice-" + n, text: { type: "plain_text", text: c }, value: input.ref + ":" + c })) },
    ]
  }
  let answer = call("chat.postMessage", message)
  if (!answer.ok && thread && threadGone.includes(answer.error)) {
    delete message.thread_ts
    thread = undefined
    answer = call("chat.postMessage", message)
  }
  if (!answer.ok) {
    if (refusedFor.includes(answer.error)) return { refused: "Slack refused the message: " + answer.error }
    throw new Error("Slack failed chat.postMessage: " + answer.error)
  }
  sent = answer.channel + "/" + answer.ts
}
return { sent: sent }
