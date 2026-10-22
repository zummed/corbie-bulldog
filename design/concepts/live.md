# Live views

## What it is

`corbie-view` is a program beside Corbie, started as `corbie start controller runner view --dir <directory>`. It shows every run as a view: parts that the run's jobs emit into its record while they run, which a person reads and answers in a browser. It is one client of Corbie: it starts runs from the definitions in its directory, follows their records, and changes the store only with `run submit`, `run reply`, `run send` and `run cancel`.

A view is a function of the record, so a restart rebuilds it from the store. Of its own the program keeps the directory it is given, with `.view/` written into it at every start, and `.view/keys`, a hash of each person's key.

## The screens

Each screen follows the store as corbie-ui's pages do.

**Home** shows the person's runs.

- Home shows a card per run the person started or decides for: name, status, step, what it waits for, who asked and where, and the latest parts at small size.
- The Ask box takes an agent of the directory, a text and files, and runs `corbie run submit <file> --input … --label person=<name>`.
- A standing run (a routine, a bridge's in run) is a card that shows its latest child's view.

**Run** shows one view at full size.

- The parts are in order of first emit, the latest of each `id`, drawn by kind.
- The drawing of the machine is a strip at the top, with the current stage, state or turn lit.
- A child's view is one plane back, inside the part of the job that started it.
- The events the current state's `on:` names, apart from `done`, `failed`, timers and tickets, are buttons that run `corbie run send`; Cancel runs `corbie run cancel`; Ask more starts a new run with the ended agent's `conversation`.

**Machines** shows the directory.

- Machines shows a tree of directories and files; a definition shows its drawing, its problems, and the secrets its `allow:` names with `set` or `not set`.
- The editor checks 250ms after input; Test runs `corbie test <file> --watch`; Start runs `corbie run submit`.
- `.view/` lists the shipped machines, each with its page, Use and Copy; `people.yaml` and `bridges/` are files too.

**Store** is corbie-ui's store view for an admin: every client's runs, the timeline, the controller's runners, clients and secrets, `/metrics`.

A view has three planes, drawn with scale, blur and one shadow.

```
  front    an ask part waiting on this person, lit
  middle   the run opened, or the cards on Home
  back     children, inside the part of the job that started them, smaller and dimmer
```

- A child the person may not read is blurred and shows its head alone: status, step, what it waits for.
- Opening a back child brings it to the middle, and its parent stays behind it as a dimmed frame with its name and status.
- Below the third plane a child is a row, and pressing it opens it.

## A day at home

`steward.yaml` names `ask: { machine: ./ask.yaml }`, a statechart that runs `small-orders.js` first and `.view/ask.yaml` for the rest. `people.yaml` says `sam: { decides: [.] }`.

1. Kit opens Home: a card `friday · running · waiting on 1 timer` and `bins · done`.
2. Kit types "The socket by the kettle is dead" to steward and presses Ask; a card `steward · running · turn 1` appears in the middle, the model lit on the strip.
3. The turn's `text` emit streams into a text part, "Handing this to the fixer"; the steward calls `fixer`, and a back card `fixer · running` appears inside the text part.
4. The fixer calls `sparks`; Kit presses the row `sparks · turn 1`; the fixer comes to the middle, the steward dims behind it, and sparks is the back plane with a text part "Kitchen sockets: ring 2".
5. The fixer calls `cut_power`, `approve: true`; the controller starts `ask.yaml`; `small-orders.js` says `refer`, so `.view/ask.yaml`'s script emits `{ kind: ask, jobs: [{ name: cut_power, args: { circuit: "ring 2" } }], ticket: decided }`; the part is in front on Sam's Home with Yes, No and a note field, and Kit's card reads `waiting on sam`.
6. Sam presses Yes; the program runs `corbie run reply <ask id> <ticket> '{"approve":["cut_power"],"note":"","by":"sam"}'`; the part drops to the middle as the line `cut_power: yes · Sam · 18:42`.
7. `cut-power.js` runs; its `log("cutting circuit ring 2")` is a line of the fixer's `log` part; the fixer calls `order_part` for £6, `small-orders.js` says `yes`, and the line `order_part: yes · small-orders · 18:43` appears.
8. The fixer ends `done` and moves to the back; the steward's last text streams "Ring 2 is off; a face plate is on order"; Kit's card takes the done colour.
9. Friday 16:00: `friday.yaml`'s timer `due` fires and runs `garden/gardener.yaml` as a child; the gardener's `show_list` tool emits a list part of the weekend's jobs onto the card.

## A day at work

`/srv/office` holds `bot.yaml`, whose tool `payments` runs `machine: ./payments/ledger.yaml`, and `bridges/slack/`. `people.yaml` says `al: { admin: true, sees: [.] }`, `dana: { sees: [support], slack: U0DANA00 }`, `lee` and `priya: { sees: [payments], decides: [payments] }`. `bridges/slack/settings.yaml` says `"#help": { machine: ./bot.yaml, guests: true }`.

1. Al starts the program; it starts `bridges/slack/in.yaml` once with `--label bridge=slack`, and Al's Home shows it `running`.
2. Dana writes in #help "@bot customer 4471 was charged twice on the 14th"; the in run emits `heard`; the program reads it and runs `corbie run submit bot.yaml --input … --label person=dana --label place=#help`.
3. Dana's Home shows `bot · running · turn 1` with the text "Asking payments."
4. The bot calls `payments`; on Dana's screen the back plane is blurred, `ledger · running · turn 1`; on Lee's and Priya's the `charges` tool's script emits a table part: `ch_81 · 14 Oct · £42.00`, `ch_82 · 14 Oct · £42.00`.
5. The ledger calls `refund { charge: ch_82, why: duplicate }`, `approve: true`; the controller starts `.view/ask.yaml`; the ask part is in front on Lee's and Priya's Home, and an out run carries it to Lee on Slack as buttons.
6. Lee presses Yes with the note "second charge"; the program runs `corbie run reply`; Priya's front part drops back as `refund: yes · Lee · 14:02`.
7. `ledger.js` refunds; the bot's text streams "Payments refunded ch_82, £42.00; it goes back to the card within five working days", from `help/refunds.md` read by `read_doc`; the run ends `done`.
8. The program starts `bridges/slack/out.yaml` with the text; Slack shows it in the thread; Dana's card reads `done`, the ledger plane still blurred, with the line `payments answered · 14:03`; Priya opens the ledger and sees the bot behind it blurred, `bot · done · dana · #help`.
9. Al opens Store: the timeline shows the bot, the ledger and the ask's three minutes; Controller shows `LEDGER_KEY set`, `GITLAB_TOKEN set`.

## Personal and central

One program, one client of Corbie, one directory. `people.yaml` is the difference.

```
  no people.yaml   one person, no sign-in, corbie-ui's token cookie; every ask is theirs
  people.yaml      a name per person: sees (directories), decides (directories), admin, and an id per bridge
```

- `corbie view key <name>` prints a one-time link, which becomes that person's cookie.
- A person sees a run they started, or whose definition is in a directory they see; a run's definition is found by its name, unique in the directory.
- An ask goes to the people who decide for the run's directory, else to whoever started the root.

## Documents outside the system

A document stays where it is kept. `.view/tools/gitlab-docs.yaml` gives an agent `list_docs`, `read_doc(path)`, `search_docs(words)` and `propose_doc(path, text, why)` over GitLab's API, with `allow: ["net:${input.docs.site}", "secret:GITLAB_TOKEN@net:${input.docs.site}"]`.

- A definition that names the file gives `docs` a `default` in its `input:`, such as `{ site: gitlab.com, project: platform/handbook, ref: main }`.
- `read_doc` returns the file at `ref` and emits a text part naming the path, the commit and the link; the words stay in GitLab and in the record's tool result.
- `propose_doc` is `approve: true`: the ask part shows the path, the text and why, and a yes opens a merge request.
- The directory itself can be a GitLab repository, changed the same way.

## Live views

A part is one value emitted under the name `view`. Nothing in a machine wants `view`, so the runner writes it into the record and no message goes to the controller.

```
  { id, kind: text, text }                                Markdown, drawn as text nodes
  { id, kind: table, columns, rows }
  { id, kind: chart, type, title, categories, series }    bar, line, area or pie, as Bulldog's Slack chart
  { id, kind: list, items: [{ text, status }] }           todo, doing, done or failed
  { id, kind: file, file: <blob>, name }                  type, size and path; a download only
  { id, kind: ask, question, choices?, shape?, jobs?, ticket }
```

1. A script calls `emit("view", part)`; a model calls a `show_*` tool, whose script does the same.
2. The runner writes the emit into the run's record, and the follow gives it with its run and job.
3. The program keeps the latest part per `id` per run, draws it by kind, and places it by run depth.
4. An ask part is shown only while its run waits on the ticket it names; the program reads the ticket's value from the record and answers with `corbie run reply`, writing `by: <name>` into the value.

Three of Corbie's own emits are parts too: `text` is a text part per turn, `log` is one part of lines, and `progress` is drawn on the status row.

A person acts back with one command each.

```
  answer an ask    corbie run reply <id> <ticket> <value>
  press an event   corbie run send <id> <event> [data], for the events the state's on: names
  stop             corbie run cancel <id>
  ask more         corbie run submit, with the earlier result's conversation in the input
```

An agent may add or replace a part of its own run, ask through `.view/person.yaml`, and show a blob.

- It cannot change another run's view, because the runner signs an emit and writes it into its own job's record.
- A kind the program does not own is drawn as a text part of its JSON, marked unknown.
- Text is Markdown drawn as text nodes under `script-src 'self'`, so a part carries no markup and no script.
- No part sends a message; only a press does, and the program writes who pressed.
- The program places and sizes every part by depth and order.
- The view is no input, so a model never reads it; a person's answer comes back as the tool's result.

Sealing is Corbie's: the controller and the root's client read the record. The program is that client and narrows within it by `sees`.

## Shipped machines and fixed pipelines

`.view/` is written whole at every start, as `.bulldog/` is, and used in place by path.

```
  ask.yaml                 an ask: emits an ask part of the held jobs, waits on ticket decided 7d, ends { approve, note, by }
  person.yaml              a question: emits an ask part, waits on ticket answer, ends with the answer
  every.yaml               a routine to copy: timers: { due: "<at>" }, a child, and back to waiting
  tools/show.yaml          show_text, show_table, show_chart, show_list, show_file, ask_person
  tools/gitlab-docs.yaml   the four document tools
  bridges/<name>           Bulldog's slack, telegram and gitlab machines as shipped; a place names a machine
```

A pipeline is a definition in the directory that names these as jobs, and the program adds nothing. The house's `ask.yaml` runs a rule and then `person.yaml`; `friday.yaml` is a copy of `every.yaml` with `fri 16:00 Europe/London` and `machine: ./garden/gardener.yaml` written in; a work report is a flow of `ops.yaml` and `show_table`.

## What Corbie gives, and what it would have to gain

Given: emits reach the record without the controller (R-inst-27); a follow carries every child's entries with instance and job (R-client-42); the root's client reads every child's tickets; `on:` acts on an emitted value; `keep:` bounds every view.

To gain, each general to any client:

1. A watch tells of a start that was refused, so Home shows it without reading receipts.
2. A head carries a timer's due time, so a standing card says `fri 16:00`.
3. A script can wait until an emit is in the record, so a bridge hands over nothing twice.
4. A definition's reach is read from Corbie, so Machines lists hosts, secrets and holds from one check.

## Risks and what is lost

- A model can fill a view with parts; the program shows the latest 40 per run.
- A view goes with its run's `keep:`, 7d by default, and there is no archive.
- Corbie seals no team: the one client reads every record, and `sees` is the program's wall.
- Any client holding a ticket's value can answer it, and `by:` is the program's word.
- Over a bucket a part arrives at the polling interval.
- Bulldog's numbered requests, disclosure checks, change pipeline, sign-in methods and guests are gone; a conversation is a chain of runs, not a chat.
