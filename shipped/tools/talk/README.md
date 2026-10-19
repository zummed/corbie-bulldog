The talk tools: ask another team, ask a person, tell someone, and read the thread, each a run that waits on Bulldog.

`ask_team` asks another team of the office, as a sub-request, and gives its answer. `ask_person` asks a person of the worker's team, or whoever asked, and gives what they say. `tell` says something to whoever asked, or to a person of the team, and goes on. `read_thread` reads the request's thread from an entry on, each noted message marked, or, with `place`, what was said before it where it was asked, in the team's requests there that everyone who reads this one reads, newest first. A worker that asks or tells anyone outside its own colleagues names this file.

## Needs

Nothing: no secret and no host. Each tool runs `.bulldog/call.yaml`, which only waits on Bulldog's answer, for up to 7 days.

## Settings

None.

## Example

```yaml
tools_file: [./.bulldog/tools/talk/talk.yaml]
```

From a team within, the path begins `../.bulldog/`.

## Changed

- 0.2.0: moved here from `.bulldog/talk.yaml`.
- 0.2.0: `read_thread`.
