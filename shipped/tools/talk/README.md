Talk: the tools a worker uses to ask another team or a person, and wait for the answer.

| Tool | What it does |
|---|---|
| `ask_team(team, question)` | asks another team of the office, and waits for its answer |
| `ask_person(who, question, choices)` | asks a person, by their id in `everyone`, and waits for the answer |

Each tool runs `./.bulldog/wait/wait.yaml`, and its result is the wait's output: `{ answer, by, thing }`, `{ answer: null, note }` after 7 days, or `{ answer: null, refused }`.

- `ask_team` is marked for approval, so the asking team's `check` is asked about the question before it leaves. Without a `check`, the question leaves at once.
- `ask_team` is refused for the asking team itself, a team already in the request's chain, and a name that is no team.
- `ask_person` may ask a person of the worker's team, or one its team file names. Anyone else is refused, and nobody is asked.
- `choices`, up to 8 short answers of up to 75 characters each, are offered as buttons. The person may still answer anything else.

## Needs

Nothing: no secret and no host.

## Settings

None.

## Example

```yaml
# steward.yaml
agent: steward
model: usual
prompt: ${input.said}
tools_file: [./.bulldog/tools/talk/talk.yaml]
```

The steward asks the garden with `ask_team({ team: garden, question: "When do we sow the beds?" })`, and is given `{ answer: "In May." }`.
