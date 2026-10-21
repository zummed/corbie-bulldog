// The sign-in page: asks for a one-time link by name, and says what the
// server says, which is the same whatever the name; and, where anyone may
// use a password, signs in by name and password, going on to the screens
// once signed in, and saying what the server says when not.
document.getElementById("signin").addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = new FormData(e.target).get("name");
  const said = document.getElementById("said");
  const res = await fetch("/signin", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({name})});
  const out = await res.json();
  said.textContent = out.said || out.error;
});

const password = document.getElementById("password");
if (password) password.addEventListener("submit", async (e) => {
  e.preventDefault();
  const f = new FormData(e.target);
  const said = document.getElementById("password-said");
  const res = await fetch("/signin/password", {method: "POST", headers: {"Content-Type": "application/json"},
    body: JSON.stringify({name: f.get("name"), password: f.get("password")})});
  const out = await res.json();
  if (res.ok) {
    location.replace("/");
    return;
  }
  said.textContent = out.error;
});
