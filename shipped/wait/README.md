Wait: a run that waits on a person or a team for an answer, for 7 days at most.

`wait.yaml` is a statechart that waits on its ticket `answer`. Bulldog carries the question and answers the ticket.

1. A worker's tool, a decision or a team whose lead is a person starts `wait.yaml`.
2. Bulldog asks the team, or carries the question to each person in their direct messages and on the screens.
3. Bulldog answers the ticket with the first answer, `{ answer, by, thing }`, and the run ends with it.
4. With no answer after 7 days, the run ends with `{ answer: null, note: "No answer came within 7 days." }`.

When Bulldog refuses the question, the run ends with `{ answer: null, refused }`, which names whom the worker may ask.

## Needs

Nothing: no secret and no host.

## Settings

Its input:

| Input | What it is |
|---|---|
| `to` | `{ team }`, or `{ people: [...] }` by their ids in `everyone` |
| `said` | the question |
| `choices` | up to 8 short answers of up to 75 characters each, offered as buttons; optional |

A decision's people are given four more, which Bulldog writes into the request machine:

| Input | What it is |
|---|---|
| `decision` | `heed`, `check` or `decide`; only `yes` or `no` answers it |
| `question` | what they are asked, without the notes; `said` holds the question, the held jobs and the notes, as a bridge carries them |
| `notes` | `[{ kind, name, says, note }]`: each decider before them, which passed |
| `jobs` | for `decide`, `[{ name, does, args }]`: the jobs Corbie holds |

## Example

The talk tools run it. A machine runs it as a job:

```yaml
job:
  machine: ./.bulldog/wait/wait.yaml
  input: { to: { people: [sam] }, said: "May the fixer order the part?", choices: [yes, no] }
```

Read next: `./.bulldog/tools/talk`.
