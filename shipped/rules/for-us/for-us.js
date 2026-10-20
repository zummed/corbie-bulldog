// for-us's script. It acts at once when anything it weighs is addressed
// to the lead, as the mentioned heed does; otherwise it asks a System One
// model whether the newest message is for the team, and acts when the
// model is at least as sure as the threshold. Its settings come in with
// their defaults filled in by the definition.
const site = "https://api.typesafe.ai"
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

// The model is given the team, the thread shown before, and what is new,
// each message with when it was said and who said it.
const team = request.team
const lines = ["The team: " + team.called + (team.about ? ", " + team.about : ""), ""]
if (shown.length) {
  lines.push("Said before:")
  for (const e of shown) lines.push(e.by.called + ": " + e.text)
  lines.push("")
}
lines.push("The newest:", input.said)
const reply = llm.decide(lines.join("\n"), {
  model: input.model, address: site, key: secrets.get("TYPESAFE_KEY"),
  questions: { for_us: { type: "noul", instructions: input.question } },
})
const sure = reply.answers.for_us && reply.answers.for_us.noul
if (typeof sure !== "number") throw new Error("the model's answer holds no probability: " + JSON.stringify(reply.answers))
const said = "(" + sure.toFixed(2) + ")"
if (sure >= input.threshold) return { says: "act", note: "For " + team.called + " " + said }
return { says: "note", note: "Not for " + team.called + " " + said }
