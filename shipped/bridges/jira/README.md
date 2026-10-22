Jira: a bridge that hears issues opened and commented on in the projects its places name, and answers there with a comment.

The in run asks Jira for the issues of each project that changed, then rests 30 seconds, since Jira cannot be asked to wait. A new bridge hears from when it first asks. Jira has no place to say anything to a person directly, so a question or a decision is never sent by Jira.

| Thing | Jira's |
|---|---|
| `id` | `issue:<id>` for a new issue, or `comment:<id>` |
| `from` | the author's account id, which never changes; for a new issue, the account that created it |
| `place` | the project's key, as `OPS` |
| `where` | the issue's key, as `OPS-12` |
| `text` | the comment, or the summary and description of a new issue |
| `mentioned` | the text mentions the bridge's account, as `[~accountid:…]` |

## Needs

- `JIRA_TOKEN`, an API token of the account in `email`, for the host in `site`.

## Settings

| Setting | What it is |
|---|---|
| `site` | the host of your Jira Cloud, as `example.atlassian.net`; the token is sent there |
| `email` | the Jira account the token is of, which the bridge signs in as |
| `places` | each project by its key, as `OPS`, with the `team` asked there, the office without one, and whether `guests` may ask |

## Leaves out

- What the bridge's own account wrote, and what an app wrote.
- A comment only some may read: one restricted to a role or a group, and an internal comment of Jira Service Management.
- A change of status, and every other change to an issue.
- A new issue or comment, when more than 500 other issues changed after its issue and before the bridge asked again.

Each one left out is logged in the in run's record, with its id, its issue and why, never its words.

## Example

```yaml
# office.yaml
bridges:
  jira:
    from: ./.bulldog/bridges/jira
    site: example.atlassian.net
    email: bot@example.com
    places:
      OPS: { team: platform }
```
