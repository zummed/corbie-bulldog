# Roost: the person's own harness on Corbie

`roost` is a placeholder name.

## 1. What it is

`corbie-roost` is a Corbie program, started as `corbie start controller runner roost` on the computer that runs Corbie, and reached at `http://127.0.0.1:8374/?token=…`. It is a client of that Corbie through the Go package, as Bulldog is: it starts runs from one directory, finds them by labels, watches them, answers their tickets and drives the bridges.

It keeps nothing of its own. Its state is the directory's files and the runs in Corbie's store, found by the labels `roost=<directory>`, `kind`, `chat`, `schedule` and `machine`; the page's token lives for one start. The directory holds the machines, `roost.yaml`, `notes/`, `drafts/` and `.roost/`.

```yaml
# ~/house/roost.yaml
me: sam
zone: Europe/London
people:
  sam: { telegram: "4471" }
  kit: { telegram: "4472" }
models:
  usual: { name: sonnet, service: claude-code }
bridges:
  telegram: { from: ./.roost/bridges/telegram, places: { direct: { machine: ./steward.yaml } } }
decide:
  everything: [sam]
```

```
  me         the person the page acts as
  people     a name, and an identity per bridge
  models     the one-word model, the one thing written into a definition
  bridges    each place names the machine it starts
  decide     who answers a held job, by the job's name; everything otherwise
```

## 2. The screens

One window holds four lists; a layer opens in front of a list. Depth has one meaning: distance from the root. The list behind a layer stays at 96% size, 2px blur and 60% opacity with its title visible, and a layer behind that at 92%, 4px and 40%.

A child run opens from its job's box in the parent's drawing, and the parent stays behind it, still live. At most three layers stand, and Escape or a click on what is behind brings it forward.

**Now** (`/`):

- Waiting on you lists each held job as a card: its name and input, `asked.does`, `asked.args` and `asked.said`, a note field, Approve and Refuse.
- Running lists each root run: status, type, machine, step, what it waits on (`1 timer`, `ticket decided`), labels and time, with its children under it.
- Today is a timeline with a lane per schedule and chat, a bar per run, and Took for each ended run.
- Ask is a text field and a machine; it starts the run with `--label chat=page`.

**Machines** (`/machines`):

- The tree is the directory: each definition with its type, name, problems, the files it names, the machines that use it, and the schedule that runs it.
- A machine's layer holds the drawing (corbie-ui's `drawing.Draw`), the editor with the check (`machine.Parse`, each problem at its line), the fields at the cursor (`machine.Describe`), each secret as `set` or `not set`, the `used:` of its last run beside each `allow:`, Test, Start, Schedule and Draft, and its runs.
- The shipped things in `.roost/` each have a page, Use and Copy.
- A person edits, checks, tests, starts, schedules, tightens, and puts a draft.

**Run** (`/run/<id>`):

- The drawing is lit by the step's `state`, with a held job's box marked and a child's box raised while its run runs.
- The record is one line per event, job and emitted value, with the model's words from the emit `text` as they arrive.
- Waits, output or `refused`, `by`, `fix`, input, files and `used:` stand beside the drawing.
- A person approves, replies, sends, cancels, and carries a failed agent on.

**Chats** (`/chats`):

- One row per place and person holds the chain of runs under its `chat` label, each with its `said` and reply.
- A person says something, which starts the next run, or stops the chat.

**Setup** (`/setup`): corbie-ui's Controller page, `/metrics`, `corbie check`, and `corbie controller cancel` on any run.

## 3. A day at home, and a day at work

Sam, at home, with `corbie start controller runner roost` running over `~/house`:

1. Sam opens Now: Waiting on you is empty, Running holds `walk-round` (`1 timer`), and Today shows `bins` ran at 18:00 and said "The green bin goes out tonight".
2. Kit writes to the bot on Telegram "the socket by the kettle is dead"; the `in` run hears it, the program starts `steward.yaml` with `said` and the chat's last `conversation`, and Running shows it.
3. Sam opens the run: the drawing lights `turn`, the model's text arrives in the record, and when the steward calls `fixer` its box rises; Sam clicks it, and the fixer's layer comes forward with the steward's behind, dimmed and live.
4. The fixer asks `sparks`, then calls `cut_power`, which has `approve: true`; the controller starts `.roost/ask.yaml`, the box is marked held, and Now shows the card "cut_power `{ circuit: kitchen-ring }`, said: before anything electrical, cut it".
5. The same arrives on Sam's Telegram with two buttons; Sam presses Approve on the page; the program runs `corbie run reply <ask-id> <ticket> '{"approve":["cut_power"],"note":"Sam: yes"}'`, and the record shows `<- decided`.
6. The fixer answers, the steward replies, the program starts `out.yaml` to Kit, and the chat row shows the reply.
7. At 09:00 the timer `due` of `walk-round` fires; `working` runs the steward with "Walk round: what is still open?", `telling` runs `out.yaml` to Sam, and the program takes no part.
8. Sam presses Draft on Machines and types "a bins machine for the blue bin on Thursdays"; `drafter.yaml` writes `drafts/bins.yaml`; the page shows it beside `bins.yaml` with its check, and Sam presses Put.

Al, with `/srv/office` on `desk`, the page over `ssh -L 8374:127.0.0.1:8374 desk`:

1. A customer writes in `#help` "charged twice for order 8812"; the Slack `in` run hears it, and the program starts `bot.yaml` with `chat=slack/#help/<thread>`.
2. The bot calls its tool `payments`, a child run of `payments/ledger.yaml`, drawn as a raised box.
3. The ledger calls `charges` (`net:ledger.example.com`), then `refund` with `approve: true`; `decide:` says `refund: [lee, priya]`, so the program sends each a Slack message with buttons, and Now shows "waiting on Lee or Priya".
4. Lee presses Approve; the `in` run hears the press from `U0LEE000`, and the program answers the ticket with the note "Lee: yes".
5. The refund runs, the bot replies in the thread, and the run ends `done` with `used: net:ledger.example.com:443 net:slack.com:443`.
6. Raj opens a GitLab issue in `platform/api` "pipeline 4471 failed"; the GitLab `in` run hears it, and `platform/ops.yaml` reads the pipeline with its `gitlab` tool and comments on the issue.
7. Dana asks in `#help` what customers are told about refund times; the bot calls `search_docs` of `handbook.yaml`, reads `help/refunds.md` from the GitLab project, and answers with path and line.
8. Priya wants a weekly refunds summary; Al presses Schedule on `ledger.yaml`, fills `fri 16:00 Europe/London`, the text and `priya`, and the page writes `payments/weekly-refunds.yaml`, starts it, and Today gains a lane.

## 4. Personal and central

One program, one directory, one `roost.yaml`. The same `steward.yaml` says `model: usual`, and `roost.yaml` says what `usual` is on this computer. What differs:

```
  Corbie       corbie init --ceiling all                  corbie init store, runners and clients enrolled
  the model    sonnet, service claude-code, signed in     claude-sonnet-5-5, secret ANTHROPIC_KEY
  bridges      telegram, places direct                    slack with guests: true; gitlab with projects
  decide       everything: [sam]                          refund: [lee, priya]
  the page     loopback, one token                        loopback on desk, over SSH or the team's proxy
  the files    Sam's directory                            a checkout of the team's GitLab repository
```

The page is one identity, `me`. Everyone else acts through a bridge, where their identity is the bridge's: a held job goes to its deciders over Slack or Telegram, and the first answer taken ends the ticket. The program has no accounts and no sign-in.

## 5. Documents outside the system

A document is read where it is. The program ships a tools file per kind of place, copied with the place written in:

```yaml
# handbook.yaml, copied from .roost/tools/gitlab-docs.yaml
read_doc:
  does: Read one file of the handbook by its path
  args: { path: string }
  job:
    script: ./.roost/tools/gitlab-docs.js
    input: { do: read, project: platform/handbook, ref: main, path: "${args.path}" }
    allow: [net:gitlab.example.com, "secret:GITLAB_TOKEN@net:gitlab.example.com"]
```

```
  list_docs      GET /api/v4/projects/<id>/repository/tree?recursive=true&ref=main
  read_doc       GET …/repository/files/<path>/raw?ref=main, from a line
  search_docs    GET …/search?scope=blobs&search=<words>, each hit with its path and line
  propose_doc    POST …/repository/commits on a branch, then …/merge_requests; approve: true
```

1. Each call is a job; the token is bound to the host, and the model never holds it.
2. The record holds each path read, and `used:` holds the host.
3. A change is a merge request, reviewed in GitLab.

`dir-docs.yaml` gives the same tools over a directory on the laptop, with `allow: ["fs.read:/home/sam/notes/**"]`. Nothing is copied in or indexed: GitLab's search is the search.

## 6. Live views

Each list is one `client.Watch` with `Labels: roost=<directory>` and `Finished: true`; a part is sent only when its HTML differs, as in corbie-ui's `page.Stream`. A run's layer is one `client.Follow`, and each entry changes the drawing:

- A step's `state` lights a flow's stages (`now`, `done`, `failed`, `skipped`, `never`), a chart's active states, or an agent's turn.
- A step whose `asks` has kind `child` raises the job's box, and an ask run under the machine marks its box held.
- An emit appends `text` to the model's line as it arrives, and writes `log`, `progress` as a bar and `own` as a line.

A person acts back with one command each, shown on hover, answered in Corbie's words (`taken`, `refused: …`):

```
  Approve, Refuse    corbie run reply <ask-id> <ticket> '{"approve":[…],"note":"…"}'
  Reply              corbie run reply <id> <ticket> <value>
  Send               corbie run send <id> <event> [data]
  Cancel             corbie run cancel <id>
  Say                a new run with said and the last run's conversation
  Carry on           a new run with a failed agent's conversation
```

## 7. Shipped machines and fixed pipelines

`.roost/` is written at every start. Each thing has a page, and is named by path or copied.

```
  ask.yaml                               a statechart with ticket decided and timers { unanswered: 7d }
  schedule.yaml                          a statechart: idle waits on due, working runs a machine, telling runs out.yaml
  bridges/telegram                       in.yaml hears from a cursor; out.yaml says text, with choices as buttons
  bridges/slack, gitlab                  the same two machines for a channel and a project's issues
  tools/notes.yaml                       write_note and read_notes over fs.write:${input.dir}/notes/**; every start carries dir
  tools/gitlab-docs.yaml, dir-docs.yaml  section 5
  drafter.yaml                           an agent with read_file over the directory and write_draft over drafts/**
  manual/                                Corbie's manual pages, for the drafter
```

A pipeline is a statechart or a flow whose jobs are machines named by path. Schedule on a machine fills three blanks of `schedule.yaml`:

```yaml
# walk-round.yaml
statechart: walk-round
initial: idle
states:
  idle:    { timers: { due: "mon 09:00 Europe/London" }, on: { due: working } }
  working: { job: { machine: ./steward.yaml, input: { said: "Walk round: what is still open?" } }, on: { done: telling, failed: idle } }
  telling: { job: { machine: ./.roost/bridges/telegram/out.yaml, input: { to: "4471", text: "${states.working.output.reply}" } }, on: { done: idle, failed: idle } }
```

- A chat is a place's machine, started by the program with `said` and `conversation`, then `out.yaml`.
- A decision with a rule first is a flow: stage `rule` runs `./small-orders.js`, stage `person` runs `ask.yaml` with `when: ${stages.rule.output.says == "refer"}`, and the output is the person's or the rule's.
- A draft never runs: the page shows it beside the file it changes, with its check, and Put copies it over.

## 8. What Corbie gives, and what it would have to gain

Corbie gives timers, `approve: true` with `ask:` and `ask_about:`, `allow:` with `used:`, secrets bound to one host, child runs by `machine:`, labels with `run ls --label` and watching, the record with every emit, `conversation`, `claude-code` with no key, `machine.Parse` and `machine.Describe`, and `keep:`. The program adds no mechanism to it.

It would have to gain, each general to any client:

- A run's head gives each timer's due time, so a schedule shows "next at 09:00" without reading the definition.
- The check reports a definition's reach, every job below it with its hosts, secrets and held jobs, which the tree now reads a second way.
- A watch reports a refused start, so a chat that could not start shows as failed and not as silence.
- A listing sums one kind of value over labelled runs' records, such as `tokens`, so Today shows the day's cost without reading every output.

## 9. Risks and what is lost

- The page is one identity, the admin's; everyone else is known only through a bridge, and a pressed button is as good as the bridge's identity.
- There are no walls inside a directory: what `ledger.yaml` read is in the root's record, which the page reads, and Bulldog's sealed teams are gone.
- A chat lives as long as its last run's `keep:`; the thread belongs to Telegram, Slack or GitLab.
- Schedules run without the program; chats and approvals over a bridge need it running, and a shut laptop answers nothing, as with Hermes.
- Test runs scripts on the page's computer as its user, with ceilings allowing everything, as corbie-ui's does.
- Requests, decisions as records, notes as team memory, sign-in and the committed history are lost with Bulldog; the directory's git is the person's.
