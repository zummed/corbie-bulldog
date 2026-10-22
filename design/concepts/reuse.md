# Rookery: a library of fixed pipelines

`rookery` is a working name: `corbie-rookery` on PATH, run as `corbie start controller runner rookery <directory>`.

## 1. What it is

Rookery is one Go program that serves one directory in a browser. The directory holds the person's pipelines, which are ordinary Corbie definitions, and `.lib/`, the library of shipped machines, written whole at every start. A pipeline names a shipped machine by its path, `machine: ./.lib/for-us/for-us.yaml`, and configures it in the job's `input:`. Rookery is a client through the Go package: it starts a pipeline from an origin (a bridge place, a schedule, the page), labels every run, answers the tickets of shipped machines, and carries each reply to its origin's out machine.

It keeps of its own:

```
  .rookery/identity   the value of the label rookery, on every run it starts
  .rookery/version    the version that ran before
```

Nothing else: a bridge's cursor is the last listener run's output, a run's origin is in its input, a decision is an ask run's ticket, and what a run read is its record.

## 2. The screens

Depth has one meaning: containment. What contains the thing in front is behind it, at 92% size, blurred 2px, still live; what the thing contains is drawn within its boxes at a quarter size, live while it runs. A click on a box brings that machine to the front; Escape brings the container back.

**Home** `/`

- Waiting on you lists each held job as a card: its name, input and `asked`, the deciders, a note, Approve and Refuse.
- Pipelines lists each definition outside `.lib/` as a card: its drawing, the origins `rookery.yaml` binds to it (`#help`, `direct`, `mon 09:00`), and today's runs by status.
- Ask is a text and a pipeline; it starts a run with `origin: { bridge: page }`.

**Library** `/library`

- Each shipped machine is a card: the first line of its page, each secret as `set` or `not set`, each setting with its default and description, the lines that name it, and its Changed lines since the version that ran before.
- Compose opens a new pipeline with the machine in one job; Copy writes the machine's directory into the person's and rewrites each line that named it.

**Pipeline** `/p/help`

- The drawing is in front, each machine's own drawing within its box, each shipped box named with its library version.
- Beside it is corbie-ui's editor, checked by `machine.Parse` with each problem at its line, the fields at the cursor from `machine.Describe`. A shipped box opens a Settings form made from the shipped definition's `input.settings` schema; a change rewrites the job's `input:` lines and nothing else.
- Permissions lists every `allow:` line below, each held job with its deciders, and beside each line the `used:` of the last run. Test and Start run `corbie test` and `corbie run submit`; Bind writes a place or a schedule into `rookery.yaml`.

**Run** `/run/<id>`

- The drawing is lit by each step's `state`: stages `now`, `done`, `failed` or `skipped`, a chart's active states, an agent's turn. A child's drawing is within its box, lit by its own record; a held job's box holds its decision card.
- The record is one row per event, job and emit. Beside it are waits, output or `refused`, `by` and `fix`, input and `used:`.

**Store** `/store`, when the config holds the controller: corbie-ui's store view rewritten inside Rookery.

## 3. A day at home, and a day at work

Sam runs `corbie start controller runner rookery ~/house`. His `rookery.yaml` names the model `usual`, Sam's and Kit's telegram identities, `decide: { everything: [sam] }`, a telegram bridge whose `direct` place binds `./steward.yaml`, and a schedule, `18:00 Europe/London` for `./bins.yaml`.

1. Home shows cards for `steward`, `fixer`, `sparks`, `bins` and `garden/gardener`, the telegram listener `running` and the schedule; Library shows `TELEGRAM_TOKEN set`.
2. Kit writes on Telegram "the socket by the kettle is dead"; the listener run ends with `heard`; Rookery starts `steward.yaml` with `said`, `context` and `origin: { bridge: telegram, to: "4472" }`, labelled `pipeline=steward`, and restarts the listener with the cursor.
3. Sam opens the run: the steward's drawing in front, its `text` streaming; the steward calls `fixer`, and the fixer's drawing appears within the tool's box; Sam clicks it, and the steward goes behind.
4. The fixer calls `sparks`, done with "circuit 4", then `cut_power`, `approve: true`; its `ask: { machine: ./decide.yaml }` runs `small-orders.js`, which refers it, then `./.lib/ask.yaml`; the card `cut_power { circuit: 4 }` is in Waiting on you and on Sam's Telegram with two buttons.
5. Sam presses Approve; Rookery reads the ticket from the ask run's record and runs `corbie run reply <ask run> <ticket> '{"approve":["cut_power"],"note":"sam: yes"}'`; `cut_power` goes `done`.
6. The steward ends `done` with `reply`; Rookery starts `./.lib/telegram/out.yaml` with `to: "4472"`, labelled `for=<run>`; the card reads `done · 48s`.
7. At 18:00 the `bins` clock run enters `come`; Rookery answers `came` and starts `bins.yaml` with `origin: { bridge: telegram, to: ["4471", "4472"] }`; two out runs follow.

Al runs `corbie start rookery /srv/office` beside the roles, the page over `ssh -L`. The pipeline bound to `#help`:

```yaml
# /srv/office/help.yaml
statechart: help
input: { said: string, context: string, origin: map }
initial: weighing
states:
  weighing:
    job: { machine: ./.lib/for-us/for-us.yaml, input: { said: "${input.said}", context: "${input.context}", settings: { threshold: 0.6 } } }
    on:
      done: [{ go: working, when: "${event.data.says == 'act'}" }, { go: quiet }]
  working:
    job: { machine: ./bot.yaml, input: { said: "${input.said}", context: "${input.context}", docs: { site: gitlab.com, project: platform/handbook, ref: main } } }
    on: { done: answered }
  answered: { end: done, output: { reply: "${states.working.output.reply}" } }
  quiet:    { end: done, output: {} }
```

1. Home shows `help` bound to `#help` and `issues` to `platform/api`, both listeners `running`; Library shows `SLACK_TOKEN`, `GITLAB_TOKEN` and `LEDGER_KEY` `set`.
2. A guest writes in `#help` "charged twice for order 8812"; Rookery starts `help.yaml` with `said`, the thread as `context`, and `origin: { bridge: slack, to: "C0HELP/1729593000.000100" }`; Al opens the run: `weighing` lit, `for-us` ending `{ says: act }`, the bot's drawing filling `working`.
3. The bot calls `payments`, whose job is `./payments/ledger.yaml`; the ledger calls `charges`, then `refund`, `approve: true`; its ask `./payments/decide.yaml` runs `./.lib/reviewer/reviewer.yaml` with `settings: { brief: "Only a duplicate charge under £100" }`, which refers, then `./.lib/ask.yaml`; `decide: { refund: [lee, priya] }` puts the card on Lee's and Priya's pages and Slack.
4. Lee presses Approve with "duplicate confirmed"; `refund` ends `done`, `used: net:ledger.example.com:443 secret:LEDGER_KEY@net:ledger.example.com`.
5. The bot replies and `help` ends `done`; Rookery starts `./.lib/slack/out.yaml` to the thread, and the guest reads the reply.
6. Raj opens issue 41 in `platform/api`; the gitlab listener hears it; `issues.yaml` runs `./platform/ops.yaml`, which calls `read_doc` of `deploys/rollback.md` through `./.lib/gitlab/docs.yaml`; the run page links the path read; the reply goes back as a comment through `./.lib/gitlab/out.yaml`.
7. After an upgrade, Library shows under `slack` "0.6.0: a heard message carries its thread as context", marked new; `help.yaml` needs no change.

## 4. Personal and central

One program, one directory, one `rookery.yaml`. What differs:

```
             home                                      work
  Corbie     corbie init --ceiling all, one process    corbie init store; runners and clients enrolled
  the model  usual: sonnet, service claude-code        usual: claude-sonnet-5-5, secret ANTHROPIC_KEY
  bridges    telegram, places direct                   slack with "#help" and guests; gitlab with projects
  decide     everything: [sam]                         refund: [lee, priya], everything: [al]
```

- The page is one identity, Rookery's client, on loopback behind one token. A held job goes to the list `decide` names for the job's name, else `everything`, over each person's bridge with `choices`; the first press answers the ticket.
- A directory with no `people` and no `bridges` is personal: every held job goes to the page, and Ask is the one origin.

## 5. Documents outside the system

A document is read where it is. `.lib/gitlab/docs.yaml` is a tools file; a pipeline gives its worker `docs` in `input:`, as `help.yaml` gives the bot, and the worker names the file in `tools_file:`. Each tool's job is the script `./docs.js` with `allow: ["net:${input.docs.site}", "secret:GITLAB_TOKEN@net:${input.docs.site}"]`.

```
  list_docs                     GET /api/v4/projects/<project>/repository/tree?recursive=true&ref=<ref>
  read_doc(path, from)          GET …/repository/files/<path>/raw?ref=<ref>
  search_docs(words)            GET …/search?scope=blobs&search=<words>, each hit with its path and line
  propose_doc(path, text, why)  a commit on a new branch and a merge request; approve: true
```

- The token is bound to the site, and the model never holds it.
- The record keeps each path read, the run page links each to GitLab, and a change is a merge request; nothing is copied or indexed.
- `.lib/directory/docs.yaml` gives the same tools over `fs.read:${input.docs.path}/**`.

## 6. Live views

Home and Library read one `client.Watch` with `Labels: { rookery: <identity> }` and `Finished: true`, and send only parts whose HTML changed. A run page is one `client.Follow`, and each entry changes the drawing:

- A step's `state` lights the stages, the active states or the turn.
- An ask of kind `child` draws the child's drawing within the job's box; kind `hold` marks the box, and the decision card is in it until the ask run ends.
- An emit `text` streams into the turn's row; `log` is a row; `progress` is a bar; a `chart` fence in `text` is drawn as a chart.

A person acts back with one command each, shown on hover and answered `taken` or `refused: …`. Approve and Refuse are `corbie run reply` on the ask run's ticket `decided`; Reply, Send and Stop are `corbie run reply`, `corbie run send` and `corbie run cancel`; Say more starts the pipeline again with the earlier runs as `context`.

## 7. Shipped machines and fixed pipelines

A shipped machine is a directory in `.lib/` with its definitions, its scripts and `README.md`, a page with Needs, Settings, Example and Changed. The library's version is Rookery's.

```
  ask.yaml                       ticket decided, timers { unanswered: 7d }
  clock.yaml                     a schedule's timer: waits on due, then on ticket came
  slack/, telegram/, gitlab/     in.yaml listens from a cursor and ends with heard; out.yaml carries one reply
  for-us/for-us.yaml             the heed, { says: act | note }; settings timeout, threshold, model, question
  reviewer/reviewer.yaml         an agent given jobs and a brief, replying { approve, refer, note }; settings brief
  summariser/summariser.yaml     a flow: each over a blob's items, then one summary; settings words, language
  gitlab/docs.yaml, directory/docs.yaml   the four docs tools
```

- **Named by path.** A pipeline names a shipped machine as `machine: ./.lib/<name>/<file>.yaml` or in `tools_file:`. Rookery's loader reads a path from the directory it serves, so a pipeline in `payments/` names `../.lib/…`.
- **Configured by input.** A shipped definition's settings are `input.settings`, a JSON Schema object with a `default` and a `description` for each, which the controller fills in. A pipeline writes only what differs, and nothing in a shipped machine is edited.
- **Upgraded by version.** A machine named by path runs the newest at the next start, and Library shows what changed. A copy is the person's own file, and the library never changes it.

A pipeline is a flow or a statechart whose jobs are shipped machines and the person's. Compose writes one from a pattern: Intake, as `help.yaml`; Decide, a flow for an ask that runs a rule script, the reviewer and `ask.yaml`, each when the one before refers; Report, a reader then the summariser. A schedule and a bridge place are not pipelines: `rookery.yaml` binds each to one.

## 8. What Corbie gives for free, and what it would have to gain

Free:

- `machine:` and `tools_file:` by path, read from the file's directory, so the library is plain files.
- `input:` as JSON Schema with `default` and `description`, filled by the controller, so a shipped machine's settings are its definition's own; `machine.Parse` and `machine.Describe` give the check and the form.
- Labels and `parent` in the head, so Rookery finds every run after a restart and keeps no list.
- `approve: true`, `ask:` with `asked`, tickets, and `state`, `asks` and emits in the record, so a decision is a run and a drawing needs nothing from the machine.

To gain, each general to any client:

1. A timer's due time in the head, so a schedule's card reads "next 18:00".
2. The check reporting a definition's reach, every job below with its hosts, secrets and holds.
3. A watch that reports a refused start, so a pipeline that could not start shows its `problems`.
4. A secret's name from input in a scope, `secret:${input.docs.secret}@net:…`, so one docs tools file serves two GitLabs.

## 9. Risks and what is lost

- A shipped machine's path is fixed in the pipeline, so a pipeline cannot choose one by input.
- Upgrading by version changes what runs under an unedited pipeline; the Changed lines are the only warning, and a copy, the only hold, never upgrades.
- The cursor is the last listener run's output, so a stop longer than `keep:` loses what the bridge would have heard.
- Rookery is one identity: every record is readable on its page, and who pressed a button is the bridge's word.
- A drawing within a drawing is small, and blur on a live plane costs the browser.
- Requests, notes and the committed history are gone with Bulldog; the thread belongs to Slack, Telegram or GitLab.
