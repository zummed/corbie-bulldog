// Asks a System One model how sure it is that the newest message is for
// the team, given the thread before it, and says yes when the model is at
// least as sure as the threshold.
const model = "jev-latest"
const question = "The newest message asks this team for something, or answers what it last said."
const threshold = 0.5
const request = input.request
const who = by => by.person || by.team || (by.guest ? "a guest" : "the routine")
const lines = ["The team: " + request.team, ""]
const thread = request.thread || []
if (thread.length) {
  lines.push("Said before:")
  for (const r of thread) {
    lines.push(who(r.by) + ": " + r.said)
    if (r.answer) lines.push(request.team + ": " + r.answer)
  }
  lines.push("")
}
lines.push("The newest, from " + who(request.by) + ":", input.said)
const reply = llm.decide(lines.join("\n"), {
  model: model, address: "https://api.typesafe.ai", key: secrets.get("TYPESAFE_KEY"),
  questions: { for_us: { type: "noul", instructions: question } },
})
const sure = reply.answers.for_us && reply.answers.for_us.noul
if (typeof sure !== "number") throw new Error("the model's answer holds no probability: " + JSON.stringify(reply.answers))
const shown = "(" + sure.toFixed(2) + ")"
if (sure >= threshold) return { says: "yes", note: "For " + request.team + " " + shown + "." }
return { says: "no", note: "Not for " + request.team + " " + shown + "." }
