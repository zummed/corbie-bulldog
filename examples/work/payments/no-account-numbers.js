// Says yes to what payments would say outside itself when the screening
// service finds no account or card number in it; anything that may hold
// one, it passes to Lee or Priya.
const said = typeof input.text === "string" ? input.text : JSON.stringify(input)
const key = secrets.get("SCREEN_KEY")
const found = http.post("https://screen.example.com/v1/screen", {
  headers: { Authorization: secrets.text`Bearer ${key}` },
  json: { text: said, find: "a customer's account number or card number" }, as: "json",
}).body.likelihood
if (found < 0.05) return { says: "yes", note: "No account number." }
return { says: "pass", note: "It may name an account number, " + found.toFixed(2) + "." }
