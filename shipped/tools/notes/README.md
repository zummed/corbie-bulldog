The notes tool: write one of the team's notes, a run that waits on Bulldog.

`write_notes(name, text, why)` writes the note whole, in place of what it held, as a commit that says who wrote it and why. It is not a decided change. Notes are not private: the builder reads them, and so does whoever decides a change. A worker has it only by naming this file; naming the talk tools gives no notes tool, and every run of a team's worker is given the team's notes all the same.

## Needs

Nothing: no secret and no host. The tool runs `.bulldog/call.yaml`, which only waits on Bulldog's answer, for up to 7 days.

## Settings

None.

## Example

```yaml
tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/notes/notes.yaml]
```

From a team within, the paths begin `../.bulldog/`.

## Changed

- 0.2.0: moved here from `.bulldog/notes.yaml`.
