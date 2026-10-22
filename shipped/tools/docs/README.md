Docs tools: the tools a worker uses to read and write its team's docs.

A team's docs are values kept in Corbie's store, at `<office id>/<team>/docs/<name>`. Bulldog gives every worker the place of its team's docs as `request.docs` in its input.

| Tool | What it does |
|---|---|
| `read_doc(name)` | reads one doc, `{ name, text }`; with no name, lists them, `{ names }` |
| `write_doc(name, text)` | writes one doc, whole, in place of what it held, `{ name }` |

- `read_doc` is allowed `kept.read` only, and `write_doc` `kept.write` only, each under `request.docs`.
- `write_doc` is marked for approval. The team's `decide` is shown the new text, and the doc is written once it says yes.
- A doc's name is one part of a kept path: small letters a to z, digits, `.`, `-` and `_`, not starting with a dot, such as `refunds.md`.
- Every request runs with the team's start list, which allows kept values only under the team's own path. A worker given another team's place is refused.

## Needs

Nothing: no secret and no host.

## Settings

None.

## Example

```yaml
# payments/ledger.yaml
agent: ledger
model: usual
prompt: ${input.said}
tools_file: [./.bulldog/tools/docs/docs.yaml]
```

The ledger calls `read_doc({ name: refunds.md })` and is given `{ name: refunds.md, text: "Refunds within 30 days." }`.
