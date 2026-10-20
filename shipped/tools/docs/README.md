The docs tools: list, read and search the docs of the team and of each team around it, and mend the team's own, each a run that waits on Bulldog.

A team's docs are the Markdown files under its `docs/` folder, subfolders included, each named by its path there without `.md`, as `refunds/policy`. They are written through a change, like any file of the office, or by `write_doc`, so a worker that finds one wrong or out of date proposes the fix; no run is given them unasked. A worker has these tools only by naming their files, each for what it does:

- `tools_file: ./.bulldog/tools/docs/read.yaml` gives `list_docs`, `read_doc` and `search_docs`. `list_docs` lists the docs a worker may read, each with its team, its size in characters and its first heading. `read_doc(name, from)` reads one by its name from a line on, and `search_docs(words)` gives the lines that hold the words as written, in any case, each with its doc, line and team; each gives up to 20,000 characters. A worker reads its own team's docs, then those of each team around it, out to the office, as a model named by one word is looked up: the nearer team's doc of a name stands before one further out, and each result names its team. Only docs are read: a name is a path in a `docs/` folder, never above it.
- `tools_file: ./.bulldog/tools/docs/write.yaml` gives `write_doc(name, text, why)`, which writes one doc of the worker's own team whole. A worker that writes docs reads them first, so it names `read.yaml` beside it.

`write_doc` is marked `approve: true`, so Corbie holds it, and it is decided by the team's `write_doc` list, or else its `everything`; its deciders are shown the doc beside what it replaces, and it is written once they say yes. A worker started at the top that holds it names the ask Bulldog ships. To have docs written at once, copy `write.yaml` into the office without `approve`, and name the copy.

## Needs

Nothing: no secret and no host. Each tool runs `.bulldog/call.yaml`, which only waits on Bulldog's answer, for up to 7 days.

## Settings

None.

## Example

A worker that reads the docs:

```yaml
tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/read.yaml]
```

One that also mends its team's:

```yaml
tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/read.yaml, ./.bulldog/tools/docs/write.yaml]
```

From a team within, the paths begin `../.bulldog/`.

## Changed

- 0.2.0: new.
- 0.3.0: `write_doc`, held for a yes. The tools are in two files, `read.yaml` and `write.yaml`, in place of `docs.yaml`: a worker that named `docs.yaml` names `read.yaml`, and `write.yaml` beside it to write; `docs` is now `list_docs`.
