// Google's sign-in script. It takes the code Google gave the person's
// browser to Google's token endpoint, with the client's secret, which it
// holds only as a handle, and the verifier of the challenge Bulldog sent.
// The ID token comes straight from Google over TLS, so its signature is
// not checked again: its claims are read, and checked.
const endpoint = "https://oauth2.googleapis.com/token"
const issuers = ["https://accounts.google.com", "accounts.google.com"]

// claimsOf gives what an ID token says, or null for one that cannot be
// read.
function claimsOf(token) {
  const parts = typeof token === "string" ? token.split(".") : []
  if (parts.length !== 3) return null
  try {
    return JSON.parse(crypto.decode(parts[1], { from: "base64url", as: "text" }))
  } catch (e) {
    return null
  }
}

const reply = http.post(endpoint, {
  form: {
    grant_type: "authorization_code", code: input.code, redirect_uri: input.redirect, client_id: input.client_id,
    client_secret: secrets.get("GOOGLE_CLIENT_SECRET"), code_verifier: input.verifier,
  },
  as: "text", timeout: 20,
})
// Google refuses a code that is used, too old or another client's, and a
// client it does not know, with a 4xx; anything else may pass, and fails
// the run.
if (reply.status >= 400 && reply.status < 500) {
  let said = ""
  try {
    const body = JSON.parse(reply.body)
    said = body.error_description || body.error || ""
  } catch (e) {}
  return { refused: "Google refused the sign-in" + (said ? ": " + said : ", answering " + reply.status) + "." }
}
if (reply.status !== 200) throw new Error("Google's token endpoint answered " + reply.status + ": " + String(reply.body).slice(0, 200))
const claims = claimsOf(JSON.parse(reply.body).id_token)
if (!claims) return { refused: "Google answered no ID token that can be read." }
if (claims.aud !== input.client_id) return { refused: "Google's ID token is for another client." }
if (!issuers.includes(claims.iss)) return { refused: "The ID token is not Google's." }
if (!(claims.exp * 1000 > Date.now())) return { refused: "Google's ID token has expired." }
if (claims.nonce !== input.nonce) return { refused: "Google's ID token is not for this sign-in." }
if (typeof claims.email !== "string" || claims.email === "") return { refused: "Google gave no address." }
if (claims.email_verified !== true) return { refused: "Google has not verified the address " + claims.email + "." }
return { identity: claims.email.toLowerCase() }
