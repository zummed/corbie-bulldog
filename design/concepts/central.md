# Roost

Roost is a working name: `corbie-roost` on PATH, run as `corbie roost <directory>`.

## What it is

Roost is one program that serves an office of teams on Corbie and draws it in a browser. A request comes in on a bridge (a Slack thread, a GitLab issue, a routine's clock, the built-in chat) and goes to one team; the team's lead runs as a Corbie machine, held jobs wait for the right yes, and the answer goes back where it was asked. Corbie runs everything; Roost keeps who asked, what was said and who decided.

It keeps Bulldog's six concepts (team, person, worker, bridge, request, decision), its labels, its sight rule, and `ask.yaml`, `call.yaml` and `clock.yaml`. It drops disclosure checks, notes, in-system docs, the builder, `put_draft`, the git history, passwords and the office tools files. A lead may be any machine type, so a fixed pipeline is a flow or a statechart with agents inside it.

Roost keeps:

  requests, entries and their files, decisions with who and when, people's keys and sessions, bridge cursors   `.roost/roost.db`, one SQLite file
  the office's identity, which labels every run                                                              `.roost/identity`
  nothing of a run                                                                                           read from Corbie by labels while `keep:` keeps it
  nothing of a doc                                                                                           read by a worker's tool; what it read is in the run's record

The office is its directory, read whole again when a file changes. Its history is the person's, under git.

## The screens

One page, seven planes; the address names the plane in front.

Depth is one rule, used in two places: what contains the thing in focus is behind it, and what it contains is within it.

  behind   the containing plane, dimmed and blurred 2px, still live, so its counts keep moving; one click brings it to the front
  front    the thing in focus, sharp, at the page's width
  within   cards with a shadow; an opened card grows to the front in 0.18s and the plane goes behind it
  sealed   a team the person is not in: a hatched face with its name, lead and counts, and no inside

The two places are the office (office, team, worker) and the request (request, sub-request, run, child run). Lists and the editor are flat.

**Desk** `/`

- Waiting on you: a card per decision (what it does, the arguments, the walk so far, Approve, Refuse) and per question (choices as buttons, or a text box).
- Your requests, newest first, each with its state.
- Ask: a team, a text, files; it opens the request on the built-in chat bridge.
- Problems with the office's files, shown to the top team's people.

**Requests** `/requests`: rows of number, team, state (`working`, `waiting on Lee`, `answered`, or the issue's own state), origin (`#help`, `platform/api#41`, `chat`, a routine), asker and time, narrowed by team, state, place and `waiting=me`. A person opens one or stops one, which is `corbie run cancel`.

**Request** `/r/230`: the thread in front, each entry a box. Within: a card per sub-request, and the lead's run card: the drawing with the current stage or state lit, each tool call a row (`job refund: a script, on payments-runner`), the model's `text` as it streams, each decision one line ("Refund one charge: yes by Lee, 14:02"). A person answers, decides, says more or stops.

**Office** `/office`: the tree of teams, each a card with name, about, lead, people, the places that ask it, and its working and waiting counts. The top team's card holds the bridges with their places, sign-in and routines. A person opens a team or adds one, which writes `team.yaml`.

**Team** `/t/payments`: people, lead, workers as cards (type icon, model, tools, what each holds for a yes and who decides it), the `decide` lists, the docs source as a link, routines with their next time, problems. A person opens a worker, runs a routine now or edits the team file.

**Worker** `/t/payments/w/ledger`: corbie-ui's editor (problems on their lines, the fields at the cursor from `machine.Describe`), the drawing, the permissions (every `allow:` line, each secret `set` or `not set`, each held tool with its deciders), Test (`corbie test`) and Try (a run with `said`). A person edits and saves once the check passes.

**Store** `/store`: corbie-ui rewritten inside Roost, for the top team's people: every client's runs and the timeline, a run's record and drawing, parties with ceilings and tags, secrets by name, `/metrics`. A person cancels any run (`corbie controller cancel`) or sets a secret from standard input.

## A day at home

1. Sam runs `corbie start controller runner roost ~/house`; the terminal prints `http://127.0.0.1:8373/?key=…`; the Desk shows The house and The garden.
2. Kit writes on Telegram "the socket by the kettle is dead"; request 12 appears on the Desk as `working`; its plane shows the steward's run starting, the Telegram thread behind.
3. The steward calls `fixer` from `crew.yaml`; a child run card opens within the steward's; the fixer calls `sparks`, a card within that, with the row `job circuits: a script, on laptop-runner`.
4. The fixer calls `cut_power`, which has `approve: true`; Corbie holds it and starts `.roost/ask.yaml`; Roost makes a decision for the house's `everything`, Sam; "Turn one circuit off: kitchen ring" appears under Waiting on you, and on Telegram as buttons.
5. Sam presses Approve; Roost runs `corbie run reply <ask-run> <ticket> '{"approve":["cut_power"]}'`; the row goes `running`, then `done`.
6. The fixer calls `order_part` for 14 pounds; `small-orders.js` says `yes`, "Under £20."; the decision line reads "Order a part, £14: yes by small-orders.js".
7. The steward answers; the Telegram out run sends it; request 12 is `answered`; Kit sees the thread; Sam, a house person, also sees what was read.
8. At 18:00 the `bins` routine's clock run fires; the `bins` flow answers "Recycling tonight", sent to Sam and Kit on Telegram as request 13.
9. Sam opens the fixer's plane, the house behind it; he adds "Never open the consumer unit" to `system:`; the check passes; Save writes `fixer.yaml`.

## A day at work

1. A guest writes in #help "my refund for order 8812 has not arrived"; the Slack in run emits it as `heard`; `for-us` notes it is for the office; request 230 to Platform support starts the bot.
2. The bot calls `ask_team` for payments; sub-request 231 opens as a card within 230; the ledger's run starts, its drawing lit at the model.
3. The ledger calls `charges`, allowed `net:ledger.example.com` with `LEDGER_KEY`, then `refund`, held by `approve: true`; the decision goes to payments' `everything`, Lee and Priya, on their Desks and by Slack direct message.
4. Lee presses Approve with the note "charged twice"; the `refund` row runs; the line reads "Refund one charge 8812: yes by Lee, 14:02".
5. The ledger answers 231; the bot answers 230 in the thread; the app's agent status there goes `active`. The guest sees the answer; Dana sees 230's thread; Lee and Priya see what payments read.
6. Raj opens issue 41 in `platform/api`; the GitLab in run hears it within 30 seconds; request 232 to platform starts ops, which calls `gitlab` with `projects/12/pipelines?ref=main` and `read_doc` on `deploys/rollback` in `platform/handbook`; the card shows the doc as read, with its link.
7. Ops answers as a comment on issue 41; 232 shows the issue's state `opened`; platform has no people, so Al sees its run.
8. Priya asks in the chat "refunds by week as a chart"; the bot writes a `chart` fence; the run card draws it as the text streams.
9. Al opens Store: the timeline shows the three requests' runs, the two bridges' listening runs and `live`; `GITLAB_TOKEN` is `set`; one runner tagged `payments`.

## Personal and central

One binary, one office directory, the same planes. What differs is in the office file.

  people    a laptop: the top team has one person, who is in every team, so nothing is sealed; central: `everyone`, each with a bridge identity
  sign-in   a laptop: the key printed at start; central: a key sent over a bridge's direct place, or one shipped OIDC method
  bridges   a laptop: the built-in chat, and Telegram when wanted; central: Slack and GitLab with `places`
  where     a laptop: `corbie start controller runner roost` on loopback; central: `corbie start roost` beside roles under systemd, behind the person's TLS
  store     a laptop: the person; central: the top team's people

A directory with workers and no `everyone` is a personal office, whose one person is whoever runs it; the house becomes central by adding people and bridges.

## Documents outside the system

A team names its docs source in its team file. Roost gives every worker's run `input.docs`, from the nearest team outward.

```yaml
docs: { from: ./.roost/docs/gitlab, site: gitlab.com, project: platform/handbook, ref: main, path: docs }
```

A worker names `tools_file: [./.roost/docs/gitlab/read.yaml]`, which gives `list_docs`, `read_doc(name, from)` and `search_docs(words)`. Each is a script job allowed `["net:${input.docs.site}", "secret:GITLAB_TOKEN@net:${input.docs.site}"]`, as the GitLab bridge is, reading the repository files API at `ref`. `write_doc(name, text, why)` commits on a branch and opens a merge request; the team merges it in GitLab, so nothing holds it. Roost never speaks to GitLab: what a worker read is in its run's record, and the team's people see it there with its link. `./.roost/docs/directory` is the same four tools over `fs.read:${input.docs.path}/**`, for a laptop.

## Live views

A run's card is drawn from its record alone, by `client.Follow`.

  the drawing   corbie-ui's, with `now` from the step's `state`
  tool calls    rows from `asks` of kind `script` and `child`; a child is a card within
  emits         `text` streams into one box; `log` is a line; `progress` is a bar; a `chart` fence in text is a chart, as the Slack bridge draws it
  a held job    a decision card; Approve answers the ask run's ticket `decided`
  a question    `ask_person` with `choices` as buttons; a press answers the call run's ticket `answer`
  the end       `done`; `failed` with `error`; `denied` with `refused`, `by` and `fix`

A person acts back with reply, send and cancel. A worker changes the screen only by emitting a chart or asking with choices; Roost adds no mechanism for it.

## Shipped machines and fixed pipelines

Roost writes `.roost/` whole at every start, each thing with a `README.md` page:

  `ask.yaml`, `call.yaml`, `clock.yaml`, `live.yaml`     as Bulldog ships them
  `bridges/slack`, `gitlab`, `telegram`, `jira`           an in and an out machine each
  `tools/talk`                                            `ask_team`, `ask_person`, `tell`, `read_thread`
  `docs/gitlab`, `docs/directory`                         the four docs tools
  `rules/for-us`, `rules/mentioned`                       heeds
  `signin/entra`, `signin/google`                         sign-in
  `pipelines/issue.yaml`                                  a fixed pipeline

A lead is any machine with `input: { said, context, request }`. A pipeline is a flow or a statechart whose jobs are the team's workers:

```yaml
statechart: issue
input: { said: string, context: string, request: map }
initial: understanding
states:
  understanding:
    job: { machine: ./ops.yaml, input: { said: "${input.said}", context: "${input.context}" } }
    on: { done: fixing }
  fixing:
    job: { machine: ./ops.yaml, input: { said: "Fix it: ${states.understanding.output}" }, approve: true }
    on: { done: answered, failed: answered }
  answered: { end: done, output: "${states.fixing.output}" }
```

A team copies a shipped pipeline into its directory and names its workers. Composition is Corbie's: a stage or a state runs a machine, a tool is a machine, and `approve: true` holds any of them.

## What Corbie gives for free, and what it would have to gain

Free:

- Labels and `run ls --label`, so Roost finds every run after a restart and keeps no list.
- The record's `asks`, emits and `state`, so a run's card and drawing need nothing from the machine.
- Tickets, and `approve: true` with `ask:`, so a decision and a question are runs that wait.
- Permissions with `refused`, `layer`, `fix` and `used`, secrets bound to hosts, and a ceiling per client.
- The check with `file: place: text`, `machine.Describe`, `corbie test`, `cli.Program`, and a viewer for the store plane.

To gain, each general to any client:

- A timer's due time in the head, which holds a count today, so a routine's next time is read.
- A reading of a definition's reach from the check: every job with its hosts, secrets and holds.
- An emit a script can wait on until it is in the record, so a bridge that hears loses nothing.
- A watch that reports a refused start.
- A secret's name from input in `allow:`, so one shipped docs tool serves two GitLabs.
- Runner tags given at a start, so a team's jobs go to its own runners.

## Risks and what is lost

- No disclosure check: payments' `system:` text and its permissions are all that keep an account number inside it.
- No notes: a worker remembers only its docs and the thread.
- A model cannot change the office; a person edits it.
- Docs are read over the network on every call, with one `GITLAB_TOKEN` per office.
- Sealed teams rest on Roost's database, not on Corbie, and every person acts as one Corbie client.
- Records go after `keep:`, so what a team read fades from its planes.
- A hatched plane can read as a fault.
- Roost stays up as a process, so it has no serverless form.
- The office idea takes weight back one setting at a time.
