Slack: Slack calls the office with what is said and each press of a button; it answers in the thread, and carries questions and decisions to a person directly.

The in machine hears: Slack calls the office at `/bridges/slack`, for its Event Subscriptions and its Interactivity alike, and only what Slack signed in the last five minutes is taken. A file said in Slack is read; a file in an answer is named, not sent. A message that names the app is marked as mentioned, for a team's heed.

## Needs

- `SLACK_TOKEN`, the app's bot token, for `slack.com` and `files.slack.com`. The app needs the `im:history` scope to read what a person types in their conversation with it.
- `SLACK_SIGNING_SECRET`, the app's signing secret, which checks each delivery and reaches no host.

## Settings

- `from`: `./.bulldog/bridges/slack`, the folder it runs, as Bulldog ships it.
- `places`: `direct`, a person's conversation with the app, and each channel by its name, with the `team` asked there, the office when none is named, and whether `guests` may ask.

## Leaves out

- What the app said, and what any other bot said.
- Edits, deletions, joins, and every event that is not a message.
- A delivery Slack did not sign, or signed more than five minutes before.

What a bot said and what is no message are each logged in the in run's record on Corbie, with its id, its place and why, and never its words; a delivery not signed is answered why.

## Example

```yaml
# in office.yaml
bridges:
  slack:
    from: ./.bulldog/bridges/slack
    places:
      direct: {}
      "#help": { guests: true }
```

## Changed

- 0.1.0: first shipped.
- 0.2.0: run in place, by `from` in its entry, and no longer copied into the office; logs what it leaves out, and marks a message that names the app as mentioned.
