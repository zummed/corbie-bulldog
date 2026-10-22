// A heed rule with no model. It says yes when the message names the
// bridge's bot, or when the latest earlier request of its thread was asked
// by the same sender and has been answered; otherwise no.
const request = input.request
if (request.mentioned) return { says: "yes", note: "The message names the bot." }
const latest = (request.thread || []).reduce((a, b) => (a && Date.parse(a.at) > Date.parse(b.at) ? a : b), null)
if (latest && latest.answer && JSON.stringify(latest.by) === JSON.stringify(request.by)) {
  return { says: "yes", note: "The message follows the answer to its sender." }
}
return { says: "no", note: "The message does not name the bot, and follows no answer to its sender." }
