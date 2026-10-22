// Slack's script, for both its machines: input.do is hear, for the in, or
// say, for the out. It speaks Slack's Web API with the app's bot token, and
// listens over Socket Mode with the app's app-level token, holding each
// only as a handle.
const api = "https://slack.com/api"
const bearer = token => secrets.text`Bearer ${secrets.get(token)}`

// How long one job listens, well within its time limit: then it ends, and
// the next job connects again.
const listenFor = 4 * 60 * 1000

// Slack takes up to 12,000 characters of Markdown in a message, alone or
// in a block beside buttons. A string's length here counts UTF-16 code
// units, never fewer than Slack counts.
const longest = 12000

// How much of the text a message with buttons gives in plain words, for
// where Slack shows no blocks, as a notification.
const fallbackFor = 150

// What Slack says when a reply cannot go under the message it answers,
// which it no longer has: the text is then said in the channel, not in
// reply.
const threadGone = ["thread_not_found", "cannot_reply_to_message"]

// What Slack says when it will not show a message's blocks: a message of
// charts or images is then said as it was written.
const blocksRefused = ["invalid_blocks", "invalid_blocks_format"]

// call calls a method of the Web API with a token, the bot's unless
// another is named, and gives Slack's answer, which says by ok whether it
// went and by error why not. No error made here holds the address called.
function call(method, body, query, token) {
  const options = { headers: { Authorization: bearer(token || "SLACK_TOKEN") }, as: "text", timeout: 30 }
  if (body) options.json = body
  if (query) options.query = query
  const reply = body ? http.post(api + "/" + method, options) : http.get(api + "/" + method, options)
  if (reply.status === 429) throw new Error("Slack answered " + method + " with 429, too many calls")
  try {
    return JSON.parse(reply.body)
  } catch (e) {
    throw new Error("Slack answered " + method + " with status " + reply.status + " and no answer it could read")
  }
}

// must gives Slack's answer to a call, and throws what Slack refuses.
function must(method, body, query, token) {
  const answer = call(method, body, query, token)
  if (!answer.ok) throw new Error("Slack refused " + method + ": " + answer.error)
  return answer
}

// stamp gives the time a Slack ts names, as 1700000001.000100.
const stamp = ts => new Date(Math.round(Number(ts) * 1000)).toISOString()

if (input.do === "hear") {
  // placeOf gives the place a channel is: direct for a person's
  // conversation with the app, or the channel's name as the settings write
  // it, as #help.
  const placeOf = channel => {
    const info = must("conversations.info", null, { channel: channel }).channel
    return info.is_im ? "direct" : "#" + info.name
  }

  // pressed gives a press of a button: its value is the out's ref and the
  // choice.
  const pressed = body => {
    const action = (body.actions || [])[0] || {}
    const value = String(action.value || "")
    const cut = value.indexOf(":")
    const container = body.container || {}
    const channel = body.channel ? body.channel.id : container.channel_id
    return {
      id: "press:" + body.user.id + ":" + action.action_ts, at: stamp(action.action_ts), kind: "press", from: body.user.id,
      place: placeOf(channel), where: channel + "/" + (container.thread_ts || container.message_ts),
      text: "", files: [], mentioned: false, answers: null,
      ref: cut < 0 ? value : value.slice(0, cut), choice: cut < 0 ? null : value.slice(cut + 1),
    }
  }

  // named gives a text with the app's own mention, <@its id>, written as
  // its name, @gromit, and whether it names the app; a text naming nobody
  // asks Slack nothing. Anyone else's mention stays as Slack gives it.
  const named = text => {
    if (!text.includes("<@")) return { text: text, mentioned: false }
    const me = must("auth.test")
    const written = text.replace(new RegExp("<@" + me.user_id + "(\\|[^>]*)?>", "g"), "@" + me.user)
    return { text: written, mentioned: written !== text }
  }

  // heard gives the thing an envelope holds: a press of a button, or a
  // message said. Left out is any other event, and what a bot said, the
  // app among them, each logged with its id, its channel and why, never
  // its words.
  const heard = envelope => {
    const body = envelope.payload || {}
    if (envelope.type === "interactive" && body.type === "block_actions") return pressed(body)
    const e = body.event || {}
    const said = !e.subtype || e.subtype === "file_share" || e.subtype === "thread_broadcast" || e.subtype === "me_message"
    let left = ""
    if (envelope.type !== "events_api" || body.type !== "event_callback" || e.type !== "message") left = e.type || body.type || envelope.type
    else if (!said) left = e.subtype
    else if (e.bot_id || !e.user) left = "a bot said it"
    if (left) {
      log("left out " + (body.event_id || envelope.envelope_id) + " in " + (e.channel || "no channel") + ": " + left)
      return null
    }
    const files = []
    for (const f of e.files || []) {
      const got = http.get(f.url_private, { headers: { Authorization: bearer("SLACK_TOKEN") }, as: "bytes", timeout: 60 })
      if (got.status !== 200) throw new Error("Slack answered status " + got.status + " for the file " + f.name)
      files.push({ name: f.name, data: got.body })
    }
    const text = named(e.text || "")
    // A message is said at its thread; one at the top of a channel, or of
    // a person's conversation with the app, begins its own.
    const reply = e.thread_ts && e.thread_ts !== e.ts
    return {
      id: e.channel + "/" + e.ts, at: stamp(e.ts), kind: "said", from: e.user,
      place: placeOf(e.channel), where: e.channel + "/" + (e.thread_ts || e.ts),
      text: text.text, files: files, mentioned: text.mentioned,
      answers: reply ? e.channel + "/" + e.thread_ts : null, ref: null, choice: null,
    }
  }

  // Slack gives the app-level token the address of a connection, good for
  // one, over which it sends each event and each press in an envelope.
  // Each is handed over as it is heard, and only then acknowledged: Slack
  // sends again one it is not told was received.
  const link = must("apps.connections.open", {}, null, "SLACK_APP_TOKEN")
  const socket = websocket.connect(link.url, { timeout: 30 })
  const until = Date.now() + listenFor
  for (let left = listenFor; left > 0; left = until - Date.now()) {
    let message
    try {
      message = socket.receive({ timeout: Math.ceil(left / 1000) })
    } catch (e) {
      if (e.code === "timeout") break
      throw e
    }
    if (message === null) break
    const envelope = JSON.parse(message)
    if (envelope.type === "disconnect") break
    if (!envelope.envelope_id) continue
    const thing = heard(envelope)
    if (thing) emit("heard", { things: [thing], cursor: null })
    socket.send(JSON.stringify({ envelope_id: envelope.envelope_id }))
  }
  socket.close()
  return { cursor: null }
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

// actionsOf gives the block of a button for each choice, each handing
// back the reference with the choice.
const actionsOf = choices => ({ type: "actions", elements: choices.map((c, n) => ({ type: "button", action_id: "choice-" + n, text: { type: "plain_text", text: c }, value: input.ref + ":" + c })) })

// fallbackOf gives what a message of blocks says in plain words, for
// where Slack shows no blocks, as a notification: the first line of its
// first block's Markdown, the title of its chart, or what its image shows.
function fallbackOf(block) {
  const words = block.type === "markdown" ? block.text.trim().split("\n")[0] : block.type === "image" ? block.alt_text : block.title
  return Array.from(words).slice(0, fallbackFor).join("")
}

// messagesOf gives the messages that say a text as Markdown, in as many as
// it takes, the last with a button for each choice beside it.
function messagesOf(text, choices) {
  const parts = split(text, longest)
  return parts.map((part, i) => ({
    message: i < parts.length - 1 || !choices.length ? { markdown_text: part }
      : { text: fallbackOf({ type: "markdown", text: part }), blocks: [{ type: "markdown", text: part }, actionsOf(choices)] },
  }))
}

// Slack shows a chart with a title of up to 50 characters, labels of up to
// 20 and axis names of up to 50; 1 to 12 series of 1 to 20 points each,
// or a pie of 1 to 12 slices; and up to two charts in a message, of up to
// 50 blocks.
const mostCharts = 2
const mostBlocks = 50

// chartOf gives the Slack chart that a chart fence's JSON describes, or
// nothing when it is not one Slack would show as written:
//   { "type": "bar", "title": "Refunds by week", "categories": ["W1", "W2"],
//     "series": [{ "name": "Refunds", "values": [4, 7] }], "x_label": "Week", "y_label": "Refunds" }
// type and title are needed; type is bar, line, area or pie; a pie has
// one series, of values above 0, one slice for each category.
function chartOf(source) {
  let c
  try {
    c = JSON.parse(source)
  } catch (e) {
    return null
  }
  const label = (s, most) => typeof s === "string" && s.trim().length > 0 && s.length <= most
  if (!c || typeof c !== "object" || !["bar", "line", "area", "pie"].includes(c.type) || !label(c.title, 50)) return null
  const categories = c.categories, series = c.series
  if (!Array.isArray(categories) || !categories.length || categories.length > 20 || !categories.every(x => label(x, 20)) ||
    new Set(categories).size !== categories.length) return null
  if (!Array.isArray(series) || !series.length || series.length > 12) return null
  const names = new Set()
  for (const s of series) {
    if (!s || !label(s.name, 20) || names.has(s.name) || !Array.isArray(s.values) || s.values.length !== categories.length ||
      !s.values.every(v => typeof v === "number" && isFinite(v))) return null
    names.add(s.name)
  }
  if (c.type === "pie") {
    if (series.length !== 1 || categories.length > 12 || !series[0].values.every(v => v > 0)) return null
    return { type: "data_visualization", title: c.title, chart: { type: "pie", segments: categories.map((l, i) => ({ label: l, value: series[0].values[i] })) } }
  }
  const axis = { categories: categories }
  for (const k of ["x_label", "y_label"]) {
    if (c[k] === undefined || c[k] === null || c[k] === "") continue
    if (!label(c[k], 50)) return null
    axis[k] = c[k]
  }
  return {
    type: "data_visualization", title: c.title,
    chart: { type: c.type, series: series.map(s => ({ name: s.name, data: s.values.map((v, i) => ({ label: categories[i], value: v })) })), axis_config: axis },
  }
}

// cut gives as much of a text as fits in most UTF-16 code units, as Slack
// counts, never cutting inside a character.
function cut(text, most) {
  let out = ""
  for (const c of text) {
    if (out.length + c.length > most) break
    out += c
  }
  return out
}

// pieces cuts a text into what Slack shows as blocks of its own, a chart
// fence it can show and an image alone on its line at an https address,
// and the Markdown between them, in order; each holds what was written
// for it, and joined again they are the text whole. Any other fence, and
// what is inside one, is Markdown.
function pieces(text) {
  const lines = text.match(/[^\n]*\n|[^\n]+$/g) || []
  const out = []
  let prose = ""
  const shown = (block, written) => {
    if (prose) out.push({ text: prose })
    prose = ""
    out.push({ block: block, text: written })
  }
  for (let i = 0; i < lines.length; i++) {
    const open = lines[i].match(/^ {0,3}(`{3,}(?![^\n]*`)|~{3,})[ \t]*([^`\s]*)/)
    if (open) {
      const close = new RegExp("^ {0,3}" + open[1][0] + "{" + open[1].length + ",}[ \\t]*\\r?\\n?$")
      let j = i + 1
      while (j < lines.length && !close.test(lines[j])) j++
      const written = lines.slice(i, j + 1).join("")
      const chart = open[2] === "chart" && j < lines.length ? chartOf(lines.slice(i + 1, j).join("")) : null
      if (chart) shown(chart, written)
      else prose += written
      i = j
      continue
    }
    const image = lines[i].match(/^ {0,3}!\[([^\]\n]*)\]\((https:\/\/[^\s)]+)\)[ \t]*\r?\n?$/)
    if (image && image[2].length <= 3000) shown({ type: "image", image_url: image[2], alt_text: cut(image[1].trim() || "an image", 2000) }, lines[i])
    else prose += lines[i]
  }
  if (prose) out.push({ text: prose })
  return out
}

// packed gives the messages that say a text holding charts or images: its
// pieces as blocks in order, as many to a message as Slack takes, the last
// with a button for each choice. Each keeps what was written for it, to
// be said as Markdown in its place if Slack will not show its blocks.
function packed(said, choices) {
  const messages = []
  let m
  const begin = () => {
    m = { blocks: [], written: "", markdown: 0, charts: 0 }
    messages.push(m)
  }
  const add = (block, written) => {
    const markdown = block.type === "markdown" ? block.text.length : 0
    const charts = block.type === "data_visualization" ? 1 : 0
    if (m.blocks.length && (m.markdown + markdown > longest || m.charts + charts > mostCharts || m.blocks.length >= mostBlocks - 1)) begin()
    m.blocks.push(block)
    m.written += written
    m.markdown += markdown
    m.charts += charts
  }
  begin()
  for (const p of said) {
    if (p.block) {
      add(p.block, p.text)
      continue
    }
    for (const part of split(p.text, longest)) {
      const shown = part.replace(/^\s*\n/, "").trimEnd()
      if (shown) add({ type: "markdown", text: shown }, part)
      else m.written += part
    }
  }
  if (choices.length) m.blocks.push(actionsOf(choices))
  return messages.map((x, i) => ({ message: { text: fallbackOf(x.blocks[0]), blocks: x.blocks }, written: x.written, last: i === messages.length - 1 }))
}

// say: to a person directly in their conversation with the app, or in a
// thread; the text in as many messages as it takes, as Markdown, which
// Slack shows, the last with a button for each choice beside it. A chart
// fence and an image alone on its line are shown as Slack's own chart and
// image, in blocks beside the Markdown, and said as written where Slack
// will not show them. What it sent is named by its last message, the one
// that holds the buttons.
let channel, thread
if (input.direct) {
  channel = must("conversations.open", { users: input.to }).channel.id
} else {
  [channel, thread] = String(input.to).split("/")
}

// post sends one message in the thread, or in the channel once Slack no
// longer has the thread, and gives Slack's answer.
const post = message => {
  const m = Object.assign({ channel: channel }, message)
  if (thread) m.thread_ts = thread
  let answer = call("chat.postMessage", m)
  if (!answer.ok && thread && threadGone.includes(answer.error)) {
    delete m.thread_ts
    thread = undefined
    answer = call("chat.postMessage", m)
  }
  return answer
}

const choices = input.ref && input.choices && input.choices.length ? input.choices : []
const said = pieces(input.text)
const messages = said.some(p => p.block) ? packed(said, choices) : messagesOf(input.text, choices)
if (!messages.length) throw new Error("there is nothing to say")
let sent
for (const m of messages) {
  let answer = post(m.message)
  if (!answer.ok && blocksRefused.includes(answer.error) && m.written !== undefined) {
    // Slack would not show a chart or an image as given, such as one at an
    // address it cannot fetch: what was written is said in its place.
    for (const again of messagesOf(m.written, m.last ? choices : [])) {
      answer = post(again.message)
      if (!answer.ok) break
    }
  }
  if (!answer.ok) throw new Error("Slack refused chat.postMessage: " + answer.error)
  sent = answer.channel + "/" + answer.ts
}
return { sent: sent }
