// A team's heed, a bare script that asks no model: it acts when anything
// it weighs is addressed to the lead, and otherwise notes it. A message is
// addressed when its bridge marked it as naming the bot, or when it is
// said by whoever the lead answered, straight after that answer or while
// the lead was answering them.
const request = input.request
const shown = request.entries || []

// Who someone is, alike however the office came to name them: a person
// by their name, a guest by the identity their bridge vouched for.
const who = (w) => w.name ? w.kind + ":" + w.name : w.kind + ":" + w.bridge + ":" + w.id

// Whether an answer went to whoever said something.
const answers = (a, e) => a && a.kind === "answer" && (a.to || []).some((to) => who(to) === who(e.by))

// The thread in the order it was said: something new may have been said
// while the lead was answering, before its answer.
const fresh = request.new || []
const thread = shown.concat(fresh).sort((a, b) => a.id - b.id)
for (const e of fresh) {
  if (e.mentioned) return { says: "act", note: "Names " + request.worker + "." }
  // The entry before it, unless the thread shown leaves out what came
  // between them.
  const i = thread.indexOf(e)
  const before = i > 1 || !request.entries_not_shown ? thread[i - 1] : undefined
  if (answers(before, e)) return { says: "act", note: "Follows " + request.worker + "'s answer." }
  if (answers(thread.slice(i + 1).find((a) => a.kind === "answer"), e)) {
    return { says: "act", note: "Said while " + request.worker + " was answering." }
  }
}
return { says: "note", note: "Not addressed to " + request.worker + "." }
