# The work office

One Slack bot over four teams. People ask in #help, in #refunds and in a direct message to the bot, and on GitLab in `platform/api`.

- The bot leads the office. In #help, where guests may ask, the shipped `for-us` lets in what a fast model says is for the office.
- The bot asks the other teams with `ask_team`. The bot and the ledger read and write their team's docs.
- Payments holds customer data. In #refunds, the shipped `mentioned` lets in what names the bot and what follows its answer in a thread. Its check keeps account numbers in, and Lee or Priya decide each refund.
- Platform has no people, so Al, the office's person, decides for it.
- Support has only people. Dana leads it, so each request to support is a question to Dana.

## Running it

On the Corbie set up on this computer (`corbie manual one-computer`), write each person's Slack and GitLab user ids in `office.yaml`, and make the Slack app as [the Slack bridge's page](../../shipped/bridges/slack/README.md) says. Set each secret with `corbie controller secret set <name>`:

| Secret | For |
|---|---|
| `ANTHROPIC_KEY` | the model |
| `SLACK_TOKEN`, `SLACK_APP_TOKEN` | the Slack bridge |
| `GITLAB_TOKEN` | the GitLab bridge, and ops |
| `STRIPE_KEY` | the ledger |
| `TYPESAFE_KEY` | `for-us` |

Then serve it with `corbie bulldog examples/work`.
