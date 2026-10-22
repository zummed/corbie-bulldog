Slack: a bridge that hears messages and presses of buttons over Socket Mode, and answers in the thread or to a person directly.

The in run opens a connection to Slack with the app-level token, so the office needs no address Slack can reach. Slack sends each message and each press over it, and the bridge hands each over, then tells Slack it was received. Each job listens 4 minutes, and the next connects again.

A message at the top of a channel, or of a person's conversation with the app, begins a thread of its own, and the answer goes in that thread. A message that names the app is heard with `mentioned`, and the app's mention reads as its name, as `@gromit`. A file said in Slack is read.

The out sends Markdown, which Slack shows. A question or a decision has a button for each choice. A message to a person directly goes to their conversation with the app.

| Thing | Slack's |
|---|---|
| `id` | `<channel>/<ts>`, or `press:<user>:<ts>` for a press |
| `from` | the sender's user id, which never changes |
| `place` | `direct` for a person's conversation with the app, or the channel's name, as `#help` |
| `where` | `<channel>/<thread ts>` |
| `answers` | `<channel>/<thread ts>` of the message a reply in a thread is under |

A thing is lost if the runner stops after Slack is told it was received and before the thing is written to the in run's record.

## Charts and images

A worker writes a chart as a fenced block that opens with ` ```chart `, holding the chart in JSON. Slack shows it as a chart of its own, among the Markdown:

````
```chart
{ "type": "bar", "title": "Refunds by week", "categories": ["W1", "W2", "W3"],
  "series": [{ "name": "Refunds", "values": [4, 7, 3] }], "x_label": "Week", "y_label": "Refunds" }
```
````

| Field | What it is |
|---|---|
| `type` | `bar`, `line`, `area` or `pie`; needed |
| `title` | up to 50 characters; needed |
| `categories` | 1 to 20 labels, each up to 20 characters, all different |
| `series` | 1 to 12, each with a different `name` of up to 20 characters, and one number in `values` for each category; a pie has one series of numbers above 0, and up to 12 categories |
| `x_label`, `y_label` | up to 50 characters each; not for a pie |

- A message holds at most two charts; the next goes in the next message.
- An image alone on its line, as `![the refunds dashboard](https://ledger.example.com/refunds.png)`, at an `https` address Slack can fetch, is shown as the image.
- A chart that does not fit the table, and a message Slack will not show, such as one with an image it cannot fetch, is sent as written.

## Needs

- `SLACK_TOKEN`, the app's bot token, for `slack.com` and `files.slack.com`. The app needs `chat:write`; `im:write`, to message a person directly; `im:history`, to hear what a person types in their conversation with it; `channels:read` and `im:read`, to read a conversation's name; and `files:read`, to read a file said.
- `SLACK_APP_TOKEN`, the app-level token, with the scope `connections:write`, for `slack.com`. Slack gives it the connection's address on `wss-primary.slack.com` or `wss-backup.slack.com`.
- Socket Mode turned on in the app's settings, with Event Subscriptions for message events and Interactivity.

## Settings

| Setting | What it is |
|---|---|
| `places` | `direct`, any person's conversation with the app, and each channel by its name, as `"#help"`, each with the `team` asked there, the office without one, and whether `guests` may ask |

## Leaves out

- What a bot said, the app among them.
- Edits, deletions, joins, and every event that is not a message.

Each is logged in the in run's record, with its id, its channel and why, never its words, and Slack is told it was received.

## Example

```yaml
# office.yaml
bridges:
  slack:
    from: ./.bulldog/bridges/slack
    places:
      direct: {}
      "#help": { guests: true }
      "#refunds": { team: payments }
```
