# Desk

The least GUI that is still the thing.

## What it is

`corbie-desk` is one program, run as `corbie desk` or beside the roles as `corbie start controller runner desk --dir ~/house`. It is corbie-ui written again with four additions: the directory of definitions is an office, a loop turns what a bridge hears into runs, a reply form follows the ask, and a child run is drawn inside the run that started it. Everything shown is read from the store, and every action is a Corbie command run in the desk's process, shown with its receipt.

It keeps nothing of its own.

  the definitions        a directory, `--dir`, which the person keeps in git or GitLab
  the runs               the store, found by labels
  a request's thread     the bridge's own service: a Slack thread, a GitLab issue, a Telegram chat
  a thread's memory      the last run's `conversation`, read from its result (R-agent-19)
  shipped machines       `.desk/`, written whole at every start, read-only

It has no database, no identity file, no git repository and no list of people. The desk is a client, the controller too where the config holds it, and a viewer as `corbie desk none`.

## The screens

Four screens, one token.

**Runs**

- The list, the timeline and the `now` panel are corbie-ui's, narrowed by `status`, `label`, `client` and `window`, with lanes by machine, client or one label's name. A running child is under its parent.
- Lanes by `thread` make it the request list, and `--label routine` the routines.
- The waiting plane is in front of the list and holds every run whose head names a ticket. For `decided` it shows each held job's `asked: { does, args, said }` with Approve and Refuse; for `answer`, the question with its `choices` as buttons, or a text field.
- Approve runs `corbie run reply <id> <ticket> '{"approve":["refund"],"note":"Lee"}'` with the ticket's value read from the record.

**Run**

- The line: machine, type, status, ID, step, started or ended, and labels as a path, `slack / #help / 1700000000.000200`.
- The drawing with its state lit, as corbie-ui draws it; an agent's model is lit during a turn.
- A child run is drawn inside the job's box of its parent at a quarter of its size, lifted by a shadow. Opening it brings it to full size; the parent stays behind, dimmed and blurred, its lit state still readable.
- This is the one place depth is used: nearer is deeper in the run tree, and what is behind is what started it. Nothing else is layered.
- Record, Output, Definition, Files as downloads and Children, as corbie-ui has them.
- Actions: Reply, Send, Cancel, and Carry on, which starts the definition again with `conversation:` from this run's result and a new prompt.

**Machines**

- The directory as corbie-ui lists it: each definition with its type, count and problems, and the files it names. A directory within is a team.
- The editor, the check, the fields at the cursor from `machine.Describe`, the drawing, and Secrets as `set` or `not set`.
- `.desk/` is listed under the directory, read-only, with each README. Copy writes a copy into a chosen directory.
- Test, and Start with input, labels, `--secret-env`, `--allow` and `--timeout`. A routine begins as Start with `--label routine=walk-round`.

**Controller**

- Runners, clients and secrets as corbie-ui shows them, and `corbie controller cancel <id>` on any run.

## The loop

As a client the desk runs one loop over the store at `--interval`, with three rules. It reads `bridges/<name>/settings.yaml` in the directory, whose `places` name a machine each: `"#help": { machine: ../../help.yaml }`.

1. One `in` run per bridge: it starts `bridges/slack/in.yaml` with `input.cursor` from the last `in` run's output and labels `bridge=slack kind=in`, and starts the next when it ends.
2. One run per heard item: for each `heard` emit of an `in` run with no run labelled `heard=<its stamp>`, it starts the place's machine with `input.heard`, labels `bridge=slack place=#help thread=<ts> heard=<stamp> team=<directory>`, and `conversation` from the thread's last `done` run. While a run of the thread runs, the next waits.
3. A heard press or reply carries `ref`, a ticket's value: it runs `corbie run reply` on the run waiting on that ticket.

On a restart the loop reads the store and goes on: a run is found by its label, and an answer sent twice gets the same receipt.

## A day at home

`~/house` holds the house's workers, `garden/`, `bridges/telegram/`, `direct.yaml` (a copy of `pipeline.yaml`: the steward, then Telegram out), `decide.yaml` (a copy with `small-orders.js` as its `rule` state) and `walk-round.yaml`. Sam ran `corbie init --ceiling all` and `corbie start controller runner desk --dir ~/house`.

1. Kit writes to the house's Telegram bot: "The kitchen sockets are dead." Runs shows `telegram-in` running in lane `telegram`.
2. The loop starts `direct.yaml` with labels `bridge=telegram place=direct thread=4472 heard=…`; a row `running statechart direct` arrives in lane `4472`.
3. `direct` enters `answering` and starts the steward; on Run, `answering` is lit and the steward's drawing is inside it with its model lit.
4. The steward calls `fixer`, and the fixer calls `sparks`; each is a smaller drawing inside the one before. Sam opens the fixer, and the steward dims behind it.
5. The fixer calls `cut_power`, marked `approve: true`; the controller holds it and starts `decide.yaml`; `rule` runs `small-orders.js`, which approves only an order, so `asking` issues `decided` and `telling` sends Sam a Telegram message with Approve and Refuse.
6. The waiting plane shows "cut_power: Turn one circuit off. circuit: kitchen ring" and what the fixer said, with Approve and Refuse.
7. Sam presses Approve; `telegram-in` hears the press; the loop runs `corbie run reply <id> <ticket> '{"approve":["cut_power"],"note":"Sam"}'`; the plane empties and `decided` lights.
8. `cut-power.js` runs and its `log` line appears in the record; the fixer and the steward answer; `direct` enters `saying` and runs `bridges/telegram/out.yaml`; the row settles `done`.
9. On Monday at 09:00 the timer `due: "mon 09:00 Europe/London"` of `walk-round.yaml` fires; `working` lights, the steward runs inside it, `telling` sends Sam the round, and the chart returns to `waiting`.

## A day at work

The office is one desk, `corbie desk --dir /srv/office --listen 0.0.0.0:8373 --token-file /etc/desk/token`, the client `office` on a bucket. `/srv/office` holds the work office's workers, two bridges, `help.yaml`, `api-issue.yaml`, `payments/decide.yaml` and `docs.yaml`. Al's desk is `corbie desk none`.

1. Dana writes in #help: "Customer 4411 says they were charged twice on Tuesday." Runs shows `slack-in` running in lane `slack`.
2. The loop starts `help.yaml` with labels `bridge=slack place=#help thread=1700000000.000200 team=office`; `answering` runs `bot.yaml`.
3. The bot calls `search_docs("charged twice")`; the Record shows `job search_docs: a script, on srv-runner`, and `used` gains `net:gitlab.com:443`.
4. The bot calls `ask_team(payments, …)`, a tool whose job is `machine: ./payments/ledger.yaml`; the ledger's drawing appears inside the bot's. Lee opens it and the bot dims behind.
5. The ledger calls `charges`, then `refund(ch_88, duplicate of ch_87)`, marked `approve: true`; the controller starts `payments/decide.yaml`, whose `telling` posts to #payments "Refund one charge: ch_88, duplicate of ch_87" with Approve and Refuse.
6. The waiting plane shows the same, and the row reads `running agent ledger  ticket decided`.
7. Lee presses Approve in Slack; `slack-in` hears it; the loop replies; `ledger.js` refunds, and `used` gains `net:ledger.example.com:443`.
8. The ledger answers; the bot replies with a `chart` fence, which Run draws as a chart; `help` enters `saying` and `bridges/slack/out.yaml` posts to the thread; the row settles `done`.
9. Dana asks in the thread: "Can it go to the same card?" The loop starts `help.yaml` again with `conversation` from the last run; lane `1700000000.000200` gains a second bar.
10. Raj opens an issue in `platform/api`; `gitlab-in` hears it; the loop starts `api-issue.yaml`, which runs `platform/ops.yaml` and answers in the issue.
11. Al sees every client's rows with lanes by `team`, the counts, and `/metrics` for Prometheus, and reads no record.

## Personal and central

One program and no mode flag: what differs is the config and three flags.

  the store        `~/.config/corbie/store`                      a bucket or a served directory
  parties          `corbie init`: client, controller and runner   a client per desk, enrolled with a ceiling; runners where the work is
  the desk         loopback, the token in the printed address    `--listen` beyond loopback with `--token-file`, behind a proxy that gives TLS; no Test
  sight            everything, as client and controller          a desk reads its own client's runs; `corbie desk none` reads every head
  the directory    a directory in Sam's home                     a clone of a GitLab repository, pulled by `pull.yaml`
  history          the directory's git                           GitLab

Sealing is a client: one desk per team gives each team its own records, and work between two desks goes over a bridge, as a request from outside does.

## Documents outside the system

A GitLab documentation repository is read by its API from a tools file, `docs.yaml`, copied from `.desk/docs/gitlab.yaml` with the project written in.

  list_docs      the repository tree of `platform/docs` at `main`
  read_doc       one file's raw text, up to 20,000 characters, from a line on
  search_docs    GitLab's search with `scope=blobs`, each line with its file
  write_doc      a commit on a new branch and a merge request, `approve: true`

Each is a script with `allow: [net:gitlab.com, "secret:GITLAB_TOKEN@net:gitlab.com"]`. The agent is given text, never the repository. At home, `notes.yaml` reads `notes/` with `allow: [fs.read:/home/sam/house/notes/**]` on the laptop's runner. The definitions can live in GitLab the same way: `pull.yaml` runs `exec.run("git", { args: ["pull"], dir: "/srv/office" })` with `allow: [exec:git]`.

## Live views

- One `client.Watch` per interval and one `client.Follow` per open run feed the pages; parts are sent by id when their HTML differs, as corbie-ui does.
- Emits are drawn by name: `log` a line, `progress` a bar, `text` the model's words as they stream, `chart` a chart from `{ type, title, categories, series }`.
- An agent changes what is shown only by what it emits, and no run can change a page.
- Acting back is Reply, Send, Cancel and Carry on, each shown with the command it runs and the receipt it got.

## Shipped machines and fixed pipelines

`.desk/` is written at every start, each thing a directory with a README. Copy is the one way to use one.

  pipeline.yaml      a chart: `answering` runs a lead, `saying` runs a bridge's out, `sorry` says that it failed
  decide.yaml        the ask: `rule` runs an ask script, `asking` issues `decided` for 7d while `telling` posts it on a bridge with `ref: ${tickets.decided}`, `unanswered` approves nothing
  ask-person.yaml    a tool's machine: `asking` issues `answer` while `telling` posts the question with `choices`
  routine.yaml       a chart: `waiting` on `timers: { due: "mon 09:00 Europe/London" }`, `working` runs a machine with `said`, `telling` posts the answer
  pull.yaml          a chart on a timer that runs `git pull` in the directory
  bridges/           `slack`, `telegram`, `gitlab`, `jira`, as Bulldog ships them
  docs/gitlab.yaml   `list_docs`, `read_doc`, `search_docs`, `write_doc`

A fixed pipeline is a chart whose states run machines in order, each with its own `allow:`, and a model runs only where a state runs an agent. Composition is one mechanism, `job: { machine: … }`: a tool, an ask, a routine and a bridge are each machines. There is no `call.yaml`, because no tool waits on the desk.

## What Corbie gives for free

- Heads carry labels and ticket names, so requests, routines and decisions are found by `run ls --label` and need no table.
- A ticket is the whole right to answer, so a decision is a reply, from the screen or from a bridge.
- The ask gives `asked: { does, args, said }`, so the reply form needs no definition of its own.
- Child runs, `conversation`, permissions with `used`, secrets, the check, `machine.Describe` and `cli.Program` are used as they are.

It would have to gain two things, each general to any client.

1. A child run that a chart starts and does not wait for, with labels of its own. With it, `in.yaml` restarts itself and starts a pipeline per heard item, and the loop keeps only its third rule.
2. The time a timer is due, in the head or in the step's `state`, so a routine's row can say when it next runs.

## Risks and what is lost

- There are no people: the token is the person. A shared desk is a team, and who decided is the `note`, the answerer's word.
- Teams in one desk are not sealed; sealing costs a desk and an enrolled client per team.
- There is no history with who and why: Save writes the file, and the directory's git is the history.
- A thread's memory is a run's result, which goes after `keep`; an older thread starts afresh, and the bridge's service still holds its text.
- A ticket's value goes out as a button's `ref` through Slack or Telegram; whoever reads it answers once.
- A job's input reading its holder's ticket, as `telling` does, is confirmed only for entry sends (C-chart-05) and must be tested before `decide.yaml` ships.
- A person who writes twice while a run runs is answered twice, in order; a run is never interrupted.
- Depth is the run tree and nothing else; rooms, walls and faces are gone.
