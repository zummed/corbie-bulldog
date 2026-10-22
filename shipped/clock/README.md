Clock: the run that keeps a routine's time.

`clock.yaml` is a statechart. Bulldog keeps one run of it for each routine of each team.

1. The clock waits for its time, `at`.
2. At that time it waits on its ticket `came`.
3. Bulldog starts the routine's request of the team, then answers `came`.
4. The clock waits for its next time.

A time that came while Bulldog was stopped is acted on once, when Bulldog starts again.

## Needs

Nothing: no secret and no host.

## Settings

Its input:

| Input | What it is |
|---|---|
| `at` | a time of day, optionally after a weekday, and a zone: `mon 09:00 Europe/London` |

## Example

A team file's routine:

```yaml
routines:
  - { at: "mon 09:00", say: "What is still open?" }
```

Bulldog starts the clock with `{ at: "mon 09:00 Europe/London" }`, the office's zone added.
