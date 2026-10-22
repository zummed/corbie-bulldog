GitLab: a bridge that hears issues and merge requests opened and commented on in the projects its places name, and answers there with a comment.

The in run asks GitLab what happened on each project, then rests 30 seconds, since GitLab cannot be asked to wait. A project new to the bridge is heard from when the bridge first asks about it. GitLab has no place to say anything to a person directly, so a question or a decision is never sent by GitLab.

| Thing | GitLab's |
|---|---|
| `id` | `note:<id>` for a comment, or `event:<id>` for an opening |
| `from` | the author's user id, which never changes |
| `place` | the project's path, as `platform/api` |
| `where` | the issue, `platform/api#7`, or the merge request, `platform/api!12` |
| `text` | the comment, or the title and description of what was opened |
| `mentioned` | the text has an @mention of the bridge's account |

## Needs

- `GITLAB_TOKEN`, a token of the account the bridge speaks as, for the host in `site`.

## Settings

| Setting | What it is |
|---|---|
| `site` | the host of the GitLab it speaks to, as `gitlab.com`, or your own; the token is sent there |
| `places` | each project by its path, as `platform/api`, with the `team` asked there, the office without one, and whether `guests` may ask |

## Leaves out

- What the bridge's own account did, and GitLab's system notes.
- Anything else that happens: a push, a close, a merge, and what happens to anything but an issue or a merge request.

Each is logged in the in run's record, with its id, its project and why, never its words.

## Example

```yaml
# office.yaml
bridges:
  gitlab:
    from: ./.bulldog/bridges/gitlab
    site: gitlab.com
    places:
      platform/api: { team: platform }
```
