The build kit: read how the office is written, and draft a change to the files of the worker's own team and of the teams within it.

`office_files`, `office_read`, `office_write`, `office_remove` and `office_check` reach the files of the worker's own team's folder and of the teams within it, the top team's worker the whole office, and work on the worker's draft; `office_put` puts the draft, and Bulldog writes it once its job runs; `manual` reads a page of the manual. A worker is a builder by naming this file, whatever it is called.

`office_put` is marked `approve: true`, so Corbie holds it, and it is decided by the innermost team whose folder holds every file it writes, by that team's `office_put` list, or else its `everything`; a change that touches a team's `team.yaml` is decided by the team around it, and one to `office.yaml` by the top team. Whoever decides is shown every file whole beside what it replaces. A worker started at the top that calls a builder, or is one, names the ask Bulldog ships. To have drafts written at once, copy this file into the office without `approve`, and name the copy.

## Needs

Nothing: no secret and no host. Each tool runs `.bulldog/call.yaml`, which only waits on Bulldog's answer, for up to 7 days.

## Settings

None.

## Example

```yaml
tools_file: [./.bulldog/tools/build/build.yaml]
```

## Changed

- 0.2.0: moved here from `.bulldog/build.yaml`.
- 0.3.0: the tools reach only the worker's own team's files and those within it; `office_put` is held for a yes by `approve: true`, and decided by the team whose folder holds the change, in place of a decision of its own that only a person approved.
