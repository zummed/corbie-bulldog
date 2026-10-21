The notes tools: the team's notes, given to each run at its start, and `write_note`, a run that waits on Bulldog.

A worker that names this file is given its team's notes at the start of each run; one that names it not is given none. `write_note(name, text, why)` writes the note whole, in place of what it held, as a commit that says who wrote it and why, at once: it is not marked `approve`. To have notes decided, copy this file into the office with `approve: true` on its job, and name the copy; the note is then held, and decided by the team's `write_note` list, or else its `everything`. Notes are not private: a worker that reads the office's files with `tools/office/read.yaml`, of the team or one around it, reads them, and so does whoever decides a change. Naming the talk tools gives no notes.

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
- 0.3.0: a worker is given its team's notes only when it names this file. `write_notes` is now `write_note`, and a team's `write_notes` list is its `write_note` list.
