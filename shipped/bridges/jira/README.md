Jira issues: it listens on the projects its places name, answers in the issue, and hears its status change.

The in machine asks Jira for the issues of each project that changed since its cursor, resting thirty seconds while nothing has; its first ask hears nothing. A new issue, each comment and each change of status is heard; it is answered with a comment. Jira has no place to say anything to a person directly.

## Needs

- `JIRA_TOKEN`, an API token of the account in `email`, for `example.atlassian.net`. For your own site, its host is written in place of example.atlassian.net in the bridge's machines and script, in a copy of its own.

## Settings

- `email`: the Jira account the token is of, which the bridge signs in as.
- `places`: each project by its key, as `OPS`, with the `team` asked there, the office when none is named, and whether `guests` may ask.

## Leaves out

- What the bridge's own account wrote.

## Example

```yaml
# in office.yaml
bridges:
  jira: { email: bot@example.com, places: { OPS: { team: platform } } }
```

## Changed

- 0.1.0: first shipped.
