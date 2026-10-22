Mentioned: a heed rule that lets in a message that names the bot or follows the bot's answer to its sender.

It is a script, and asks no model.

| It says | When |
|---|---|
| `yes` | the message names the bridge's bot, `request.mentioned` |
| `yes` | the latest earlier request of the thread was asked by the same sender, and has an answer |
| `no` | otherwise |

It never says `pass`.

## Needs

Nothing: no secret and no host. It runs with no permissions.

## Settings

None.

## Example

```yaml
# payments/team.yaml
heed: [./.bulldog/rules/mentioned/mentioned.js]
```

In #refunds, Dana's "@bot was order 4411 refunded?" comes in, with the note "The message names the bot." Her "thanks!" after the answer comes in too. Raj's "lunch?" does not.
