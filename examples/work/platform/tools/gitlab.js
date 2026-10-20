// Reads one path of GitLab's API. A reply that is not a 2xx is thrown with
// its status and the start of its body, so the reason GitLab gave reaches
// the team.
const token = secrets.get("GITLAB_TOKEN")
const reply = http.get("https://gitlab.example.com/api/v4/" + input.path, { headers: { "PRIVATE-TOKEN": token }, as: "text" })
if (reply.status < 200 || reply.status > 299) {
  throw new Error("GitLab answered " + reply.status + ": " + reply.body.slice(0, 200))
}
return JSON.parse(reply.body)
