Jira issues: it listens on the projects its places name, answers in the issue, and hears its status change.

The in machine asks Jira for the issues of each project that changed since its cursor, resting thirty seconds while nothing has; its first ask hears nothing. A new issue, each comment and each change of status is heard; it is answered with a comment. Jira has no place to say anything to a person directly.

## Needs

- `JIRA_TOKEN`, an API token of the account in `email`, for the host in `site`.

## Settings

- `from`: `./.bulldog/bridges/jira`, the folder it runs, as Bulldog ships it.
- `site`: the host of your Jira Cloud, as `example.atlassian.net`. The token is sent there.
- `email`: the Jira account the token is of, which the bridge signs in as.
- `places`: each project by its key, as `OPS`, with the `team` asked there, the office when none is named, and whether `guests` may ask.

## Leaves out

- What the bridge's own account wrote.

## Example

```yaml
# in office.yaml
bridges:
  jira: { from: ./.bulldog/bridges/jira, site: example.atlassian.net, email: bot@example.com, places: { OPS: { team: platform } } }
```

## Changed

- 0.1.0: first shipped.
- 0.2.0: run in place, by `from` in its entry, and no longer copied into the office; reaches the host in `site`, and sends JIRA_TOKEN there, in place of example.atlassian.net.
