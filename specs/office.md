# Office

The office is a directory of YAML: its teams, people, workers and bridges. Bulldog reads it whole, reports every problem together, and reads it again when any file changes.

## The directory

```
house/
  office.yaml             the top team, `office`
  steward.yaml            a worker: the office's lead
  fixer.yaml              a worker: the steward's colleague
  sparks.yaml             a worker: the fixer's colleague
  clerk.yaml              a worker: a decider in the office's `decide`
  small-orders.js         a rule
  tools/                  the scripts the fixer's tools run
  garden/
    team.yaml             a team within the office, `garden`
    gardener.yaml         a worker: the garden's lead
  .bulldog/id             the office's id, the one file Bulldog writes
```

- A team is a directory directly inside a team's directory that holds `team.yaml`. The top team is `office`, and its directory is the office's.
- A worker is a Corbie definition, an agent, a flow or a statechart, directly in a team's directory. Its name is its file's name, and its definition's name is the same.
- A definition anywhere else, such as `tools/`, is not a worker. Workers use it.
- `./.bulldog/<path>` names what Bulldog ships, written from any directory of the office. Bulldog serves it from its binary, and nothing of it is on disk.
- A path is read from the directory of the file that names it.
- Every other path a team file or a definition names is inside the office directory. A path that leaves it is a problem.

## A team file

The top team's file is `office.yaml`. It holds what every team file holds, and the office's zone, people and bridges.

```yaml
# office.yaml
name: The house
about: The house and everything in it, but the garden.
zone: Europe/London
everyone:
  sam: { name: Sam, telegram: "4471" }
  kit: { name: Kit, telegram: "4472" }
bridges:
  telegram:
    from: ./.bulldog/bridges/telegram
    places:
      direct: {}                       # Sam and Kit ask the house in a direct message
models:
  usual: { name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }
people: [sam, kit]
lead: steward
decide: [./small-orders.js, clerk, sam]
routines:
  - { at: "mon 09:00", say: "What is still open?" }
```

```yaml
# garden/team.yaml
name: The garden
about: The beds, the lawn and the shed.
people: [sam]
lead: gardener
models:
  usual: { name: claude-opus-5-5, secret: ANTHROPIC_KEY }   # the garden's own
routines:
  - { at: "fri 16:00", say: "What needs doing before the weekend?" }
```

| Key | What it is |
|---|---|
| `name` | what the team is called, in any words |
| `about` | one line: what the team looks after |
| `people` | the team's people, by name |
| `lead` | a worker of the team, or a person, that takes every request to the team; every team has one |
| `heed` | the decision on a message at a channel, before the lead starts; without it, everything comes in |
| `check` | the decision on what leaves the team: an answer to someone outside it, and a question to another team; without it, everything leaves |
| `decide` | the decision on jobs Corbie holds for approval; without it, the team's people are asked |
| `routines` | `{ at, say }`: `say` is said to the team's lead at `at`, a time of day (`"18:00"`), optionally after a weekday (`"mon 09:00"`), in the office's zone |
| `models` | names for models, each a model as Corbie writes one |
| `zone` | `office.yaml` only: the office's time zone; UTC without it |
| `everyone` | `office.yaml` only: every person of the office |
| `bridges` | `office.yaml` only: the bridges, in the order written (Bridges) |

- A key written as an empty list, such as `heed: []`, is the same as none.
- A team with no people has the office's people as its people, for being asked and for seeing its requests.
- With no `decide` and no people in the team or the office, nothing held is approved.
- A decision is a list of deciders, and Requests says how one is asked.

```yaml
# payments/team.yaml, in the work office
name: Payments
about: Charges, refunds and the ledger. Holds customer data.
people: [lee, priya]
lead: ledger
heed: [./.bulldog/rules/mentioned/mentioned.js]  # in #refunds, only what names the bot
check: [./no-account-numbers.js, lee, priya]     # may this leave payments?
decide: [lee, priya]                              # may this held job run?
```

| Decider | Written as |
|---|---|
| a rule | its path: a script (`./no-account-numbers.js`) or a definition (`./.bulldog/rules/for-us/for-us.yaml`), but not a worker's file |
| a worker | its name, of any team; it declares `reply: { says: string, note: "string?" }` |
| a person | their id; people may only end a list |

A decider never asks for approval itself. A worker or a rule that holds a job for approval, or lists `ask_about`, is a problem of the team file that names it.

## People

```yaml
everyone:
  al:   { name: Al, slack: U0AL0000, gitlab: "1041" }
  dana: { name: Dana, slack: U0DANA00 }
```

- A person is written once, in `everyone`, with `name` and, under each bridge's name, their lasting id on that bridge: never a handle they can change.
- One identity on one bridge names one person. Identities are compared exactly as written.
- A team's `people`, its decisions and its `lead` name people by their id, the key of their entry in `everyone`, such as `sam`.

## Workers and models

A worker is sent to Corbie as it is written, with one change: a model named by one word is filled in.

```yaml
# steward.yaml
agent: steward
model: usual
system: |
  You look after the house for Sam and Kit. Hand repairs to the fixer, and ask the garden about the garden.
  ${input.context}
prompt: ${input.said}
tools_file: [./.bulldog/tools/talk/talk.yaml, ./.bulldog/tools/docs/docs.yaml]
tools:
  fixer:
    does: Repairs about the house
    args: { repair: string }
    job: { machine: ./fixer.yaml, input: { said: "${args.repair}", context: "${input.context}", request: "${input.request}" } }
```

`model: usual` is the nearest team's `usual`: the worker's own team's first, then each team around it, the office's last. The steward's is the office's `claude-sonnet-5-5`, and the gardener's the garden's `claude-opus-5-5`. A word no team gives is a problem of the worker.

## What Bulldog ships

| Path | What it is |
|---|---|
| `./.bulldog/wait/wait.yaml` | waits on a person or a team, for 7 days at most |
| `./.bulldog/clock/clock.yaml` | a routine's clock |
| `./.bulldog/docs/docs.yaml` | reads, lists and writes a team's docs for the screens |
| `./.bulldog/tools/talk/talk.yaml` | the tools `ask_team` and `ask_person` |
| `./.bulldog/tools/docs/docs.yaml` | the tools `read_doc` and `write_doc` |
| `./.bulldog/rules/mentioned/mentioned.js` | a heed rule: yes when the message names the bot, or follows the lead's answer to the same sender in the thread |
| `./.bulldog/rules/for-us/for-us.yaml` | a heed rule: asks a fast model whether the message is for the team; needs `TYPESAFE_KEY` |
| `./.bulldog/bridges/slack`, `telegram`, `gitlab`, `jira` | the bridges (Bridges) |

Each shipped directory has a `README.md`: its page.

## Docs

A team's docs are values kept in Corbie's store, at `<office id>/<team>/docs/<name>`. A doc's name is one part of a kept path: small letters, digits, `.`, `-` and `_`, not starting with a dot, such as `refunds.md`.

| Tool | What it does |
|---|---|
| `read_doc(name)` | reads one of the team's docs; with no name, lists them; allowed `kept.read` only |
| `write_doc(name, text)` | writes one of the team's docs, whole; allowed `kept.write` only; marked for approval, so the team's `decide` is shown the new text first |

Every run Bulldog starts for a team, a request or a docs run, is started with this list, which narrows everything below it:

```
network  disk  exec  env  secret:*@*
kept.read:<office id>/<team>/**
kept.write:<office id>/<team>/**
```

So a team's runs read and write only its own kept values, whatever its definitions allow. On the screens, a team's people read and edit its docs through a run of `./.bulldog/docs/docs.yaml`, so Bulldog holds no key to kept values.

## Names

- Names of people, workers, teams and bridges start with a small letter, and hold small letters, figures and dashes. A bridge is not called `name`.
- Each name of a person, a worker or a team is used once in the office.
- `office`, `team`, `bulldog`, `wait` and `clock` are reserved.

## Reading the office

- Bulldog reads the office again when any file in it changes.
- A key a team file does not take, or one only `office.yaml` takes written in another team file, is a problem.
- A problem in how `office.yaml` or a team file is written, such as an unknown key, a name, a person, a lead or a routine, refuses the reading, and the last good reading stays in force.
- A worker, a rule or a bridge that cannot be read, or that Corbie's parser refuses, stops only itself and the teams that use it, even where `office.yaml` or a team file names it. A broken worker stops its own team, and each team that names it as a decider.
- Every problem is shown.
- `corbie bulldog check <directory>` reads the office as serving does, prints every problem, each with its file, and starts nothing. It exits with a failure status when there is any problem.

## The office's id and secret

Bulldog writes `.bulldog/id` when the office has none, at any reading of it: a random id of 16 hex digits, which names the office in labels and kept paths. It is the one file Bulldog writes in the office.

- While Bulldog serves, a changed or removed id is refused, and the office keeps its id until Bulldog starts again.
- Started with a new id, the office is a new office, with none of the old one's requests, docs or sign-ins. Keep `.bulldog/id` with the office, in git.

The office's secret is made the first time Bulldog serves the office, or prints a key, as this user on this computer. It is kept in Bulldog's own directory in the user's config directory, under the office's id, never in the office directory. It signs sign-in keys, and keys the hashes in labels, run names and `ref`s.

- Replacing the secret signs everyone out.
- Served as another user or on another computer, the office has a new secret: everyone is signed out, key links printed before sign nobody in, and what was heard in the last day is asked again.
