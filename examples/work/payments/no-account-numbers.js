// Payments' check: an account number never leaves payments, and Lee or Priya decide on any other long number.
const text = input.answer ?? input.question.said
if (/\b\d{8}\b/.test(text)) return { says: "no", note: "It holds an account number." }
if (/\d{6,}/.test(text)) return { says: "pass", note: "It holds a long number." }
return { says: "yes", note: "No account numbers." }
