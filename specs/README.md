# Bulldog

Bulldog is an office of teams on Corbie. People and workers in teams are asked things on Slack, Telegram, GitLab and Jira, on Bulldog's screens or at a set time, and the answer goes back where the question was asked. Corbie runs everything, and Bulldog keeps nothing but the office's id and secret.

## Six concepts

| Concept | What it is |
|---|---|
| team | a directory with a team file: its people, its lead, its decisions, its routines, its models |
| person | someone real, written once in `office.yaml`, with who they are on each bridge |
| worker | a Corbie definition, an agent, a flow or a statechart, directly in a team's directory |
| bridge | a directory of two Corbie definitions, `in.yaml` and `out.yaml`, joining an outside service |
| request | one thing asked of a team: one run of the team's request machine |
| decision | a yes or no asked of an ordered list of deciders; a team writes three: `heed`, `check`, `decide` |

## One machine per team, one run per request

For each team, Bulldog fills out its team file into one Corbie flow, the team's **request machine**, with the stages `heed`, `work` and `check`. Every request is one run of it.

1. Someone asks: a bridge hears a message, a person asks on the screens, a worker asks another team with `ask_team`, or a routine's time comes.
2. Bulldog starts a run of the team's request machine, labelled with the office and the team.
3. `heed` asks whether a message at a channel is for the team, `work` runs the team's lead, and `check` asks whether the answer may leave the team.
4. While the lead works, it may wait on a person, on another team or on a decision. Bulldog carries each question out and answers its ticket.
5. When the run ends, Bulldog carries its answer back where the request came from.

Permissions, secrets, approval, waiting, history and records are Corbie's. A team's docs are values kept in Corbie's store.

## A day at home

The house is one family's office: Sam and Kit, a top team that looks after the house, and a garden team within it.

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

1. At 08:12 Kit sends the house a photo on Telegram: "The socket by the boiler sparks."
2. Bulldog starts a request of `office`. A direct message has no heed, so the steward starts at once.
3. The steward hands the repair to the fixer, its colleague. The fixer orders a part with `order_part`, for £35, which waits for a yes.
4. The office's `decide` asks in order. `small-orders.js` passes: "More than I order alone." The clerk passes: "The house has the money for it."
5. Sam gets the question on Telegram, both notes above the buttons `yes` and `no`, and presses `yes`.
6. The part is ordered, and the steward answers Kit in the same chat.
7. On Monday at 09:00 the routine asks the steward "What is still open?", and the answer goes to Sam and Kit.

## A day at work

The work office is one Slack bot over three teams: the office, whose lead is the bot; payments, which holds customer data; and platform, one worker with no people.

```yaml
# payments/team.yaml
name: Payments
about: Charges, refunds and the ledger. Holds customer data.
people: [lee, priya]
lead: ledger
heed: [./.bulldog/rules/mentioned/mentioned.js]  # in #refunds, only what names the bot
check: [./no-account-numbers.js, lee, priya]     # may this leave payments?
decide: [lee, priya]                              # may this held job run?
```

1. At 10:02 a guest asks in #help: "Was order 4411 refunded?"
2. Bulldog starts a request of `office`. Its heed, the shipped `for-us`, says yes, and the bot starts.
3. The bot asks payments with `ask_team`. The office has no `check`, so the question leaves at once.
4. Bulldog starts a request of payments. A team asked it, so there is no heed, and the ledger starts.
5. The ledger answers "Refunded £40 on 3 May." Payments' check runs `no-account-numbers.js`, which says yes.
6. Bulldog answers the bot's wait with that answer, and the bot answers the guest in the thread in #help.

Later, the ledger asks platform about account 12345678. Payments' check says no, and the ledger's tool gets "Payments did not let its question out." Platform never sees the question.

## What holds it together

- Bulldog keeps nothing of its own but the office's id and secret. Every request, question, routine and bridge is a Corbie run, found again by its labels.
- Every run Bulldog starts carries labels, `bulldog=<office id>` and `kind` among them. Labels and run names hold no words of a request and no plain id of a person.
- Bulldog starts runs and answers their tickets. Nothing in Corbie calls Bulldog.
- What a request machine gives is the only thing that leaves a team. Everything a team sends to another team passes its `check`.
- Bulldog talks to no outside service itself. Every way in and out but its own screens is a bridge.
- A run Bulldog starts in answer to something is named from that thing, so a second start of it is dropped.
- Every run Bulldog starts for a team may read and write kept values only under that team's own path.

## Where the sealing stops

A team is sealed: what it read is shown only to its people. These are the limits of that.

- Whoever holds Bulldog's client identity, usually the computer's owner, reads every team's records.
- The check covers what Bulldog carries. A worker's own permissions let it send anything wherever they allow.
- A check that asks a model is a filter, not a guarantee.
- A worker allowed to write the office directory can change the deciders, and how people are messaged.
- Whoever runs the store sees the names of every team's docs.

## Running it

| Command | What it does |
|---|---|
| `corbie bulldog <directory>` | serves the office: its requests, bridges, routines and screens |
| `corbie bulldog check <directory>` | prints every problem of the office, and starts nothing |
| `corbie bulldog key <directory> <person>` | prints a person's key link, which signs them in |
| `mise run demo` | serves the house on a stand-in model and a Corbie in the same process, with nothing to set up |

Bulldog is a Corbie client on the Corbie its config names. After a restart it finds everything again in the store, and doing a thing twice changes nothing.

## Areas

Each area has an overview here and its rules in `rules/`, and is one package.

**[Office](office.md).** The office directory: teams, people, workers, bridges, routines, models and docs, read with every problem found together, and what Bulldog ships.

**[Requests](requests.md).** The request machine, decisions, waiting on a person or a team, threads, routines, labels, and carrying an answer back.

**[Bridges](bridges.md).** What a bridge hears and how Bulldog acts on it, and one message carried out.

**[Screens](screens.md).** Signing in, who sees what, the three screens and the API behind them.

Beside them, with no area of their own:

| Package | What it does |
|---|---|
| `internal/testkit` | a whole Corbie in the test's process, a stand-in model and a clock the test moves |
| `shipped` | what Bulldog ships, served at `./.bulldog/` |
| `cmd/corbie-bulldog` | the program: serve, `check` and `key` |
| `cmd/demo` | the house on a stand-in model |

## Words

One word for one thing, the same in the specs, the code and the screens.

| Word | What it means |
|---|---|
| office | the directory Bulldog serves, and its top team, `office` |
| team | a directory with a team file, inside the office |
| person | an entry of `everyone` in `office.yaml` |
| guest | someone a bridge hears at a place that lets guests ask, whom no entry names |
| worker | a Corbie definition directly in a team's directory |
| lead | the worker, or the person, that takes every request to a team |
| colleague | a worker another worker of its team uses as a tool |
| rule | a decider named by its path: a script, or a shipped definition |
| decider | a rule, a worker or a person in a decision's list |
| decision | `heed`, `check` or `decide`: a yes or a no from an ordered list of deciders |
| bridge | a directory with `in.yaml` and `out.yaml` |
| place | where a bridge takes requests, as its settings list them, or `direct` |
| channel | a listed place other than `direct` |
| thing | one message, or one press of a button, that a bridge's in run hands over |
| request | one run of a team's request machine |
| request machine | the flow Bulldog fills out from a team file |
| source | where a request came from, and where its answer goes: a place, the chat, another team's wait, or a routine |
| chat | asking a team on the screens |
| sub-request | a request a worker makes of another team with `ask_team` |
| wait | a run of the shipped `wait.yaml`, waiting on a person or a team |
| fork | a wait or a decision of a request, drawn as a line off WORK |
| thread | the requests asked at one `where` on a bridge, in one chat, or by one request of one team |
| routine | something said to a team's lead at a time of day |
| doc | a text a team keeps in Corbie's store under its name |
| outcome | how a request ended: `answered`, `noted`, `withheld` or `failed` |

## What Corbie should gain

- **A start keyed by the caller's own id.** Bulldog names a request from the thing's own time and a keyed hash of its id, looks for that name before it starts, and lets go of a thing more than a day old. Corbie reads the time in a run's name as when the start was sent: a start not yet taken and named more than 15 minutes ago is reported as never taken up, and one named more than a day ago loses its uploads to housekeeping. A start keyed by the caller's id, with no time in the run's name, would need none of this.
