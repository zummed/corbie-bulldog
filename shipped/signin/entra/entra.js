// Microsoft Entra ID's sign-in script. It takes the code the directory
// gave the person's browser to the directory's token endpoint, with the
// client's secret, which it holds only as a handle, and the verifier of
// the challenge Bulldog sent. The ID token comes straight from Microsoft
// over TLS, so its signature is not checked again: its claims are read,
// and checked.
const endpoint = "https://login.microsoftonline.com/" + input.tenant + "/oauth2/v2.0/token"
const site = "login.microsoftonline.com"

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
    client_secret: secrets.get("ENTRA_CLIENT_SECRET"), code_verifier: input.verifier,
  },
  as: "text", timeout: 20,
})
// The directory refuses a code that is used, too old or another app's,
// and a secret that has expired, with a 4xx; anything else may pass, and
// fails the run.
if (reply.status >= 400 && reply.status < 500) {
  let said = ""
  try {
    const body = JSON.parse(reply.body)
    said = (body.error_description || body.error || "").split("\n")[0]
  } catch (e) {}
  return { refused: "Microsoft refused the sign-in" + (said ? ": " + said : ", answering " + reply.status) + "." }
}
if (reply.status !== 200) throw new Error("Microsoft's token endpoint answered " + reply.status + ": " + String(reply.body).slice(0, 200))
const claims = claimsOf(JSON.parse(reply.body).id_token)
if (!claims) return { refused: "Microsoft answered no ID token that can be read." }
if (claims.aud !== input.client_id) return { refused: "Microsoft's ID token is for another app." }
if (claims.tid !== input.tenant) return { refused: "That account is of another directory than the office's." }
if (claims.iss !== "https://" + site + "/" + claims.tid + "/v2.0") return { refused: "The ID token is not Microsoft's." }
if (!(claims.exp * 1000 > Date.now())) return { refused: "Microsoft's ID token has expired." }
if (claims.nonce !== input.nonce) return { refused: "Microsoft's ID token is not for this sign-in." }
// The sign-in name alone: an account's email is not checked by the
// directory, and a token asked for with the profile scope carries the name.
const name = claims.preferred_username
if (typeof name !== "string" || name === "") return { refused: "Microsoft gave no sign-in name for the account." }
return { identity: name.toLowerCase() }
