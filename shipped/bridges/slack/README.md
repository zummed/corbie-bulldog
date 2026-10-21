Slack: listens over Socket Mode for what is said and each press of a button; it answers in the thread, and carries questions and decisions to a person directly.

The in machine listens: it opens a connection to Slack with the app-level token, so the office needs no address Slack can reach, and hands over each thing as Slack sends it, then tells Slack it was received. Each run listens a few minutes and the next connects again. What is said to the app directly is taken as in a channel: a new message begins a request of its own, or answers a question the app sent the person directly while it waits on them, and the answer goes in its thread, where a reply goes on with it. A file said in Slack is read; a file in an answer is named, not sent. A message that names the app is marked as mentioned, for a team's heed, and the app's mention in it reads as its name, as `@gromit`; anyone else's stays as Slack gives it. An answer goes as Markdown, which Slack shows, a decision's beside its buttons, as is a question's that offers choices. A chart and an image a worker writes are shown as Slack's own, as below. While a team works on a message, the app reacts to it with 👀, and takes the reaction off once the team is done with it, whether it answered or said nothing. The app needs Socket Mode turned on, with its Event Subscriptions, for the message events, and its Interactivity.

A thing is lost if the runner stops after handing it over and telling Slack it was received, and before the handover is written down: Corbie does not yet let a script wait until what it hands over is kept.

## Charts and images

A worker writes a chart as a fence marked `chart`, holding the chart in JSON, and Slack shows it as a chart of its own, among the Markdown:

````
```chart
{ "type": "bar", "title": "Refunds by week", "categories": ["W1", "W2", "W3"],
  "series": [{ "name": "Refunds", "values": [4, 7, 3] }], "x_label": "Week", "y_label": "Refunds" }
```
````

- `type`: `bar`, `line`, `area` or `pie`, needed.
- `title`: up to 50 characters, needed.
- `categories`: 1 to 20 labels, each up to 20 characters and different.
- `series`: 1 to 12, each with a `name` up to 20 characters and different, and one number in `values` for each category. A pie has one series, of numbers above 0, and up to 12 categories, one slice each.
- `x_label`, `y_label`: up to 50 characters each, not for a pie.

A message holds two charts at most, and the next goes on in the next message. An image alone on its line, as `![the refunds dashboard](https://ledger.example.com/refunds.png)`, at an `https` address Slack can fetch, is shown as the image. A chart that does not fit the above, and a message Slack will not show, as one with an image it cannot fetch, is said as it was written. A table in Markdown is shown by Slack as one. Every other bridge, and the screens, show the fence as written.

## Needs

- `SLACK_TOKEN`, the app's bot token, for `slack.com` and `files.slack.com`. The app needs the `im:history` scope to read what a person types in their conversation with it, and the `reactions:write` scope to mark what a team works on: without it, the top team's people see the bridge failing, with Slack's `missing_scope`, and every answer still goes.
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
- 0.2.0: run in place, by `from` in its entry, and no longer copied into the office; logs what it leaves out, and marks a message that names the app as mentioned; listens over Socket Mode in place of being called at the office's address: may now connect to wss-primary.slack.com and wss-backup.slack.com, needs SLACK_APP_TOKEN, and no longer needs SLACK_SIGNING_SECRET; sends answers as Markdown, which Slack shows, and writes the app's own mention as its name; takes no setting but those above, each checked when the office is read.
- 0.5.0: shows a chart fence and an image alone on its line as Slack's own chart and image, saying them as written when Slack will not show them; a question that offers choices is sent with a button for each, as a decision is, and a press answers it.
- 0.5.1: gives a message said to the app directly at its thread, as in a channel, so each new message begins a request of its own and is answered in its thread, and no longer asks Slack what the app said before; reacts with 👀 to a message while a team works on it, and needs the `reactions:write` scope for that; names a message heard by its channel and its time, `C0HELP00/1700000009.000100`, in place of the id of the event Slack sent it in, so a message heard just before the upgrade, and sent again by Slack after it, is taken a second time, and one heard before it that a team starts on after it is not marked, the bridge showing failing until the next mark goes.
