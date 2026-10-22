# The house

Sam and Kit's office. They ask the house in a direct message to its Telegram bot.

- The steward leads the house. It hands repairs to the fixer, which asks sparks about the wiring.
- The fixer's `cut_power` and `order_part` wait for the house's `decide`. `small-orders.js` says yes at £20 or less, the clerk says no to a repair over £100, and Sam decides the rest.
- The garden is a team within the house, with its own model. A doc the gardener writes waits for Sam.
- On Monday at 09:00 the steward is asked what is still open, and on Friday at 16:00 the gardener what needs doing before the weekend.

## Running it

`mise run demo` serves the house on a stand-in model, with nothing to set up.

To serve it on the Corbie set up on this computer (`corbie manual one-computer`), write Sam's and Kit's Telegram user ids in `office.yaml`, then:

```
corbie controller secret set ANTHROPIC_KEY
corbie controller secret set TELEGRAM_TOKEN
corbie bulldog examples/house
```

`corbie bulldog key examples/house sam` prints Sam's key link, which signs Sam in on the screens.
