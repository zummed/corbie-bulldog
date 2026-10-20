Mentioned: a team's heed that acts on what names the bot or follows straight on its answer, and notes the rest, with no model.

It acts when a message it weighs is marked as naming the bot, as the shipped Slack bridge marks one, or is said by whoever the lead answered, straight after that answer, "Follows <lead>'s answer.", or while the lead was answering them, "Said while <lead> was answering.". Anything else it notes, with "Not addressed to <lead>.". It acts if any one message it weighs is addressed.

## Needs

Nothing: no secret and no host. It is a bare script, which runs with no permission.

## Settings

None.

## Example

```yaml
# in a team file
heed: ./.bulldog/rules/mentioned/mentioned.js
```

From a team within, the path begins `../.bulldog/`.

## Changed

- 0.3.0: weighs messages in the order they were said, so one said while the lead was answering no longer stands between its answer and what follows it, and one by whoever it answered is acted on as said while it was answering.
- 0.2.0: first shipped.
