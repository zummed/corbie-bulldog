For us: a team's heed that acts on what names the bot or follows straight on its answer, and asks a fast model of the rest.

It acts at once when a message it weighs is addressed, as the mentioned heed does. Otherwise it asks a System One model whether the newest message is for the team, given the team, the thread before it and what is new, and acts when the model is at least as sure as `threshold`, or notes "Not for <team> (0.12)", with how sure it was. Past `timeout` it acts, with the note "timed out", whatever it was doing.

## Needs

- `TYPESAFE_KEY`, a key of TypeSafe's, for `api.typesafe.ai`.

## Settings

- `timeout`: `10s`. How long it may take; past it, the lead starts. It is the one limit on a heed, and is kept by Corbie's controller, so it holds when a runner is gone.
- `threshold`: `0.5`. How sure the model must be that the message is for the team.
- `model`: `jev-latest`. The fast decision model asked.
- `question`: `The newest message asks this team for something, or answers what it last said.` The question put to the model.

## Example

```yaml
# in a team file
heed: ./.bulldog/rules/for-us/for-us.yaml
# or, with settings
heed: { rule: ./.bulldog/rules/for-us/for-us.yaml, timeout: 5s, threshold: 0.6 }
```

From a team within, the path begins `../.bulldog/`.

## Changed

- 0.2.0: first shipped.
