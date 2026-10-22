# Depth as the structure

## What it is

`corbie-desk` is one Go program, started as `corbie desk <directory>` or by `corbie start … desk`. It is a client: it starts runs from the directory's definitions, labels each `desk`, `team`, `request` and `worker`, and reads them with `client.Watch` and `client.Follow`. The window shows one sheet in focus, every sheet that contains it beneath, blurred, and what waits on the person in front; each level (store, office, team, request, run, child run, job) is a sheet.

It keeps of its own:

| Kept | Where |
|---|---|
| the office: `desk.yaml`, a `team.yaml` per team directory, workers as Corbie definitions sent as written | the directory, a git repository it commits to and never pushes |
| requests, entries and files, decisions (who, what, note, when), bridge cursors, sign-in sessions | `.desk/desk.db`, one SQLite file |
| the identity that labels every run; the shipped machines | `.desk/identity`; `.desk/`, written whole at every start |
| nothing of a run | read from Corbie while `keep:` holds it |

## The depth

| Cue | Meaning |
|---|---|
| in front: `translateZ(80px)`, sharp, hard shadow | waits on you: a decision, or a question put to you |
| the focus sheet: `translateZ(0)`, sharp, a soft shadow beneath | what you read and act on |
| beneath: `translateZ(-80px)`, blur 2px and wash 20% per level, top band sharp | what contains the focus |
| a card lifts `translateZ(16px)` on hover | it opens as a sheet; a sealed card does not lift |

The parent shows a band of 32px and each sheet above it an edge of 12px. The band holds the parent's name, status and a chip per sibling; an edge shows its name on hover.

1. In: Enter or a click on a card. The card's box becomes the sheet, `transform: translate(x, y) scale(w / W, h / H)` to none in 320ms, while its sheet sinks one level.
2. Out: Escape, or a click on a band or an edge. The focus sheet goes to `translateZ(240px)` and `opacity: 0`; the sheet beneath sharpens.
3. Sideways: the arrow keys, or a chip in the band. The sheet slides out `translateX(-40px)`, the sibling slides in from the right; no depth changes.

```
.stage { perspective: 1200px; perspective-origin: 50% 0; overflow: hidden }
.sheet { position: absolute; inset: var(--top) 0 0; transform: translateZ(calc(var(--n) * -80px));
         transition: transform .32s var(--ease), filter .32s, opacity .32s }
.sheet:not([data-n="0"]) > .body { filter: blur(calc(var(--n) * 2px)) }
.sheet:not([data-n="0"])::after { inset: 0; background: var(--paper); opacity: calc(var(--n) * .2) }
```

`--n` is a sheet's distance from the focus and `--top` the bands and edges above it; `translateZ(-80px)` shrinks a sheet by 6% toward the top centre, so its band keeps its place. `prefers-reduced-motion` stops every transition; `prefers-contrast: more` removes blur and wash, and the bands alone show the hierarchy.

## The screens

**The store**, the deepest, when the program is the controller: corbie-ui rewritten. Every client's runs with status, step, waits and labels, a timeline with a lane per `team`, runners, clients with ceilings, secrets `set` or `not set`.

**The office**, `desk.yaml`: Ask (a text and a team); teams as cards with name, about, lead, people and counts of requests `working` and `waiting on someone`; bridges as cards with their in-run's Corbie status.

**A team**: requests as cards, ordered waiting on you, working, waiting on someone, answered; the pipeline's drawing with the team's workers as cards on its states; routines with `at`, `to`, next due and Run now; the docs card.

**A worker**: type, name and model, `usual` resolved; the drawing beside the definition's text, checked as typed, each problem at its line; an aside with every `allow:` entry, each secret `set` or `not set`, each tool with who decides it when held. Save commits with who and why; Test runs `corbie test`; Start runs `corbie run submit`.

**A request**: the thread, each entry with who, where from, when, text and files; decisions as one line each; the answer; its runs and sub-requests as cards. Say more starts the lead again with the thread as `context`; Stop runs `corbie run cancel`. A sealed sub-request is a flat card, `Read · payments`.

**A run**: a header, `agent steward · running · step 4 · waiting on tools`, and the input it was given; the drawing with marks `now`, `done`, `failed`, `skipped`, each job on it a card with a child run's status and step; the record as rows, `text` emits streaming, `chart` and `table` emits drawn; an aside with `allow:`, `used` so far, tickets and timers.

**A job**: input; output, or `refused`, `by` and `fix`; emits; `used`; for a held job, who approved it and the note.

**The decision card**, in front: the tool's `does`, `args` and `asked.said`, the job's `allow:`, the deciders in order, Note, Approve, Refuse. Approve runs `corbie run reply <ask run> <ticket> '{"approve": [...], "note": "…"}'` on `.desk/ask.yaml`'s ticket, which only the program's client reads.

## A day at home

Sam runs `corbie start controller runner desk` on the laptop in `examples/house`.

1. Sam opens `http://127.0.0.1:8373/?token=…`: the office sheet The house, the store's edge above it; cards for Ask, the garden, six workers, the telegram bridge `running`.
2. Sam types "The hall light is out" and presses Ask: request 1 arrives as a card, `working`; Enter on it opens the request sheet under the band `The house · steward`, with entry 1 and the run card `statechart request · running · step 2`.
3. Sam opens the run: `work` is `now` on the drawing, and on it the steward's card, `agent steward · running · turn 2 · waiting on tools`.
4. Sam opens the steward: the model `sonnet` under the spark; the record streams the model's `text`; the `fixer` tool card goes `now` and gets a child run.
5. Sam opens the fixer: above it, edge house, edge request 1, edge run, band steward. The `sparks` card ends `done` with "circuit 4"; `cut_power` is held.
6. The decision card rises from the fixer's level: `cut_power`, `circuit: 4`, the fixer's words, decided by `everything: sam`. Sam presses Approve with "go on"; the card sinks back, and `cut_power` goes `now`, then `done` with "Circuit 4 is off."
7. `order_part` with `pounds: 12` is held and no card rises: `small-orders.js` answered `yes`, and the job card reads `approved by small-orders.js: Under £20.`
8. The fixer ends, the steward answers, and request 1 reads `answered`. Sam presses Escape four times; each sheet lifts off and the one beneath sharpens.
9. On Friday at 16:00 the garden's routine starts the gardener and the garden's card pulses; Kit's telegram message, "which bin tonight?", arrives as request 2.

## A day at work

The office runs on a server, `--listen 0.0.0.0:8373`, behind TLS.

1. Dana writes in `#help`: "Customer 8812 says they were charged twice on the 3rd." The slack bridge's in-run emits `heard`; the program makes request 7, origin `#help`, asker Dana.
2. Al, signed in through Entra, sees the office sheet: cards for support, payments and platform, request 7 `working`, bridges slack and gitlab `running`.
3. The pipeline run passes `heed` (for-us.yaml says `act`) and starts the bot. The bot calls `ask_team` for payments: a `call.yaml` child run waits on `ticket: answer`; the program makes sub-request 7.1 and starts the ledger.
4. Lee opens the payments sheet, then request 7.1's run: the ledger's drawing, `charges` done with two charges at 14:02 on the 3rd, `refund` held.
5. The decision card rises for Lee and Priya: `refund`, `charge: ch_2231, why: duplicate`, `allow: net:ledger.example.com, secret:LEDGER_KEY@net:ledger.example.com`. Lee approves with "duplicate confirmed".
6. The refund job ends `done`, `used: net:ledger.example.com:443, secret:LEDGER_KEY@net:ledger.example.com`. The ledger answers, `no-account-numbers.yaml` says `yes`, the program replies to the call's ticket, and the bot's next turn reads the answer.
7. The bot answers in the thread through the out bridge's run; request 7 reads `answered`. Al sees 7.1 as a flat card with its answer and "Refund one charge: approved by Lee, 14:09"; the ledger's record is Lee's and Priya's alone.
8. Al presses Escape on the office: the store, with lanes by `team`, runners, clients, `LEDGER_KEY set`, `GITLAB_TOKEN set`.

## Personal and central

| | Personal | Central |
|---|---|---|
| `desk.yaml` | no `people`, no `signin` | `people` with bridge identities, `signin`, teams |
| who | the one person, on loopback, `?token=` to a cookie | people signed in by google, entra or password; a guest where `guests: true` |
| teams | one team, no walls | a team's records seen by its people; the asker sees the answer |
| decisions | every held job rises for the one person | the list named for the tool, else `everything` |

## Documents outside the system

A team names its docs in `team.yaml`: `docs: { host: gitlab.com, project: acme/support-docs, ref: main }`. The program never reads them itself; a worker reads them through the shipped `.desk/tools/docs/read.yaml`, scripts calling GitLab's API with `allow: ["net:${input.docs.host}", "secret:GITLAB_TOKEN@net:${input.docs.host}"]`.

| Tool | Request |
|---|---|
| `list_docs` | `GET /projects/:id/repository/tree?recursive=true` |
| `read_doc(path)` | `GET /projects/:id/repository/files/:path/raw?ref=main` |
| `search_docs(words)` | `GET /projects/:id/search?scope=blobs` |
| `write_doc(path, text, why)`, held | a commit on a new branch and a merge request for GitLab's reviewers |

The team's docs card shows the repository, the ref, and each doc its runs read in the last seven days, each a link to GitLab.

## Live views

One `client.Watch` covers every run labelled `desk`, and one `client.Follow` each open run sheet and the sheets beneath it; parts go by id over a WebSocket, as corbie-ui's `page.Stream`.

- A card on any sheet takes its run's status, step and waits from the head; only its dot moves.
- A job card goes `now` when its step asks for it and `done`, `failed` or `denied` on its result.
- The record grows by rows; a turn's `text` emits join one row; an emit named `chart` or `table` is drawn as one, and no other emit changes the GUI.
- A held job makes the decision card rise from the level of its run, blurred and small at that depth, sharp at the front, whichever sheet is in focus.
- A person acts back with Reply, Send, Cancel, Approve and Refuse, each one Corbie command shown on hover, and Say more; an agent ignores events, so no run is interrupted.

## Shipped machines and fixed pipelines

`.desk/` is written whole at every start, each thing with a `README.md`.

| Shipped | What it is |
|---|---|
| `ask.yaml`, `call.yaml`, `clock.yaml`, `live.yaml` | the ask (`ticket: decided`), a tool that waits (`ticket: answer`), a routine's timer, the program's own run |
| `pipelines/request.yaml` | a statechart: `heed` runs the team's rule, `work` the lead, `disclose` the disclosure rule, `answer` ends |
| `tools/talk`, `tools/docs`, `tools/notes` | `ask_team`, `ask_person`, `tell`, `read_thread`; the GitLab tools; `write_note` |
| `bridges/{slack,gitlab,jira,telegram}`; `rules/`; `signin/` | an in and an out machine each; heeds; google and entra |

A team's `team.yaml` names `pipeline: ./request.yaml`, a copy Use makes from the shipped one with `machine: ./bot.yaml` filled in for `work`; a team without one takes no requests. The pipeline is one Corbie run tree, so the stack beneath a job reads request, pipeline run, lead run, tool run, job.

## What Corbie gives for free, and what it would have to gain

Free: `parent` in the head, so the stack is read from heads alone; labels, so `corbie run ls --label desk=house` lists the office; `state` per type in each step, which marks the drawing; `Follow` with emits; tickets, so the program is any ask or tool that waits; `asked`, `used`, `refused`, `layer` and `fix`; `cli.Program`, which gives `corbie desk`, its `manual` and its `mcp`.

Asks, each general to any client:

1. A timer's due time in the head, so the routine card says `next fri 16:00` without computing it.
2. The reach of a definition from the check: every job's hosts, secrets, denials and holds, so the aside parses no `allow:` list itself.
3. A watch that reports a refused start, so a request whose run never started shows its `problems`.
4. A job's result naming the held job it ran after, so a job card says "approved by Lee" from the record alone.
5. A secret's name from input in a scope, `secret:${input.docs.secret}@net:…`, so the docs tools serve any token name.

## Risks and what is lost

- `filter: blur` on a window-sized sheet is re-rasterised on every change beneath, and perspective with filters flickers on some browsers; `prefers-contrast: more` is the fallback.
- One sheet is sharp at a time: reading the thread while watching the run is Escape then Enter, not two columns; what a person needs from beneath must be in the band.
- Blur must not leak a sealed body: a sheet beneath is sent only what the person may read.
- Lost from Bulldog: docs leave for GitLab, disclosure is only the pipeline's `disclose` state, and the hand-drawn request trace is gone. From corbie-ui: nothing.
- A model's share of the GUI is `chart` and `table` emits, by design.
