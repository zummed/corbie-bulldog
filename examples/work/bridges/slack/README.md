Slack: Slack calls the office with what is said and each press of a button; it answers in the thread, and carries questions and decisions to a person directly.

The in machine hears: Slack calls the office at `/bridges/slack`, for its Event Subscriptions and its Interactivity alike, and only what Slack signed in the last five minutes is taken. A file said in Slack is read; a file in an answer is named, not sent.

## Needs

- `SLACK_TOKEN`, the app's bot token, for `slack.com` and `files.slack.com`. The app needs the `im:history` scope to read what a person types in their conversation with it.
- `SLACK_SIGNING_SECRET`, the app's signing secret, which checks each delivery and reaches no host.

## Settings

- `places`: `direct`, a person's conversation with the app, and each channel by its name, with the `team` asked there, the office when none is named, and whether `guests` may ask.

## Leaves out

- What the app said, and what any other bot said.
- Edits, deletions, joins, and every event that is not a message.
- A delivery Slack did not sign, or signed more than five minutes before.

## Example

```yaml
# in office.yaml
bridges:
  slack:
    places:
      direct: {}
      "#help": { guests: true }
```

## Changed

- 0.1.0: first shipped.
