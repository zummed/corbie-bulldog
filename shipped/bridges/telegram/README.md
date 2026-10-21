Telegram: it listens for what is said to the bot, and carries questions and decisions to a person directly, with a button for each choice.

The in machine asks Telegram for what is new since its cursor, waiting up to fifty seconds while nothing is. Its first run hears nothing and marks where Telegram stands. A chat holds no threads, only replies, so everything said is threadless, and a reply to any message of an answer goes on from it.

## Needs

- `TELEGRAM_TOKEN`, the bot's token from @BotFather, for `api.telegram.org`.

## Settings

- `from`: `./.bulldog/bridges/telegram`, the folder it runs, as Bulldog ships it.
- `places`: `direct`, anything said to the bot in a person's own chat with it, and each group by its chat id, with the `team` asked there, the office when none is named, and whether `guests` may ask.

## Leaves out

- What the bot said, and what any other bot said.
- Every kind of update but a message and a press of a button.

Each is logged in the in run's record on Corbie, with its id, its place and why, and never its words.

## Example

```yaml
# in office.yaml
bridges:
  telegram: { from: ./.bulldog/bridges/telegram, places: { direct: {} } }
```

## Changed

- 0.1.0: first shipped.
- 0.2.0: run in place, by `from` in its entry, and no longer copied into the office; logs what it leaves out; takes no setting but those above, each checked when the office is read.
- 0.5.0: a question that offers choices is sent with a button for each, as a decision is, in place of asking for a reply, and a press answers it; more than three choices, or long ones, are each on a row of their own.
