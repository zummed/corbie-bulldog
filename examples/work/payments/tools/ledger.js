// Speaks to the ledger's API: the charges of a customer, or a refund.
const key = secrets.get("LEDGER_KEY")
const headers = { Authorization: secrets.text`Bearer ${key}` }
if (input.do === "charges") {
  return http.get("https://ledger.example.com/customers/" + input.customer + "/charges", { headers: headers, as: "json" }).body
}
return http.post("https://ledger.example.com/charges/" + input.charge + "/refund", { headers: headers, json: { why: input.why }, as: "json" }).body
