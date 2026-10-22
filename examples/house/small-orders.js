// small-orders.js: the held jobs are Corbie's: name, input, and for an agent's tool call, asked.args
const pounds = input.jobs.map(j => (j.asked?.args ?? j.input).pounds ?? Infinity)
if (pounds.every(p => p <= 20)) return { says: "yes", note: "Under £20." }
return { says: "pass", note: "More than I order alone." }
