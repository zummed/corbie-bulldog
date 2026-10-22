// charges finds charges with Stripe's search, and refund refunds one.
const key = { Authorization: secrets.text`Bearer ${secrets.get("STRIPE_KEY")}` }
const reply = input.do === "refund"
  ? http.post("https://api.stripe.com/v1/refunds", { headers: key, form: { charge: input.charge }, as: "json" })
  : http.get("https://api.stripe.com/v1/charges/search", { headers: key, query: { query: input.query }, as: "json" })
if (reply.status !== 200) throw new Error("Stripe answered " + reply.status + ": " + reply.body.error.message)
return input.do === "refund" ? reply.body : reply.body.data
