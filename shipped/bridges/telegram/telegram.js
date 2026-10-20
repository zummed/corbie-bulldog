// The Telegram bridge's script, for both its machines: input.do is hear,
// for the in machine, or say, for the out machine. It speaks Telegram's
// Bot API with the bot's token, which it holds only as a handle.
const api = "https://api.telegram.org"
const token = secrets.get("TELEGRAM_TOKEN")

// Telegram takes 1 to 4096 characters in a message, after entities are
// parsed, and counts them as UTF-16 code units, which is how a string's
// length is counted here.
const longest = 4096

// The largest file Bulldog keeps of what was said.
const largestFile = 16 << 20

// call calls a method of the Bot API and gives its result, or { refused }
// with Telegram's words when it refuses the call for good. Telegram says
// how a call went by its error_code: 400 is a call it will never take, 403
// a chat the bot may not write to; anything else, such as 429 or a
// failure of its own, is thrown, so the run fails and is tried again. No
// error made here holds the address called, since it holds the token.
function call(method, options) {
  const reply = http.post(secrets.text`${api}/bot${token}/${method}`, Object.assign({ as: "text", timeout: 70 }, options))
  let body
  try {
    body = JSON.parse(reply.body)
  } catch (e) {
    throw new Error("Telegram answered " + method + " with status " + reply.status + " and no answer it could read")
  }
  if (body.ok) return body.result
  if (body.error_code === 400 || body.error_code === 403) return { refused: "Telegram refused " + method + ": " + body.description }
  throw new Error("Telegram failed " + method + " with " + body.error_code + ": " + body.description)
}

// must gives a call's result, and throws a refusal: while hearing, what
// Telegram refuses is not a thing heard but a fault of the bridge.
function must(method, options) {
  const result = call(method, options)
  if (result && result.refused) throw new Error(result.refused)
  return result
}

// place gives where a message was said, as narrow as Telegram allows: a
// person's chat with the bot is them, and a topic of a group is its own.
function place(chat, from, topic) {
  if (chat.type === "private") return String(from.id)
  return topic ? chat.id + "/" + topic : String(chat.id)
}

// filesOf gives the files a message holds, each read whole out of
// Telegram: a document, or the largest size of a photo. One too large to
// keep, or that Telegram refuses for good, is left out and named, so one
// file never stops the bridge hearing what comes after it; a failure that
// may pass is thrown, and the run is tried again.
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
      left.push(f.name + ", as too large to keep")
      continue
    }
    const file = call("getFile", { json: { file_id: f.id } })
    if (file.refused) {
      left.push(f.name + ", as " + file.refused)
      continue
    }
    const data = http.get(secrets.text`${api}/file/bot${token}/${file.file_path}`, { as: "bytes", timeout: 70 })
    if (data.status >= 400 && data.status < 500) {
      left.push(f.name + ", as Telegram answered status " + data.status + " for it")
      continue
    }
    if (data.status !== 200) throw new Error("Telegram answered status " + data.status + " for the file " + f.name)
    files.push({ name: f.name, data: data.body })
  }
  return { files: files, left: left }
}

// chosen gives the choice a press of a button made: the button's data is
// Bulldog's reference and the choice's place among the buttons, which
// keeps it within the 64 bytes Telegram allows, and the choice is the
// text of that button on the message pressed. A press on a message
// Telegram no longer shows makes no choice, which Bulldog answers.
function chosen(q) {
  const data = String(q.data || "")
  const cut = data.indexOf(":")
  if (cut < 0) return { answers: data, choice: "" }
  const rows = (q.message && q.message.reply_markup && q.message.reply_markup.inline_keyboard) || []
  for (const row of rows) for (const button of row) if (button.callback_data === data) return { answers: data.slice(0, cut), choice: button.text }
  return { answers: data.slice(0, cut), choice: "" }
}

if (input.do === "hear") {
  // The first run asks for the latest update alone, which marks all
  // before it as read, and hears nothing of them. Ten at a time keep what
  // a run ends with well within what Corbie carries.
  const first = input.cursor === null || input.cursor === undefined
  const updates = first
    ? must("getUpdates", { json: { offset: -1, timeout: 0 } })
    : must("getUpdates", { json: { offset: input.cursor, timeout: 50, limit: 10, allowed_updates: ["message", "callback_query"] } })
  let cursor = first ? 0 : input.cursor
  const heard = []
  for (const u of updates) {
    cursor = Math.max(cursor, u.update_id + 1)
    if (first) continue
    // Every message said is heard. Left out is only what a bot said, and
    // any other kind of update, each logged with its id, its place and
    // why, and never its words.
    const m = u.message
    const q = u.callback_query
    if (m && (!m.from || m.from.is_bot)) log("left out " + m.chat.id + ":" + m.message_id + " in " + m.chat.id + ": bot")
    if (!m && !q) {
      const kind = Object.keys(u).find(k => k !== "update_id") || "empty"
      const of = u[kind] || {}
      const chat = of.chat || (of.message && of.message.chat)
      log("left out update " + u.update_id + " in " + (chat ? chat.id : "no chat") + ": " + kind)
    }
    if (m && m.from && !m.from.is_bot) {
      const direct = m.chat.type === "private"
      const got = filesOf(m)
      let text = m.text || m.caption || ""
      for (const left of got.left) text += (text ? "\n" : "") + "[a file left out: " + left + "]"
      // A chat, and a topic of a group, holds no threads: a reply is how
      // something is said more in what went before.
      const thing = {
        id: m.chat.id + ":" + m.message_id, kind: "said", from: String(m.from.id),
        heard_in: direct ? "direct" : String(m.chat.id), place: place(m.chat, m.from, m.is_topic_message ? m.message_thread_id : 0),
        threadless: true, text: text, files: got.files,
      }
      if (m.reply_to_message && !(m.is_topic_message && m.reply_to_message.message_id === m.message_thread_id)) {
        thing.answers = m.chat.id + ":" + m.reply_to_message.message_id
      }
      heard.push(thing)
    }
    if (q && q.from) {
      // The press is answered at once, so the person's client stops
      // waiting; what Bulldog says of it follows as a message.
      call("answerCallbackQuery", { json: { callback_query_id: q.id } })
      const chat = q.message ? q.message.chat : { type: "private" }
      const press = chosen(q)
      heard.push({
        id: "press:" + q.id, kind: "choice", from: String(q.from.id),
        heard_in: chat.type === "private" ? "direct" : String(chat.id),
        place: place(chat, q.from, q.message && q.message.is_topic_message ? q.message.message_thread_id : 0),
        answers: press.answers, choice: press.choice,
      })
    }
  }
  return { heard: heard, cursor: cursor }
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

// say: the text in as many messages as it takes, the first in reply to
// what it answers, even when Telegram no longer has that, and the last
// with a button for each choice, or asking for a reply to a question;
// then each file. What it sent is named by every message it sent, so a
// reply to any of them answers it.
const [chat, topic] = String(input.to).split("/")
const parts = split(blobs.read(input.text), longest)
const files = input.files || []
if (!parts.length && !files.length) return { refused: "there is nothing to say" }
let markup
if (input.ref && input.choices && input.choices.length) {
  markup = { inline_keyboard: [input.choices.map((c, n) => ({ text: c, callback_data: input.ref + ":" + n }))] }
} else if (input.ref) {
  markup = { force_reply: true }
}
let replyTo
if (input.answers) {
  const [answeredChat, message] = String(input.answers).split(":")
  if (answeredChat === chat && message) replyTo = { message_id: Number(message), allow_sending_without_reply: true }
}
const sent = []
for (let i = 0; i < parts.length; i++) {
  const message = { chat_id: chat, text: parts[i] }
  if (topic) message.message_thread_id = Number(topic)
  if (i === 0 && replyTo) message.reply_parameters = replyTo
  if (i === parts.length - 1 && markup) message.reply_markup = markup
  const result = call("sendMessage", { json: message })
  if (result.refused) return { refused: result.refused }
  sent.push(chat + ":" + result.message_id)
}
for (const f of files) {
  const upload = { chat_id: chat, document: { data: f.data, name: f.name } }
  if (topic) upload.message_thread_id = topic
  const result = call("sendDocument", { upload: upload })
  if (result.refused) return { refused: result.refused }
  sent.push(chat + ":" + result.message_id)
}
return { sent: sent }
