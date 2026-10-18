// The sign-in page: asks for a one-time link by name, and says what the
// server says, which is the same whatever the name.
document.getElementById("signin").addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = new FormData(e.target).get("name");
  const said = document.getElementById("said");
  const res = await fetch("/signin", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({name})});
  const out = await res.json();
  said.textContent = out.said || out.error;
});
