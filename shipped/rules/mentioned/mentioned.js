// A heed rule with no model. It says yes when the message names the
// bridge's bot, or when the latest earlier request of its thread was asked
// by the same sender and has been answered; otherwise no.
const request = input.request
if (request.mentioned) return { says: "yes", note: "The message names the bot." }
const thread = request.thread || []
const latest = thread[thread.length - 1]
if (latest && latest.answer && JSON.stringify(latest.by) === JSON.stringify(request.by)) {
  return { says: "yes", note: "The message follows the answer to its sender." }
}
return { says: "no", note: "The message does not name the bot, and follows no answer to its sender." }
