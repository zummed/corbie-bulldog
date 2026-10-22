For us: a heed rule that asks a fast model whether a message is for the team.

1. When `./.bulldog/rules/mentioned/mentioned.js` says yes, it says yes, and asks no model.
2. Otherwise it asks a System One model how sure it is that the message is for the team, given the thread before it.
3. It says yes when the model is at least as sure as `threshold`, and no otherwise. The note gives the team and how sure the model was, as "Not for payments (0.12)."
4. When the model has not answered within `timeout`, it says yes.

It never says `pass`. It is given `request` and `said`, the message weighed.

## Needs

- `TYPESAFE_KEY`, a key of TypeSafe's, for `api.typesafe.ai`.

## Settings

| Setting | Default | What it is |
|---|---|---|
| `timeout` | `10s` | how long the model may take; past it, the message comes in |
| `threshold` | `0.5` | how sure the model must be that the message is for the team |
| `model` | `jev-latest` | the System One model asked |
| `question` | `The newest message asks this team for something, or answers what it last said.` | the question put to the model |

The settings are the defaults in `for-us.yaml`'s input. To change them, copy the directories `rules/for-us` and `rules/mentioned` into the office as `./rules/`, change the defaults in the copy, and name `./rules/for-us/for-us.yaml`.

## Example

```yaml
# office.yaml, in the work office
heed: [./.bulldog/rules/for-us/for-us.yaml]
```

In #help, "Was order 4411 refunded?" comes in, with the note "For office (0.93)."
