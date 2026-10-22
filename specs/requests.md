# Requests

A request is one thing asked of a team, and one run of the team's request machine. Bulldog starts the run, answers the tickets it waits on, and carries its answer back where it was asked.

## Where a request comes from

The request's `heard` says how it came:

| `heard` | How |
|---|---|
| `channel` | a message at a channel: a bridge's listed place other than `direct` |
| `direct` | a direct message to a bridge's bot, where the bridge lists `direct` |
| `chat` | a person asked on the screens |
| `team` | a worker of another team asked with `ask_team` |
| `routine` | a routine of the team, at its time |

## The request machine

For each team, Bulldog fills out the team file into one Corbie flow, named after the team. It is written as if it were at the office's top directory, so every path starts `./`, and its decisions are statecharts written in place. It is checked with Corbie's parser, and shown on the team's page as it is sent to Corbie.

| Stage | Job | When |
|---|---|---|
| `heed` | the heed decision | the team has a `heed`, and `heard` is `channel` |
| `work` | the lead: its worker, or `wait.yaml` to the person who leads | the heed said yes, or there is no heed stage |
| `check` | the check decision, on the answer | the team has a `check`, the answer is not empty, and the asker is not one of the team's people and not the team's own routine |

This is payments' request machine, from its team file in Office with `decide: [lee, priya]`, without its `ask:`, which Held jobs shows.

```yaml
flow: payments
keep: 30d
input: {said: string, context: string, request: map, files: "list?"}
stages:
  heed:
    when: "${input.request.heard == 'channel'}"
    job:
      input: {request: "${input.request}", said: "${input.said}"}
      machine:
        statechart: payments-heed
        input: {request: map, said: string}
        initial: d1
        states:
          d1:
            job:
              script: ./.bulldog/rules/mentioned/mentioned.js
              input:
                request: "${input.request}"
                said: "${input.said}"
                notes: []
            on:
              done:
                - {go: said_yes, when: "${event.data?.says == 'yes'}"}
                - {go: said_no, when: "${event.data?.says == 'no'}"}
                - passed
          said_yes: {end: done, output: {says: "yes", note: "${event.data.reply?.note ?? event.data.note ?? null}"}}
          said_no: {end: done, output: {says: "no", note: "${event.data.reply?.note ?? event.data.note ?? null}"}}
          passed: {end: done, output: {says: "no", note: "Every decider passed."}}
  work:
    when: "${stages.heed.skipped || stages.heed.output.says == 'yes'}"
    job:
      machine: ./payments/ledger.yaml
      input: {said: "${input.said}", context: "${input.context}", request: "${input.request}", files: "${input.files ?? []}"}
  check:
    when: "${(typeof stages.work.output?.reply == 'string' ? stages.work.output?.reply : typeof stages.work.output?.reply?.answer == 'string' ? stages.work.output?.reply.answer : '') != '' && input.request.heard != 'routine' && !['lee', 'priya'].includes(input.request.by?.person)}"
    job:
      input: {request: "${input.request}", answer: "${(typeof stages.work.output?.reply == 'string' ? stages.work.output?.reply : typeof stages.work.output?.reply?.answer == 'string' ? stages.work.output?.reply.answer : '')}"}
      machine:
        statechart: payments-check
        input: {request: map, answer: string}
        initial: d1
        states:
          d1:
            job:
              script: ./payments/no-account-numbers.js
              input:
                request: "${input.request}"
                answer: "${input.answer}"
                notes: []
            on:
              done:
                - {go: said_yes, when: "${event.data?.says == 'yes'}"}
                - {go: said_no, when: "${event.data?.says == 'no'}"}
                - people
          people:
            job:
              machine: ./.bulldog/wait/wait.yaml
              input:
                to: {people: [lee, priya]}
                said: "May Payments give this answer?\n\n${input.answer}\n\n./payments/no-account-numbers.js passed${states.d1.output?.note ? ': ' + states.d1.output?.note : '.'}"
                choices: ["yes", "no"]
                decision: check
            on:
              done:
                - {go: said_yes, when: "${event.data.answer == 'yes'}"}
                - said_no
          said_yes: {end: done, output: {says: "yes", note: "${event.data.reply?.note ?? event.data.note ?? null}"}}
          said_no: {end: done, output: {says: "no", note: "${event.data.reply?.note ?? event.data.note ?? null}"}}
output:
  outcome: "${stages.heed.failed || stages.work.failed || stages.check.failed ? 'failed' : stages.heed.output?.says == 'no' ? 'noted' : stages.check.output?.says == 'no' ? 'withheld' : 'answered'}"
  answer: "${stages.heed.failed || stages.work.failed || stages.check.failed ? 'Payments could not answer: its work failed.' : stages.heed.output?.says == 'no' ? null : stages.check.output?.says == 'no' ? 'Payments did not let its answer out.' : (typeof stages.work.output?.reply == 'string' ? stages.work.output?.reply : typeof stages.work.output?.reply?.answer == 'string' ? stages.work.output?.reply.answer : '')}"
```

A team without a heed has no `heed` stage, and its `work` has no `when:`. A team without a check has no `check` stage.

Dana asks in #refunds "@bot was order 4411 refunded?". This is the run, as Corbie records it, with each event's data cut short:

```
step 1  <- start {"context":…,"request":{"by":{"person":"dana"},…,"heard":"channel","mentioned":true,…},"said":"@bot was order 4411 refunded?"}
step 1  -> job heed: a machine
step 2  <- done from heed {"note":"The message names the bot.","says":"yes"}
step 2  -> job work: a machine
step 3  <- done from work {"conversation":…,"reply":"Refunded £40 to order 4411.",…}
step 3  -> job check: a machine
step 4  <- done from check {"note":"No account numbers.","says":"yes"}
step 4  done
output: {"answer":"Refunded £40 to order 4411.","outcome":"answered"}
```

## The answer and the outcome

The request machine's output is `{ answer, outcome }`:

| `outcome` | When | `answer` |
|---|---|---|
| `answered` | the lead answered, and the check, if it ran, said yes | the lead's answer |
| `noted` | the heed said no | none |
| `withheld` | the check said no | "Payments did not let its answer out." |
| `failed` | a stage failed | "Payments could not answer: its work failed." |

The lead's answer depends on what the lead is:

| Lead | Its answer |
|---|---|
| an agent | its `reply`: the text, or the text `answer` of the data it declares |
| a person | `wait.yaml`'s `answer` |
| a flow or a statechart | its output: text, or its text `answer` |

- The output is the only thing of a request that leaves its team: the reply, a ticket's answer, and the thread a later request is given. A stage's output never leaves.
- A lead's answer that is not text, and has no text `answer`, is the empty text.
- The asker never sees an error. The team's people see it on the request.
- A run that fails outside its stages, such as one Corbie refuses, answers as a failed request.
- A request is kept 30 days after it ends.

## What the lead is given

| Input | What it is |
|---|---|
| `said` | the new message |
| `context` | the same as `request`, as text a model reads well |
| `request` | what Bulldog knows of the request, as data |
| `files` | the files said with it, each `{ name, data }` with `data` a blob |

| `request` | What it is |
|---|---|
| `team` | the team asked |
| `by` | who asked: `{ person }`, `{ guest }` with the identity the bridge gave, `{ team }`, or `{ routine }` with its `at` |
| `heard` | `channel`, `direct`, `chat`, `team` or `routine` |
| `place` | for a request heard on a bridge, `{ bridge, place, guests, where, thing }`, `thing` the id of the thing heard; for a sub-request, the `bridge`, `place` and `guests` of the request at the top of its chain |
| `thread` | the requests before it in its thread, each `{ by, said, answer, at }`, oldest first, the newest within 20,000 characters; `answer` is left out while there is none |
| `teams` | the teams it may ask, each `{ name, about }`: every team that takes requests, but its own and those of its chain |
| `open` | for a routine, the team's running requests, each `{ by, said, at }` |
| `chain` | the teams whose requests asked this one, top first |
| `docs` | the kept path of the team's docs, `<office id>/<team>/docs` |
| `mentioned` | the message names the bridge's bot |

`context` says, a line for each:

- the date and time in the office's zone;
- the team, what it looks after, its people and its workers;
- who asked and where: a person with their teams, a guest with the identity the bridge gave, another team with the chain, or the routine;
- the thread before, and how many of its requests are left out;
- the teams it may ask with `ask_team`, each with its `about`;
- for a routine, the team's open requests.

Inside a team, workers call each other as Corbie tools, and a colleague is given what its caller gives it.

A sub-request is given its own thread only: the requests the same asking request made of the same team. Of the requests above it, it is given the asking team, the chain and where the chain began, and nothing else.

## Decisions

A decision asks its deciders in order. Each gives `says`, `yes`, `no` or `pass`, with a `note`.

- The first `yes` or `no` is the answer.
- When every decider passes, the answer is `no`.
- A decider that fails fails the decision: a failed heed or check fails the request, and a failed `decide` approves nothing.
- Each decider is given the notes of those before it. People see them above `yes` and `no`.
- The people at the end of a list are asked together, and the first of them to answer gives the answer.
- A worker passes on a job its own machine holds: whatever it says, its word is a pass with the note "The held job is its own."

What a decider is given:

| Decision | `request` | `said` | `answer` | `question` | `jobs` | `notes` |
|---|---|---|---|---|---|---|
| heed | yes | the message | | | | yes |
| check, on an answer | yes | | the answer | | | yes |
| check, on a question to a team | yes | | | `{ team, said }` | | yes |
| decide | yes | | | | Corbie's held jobs | yes |

`notes` is a list of `{ kind, name, says, note }`, one for each decider before it: `kind` is `rule` or `worker`, `name` a rule's path from the office's top, such as `./payments/no-account-numbers.js`, or a worker's name.

What a decider gives back:

| Decider | Gives |
|---|---|
| a rule | `{ says, note }` |
| a worker | its `reply`, `{ says, note }` |
| people | `yes` or `no`, from whoever answered first |

```js
// small-orders.js: each held job is Corbie's: its name, its input, and for an agent's tool call, asked.args
const pounds = input.jobs.map(j => (j.asked?.args ?? j.input).pounds ?? Infinity)
if (pounds.every(p => p <= 20)) return { says: "yes", note: "Under £20." }
return { says: "pass", note: "More than I order alone." }
```

Bulldog writes each decision as a statechart in place, as payments' heed and check above: one state for each decider before the people, `d1`, `d2` and on, then `people`, then the final states `said_yes`, `said_no` and `passed`. The charts are named `<team>-heed`, `<team>-check`, `<team>-question` for the check on a question, and `<team>-decide`. A rule that is a script runs with no permissions.

The people are asked with `wait.yaml`, given `decision`, the name of the decision, and the choices `yes` and `no`. Its `said` is what is decided, then a line for each decider before them, such as `./small-orders.js passed: More than I order alone.`

The heed rules Bulldog ships, `mentioned` and `for-us`, answer `yes` or `no`, never `pass`.

## Held jobs

The request machine's `ask:` is a flow written in place, given the jobs Corbie holds for approval in one step, and `asking`, the machine that holds them.

- A held job that asks a team, the talk tool `ask_team`, goes to the `check` decision, on its question. Without a check, it is approved at once.
- Every other held job goes to the `decide` decision. Without a `decide`, the team's people are asked.
- The flow approves the held jobs of each part that said yes.
- A question the check refused gives the tool "Payments did not let its question out.", and a `decide` that said no gives its note.

This is payments' `ask:`, in the same file:

```yaml
ask:
  input: {request: "${input.request}"}
  machine:
    flow: payments-approval
    input: {request: map, asking: map, jobs: list}
    stages:
      check:
        each: "${input.jobs.filter(j => j.input?.to?.team)}"
        job:
          input: {request: "${input.request}", asking: "${input.asking}", question: {team: "${item.input.to.team}", said: "${item.input.said}"}}
          machine:
            statechart: payments-question
            input: {request: map, asking: map, question: map}
            initial: d1
            states:
              d1:
                job:
                  script: ./payments/no-account-numbers.js
                  input:
                    request: "${input.request}"
                    question: "${input.question}"
                    notes: []
                on:
                  done:
                    - {go: said_yes, when: "${event.data?.says == 'yes'}"}
                    - {go: said_no, when: "${event.data?.says == 'no'}"}
                    - people
              people:
                job:
                  machine: ./.bulldog/wait/wait.yaml
                  input:
                    to: {people: [lee, priya]}
                    said: "May Payments ask ${input.question.team} this?\n\n${input.question.said}\n\n./payments/no-account-numbers.js passed${states.d1.output?.note ? ': ' + states.d1.output?.note : '.'}"
                    choices: ["yes", "no"]
                    decision: check
                on:
                  done:
                    - {go: said_yes, when: "${event.data.answer == 'yes'}"}
                    - said_no
              said_yes: {end: done, output: {says: "yes", note: "${event.data.reply?.note ?? event.data.note ?? null}"}}
              said_no: {end: done, output: {says: "no", note: "${event.data.reply?.note ?? event.data.note ?? null}"}}
      decide:
        when: "${input.jobs.some(j => !j.input?.to?.team)}"
        job:
          input: {request: "${input.request}", asking: "${input.asking}", jobs: "${input.jobs.filter(j => !j.input?.to?.team)}"}
          machine:
            statechart: payments-decide
            input: {request: map, asking: map, jobs: list}
            initial: people
            states:
              people:
                job:
                  machine: ./.bulldog/wait/wait.yaml
                  input:
                    to: {people: [lee, priya]}
                    said: "May these run?\n\n${input.jobs.map(j => j.name + ': ' + JSON.stringify(j.asked?.args ?? j.input)).join(`\n`)}"
                    choices: ["yes", "no"]
                    decision: decide
                on:
                  done:
                    - {go: said_yes, when: "${event.data.answer == 'yes'}"}
                    - said_no
              said_yes: {end: done, output: {says: "yes", note: "${event.data.reply?.note ?? event.data.note ?? null}"}}
              said_no: {end: done, output: {says: "no", note: "${event.data.reply?.note ?? event.data.note ?? null}"}}
    output:
      approve: "${[...stages.check.output.filter(c => c.output?.says == 'yes').map(c => c.item.name), ...(stages.decide.output?.says == 'yes' ? input.jobs.filter(j => !j.input?.to?.team).map(j => j.name) : [])]}"
      note: "${[stages.check.output.some(c => c.output?.says != 'yes') ? 'Payments did not let its question out.' : null, stages.decide.output?.says == 'no' ? stages.decide.output.note : null].filter(n => n).join(' ') || null}"
```

The ledger asks platform "Is the charge service up?". Corbie holds the tool's job, the ask's `check` stage runs `no-account-numbers.js` on the question, which says yes, and the ask approves `ask_team`. Asked about account 12345678, the same rule says no, and the ledger is told `denied: the ask did not approve the job: Payments did not let its question out.`

## Waiting on a person or a team

Bulldog ships `wait.yaml`. It waits on its ticket `answer`, and ends with the answer, or after 7 days with no answer.

```yaml
statechart: wait
input: { to: map, said: string, choices: "list?" }
initial: waiting
states:
  waiting:
    ticket: answer
    timers: { too_long: 7d }
    on: { answer: answered, too_long: unanswered }
  answered: { end: done, output: "${event.data}" }
  unanswered: { end: done, output: { answer: null, note: "No answer came within 7 days." } }
```

`to` is `{ team }` or `{ people: [...] }`, and a decision's people are given `decision` too. A worker uses it through the talk tools, by naming `./.bulldog/tools/talk/talk.yaml` in `tools_file`:

| Tool | What it does |
|---|---|
| `ask_team(team, question)` | asks another team and waits for its answer; marked for approval, so it passes the team's `check` first |
| `ask_person(who, question, choices)` | asks a person and waits for the answer; `choices`, up to 8 short answers, are offered as buttons |

The same `wait.yaml` asks the people at the end of a decision, and is the work of a team whose lead is a person.

Bulldog follows every run labelled `bulldog=<office id>`, those that have ended among them, and acts on each run of `wait` as it changes:

- The asking team is the run's `team=` label, which it carries from its request, and the chain is the request's `chain`, which Bulldog wrote. Bulldog never takes either from what a worker wrote.
- **To a team**, Bulldog starts a request of that team, named from the wait run. When the sub-request ends, Bulldog answers the ticket with `{ answer }`, the sub-request's answer, or null when it gave none. When the wait run ends first, because its request was stopped, Bulldog cancels the sub-request.
- An ask to the asking team itself, to a team in the chain, or to a team that takes no requests is refused: Bulldog answers `{ answer: null, refused }`, and starts nothing. `refused` says why, then the teams it may ask: "The team office asked this request, so it is not asked again. It may ask platform."
- **To people**, Bulldog carries the question to each person, in their direct messages on each bridge whose out takes `direct`, with the choices as buttons, and shows it on the screens. The message is "Payments asks:", a blank line, and the question. A question or a decision is never carried to a channel.
- A person may be asked only when they are of the asking team or named in its team file. Otherwise the ask is refused as above, naming the people it may ask, and nobody is asked.

Bulldog answers a ticket only when all of these hold:

1. The answerer is a person the wait waits on: by their identity on the bridge it came by, or signed in on the screens.
2. The ticket still waits.
3. For a decision, the answer is one of the choices offered.

The answer is `{ answer, by: <person>, thing: <the thing's id> }`, with no `thing` on the screens. A guest never answers.

## Threads

Requests form a thread when they were asked at the same `where` on a bridge, in one chat on the screens, or by one request of the same team. A request is given the requests before it in its thread, what each said and what it answered, as its output gave it, each at the time it was asked. Requests of a thread start at once and run side by side. A routine's request is a thread of its own.

## Routines

Bulldog keeps one run of the shipped `clock.yaml` for each routine of every team that takes requests, labelled with a keyed hash of the team, the routine and the office's zone.

```yaml
statechart: clock
keep: 12h
input: { at: string }
initial: waiting
states:
  waiting: { timers: { due: "${input.at}" }, on: { due: come } }
  come: { ticket: came, on: { came: waiting } }
```

1. Bulldog starts the clock with `at`, the routine's time and the office's zone: `mon 09:00 Europe/London`.
2. At that time the clock moves to `come` and waits on its ticket `came`.
3. Bulldog answers `came`, and the clock waits for the next time.
4. Bulldog starts a request of the team, its `said` the routine's `say`, named from the clock run and the step that took `came`.

A time that came while Bulldog was stopped is acted on once. When a routine changes or is removed, Bulldog cancels its clock, and starts a new one for the changed routine. A second clock of one routine is cancelled.

The step that took `came` carries the time Bulldog answered it. The step the clock fell due in carries the time it was due, days ago after a stop, and Corbie's housekeeping removes what was uploaded for a start named more than a day ago.

## Carrying the answer back

When a request ends, Bulldog carries its answer to its source:

| Source | Where the answer goes |
|---|---|
| a place on a bridge | an out run of that bridge, to the request's `where`, replying to the thing heard |
| the chat | the screens |
| another team | the asking wait's ticket |
| a routine | the team's people, in their direct messages on each bridge whose out takes `direct`, and the screens |

A routine's answer is carried as "Payments, on "What is still open?":", a blank line, and the answer. An empty answer, a noted request and a stopped one say nothing.

## Stopping a request

A person who may see a request may stop it on the screens. Bulldog cancels its run, and Corbie cancels everything below it. A sub-request it asked is cancelled when its wait ends.

## Labels and names

Every run Bulldog starts carries labels:

| Label | On |
|---|---|
| `bulldog=<office id>` | every run |
| `kind=request`, `bridge`, `clock`, `out` or `docs` | every run |
| `team=<team>` | a request, a clock, a docs run |
| `thread=<keyed hash of its thread>` | a request |
| `wait=<the wait run>` | a sub-request, and an out run carrying a question |
| `to=<keyed hash of the person>` | an out run carrying a question |
| `request=<the request run>` | an out run carrying an answer |
| `made=<fingerprint>` | a bridge's in run, a clock |
| `bridge=<name>` | a bridge's in and out runs |

A child carries its parent's labels, so a wait carries its request's `team` and `thread`. Labels and run names hold no words of a request and no plain id of a person. What comes from a service is hashed with the office's secret.

Bulldog names each run it starts in answer to something from that thing, and starts it only when no run has the name:

| Run | Named from |
|---|---|
| a request heard on a bridge | the thing's time and a keyed hash of the bridge and the thing's id (Bridges) |
| a sub-request | the wait run's time and a keyed hash of its name |
| a routine's request | the time of the step that took `came`, and a keyed hash of the clock and the step |
| an out run carrying an answer | the request's end, and a keyed hash of the request, and for a routine of the person and the bridge |
| an out run carrying a question | the wait run's time, and a keyed hash of the wait, the person and the bridge |
| a request in the chat | a new name |

## After a restart

Bulldog follows its runs by their labels: clocks waiting on `came`, waits on `answer`, requests that ended, and sub-requests whose wait ended. After a restart it finds all of them again in the store, and acts on each again. Doing a thing twice changes nothing: a start under a name that exists is dropped, a ticket is answered once, and an out run is named from what it carries.
