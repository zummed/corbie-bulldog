For us: a heed rule that asks a fast model whether a message is for the team.

1. When `./.bulldog/rules/mentioned/mentioned.js` says yes, it says yes, and asks no model.
2. Otherwise it asks System One's `jev-latest` how sure it is that the message is for the team, given the thread before it: "The newest message asks this team for something, or answers what it last said."
3. It says yes when the model is at least 0.5 sure, and no otherwise. The note gives the team and how sure the model was, as "Not for payments (0.12)."
4. When the model has not answered within 10 seconds, it says yes.

It never says `pass`. It is given `request` and `said`, the message weighed.

## Needs

- `TYPESAFE_KEY`, a key of TypeSafe's, for `api.typesafe.ai`.

## Settings

None.

## Example

```yaml
# office.yaml, in the work office
heed: [./.bulldog/rules/for-us/for-us.yaml]
```

In #help, "Was order 4411 refunded?" comes in, with the note "For office (0.93)."
