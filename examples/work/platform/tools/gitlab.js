// Reads one path of GitLab's API.
const token = secrets.get("GITLAB_TOKEN")
const reply = http.get("https://gitlab.example.com/api/v4/" + input.path, { headers: { "PRIVATE-TOKEN": token }, as: "json" })
return reply.body
