Docs: the run that lists, reads and writes one team's docs for the screens.

A team's docs are values kept in Corbie's store, at `<office id>/<team>/docs/<name>`. A doc's name is one part of a kept path: small letters a to z, digits, `.`, `-` and `_`, not starting with a dot, such as `refunds.md`.

When a person of a team opens or saves one of its docs on the screens, Bulldog does it through a run of `docs.yaml`, so it holds no key to kept values. The run is started with the team's start list, which allows kept values only under the team's own path.

| `do` | What the run gives |
|---|---|
| `list` | `{ names }`: every doc's name, in order |
| `read` | `{ name, text }`; `text` is `null` when there is no such doc |
| `write` | `{ name }`, once `text` is kept as the doc, whole |

`docs.js` does the work. The docs tools run the same script (`./.bulldog/tools/docs`).

## Needs

Nothing: no secret and no host.

## Settings

Its input:

| Input | What it is |
|---|---|
| `place` | the kept path of the team's docs, `<office id>/<team>/docs` |
| `do` | `list`, `read` or `write` |
| `name` | the doc's name, for `read` and `write` |
| `text` | the doc's text, for `write` |

## Example

Lee saves `refunds.md` on payments' page. Bulldog starts `docs.yaml` with:

```yaml
{ place: 1f3a9c0b7d2e4a68/payments/docs, do: write, name: refunds.md, text: "Refunds within 30 days." }
```

and the run ends with `{ name: refunds.md }`.
