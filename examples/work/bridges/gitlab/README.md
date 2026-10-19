GitLab issues and merge requests: it listens on the projects its places name, answers in the issue or merge request, and hears it closed.

The in machine asks GitLab what happened on each project since its cursor, resting thirty seconds while nothing has; its first ask hears nothing and marks where each project stands. The opening of an issue or merge request, and each comment on one, is heard; it is answered with a comment. GitLab has no place to say anything to a person directly.

## Needs

- `GITLAB_TOKEN`, a token of the account the bridge speaks as, for `gitlab.com`. For a GitLab of your own, its host is written in place of gitlab.com in the bridge's machines and script, in a copy of its own.

## Settings

- `places`: each project by its path, as `platform/api`, with the `team` asked there, the office when none is named, and whether `guests` may ask.

## Leaves out

- What the bridge's own account did, and GitLab's system notes.
- What happened to anything but an issue or a merge request.

## Example

```yaml
# in office.yaml
bridges:
  gitlab: { places: { platform/api: { team: platform } } }
```

## Changed

- 0.1.0: first shipped.
