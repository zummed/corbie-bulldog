# Perch

`corbie-perch` is one program, run as `corbie perch --dir ~/house`. It is a client of Corbie, and acts as the controller where the config has the controller's keys; the roles run with `corbie start`. Every action is a Corbie command run in its process, shown with what the controller answered.

- It shows a directory of definitions, the runs they make, and what waits on a person.
- It drives bridges with a loop over the store.
- It opens a child run in front of the run that started it.

It keeps nothing of its own.

```
  the definitions      the directory, which the person keeps in git or GitLab
  the runs             the store, found by the client's name and the labels bridge, place and thread
  a thread             the bridge's own service: Slack, GitLab, Telegram
  a thread's memory    the last run's conversation, passed as input
  shipped machines     .perch/
  the token            printed at start, valid until the perch stops
```

The word names the program; `.perch/` is named after it.

## The ideas a person learns

- A machine is a YAML file in the directory. New copies a template, `pipeline.yaml`, `decide.yaml` or `routine.yaml`, into the directory under the name typed.
- A run is one execution, found by its labels, kept for its definition's `keep:` after it ends.
- A bridge is a shipped directory, `.perch/bridges/<name>/`, of the machines `in.yaml`, `out.yaml` and `ask.yaml`. The person's `bridges/<name>/settings.yaml` is the input of its `in` run: it names a machine per place; a place is one chat, channel or project the bridge reads. A thread is one conversation in a place, named by the bridge's own id for it, and carried as the label `thread`; a run of a thread starts with the last done run's `conversation`.
- A held job waits until its ask, a `decide.yaml`, approves it; the ask's answer is a reply on the ticket, from the screen or from a bridge.
- A model is written as Corbie writes it, with its key as a secret; the perch names no model.
- A view is what a run emits, drawn by kind.
- A perch is one client, or with `none` a viewer on a bucket or a directory that reads every run's status and no record. A client reads its own runs whole, and every run's status where the store lets a viewer read, as on a directory; work between perches goes over a bridge.

## The screens

There are five screens, and one token opens them all. A screen is narrowed by the parameters of its address.

**Runs** `/runs`

From top to bottom it shows the counts and the `now` panel, the standing row, the newest failure line, the timeline and the list.

- The list and timeline are corbie-ui's, narrowed by `status`, `label`, `client` and `window`, with lanes by machine, client or a label's name; `failure` is added as a narrowing. The `now` panel is corbie-ui's, with a count per `failure` added and the running count in `--ink` and not the gradient.
- The standing row holds one chip per bridge named in `bridges/<name>/settings.yaml`: the `in` run's status dot, machine and what it waits on, as a row prints them, or `telegram · no in run`. A chip opens the `in` run as a row does.
- The newest failure line holds the latest run that failed in the window and whose parent did not fail, as its status dot, machine, `failed (denied)` and its labels as a path, then `+N` for the others; it opens the root as a row does, and is absent at zero.
- A row carries the labels as a path: `slack / help / 1700000000.000200`.
- The waiting plane lists every run that waits on a ticket, each card headed by the root's labels as a path, `telegram / direct / 4472`, and the chain from the root to the run that holds the job, `direct › steward › fixer`, read from the heads' `labels` and `parent`.
- A `decided` card shows each held job as its `asked: { does, args, said }` for a tool call, else its name and input, then a note field, Approve and Deny, and under them the reply each sends, that a job not approved fails `denied`, and the time left on the ticket, `6d 23h`, read from the issuing run's `wake` ask whose `what` begins `timer.`.
- Any other ticket's card shows a value typed as JSON.
- Approve runs `corbie run reply <ask-run> <ticket> '{"approve":["refund"],"note":"Lee"}'`, with the note typed on the screen. The perch reads the ticket's value from the ask run's record, which it may read as the root's client.

**Run** `/run/<id>`, top to bottom:

```
  chain row     32px, help › bot › ledger
  parent band   36px, steward · answering · running
  drawing       at most 40% of the height, the current state highlighted, chips over job boxes
  replay bar    24px, a range over the front run's steps
  failure row   one line, on a failed run
  record        the rest, scrolling
  tabs          Output, Definition, Files, Children
  actions       the chain row's right end: Cancel, Send, and Carry on on a done root with the label `thread`; the command and the receipt print under the pressed button, as What games teach says; Reply is the card on the waiting plane
  right column  360px, the waiting plane
```

- The chain row has one chip per run from the root, rebuilt from each run's `parent` so a reload keeps it from one id.
- The drawing is corbie-ui's at full width. Each job box's second line is the latest fact the record holds for the job, and the box is as wide as the longer of its name and that line; a job that is a child run carries a chip (status dot, name, and its last step's `state`, or `turn N` for an agent, read from the child's record) in its box instead. Pressing a box opens the job's sheet under the drawing. A timer's arrow is the only dashed stroke: `when:` is a label on its stage, and `skipped` a `--line` stroke.
- The parent band is drawn by the page: the parent's name, status and highlighted state. Below the band the parent's drawing is the behind plane.
- The record is as corbie-ui prints it, the model's `text` streamed into the turn's row and views drawn in place. At a step set by the replay bar it ends at that step with `12 more, to 09:14:01`.

Actions on a root, each one Corbie command:

```
  Reply           corbie run reply
  Send            corbie run send, on a statechart root; a flow and an agent ignore events
  Cancel          corbie run cancel, which ends every run below
  Carry on        starts a run of the thread as step 2 of The loop does, with `said` typed on the screen in place of a heard message, and waits while a run of the thread runs
```

A child's page shows the root's actions: what a child does is sent to its parent as an event, and a reply goes to the run that issued the ticket, so the perch acts on the root alone.

**Machines** `/machines`

- The directory as corbie-ui lists it: every definition there or below it, with its type, problems and the files it names. `.perch/` is listed as well, read-only, with each README and its Changed section whole.
- The editor, the check, the fields at the cursor, the drawing and Secrets are corbie-ui's. Beside the `allow:` lines, the page lists the last run's `used`.
- After New, the template's README Settings section says which lines to change, and the check reports what is still blank. Test and Start are corbie-ui's.

**Controller** `/controller`: runners, clients and secrets as corbie-ui shows them, and `corbie controller cancel <id>`.

**Metrics** `/metrics`: `metrics.Text` as corbie-ui prints it, whatever the perch acts as.

### Depth

There are two planes, built with CSS alone, and each has one meaning.

```
  behind    the run that started the one in front, at scale(.94) from top centre and opacity .55, its top 24px showing above the front drawing
  in front  what waits on a person, from any run shown, on --plate2 with box-shadow 0 8px 24px rgba(0,0,0,.18)
```

- In the dark theme the waiting plane has no shadow and is raised by `--plate2`, two steps above the page's background.
- A child is a chip, not a drawing: a quarter-size drawing is 3px text, and 1/64 at the third level.
- Chips are HTML placed over the SVG by `getBBox()` of `g[data-job]`, an attribute added to each box in `drawing/`.
- At 1200px and wider the waiting plane is a 360px right column; narrower, it is above the list on Runs and above the drawing on Run.
- On Run the waiting plane lists only the runs below the root shown.
- When a child is opened from its chip, the drawing on top scales to .94 and dims over .32s with `--ease`, the child's drawing comes in from scale(1.04) and opacity 0 to its place, and a chip is added to the chain row; Back reverses it. A waiting card collapses by `grid-template-rows` from `1fr` to `0fr` over .24s, `place` keeping it until the collapse ends, and the cards below move up by layout. Under `prefers-reduced-motion` nothing is animated.

## The loop

One loop over the store, every `--interval`, reads `bridges/<name>/settings.yaml`, whose `places` name a machine each, and each time does the three steps below.

1. The loop starts one `in` run per bridge, `.perch/bridges/<name>/in.yaml`, with `settings` and `cursor` from the last done `in` run's output, labelled `bridge=<name>` and no `place`, follows its record, and starts the next when it ends.
2. The loop starts one run per heard message: the place's machine, under the name `<client>/<stamp>-<id>`, the stamp from the message's time and the `id` from the thing the `in` run emitted as `heard` or ended with, with `said`, `thread` and `conversation` from the thread's last done run; a name already in the store is not started again, and while a run of the thread runs the next waits.
3. For a heard press, which carries `answers` (the `ref` the out machine was given, a run id) and `from` (the presser's id), the loop reads the runs below `answers` until one waits on a ticket, reads `people` from that run's input, and when `from` is in `people` reads the ticket's value and replies `{ approve: [every held job's name], note: <from> }` for Approve and `{ approve: [], note: <from> }` for Deny. When no run below `answers` waits on a ticket, the ticket was answered, and the press is dropped.

After a restart the loop reads the store and continues.

## A day at home

`~/house` holds:

```
  steward.yaml, crew.yaml, fixer.yaml, sparks.yaml, clerk.yaml, builder.yaml, bins.yaml, tools/, garden/   the house from Bulldog's example
  bridges/telegram/settings.yaml   places: { direct: { machine: ./direct.yaml } }
  direct.yaml      New from pipeline.yaml
  decide.yaml      New from decide.yaml
  walk-round.yaml  New from routine.yaml
  notes/           read through docs/dir.yaml
```

- The agents are without the talk and notes tools, and each names `ask: { machine: ./decide.yaml }`.
- `direct.yaml` runs the steward, then Telegram's `out.yaml`, with `ask: { machine: ./decide.yaml }`.
- In `decide.yaml`, `rule` runs `small-orders.js`, which reads `input.jobs[0].asked.args.pounds`, and `person` runs Telegram's `ask.yaml` with `people: [4471]`.
- In `walk-round.yaml`, `alarm` is `mon 09:00` and `working` runs the steward.

Sam ran `corbie init --ceiling all`, `corbie controller secret set ANTHROPIC_KEY`, `corbie controller secret set TELEGRAM_TOKEN`, `corbie start` and `corbie perch --dir ~/house`, and pressed Start on `walk-round.yaml`, which runs until cancelled. Runs shows one row, `walk-round` running in the last lane, `1 timer, next mon 09:00`; Machines lists steward, fixer, sparks, clerk, builder, bins, direct, decide, walk-round, gardener and groundsman, `crew.yaml` under the steward's `tools_file:`, and `.perch/` read-only.

1. Kit writes to the house's bot: "The kitchen sockets are dead." Runs, with lanes by `thread`, shows `telegram-in` running in the last lane, which holds runs with no thread.
2. The loop starts `direct.yaml` under `<client>/<stamp>-<id>`, labels `bridge=telegram place=direct thread=4472`; a row appears in lane `4472`.
3. Run shows `answering` highlighted and a chip `steward · turn 1` in its box; the model's words stream into the record.
4. The steward calls `fixer`, the fixer calls `sparks`; Sam opens the steward from its chip, then the fixer from the `fixer · turn 1` chip in the steward's tool box: the chain shows `direct › steward › fixer`, the steward dims behind, and `sparks · turn 2` is a chip in the fixer's tool box.
5. The fixer calls `cut_power`, `approve: true`; the controller holds it and starts `decide.yaml` as a child; `rule` runs `small-orders.js`, which refers; `person` runs Telegram's `ask.yaml`, whose inner state `telling` posts Approve and Deny to `4471`, `ref` the fixer's run id. The fixer's drawing shows `cut_power` reading `held` and `decide · person` as a chip in its ask box; the chain shows `direct › steward › fixer › decide` when Sam opens it.
6. The waiting plane shows `cut_power: Turn one circuit off · circuit: kitchen ring` with what the fixer said.
7. Sam presses Approve on Telegram; `telegram-in` hears the press from `4471`; the loop replies, as step 3 of The loop says; the plane empties, `tools/cut-power.js` runs, and its `log` line appears in the fixer's record.
8. `order_part` for 12 pounds is held; `small-orders.js` approves it, `decide` ends `done` with `Under £20.`, and no message goes out. The plane stays empty; the fixer's record shows `decide: done · Under £20.` and `tools/order.js` runs.
9. The steward answers; `saying` runs `out.yaml`; `answered` ends with `conversation` in its output; the row shows `done`.
10. Kit writes "And the hall?"; the loop starts `direct.yaml` with `conversation` from the thread's last done run; a second bar appears in lane `4472`.
11. Monday 09:00: the timer `alarm` of `walk-round.yaml` is due, whether the perch runs or not; `working` runs the steward, `telling` posts the round to `4471`, and the row shows `next mon 09:00` from the step's `wake` ask.

## A day at work

`/srv/office` is a clone of the office's GitLab repository: the agents, two bridges, `help.yaml` and `api-issue.yaml`. The perch runs beside the roles on the server as the client `office`; Al opens it over `ssh -L 8373:127.0.0.1:8373`.

```
  bridges/slack/settings.yaml    places: { help: { machine: ./help.yaml }, payments: {} }
  bridges/gitlab/settings.yaml   places: { platform/api: { machine: ./api-issue.yaml } }
  bot.yaml                       tools_file: [./.perch/docs/gitlab.yaml], input: { docs: { site: gitlab.example.com, project: docs/help } }
  platform/ops.yaml              the same, project: platform/runbooks
  help.yaml, api-issue.yaml      New from pipeline.yaml
  payments/ledger.yaml           ask: { machine: ./decide.yaml }
  payments/decide.yaml           New from decide.yaml; its `rule` refers everything, and `person` runs Slack's `ask.yaml` with `people: [U0LEE000, U0PRIYA0]` and `place: payments`
```

The controller holds `ANTHROPIC_KEY`, `SLACK_TOKEN`, `SLACK_APP_TOKEN` and `GITLAB_TOKEN`.

1. Dana writes in #help: "Customer 4411 says they were charged twice on Tuesday." Runs shows `slack-in` running.
2. The loop starts `help.yaml`, labels `bridge=slack place=help thread=1700000000.000200`; a row appears in lane `1700000000.000200`; `answering` runs `bot.yaml`.
3. The bot calls `search_docs("charged twice")`; the record shows `job search_docs: a script, on srv-runner`.
4. Al opens the row, then the bot from its chip; the bot calls `ask_payments`, a tool whose job is `machine: ./payments/ledger.yaml`, and `ledger · turn 1` is a chip in the tool's box.
5. The ledger calls `charges`, which emits a `view` of kind `table`, drawn in the record: `ch_87 · £42.00`, `ch_88 · £42.00`; then `refund(ch_88)`, `approve: true`; `decide.yaml` refers, and `ask.yaml` posts to #payments with Approve and Deny.
6. The waiting plane shows the same; the row shows `running agent ledger · ticket decided`.
7. Lee presses Approve in Slack; the loop checks `U0LEE000` against `people`, replies `{ approve: ["refund"], note: "U0LEE000" }`; `ledger.js` refunds, and `used` lists `net:ledger.example.com:443`. Machines, beside `ledger.yaml`'s `allow:` lines, lists `net:ledger.example.com:443` under `used`.
8. The ledger answers; the bot calls `show_chart` with refunds by week, drawn in the record; `saying` posts to the thread; the row shows `done`.
9. Dana asks in the thread "Can it go to the same card?"; the loop starts `help.yaml` again with the last run's `conversation`; a second bar appears in lane `1700000000.000200`.
10. Raj opens an issue in `platform/api`; `gitlab-in` hears it; `api-issue.yaml` runs `platform/ops.yaml`, which reads `deploys/rollback.md` with `read_doc`, and `out.yaml` comments on the issue. A row appears in lane `12`, labels `bridge=gitlab place=platform/api thread=12`. The jira bridge is the same shape, labels `bridge=jira place=<project> thread=<key>`.
11. Al pulls the clone after Priya merged a change to `ledger.yaml`; Machines shows the new text and its check, and the next row of `help.yaml` runs it.
12. The admin's perch, run on the server with the controller's config, shows every client's rows with lanes by client, and `/metrics`.

## Personal and central

It is one program with no mode flag. At home:

```
  the store      ~/.config/corbie/store
  processes      corbie init: client, controller and runner
  the page       this computer, the token in the printed address
  who approves   the one person, by screen or Telegram
  the admin      the same page
  the directory  Sam's directory
```

At work:

```
  the store      a bucket, or a served directory
  processes      a client per perch, enrolled with a ceiling; runners where the work is
  the page       this computer on the server, over SSH
  who approves   the `people` list of each ask, checked against the presser's bridge id
  the admin      corbie perch none on a bucket; the perch with the controller's config on a served directory
  the directory  a clone of a GitLab repository, pulled as any clone is
```

- On the server, whoever acts opens the page over SSH; everyone else acts through a bridge.
- A served directory has no viewers, so there the admin's perch runs with the controller's config, as `corbie ui controller` does. Either way it reads every run's status and no record.
- The page keeps corbie-ui's rule: a perch that acts listens on this computer only; `none` may listen on any address. There is no proxy and no TLS to set up.
- Security is one sentence: whoever shows the token acts as the perch's client, and a press from a bridge is acted on only when its id is in the ask's `people`.
- One config has one perch, since two would each start the `in` runs.

## Documents outside the system

A GitLab documentation repository is read in place by `.perch/docs/gitlab.yaml`, a tools file named in `tools_file:`. The agent's `input.docs` gives the site and the project, and `input.docs.ref` defaults to the repository's default branch.

```
  list_docs                 the repository tree at ref
  read_doc(path, from)      one file's raw text, up to 20,000 characters, from a line on
  search_docs(words)        GitLab's search with scope=blobs, each line with its file
```

Each is a script with `allow: ["net:${input.docs.site}", "secret:GITLAB_TOKEN@net:${input.docs.site}"]`, filled from the agent's input as R-perm-28 allows; the token name is fixed, because R-sec-26 says a secret's name is always written out.

- An agent names `.perch/docs/gitlab.yaml` in `tools_file:` and `docs` in its `input:`.
- An agent called as a tool is given `docs` in the calling job's `input:`.
- An agent that names no docs file has no docs tools.
- `.perch/docs/dir.yaml` gives the same three tools over `fs.read:${input.docs.path}/**`; the house's `notes/` is read that way.

## Live views

The brief's "a dynamic gui that agents can help with live" is read as: a run adds to the page while it runs, within the vocabulary below; no model writes the page's code or layout.

A page is a function of the record: the lists are drawn from one `client.Watch` narrowed to the perch's client, a Run page from one `client.Follow` on the root with every child's entries (R-client-42), and a part is sent by id when its HTML differs, as `page.Stream` does.

Text is `log` and the model's `text`. The vocabulary is values emitted under one name, `view`, of which the latest per `id` is kept:

```
  { id, kind: table, columns, rows }
  { id, kind: chart, type, title, categories, series }       bar, line, area or pie, as SVG
  { id, kind: list, items: [{ text, status }] }
```

Corbie's own emits are parts: `text` per turn, `log` as one part of lines, `progress` on the status row. An unknown kind is drawn as its JSON, marked unknown. `.perch/show.yaml` gives a model three `show_*` tools, each a script that emits; a script emits directly.

A machine may add a part and replace one by its `id`. It may not carry markup or script (text is drawn as nodes under `script-src 'self'`), change another run's view (the runner writes an emit into its own job's record), move the person's focus, or change a layout: the page places every part by run and order.

## Shipped machines and fixed pipelines

`.perch/` is written whole at every start, read-only, each entry a directory with a README of Needs, Settings, Example and Changed. A shipped entry is one of two kinds, each used one way: a machine, named by path and given values in the job's input, or a template, which New copies.

```
  bridges/<slack|telegram|gitlab|jira>/in.yaml, out.yaml   as Bulldog ships them
  bridges/<name>/ask.yaml         the ask for one bridge, run by `decide.yaml`
  show.yaml                       the three show tools
  docs/gitlab.yaml, docs/dir.yaml the docs tools
  pipeline.yaml                   a template: a statechart that runs an agent, then a bridge's out.yaml, and outputs the conversation
  decide.yaml                     a template: a rule, then a person
  routine.yaml                    a template: a timer, a machine, a bridge out
```

- In `ask.yaml`, the state `asking` issues the ticket `decided` for 7d; its inner state `telling` posts the held jobs over `out.yaml` with `ref` and `people`. When no answer comes in 7d, it approves nothing.
- In `pipeline.yaml`, `answering` runs the agent the template names, with `said` and `conversation`, `saying` runs a bridge's `out.yaml`, and `answered` outputs the conversation.
- In `decide.yaml`, `rule` runs a script given the held jobs, answering `{ approve, note }` or `{ refer, note }`; `person` runs a bridge's `ask.yaml` when it refers.
- In `routine.yaml`, `waiting` is on `timers: { alarm }`, `working` runs a machine with `said`, and `telling` posts the answer.

A model runs only where a state runs an agent. A tool, an ask, a routine and a bridge are each machines, so composition is one mechanism, `job: { machine: … }`.

What a harness gives, in its controlled form here:

```
  persona             system: in the definition
  memory              the last run's conversation; notes as a directory read by docs/dir.yaml
  skills              shipped machines, by path
  cron                routine.yaml, started once from Machines, due with the perch stopped
  messaging           bridges
  approval            approve: true, decide.yaml, people
  session search      none: run ls --label thread=…, within keep:
  steering a run      none: Carry on after it ends
  self-written skills none
```

## From corbie-ui and Bulldog

From corbie-ui, kept:

```
  page.Stream, ui.js                 parts sent by id
  drawing/                           with data-job added
  serve/token.go                     and its rule on where to listen
  machines/check.go                  lineOf, fieldsAt, secretsNamed
  runs/record.go                     the record's words
  metrics.Text                       /metrics
  the Runs list, timeline and now
  the Controller page
  the ui.css tokens and motion, with --plate2 and --wait
```

From corbie-ui, dropped:

```
  the .split run page                    the Run page is stacked
  typed JSON as the only reply           a form from asked
  glow, the bar's end ring, ×3 on flow   motion with no change behind it
```

From Bulldog, kept:

```
  the bridges' machines and scripts
  the README shape, shipped.Write
  both examples                      as directories of definitions
```

From Bulldog, dropped:

```
  SQLite, the git history            the store is the only record
  people, sign-in, guests            one token, and `people` on an ask
  requests, the decision walk        a thread is the bridge's; an ask is decide.yaml
  teams, disclosure, heed            one perch is one client
  notes and office tools             docs/dir.yaml
  call.yaml, clock.yaml, live.yaml   a run waits on a ticket or a timer, never on the perch
```

## Corbie's part

Given, and checked in the code:

- An ask run is a child with `parent` set, and its input holds `asking.instance` and the ask's own `input:` (`controller/take.go`, `startAsk`), so `to` and `ref` need no table.
- `conversation` is a filled value (`agent/step.go`), and a start resolves a blob that names its run (`client/client.go`), so thread memory is one input.
- A timer's due time is the `at` of the step's `wake` ask whose `what` begins `timer.`, so a routine's row needs no change to the run's listing.
- A step entry's `asks[]` carry a `hold` with the job and its `shown`, a `script` with `runner`, `attempt` and the due time `at`, and an `again` wake for a retry; a `failed from <job>` event's data carries `status`, `error`, `refused`, `layer` and `fix`; an answer to a ticket is an event with `answer: true`; the head carries `keep` (`instance.Ask`, `machine.Error.Value`, `machine.Event.Answer`, `instance.Head`).
- A program names its own run (`Start.Run`), and a start for a run that exists is dropped (R-ctl-14), so a start sent twice is one.
- A label value is not checked by Corbie. A named run is `<client>/<stamp>-<rest>` (`store.CheckInstance`), the stamp a UTC time to the millisecond and the rest free of `/`, `\` and control characters.
- Labels, tickets, `asked`, `used`, `machine.Describe`, the check and `cli.Program` exist as the note uses them.

Nothing is added to Corbie for the perch.

## What games teach

Perch takes from games what makes orchestrating many actors legible: the whole world on one screen, an order given from where the person looks and its consequence shown, attention drawn only to what needs the person, and nothing the store does not hold. It has no decoration and no cue with a second meaning: every cue below is read from a field of a head, a record entry, a definition or a receipt, except `store lost`, the stream's `store` message, and `hot`, the outline a hovered button adds to what its command changes.

- Minecraft's F3 screen prints a block's id and every property as stored. On Run, a job's sheet is a row per field the record holds, absent until its entry exists. It reads the step's ask (`kind`, `name`, `attempt`, `runner`, `child`, `at`), the job's emits, the `done` or `failed` events whose `from` is the job's name or an attempt's full name, the `wake` ask whose `what` begins `again.`, and a held job's `job.shown`.
- Dead Space draws health on the suit. On Run, a job box's second line, and the line above a state, read one of:

  ```
    turn 2/20 · tools         the agent's `turn` and `waiting`, and `limits.turns`
    due in 2m                 a script ask's `at`
    try 2/3                   a script ask's `attempt` and the job's `attempts`
    5/10                      `progress`
    held                      the `hold` ask
    denied                    the `failed` event of a job not approved
    done · 17s                each step's `time`
    not run                   no entry for the job
    ticket decided · 6d 23h   above a state: the head's `tickets` and the `wake` ask whose `what` begins `timer.`
  ```

  A countdown is from now, or from the replay step's `time` when `at=` is set.
- RimWorld's colonist bar keeps every colonist on screen with their state. The standing row on Runs reads the heads of the roots whose `labels` hold `bridge` and no `place`, and `bridges/<name>/settings.yaml`.
- Civilization's end-turn button reads `1 unit needs orders` and moves the view to the unit. The header of Run, Machines, Controller and Metrics carries `N wait on a person ›` in `--wait`, and the tab's title `(N) Perch`, for this client's runs whose `tickets` the screen's waiting plane does not list; a press opens the root of the oldest by `started`. It reads heads alone, only when the perch acts as a client, and is absent at zero.
- StarCraft's alert says where the attack is, and space moves the view there. The newest failure line reads heads alone: `status`, `ended`, `parent` and `failure`.
- Factorio's assembler reads `no ingredients` or `output full` on hover. The `now` panel adds a count per `failure`, and the failure row names the deepest job that failed, as chain chips, with `refused`, `layer` and `fix`. It reads the `failed from <job>` events and `fail` asks of every run below, deepest by `parent`.
- FTL keeps the reticle on a room until the shot lands. Approve or Deny locks the card and prints the command and the receipt untouched: `refused: …` unlocks it, except `refused: the ticket was already answered`, which like `taken` keeps it locked until the issuing run's head no longer names the ticket, when the card collapses. A card that collapses with no press on the screen was answered through a bridge, or its ask run ended.
- Mini Metro moves nothing but the trains. Nothing moves without a change behind it: a part `mark` finds new, ended or moved plays its keyframe once; a taken arrow, whose `data-key` is `taken:<from>:<to>` only while taken, runs an `--ink` dot along itself over .9s, its stroke solid; a child opened or closed, and a card collapsing, move as Depth says. The header's `live` dot is still, `--amber` with `store lost` while the stream reads `store: lost`.
- StarCraft II's replay bar shows the match at the thumb's time. The replay bar sets `at=` in the address to a step's `time`, and every plane, chip, second line and sheet but the waiting plane is drawn from its run's step and emit entries at or before it. It reads the `time` of every entry, and `step`, `state`, `events`, `asks` and `status` of own runs; the left end reads `earlier steps removed` when the record's lowest step is not 1.
- Into the Breach shows every tile a move changes before the move. An action button hovered or focused adds `hot`, a 2px `--ink` outline, to what its command changes, by `data-key` from its `data-hot`:

  ```
    Approve, Deny   each held job's box and the card; on Runs the root's row and bar
    Cancel          every chip in the chain row and the job boxes
    Carry on        nothing; the new run is a row the screen does not hold yet, and `run submit` prints its ID under the button
    Reply           the state that waits on the ticket
    Send            the arrow out of the current state whose event is the name typed, and the state it goes to
  ```

  It reads the `hold` ask's `job`, the step's `state` and the definition's `ticket:` and `on:` lines.

## Open questions

- The name is Perch or Desk. I recommend Perch, since `desk` already names a computer in the manual.
- It is unconfirmed whether the check accepts a root pipeline whose held jobs are in machines below it (R-mach-114). I recommend trying `corbie test direct.yaml` on day 4; if refused, the agent names the ask, as Bulldog's do.
- It is unconfirmed whether Bulldog's `slack.js` reads more of a place than its key. I recommend reading it on day 4 and reshaping `settings` once.

- A team is a perch of its own: one client, its own runs, and work between teams over a bridge. Bulldog's sealed teams are kept by that alone, and the perch has no people, teams or decisions of its own. I recommend keeping it so until an office needs two teams on one perch; `concepts/central.md` is the design with teams.
- Depth is two planes and a chip. `concepts/depth.md` makes every level a plane, with blur and parallax, and keeps a database for it. I recommend building the two planes first and judging on the laptop whether a third cue earns its place.

## The first session on the laptop

One day per step:

1. Copy `corbie-bulldog/examples/house` to `~/house`; drop `office.yaml` and the `talk` and `notes` tools from each agent; write each `model:` as Corbie writes it, `{ name: claude-sonnet-5-5, secret: ANTHROPIC_KEY }`; copy Bulldog's `shipped/ask.yaml` to `~/house/decide.yaml` and point `ask:` at it: it waits on the ticket `decided`, which day 3 approves from the screen and day 4 replaces by New from `decide.yaml`. Build `corbie-perch` from corbie-ui's packages: Runs and Controller as they are, the Run page stacked with the chain row, the parent band and the parent's drawing behind, and a child opened from the Children tab. Done when Start on `steward.yaml` with `{ said: "", context: "The kitchen sockets are dead." }`, since its `prompt` is `${input.context}`, shows a row within the hour, and opening the fixer from Children shows the steward behind it by the end of the day.
2. Add `data-job` to `drawing/` and the chips over job boxes; open the fixer from the steward's run after Start.
3. Build the waiting plane from each run's ticket names and the reply form from `asked`, and approve `cut_power` on the screen.
4. Write `.perch/` with `pipeline.yaml`, `routine.yaml` and the Telegram bridge, the loop's three steps, and `direct.yaml` and `decide.yaml` by New; reshape `small-orders.js` to read `input.jobs[0].asked.args.pounds` and answer `{ approve, note }` or `{ refer, note }`; test the `decide.yaml` shape.
5. Add the `view` emits with `show.yaml`, and read the house's `notes/` through `docs/dir.yaml`.
6. Give each job box its second line, computed in `drawing/` from the entries the Run page follows, and redraw the Run page's parts once a minute and on change, so `due in 2m` counts down through the one template. Done when, on the fixer's page with no child opened, the fixer's model box reads `turn 2/20 · tools` and the `cut_power` box reads `done · 17s`, the seconds from the record's step times.
7. Build the lock on a pressed card, kept across a part swap while `.receipt` reads `taken`, and the card's collapse. Done when approving `cut_power` collapses its card and rewrites `cut_power`'s second line within one interval, and a press on Telegram collapses it with no press on the screen.
