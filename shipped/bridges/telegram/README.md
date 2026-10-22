Telegram: a bridge that hears what is said to the bot on Telegram, and sends answers and questions there.

The in run asks Telegram for what is new, waiting up to 50 seconds at a time while nothing is. A new bridge hears nothing said before it started. A chat holds no threads, so every message at one chat, or at one topic of a group, has the same `where`, and a reply is how a person answers a message.

The out sends one message. A question with choices has a button for each, and a question without asks for a reply. A message to a person directly goes to their chat with the bot.

| Thing | Telegram's |
|---|---|
| `id` | `<chat>:<message>`, or `press:<id>` for a press |
| `from` | the sender's user id, which never changes |
| `place` | `direct` for a person's chat with the bot, or a group's chat id |
| `where` | the chat id, or `<chat>/<topic>` for a topic of a group |
| `mentioned` | the message has an @mention of the bot |
| `answers` | `<chat>:<message>` of the message it replies to |

## Needs

- `TELEGRAM_TOKEN`, the bot's token from @BotFather, for `api.telegram.org`.

## Settings

| Setting | What it is |
|---|---|
| `places` | `direct`, any person's chat with the bot, and each group by its chat id, each with the `team` asked there, the office without one, and whether `guests` may ask |

## Leaves out

- What a bot said, the bot itself among them.
- Every kind of update but a message and a press of a button.
- A file over 16 MiB, or one Telegram will not give; the message is heard with a line naming the file.

Each is logged in the in run's record, with its id and why, never its words.

## Example

```yaml
# office.yaml
bridges:
  telegram:
    from: ./.bulldog/bridges/telegram
    places:
      direct: {}
      "-1001234": { team: garden, guests: true }
```
