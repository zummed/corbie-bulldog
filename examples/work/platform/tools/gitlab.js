// gitlab and retry each make one call to GitLab's API, by its path.
const reply = http.request("https://gitlab.com/api/v4/" + input.path, {
  method: input.method,
  headers: { "PRIVATE-TOKEN": secrets.get("GITLAB_TOKEN") },
  as: "json",
})
if (reply.status >= 300) throw new Error("GitLab answered " + reply.status + ": " + JSON.stringify(reply.body))
return reply.body
