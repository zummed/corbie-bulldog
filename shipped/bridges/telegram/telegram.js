// Telegram's script, for both its machines: input.do is hear, for the in,
// or say, for the out. It speaks Telegram's Bot API with the bot's token,
// which it holds only as a handle.
const api = "https://api.telegram.org"
const token = secrets.get("TELEGRAM_TOKEN")

// How long one job hears, well within its time limit.
const listenFor = 4 * 60 * 1000

// Telegram takes 1 to 4096 characters in a message, counted as UTF-16 code
// units, as a string's length is here.
const longest = 4096

// The largest file the bridge reads of what was said.
const largestFile = 16 << 20

// call calls a method of the Bot API and gives Telegram's answer:
// { ok, result } or { ok: false, error_code, description }. No error made
// here holds the address called, since it holds the token.
function call(method, options) {
  const reply = http.post(secrets.text`${api}/bot${token}/${method}`, Object.assign({ as: "text", timeout: 70 }, options))
  try {
    return JSON.parse(reply.body)
  } catch (e) {
    throw new Error("Telegram answered " + method + " with status " + reply.status + " and no answer it could read")
  }
}

// must gives a call's result, and throws what Telegram refuses.
function must(method, options) {
  const answer = call(method, options)
  if (!answer.ok) throw new Error("Telegram refused " + method + " with " + answer.error_code + ": " + answer.description)
  return answer.result
}

// filesOf gives the files a message holds, each read whole: a document, or
// the largest size of a photo. A file too large, or one Telegram will
// never give (400), is left out and named, so one file never stops the
// bridge hearing what comes after it.
function filesOf(m) {
  const wanted = []
  if (m.document) wanted.push({ id: m.document.file_id, name: m.document.file_name || "document", size: m.document.file_size })
  if (m.photo && m.photo.length) {
    const p = m.photo[m.photo.length - 1]
    wanted.push({ id: p.file_id, name: "photo-" + m.message_id + ".jpg", size: p.file_size })
  }
  const files = []
  const left = []
  for (const f of wanted) {
    if (f.size > largestFile) {
      left.push(f.name + ", as too large")
      continue
    }
    const file = call("getFile", { json: { file_id: f.id } })
    if (!file.ok && file.error_code === 400) {
      left.push(f.name + ", as Telegram said: " + file.description)
      continue
    }
    if (!file.ok) throw new Error("Telegram refused getFile with " + file.error_code + ": " + file.description)
    const data = http.get(secrets.text`${api}/file/bot${token}/${file.result.file_path}`, { as: "bytes", timeout: 70 })
    if (data.status !== 200) throw new Error("Telegram answered status " + data.status + " for the file " + f.name)
    files.push({ name: f.name, data: data.body })
  }
  return { files: files, left: left }
}

// names says whether a message names the bot: an @mention of its username,
// or a mention of the bot by its id.
function names(m, me) {
  const text = m.text || m.caption || ""
  return (m.entities || m.caption_entities || []).some(e =>
    (e.type === "mention" && text.substr(e.offset, e.length).toLowerCase() === "@" + me.username.toLowerCase()) ||
    (e.type === "text_mention" && e.user && e.user.id === me.id))
}

// chosen gives a press's ref and choice: a button's data is the out's ref
// and the choice's place among the buttons, which fits the 64 bytes
// Telegram allows, and the choice is that button's text.
function chosen(q) {
  const data = String(q.data || "")
  const cut = data.lastIndexOf(":")
  if (cut < 0) return { ref: data, choice: null }
  const rows = (q.message && q.message.reply_markup && q.message.reply_markup.inline_keyboard) || []
  for (const row of rows) for (const button of row) if (button.callback_data === data) return { ref: data.slice(0, cut), choice: button.text }
  return { ref: data.slice(0, cut), choice: null }
}

// heard gives the thing an update holds: a message said, or a press of a
// button. Left out is what a bot said and any other kind of update, each
// logged with its id and why, never its words.
function heard(u, me) {
  const m = u.message
  const q = u.callback_query
  if (m && (!m.from || m.from.is_bot)) {
    log("left out " + m.chat.id + ":" + m.message_id + ": a bot said it")
    return null
  }
  if (m) {
    const got = filesOf(m)
    let text = m.text || m.caption || ""
    for (const left of got.left) text += (text ? "\n" : "") + "[a file left out: " + left + "]"
    // A chat holds no threads, and a topic of a group is a place of its own.
    const topic = m.is_topic_message ? m.message_thread_id : 0
    const replied = m.reply_to_message && m.reply_to_message.message_id !== topic
    return {
      id: m.chat.id + ":" + m.message_id, at: new Date(m.date * 1000).toISOString(), kind: "said", from: String(m.from.id),
      place: m.chat.type === "private" ? "direct" : String(m.chat.id), where: topic ? m.chat.id + "/" + topic : String(m.chat.id),
      text: text, files: got.files, mentioned: names(m, me),
      answers: replied ? m.chat.id + ":" + m.reply_to_message.message_id : null, ref: null, choice: null,
    }
  }
  if (q) {
    // Answered at once, so the person's app stops waiting; whether Telegram
    // takes it changes nothing heard.
    call("answerCallbackQuery", { json: { callback_query_id: q.id } })
    const chat = q.message ? q.message.chat : { id: q.from.id, type: "private" }
    const topic = q.message && q.message.is_topic_message ? q.message.message_thread_id : 0
    const press = chosen(q)
    return {
      id: "press:" + q.id, at: new Date().toISOString(), kind: "press", from: String(q.from.id),
      place: chat.type === "private" ? "direct" : String(chat.id), where: topic ? chat.id + "/" + topic : String(chat.id),
      text: "", files: [], mentioned: false, answers: null, ref: press.ref, choice: press.choice,
    }
  }
  log("left out update " + u.update_id + ": " + (Object.keys(u).find(k => k !== "update_id") || "empty"))
  return null
}

if (input.do === "hear") {
  const me = must("getMe", {})
  let cursor = input.cursor
  if (cursor === null || cursor === undefined) {
    // A new bridge asks for the latest update alone, which has Telegram
    // forget all before it, and hears none of them.
    const latest = must("getUpdates", { json: { offset: -1, timeout: 0 } })
    cursor = latest.length ? latest[latest.length - 1].update_id + 1 : 0
    emit("heard", { things: [], cursor: cursor })
  }
  const until = Date.now() + listenFor
  for (let left = listenFor; left > 0; left = until - Date.now()) {
    const wait = Math.min(50, Math.ceil(left / 1000))
    const updates = must("getUpdates", { json: { offset: cursor, timeout: wait, limit: 100, allowed_updates: ["message", "callback_query"] }, timeout: wait + 20 })
    if (!updates.length) continue
    const things = []
    for (const u of updates) {
      cursor = Math.max(cursor, u.update_id + 1)
      const thing = heard(u, me)
      if (thing) things.push(thing)
    }
    emit("heard", { things: things, cursor: cursor })
  }
  return { cursor: cursor }
}

// split cuts text into parts Telegram takes, in order, never inside a
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

// say: the text in as many messages as it takes, the first replying to
// what it answers, even when Telegram no longer has that, and the last
// with a button for each choice, or asking for a reply to a question that
// offers none. A few short choices share one row; otherwise each has a row
// of its own, since Telegram cuts a button's text to fit its row.
const [chat, topic] = String(input.to).split("/")
const parts = split(input.text, longest)
if (!parts.length) throw new Error("there is nothing to say")
let markup
if (input.ref && input.choices && input.choices.length) {
  const buttons = input.choices.map((c, n) => ({ text: c, callback_data: input.ref + ":" + n }))
  const oneRow = buttons.length <= 3 && input.choices.join("").length <= 30
  markup = { inline_keyboard: oneRow ? [buttons] : buttons.map(b => [b]) }
} else if (input.ref) {
  markup = { force_reply: true }
}
let replyTo
if (input.answers) {
  const [answeredChat, message] = String(input.answers).split(":")
  if (answeredChat === chat && message) replyTo = { message_id: Number(message), allow_sending_without_reply: true }
}
let sent
for (let i = 0; i < parts.length; i++) {
  const message = { chat_id: chat, text: parts[i] }
  if (topic) message.message_thread_id = Number(topic)
  if (i === 0 && replyTo) message.reply_parameters = replyTo
  if (i === parts.length - 1 && markup) message.reply_markup = markup
  sent = chat + ":" + must("sendMessage", { json: message }).message_id
}
return { sent: sent }
