The office tools: read how the office is written, and draft a change to the files of the worker's own team and of the teams within it, each a run that waits on Bulldog.

Every tool reaches the files of the worker's own team's folder and of the teams within it, the top team's worker the whole office, and never Bulldog's own in `.bulldog/` or the office's repository. A worker has these tools only by naming their files, each for what it does:

- `tools_file: ./.bulldog/tools/office/read.yaml` gives `list_files`, `read_file`, `search_files` and `manual`. `list_files` lists every file the worker reaches and what its draft does to each; `read_file(path)` reads one, as the draft has it, or as it is; `search_files(words)` gives the lines of those files that hold the words as written, in any case, each with its file and line, up to 20,000 characters; `manual(page)` reads a page of the manual.
- `tools_file: ./.bulldog/tools/office/write.yaml` gives `write_file`, `remove_file`, `check_draft` and `put_draft`, which work on the worker's draft: `write_file(path, text)` puts the whole new text of a file into it, `remove_file(path)` has it remove one, `check_draft` reads the office as the draft would make it, with every problem, and `put_draft(why)` puts the draft, and Bulldog writes it once its job runs. A worker that drafts reads the office first, so it names `read.yaml` beside it.

`put_draft` is marked `approve: true`, so Corbie holds it, and it is decided by the innermost team whose folder holds every file it writes, by that team's `put_draft` list, or else its `everything`; a change that touches a team's `team.yaml` is decided by the team around it, and one to `office.yaml` by the top team. Whoever decides is shown every file whole beside what it replaces. A worker started at the top that drafts, or calls a worker that does, names the ask Bulldog ships. To have drafts written at once, copy `write.yaml` into the office without `approve`, and name the copy.

## Needs

Nothing: no secret and no host. Each tool runs `.bulldog/call.yaml`, which only waits on Bulldog's answer, for up to 7 days.

## Settings

None.

## Example

A worker that reads how the office is written:

```yaml
tools_file: [./.bulldog/tools/office/read.yaml]
```

One that also drafts changes to it:

```yaml
tools_file: [./.bulldog/tools/office/read.yaml, ./.bulldog/tools/office/write.yaml]
```

From a team within, the paths begin `../.bulldog/`.

## Changed

- 0.3.0: new, in place of `tools/build/build.yaml`, which is gone: a worker that named it names `read.yaml` and `write.yaml` here. The tools are renamed: `list_files` is `list_files`, `read_file` `read_file`, `write_file` `write_file`, `remove_file` `remove_file`, `check_draft` `check_draft` and `put_draft` `put_draft`, and a team's `put_draft` list is its `put_draft` list; `search_files` is new. The tools reach only the worker's own team's files and those within it, and `put_draft` is held for a yes by `approve: true`, decided by the team whose folder holds the change.
