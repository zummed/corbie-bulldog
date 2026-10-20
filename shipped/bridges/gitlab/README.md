GitLab issues and merge requests: it listens on the projects its places name, answers in the issue or merge request, and hears it closed.

The in machine asks GitLab what happened on each project since its cursor, resting thirty seconds while nothing has; its first ask hears nothing and marks where each project stands. The opening of an issue or merge request, and each comment on one, is heard; it is answered with a comment. GitLab has no place to say anything to a person directly.

## Needs

- `GITLAB_TOKEN`, a token of the account the bridge speaks as, for the host in `site`.

## Settings

- `from`: `./.bulldog/bridges/gitlab`, the folder it runs, as Bulldog ships it.
- `site`: the host of the GitLab it speaks to, as `gitlab.com`, or your own. The token is sent there.
- `places`: each project by its path, as `platform/api`, with the `team` asked there, the office when none is named, and whether `guests` may ask.

## Leaves out

- What the bridge's own account did, and GitLab's system notes.
- What happened to anything but an issue or a merge request.

Each is logged in the in run's record on Corbie, with its id, its place and why, and never its words.

## Example

```yaml
# in office.yaml
bridges:
  gitlab: { from: ./.bulldog/bridges/gitlab, site: gitlab.com, places: { platform/api: { team: platform } } }
```

## Changed

- 0.1.0: first shipped.
- 0.2.0: run in place, by `from` in its entry, and no longer copied into the office; reaches the host in `site`, and sends GITLAB_TOKEN there, in place of gitlab.com; logs what it leaves out.
