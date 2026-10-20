The docs tools: list, read and search the docs of the team and of each team around it, each a run that waits on Bulldog.

A team's docs are the Markdown files under its `docs/` folder, subfolders included, each named by its path there without `.md`, as `refunds/policy`. They are written through a change, like any file of the office; no tool writes one as `write_notes` writes a note, and no run is given them unasked. `docs` lists the docs a worker may read, each with its team, its size in characters and its first heading. `read_doc(name, from)` reads one by its name from a line on, and `search_docs(words)` gives the lines that hold the words as written, in any case, each with its doc, line and team; each gives up to 20,000 characters. A worker reads its own team's docs, then those of each team around it, out to the office, as a model named by one word is looked up: the nearer team's doc of a name stands before one further out, and each result names its team. Only docs are read: a name is a path in a `docs/` folder, never above it. A worker has the tools only by naming this file.

## Needs

Nothing: no secret and no host. Each tool runs `.bulldog/call.yaml`, which only waits on Bulldog's answer, for up to 7 days.

## Settings

None.

## Example

```yaml
tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/docs.yaml]
```

From a team within, the paths begin `../.bulldog/`.

## Changed

- 0.2.0: new.
