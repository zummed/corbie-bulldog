Slack: listens over Socket Mode for what is said and each press of a button; it answers in the thread, and carries questions and decisions to a person directly.

The in machine listens: it opens a connection to Slack with the app-level token, so the office needs no address Slack can reach, and hands over each thing as Slack sends it, then tells Slack it was received. Each run listens a few minutes and the next connects again. A file said in Slack is read; a file in an answer is named, not sent. A message that names the app is marked as mentioned, for a team's heed, and the app's mention in it reads as its name, as `@gromit`; anyone else's stays as Slack gives it. An answer goes as Markdown, which Slack shows, a decision's beside its buttons. The app needs Socket Mode turned on, with its Event Subscriptions, for the message events, and its Interactivity.

A thing is lost if the runner stops after handing it over and telling Slack it was received, and before the handover is written down: Corbie does not yet let a script wait until what it hands over is kept.

## Needs

- `SLACK_TOKEN`, the app's bot token, for `slack.com` and `files.slack.com`. The app needs the `im:history` scope to read what a person types in their conversation with it.
- `SLACK_APP_TOKEN`, the app-level token, with the `connections:write` scope, for `slack.com`, which gives the connection's address on `wss-primary.slack.com` or `wss-backup.slack.com`.

## Settings

- `from`: `./.bulldog/bridges/slack`, the folder it runs, as Bulldog ships it.
- `places`: `direct`, a person's conversation with the app, and each channel by its name, with the `team` asked there, the office when none is named, and whether `guests` may ask.

## Leaves out

- What the app said, and what any other bot said.
- Edits, deletions, joins, and every event that is not a message.

Each is logged in the in run's record on Corbie, with its id, its place and why, and never its words, and told to Slack as received.

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
- 0.2.0: run in place, by `from` in its entry, and no longer copied into the office; logs what it leaves out, and marks a message that names the app as mentioned; listens over Socket Mode in place of being called at the office's address: may now connect to wss-primary.slack.com and wss-backup.slack.com, needs SLACK_APP_TOKEN, and no longer needs SLACK_SIGNING_SECRET; sends answers as Markdown, which Slack shows, and writes the app's own mention as its name.
