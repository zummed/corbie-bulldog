// A team's heed, a bare script that asks no model: it acts when anything
// it weighs is addressed to the lead, and otherwise notes it. A message is
// addressed when its bridge marked it as naming the bot, or when it
// follows straight on the lead's answer and is said by whoever that
// answer went to.
const request = input.request
const shown = request.entries || []

// Who someone is, alike however the office came to name them: a person
// by their name, a guest by the identity their bridge vouched for.
const who = (w) => w.name ? w.kind + ":" + w.name : w.kind + ":" + w.bridge + ":" + w.id

// The entry before the first new one, unless the thread shown leaves out
// what came between them.
let before = shown.length > 1 || !request.entries_not_shown ? shown[shown.length - 1] : undefined
for (const e of request.new || []) {
  if (e.mentioned) return { says: "act", note: "Names " + request.worker + "." }
  if (before && before.kind === "answer" && (before.to || []).some((to) => who(to) === who(e.by))) {
    return { says: "act", note: "Follows " + request.worker + "'s answer." }
  }
  before = e
}
return { says: "note", note: "Not addressed to " + request.worker + "." }
