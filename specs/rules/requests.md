# Requests

The rules behind the [Requests overview](../requests.md). Area code: `requests`. A test runs the house or the work office (`examples/house`, `examples/work`) on a whole Corbie in its own process, with a stand-in model and a clock it moves.

## Where a request comes from

**R-requests-49** A request is one run of its team's request machine, labelled `kind=request`, and its `heard` is `channel`, `direct`, `chat`, `team` or `routine`, as it came.
*Accept:* in the work office, a guest's message in #help starts a run of `office` with `heard: channel`; Lee's direct message to the bot, `direct`; Dana's question on the screens, `chat`; the bot's `ask_team` to payments, a run of `payments` with `heard: team`. The house's Monday routine starts a run of `office` with `heard: routine`.

## The request machine

**R-requests-50** Bulldog fills out each team file into one Corbie flow named after the team, written as if it were at the office's top directory so every path starts `./`, with `keep: 30d` and its decisions written in place as statecharts, and checks it with Corbie's parser.
*Accept:* payments' request machine is the one in the overview, and Corbie's parser takes it with no problem. The garden's has one stage, `work`, whose job is `./garden/gardener.yaml`, and no `when:`.

**R-requests-51** The `heed` stage runs when the team has a `heed` and `heard` is `channel`, and `work` runs when the heed said yes or there is no heed stage.
*Accept:* in #refunds, Dana's message that names the bot starts the ledger, and one that does not ends `noted` with no work. The bot's `ask_team` to payments starts the ledger with no heed.

**R-requests-52** The `check` stage runs on the answer when the team has a `check`, the answer is not empty, and the request is neither the team's own routine nor, other than at a channel, asked by one of the team's people.
*Accept:* the ledger's answer to Dana in #refunds is checked, and so is its answer to Lee there: one holding 12345678 is withheld. Its answer to Lee in the chat is not checked, and neither is an empty answer nor the answer to a routine of payments.

**R-requests-53** The request machine's output is `{ answer, outcome }`. `outcome` is `answered`, `noted`, `withheld` or `failed`, and a withheld or failed request's answer is the overview's fixed line.
*Accept:* payments' request machine gives `answered` to Dana; `noted`, with no answer, to a message in #refunds that does not name the bot; `withheld`, with "Payments did not let its answer out.", to an answer holding 12345678. A ledger whose model fails, a heed rule that throws and a check rule that throws each give `failed`, with "Payments could not answer: its work failed." An empty answer gives `answered` with "".

**R-requests-54** The lead's answer is an agent's `reply`, its text or the text `answer` of the data it declares; `wait.yaml`'s `answer` for a person; or a flow's or a statechart's output, text or its text `answer`.
*Accept:* the ledger's reply is payments' answer. With `lead: sam` in `garden/team.yaml`, Sam's answer to the wait is the garden's answer. A flow lead whose output is `{ answer: "Done.", rows: 3 }` answers "Done.".

**C-requests-32** What the request machine gives is the only thing of a request that leaves its team: the reply, a ticket's answer and the thread a later request is given are its output, never a stage's. A stage's output is what the team read, before the check. *Test:* `requests_only_output_leaves`

**C-requests-33** The asker never sees an error: a failed request's answer is the fixed line, and its error is shown only to the team's people. An error can hold what the team read. *Test:* `requests_error_never_sent_to_asker`

## What the lead is given

**R-requests-55** The lead is given `said`, `context`, `request` and `files`. `request` holds `team`, `by`, `heard`, `place`, `thread`, `teams`, `open`, `chain`, `docs` and `mentioned`, and `context` says the same as text, with the date and time in the office's zone and the team's people and workers.
*Accept:* Dana's request in #refunds gives the ledger `by: { person: dana }`, `heard: channel`, `place` with `bridge: slack`, `place: "#refunds"`, `guests: false`, its `where` and the id of the thing heard, `teams` naming `office`, `platform` and `support`, each with its `about`, `chain: []` and `docs: <office id>/payments/docs`. Its `context` names Dana, #refunds, and the time in Europe/London.

**R-requests-56** `thread` is the requests before this one in its thread, each with what it said and answered, the newest within 20,000 characters, and `context` says how many are left out.
*Accept:* in a Telegram chat of 40 requests of 1,000 characters each, the 41st is given the newest that fit within 20,000 characters, and its `context` says how many are left out.

**R-requests-57** A routine's request is given `open`: the team's running requests, each with who asked, what was said, and when.
*Accept:* the house's Monday routine, while Kit's request about the socket still waits on Sam, gives the steward that request in `open`.

**C-requests-34** A sub-request is given its own thread only, the requests the same asking request made of the same team, and of the requests above it only the asking team, the chain and where the chain began. What was said in the asking team was said inside it, and one team's answer must not reach another team that no check let it go to. *Test:* `requests_sub_request_given_own_thread_only`

## Decisions

**R-requests-58** A decision asks its deciders in order, each giving `says`, `yes`, `no` or `pass`, with a `note`. The first `yes` or `no` is the answer, and when every decider passes, the answer is `no`.
*Accept:* the house's `decide` on the fixer's £35 order: `small-orders.js` passes, the clerk passes, and Sam's `yes` approves it. A heed of one rule that passes ends in `passed` with `says: no`, and the request is `noted`.

**R-requests-59** A decider that fails fails its decision: a failed heed or check fails the request, and a failed `decide` approves nothing.
*Accept:* a check rule that throws ends Dana's request `failed`. A `decide` rule that throws denies the fixer's `order_part`, and the fixer is told it was denied.

**R-requests-60** Each decider is given the notes of those before it. The people's wait is given what they are asked as `question`, the notes as `notes`, and for `decide` the held jobs as `jobs`, each `{ name, does, args }`. Its `said` holds the same as text, above `yes` and `no`.
*Accept:* the clerk is given `small-orders.js`'s note. Sam's wait is given `question: "May these run?"`, both notes, and the fixer's job as `{ name: order_part, does: "Order a part for a repair, at its price in pounds", args: { pounds: 35 } }`. Sam's question on Telegram holds the job and both notes above its buttons.

**R-requests-61** The people at the end of a list are asked together, and the first of them to answer gives the answer.
*Accept:* Lee and Priya are both asked about the ledger's refund. Lee's `yes` is the answer, and Priya's press afterwards changes nothing.

**R-requests-62** A decider is given `request` and `notes`, with `said`, an `answer`, a `question` or `jobs` as its decision has them; a rule gives `{ says, note }`, a worker its `reply`, and people the `yes` or `no` of whoever answered first.
*Accept:* payments' heed rule is given Dana's message as `said`. Payments' check rule is given the ledger's answer to Dana, and `{ team: platform, said }` for the ledger's question to platform. The house's `small-orders.js` is given the fixer's `order_part` job, with `asked.args` holding `pounds: 35`, and the clerk is given `small-orders.js`'s note as `{ kind: rule, name: ./small-orders.js, says: pass, note }`.

**C-requests-35** A worker passes on a job its own machine holds. An action's author cannot be the check on it. *Test:* `requests_worker_never_decides_own_job`

**R-requests-63** The heed rules Bulldog ships answer `yes` or `no`, never `pass`. `mentioned` says yes when the message names the bot or follows the lead's answer to the same sender in the thread, and `for-us` says what a fast model answers.
*Accept:* `mentioned` says yes to "@bot refund 4411", yes to Dana's next message after the ledger answered her in that thread, and no to "lunch?". `for-us`, with its model stood in, says yes or no as the stand-in answers.

## Held jobs

**R-requests-64** The request machine's `ask:` is a flow written in place. A held job that asks a team goes to the check decision, on its question, and is approved at once without a check; every other held job goes to the `decide` decision, or to the team's people without one; the flow approves each part that said yes.
*Accept:* the ledger's `ask_team` to platform runs payments' check on the question, and on yes the wait starts. The ledger's `refund` waits for Lee or Priya. A held job of the garden is put to Sam, and the bot's `ask_team` to payments starts at once.

**C-requests-36** A question asked with `ask_team` passes the asking team's check, as an answer to another team does. Bulldog acts on every run of a machine named `wait`, so a definition of the team's own that runs `wait.yaml` without approval asks without the check. A question the check refuses gives the tool "Payments did not let its question out." and reaches no other team. A question can carry what the team read, such as a card number. *Test:* `requests_question_to_a_team_passes_check`

## Waiting on a person or a team

**R-requests-65** `wait.yaml`, given `{ to, said, choices }`, waits on its ticket `answer` and ends with the answer, or after 7 days with `{ answer: null, note: "No answer came within 7 days." }`.
*Accept:* a wait answered with `{ answer: "yes", by: "lee" }` ends with that. A wait nobody answers ends, when the test clock has moved 7 days, with the note.

**R-requests-66** `ask_team(team, question)` runs `wait.yaml` to the team, marked for approval, and `ask_person(who, question, choices)` runs it to the person by their id in `everyone`, with up to 8 choices of up to 75 characters each; a question offering more is refused.
*Accept:* the ledger's `ask_team` makes a wait with `to: { team: platform }`, held for the ask. The steward's `ask_person` to Sam with the choices `today` and `tomorrow` makes a wait with those choices. One with 9 choices, or with a choice of 76 characters, is refused, and nobody is sent it.

**R-requests-67** For a wait to a team, Bulldog starts a request of that team, named from the wait run, and answers the ticket with `{ answer }` when the sub-request ends, the answer null when it gave none; when the wait ends first, Bulldog cancels the sub-request.
*Accept:* the bot's question to payments starts one request of payments; when it ends `answered`, the bot's tool gets payments' answer. Dana's request in #help stopped while payments works cancels payments' request.

**C-requests-37** The asking team is the wait run's `team=` label and the chain the request's `chain`, both written by Bulldog, never anything a worker wrote. A model fills the tool's arguments. *Test:* `requests_asking_team_from_label`

**C-requests-38** A question to the asking team itself, to a team in the chain, to a team that takes no requests, or to no team and no people is refused with `{ answer: null, refused }`, naming the teams it may ask, and starts nothing. A sub-request is a new root, which Corbie's limit on levels does not count, so this is the only bound on a loop between teams. *Test:* `requests_ask_in_a_loop_refused`

**R-requests-68** For a wait to people, Bulldog carries the question to each person in their direct messages, on each bridge whose out takes `direct`, with the choices as buttons, and shows it on the screens.
*Accept:* Sam's question about the order goes to Telegram chat `4471`, with the buttons `yes` and `no`, and is a card on Sam's Requests screen. A question to Al in the work office goes to Slack, never to GitLab.

**C-requests-39** A person may be asked only when they are of the asking team or named in its team file; otherwise the question is refused, and nobody is asked. Until it is refused, it is shown to no one and no answer to it is taken. Whom to ask is a model's choice, and a question can carry what the team read. *Test:* `requests_ask_only_the_teams_own_people`

**C-requests-40** A ticket is answered only when the answerer is a person the wait waits on, by their identity on the bridge or signed in on the screens; the ticket still waits; and, for a decision, the answer is one of the choices offered, whatever its case and the spaces around it. A guest never answers. Corbie takes an answer from anyone who holds the ticket, and does not check who gave it. *Test:* `requests_only_the_asked_answer`

## Threads

**R-requests-69** Requests asked at the same `where` on a bridge, in one chat on the screens, or by one request of the same team form a thread, and they start at once and run side by side.
*Accept:* Dana's two messages in one Slack thread make two requests of one thread, both running at once; the second is given the first, with its answer if it has one. The bot's two questions to payments from one request are one thread, and its question to platform another.

## Routines

**R-requests-70** Bulldog keeps one clock run for each routine. When the clock waits on `came`, Bulldog answers `came`, and starts a request of the team named from the clock run and the step that took `came`; a time that came while Bulldog was stopped is acted on once.
*Accept:* with the test clock moved from Monday 08:59 to 09:00, one request of `office` starts with `said: "What is still open?"`. With Bulldog stopped over two Mondays and started again, one more starts.

**R-requests-71** When a routine changes or is removed, Bulldog cancels its clock, and starts a new one for a changed routine. A second clock of one routine is cancelled.
*Accept:* changing the house's routine to `"tue 09:00"` cancels the Monday clock and starts one for Tuesday. Removing it cancels the clock, and starts none. Two clocks were started in all.

## Carrying the answer back

**R-requests-72** When a request ends, Bulldog carries its answer to its source: by an out run to its `where` for a place on a bridge, to the screens for the chat, to the asking wait's ticket for another team, and to the team's people in their direct messages for a routine. An empty answer, a noted request and a stopped one say nothing, and an answer is carried only within a day of its request's end.
*Accept:* the guest's question in #help is answered in its thread, replying to the guest's message. Kit's request is answered in Kit's Telegram chat. The Monday routine's answer goes to Sam and Kit. A noted message in #refunds is answered with nothing, and so is a question in the chat. Bulldog started again a week after Dana was answered in #refunds carries nothing there.

## Stopping a request

**R-requests-73** The person who asked a request, and its team's people, may stop it: Bulldog cancels its run, and a sub-request it asked is cancelled when its wait ends.
*Accept:* Dana stops her request in the chat while the bot waits on payments. The office's run is cancelled, and so is payments' request.

## Labels and names

**R-requests-74** Every run Bulldog starts carries `bulldog=<office id>` and `kind`; a request, `team` and `thread`; a sub-request, `wait`; a clock and a docs run `team`; a bridge's in run and a clock `made`; a bridge's in and out runs `bridge`; an out run carrying a question, `wait` and `to`; and an out run carrying an answer, `request`.
*Accept:* listing the runs of a running work office by `bulldog=<office id>` gives every run Bulldog started, and `kind=request,team=payments` gives payments' requests alone.

**C-requests-41** Labels and run names hold no words of a request and no plain id of a person, and what comes from a service is hashed with the office's secret. Corbie shows labels and run names to anyone who may read heads. *Test:* `requests_labels_hold_no_words`

**C-requests-42** Each run Bulldog starts in answer to something is named from that thing, and started only when no run has the name. After a restart Bulldog reads again what it read before, and a run named afresh would start twice. *Test:* `requests_started_once`

**R-requests-75** After a restart Bulldog finds everything again in the store, and doing a thing twice changes nothing.
*Accept:* Bulldog is stopped while payments' check waits on Lee, and started again. Lee's press ends the request once, and Dana is answered once. Started again a week after the bot's question to payments was answered, when the wait that asked it is past keeping, Bulldog reports nothing.

## Interfaces

Uses:

- **Office.** The reading of the office, the loader, the start list of a team's runs, and the office's id and secret.
- **Corbie.** Its parser for each request machine; its client, to start, list, show, follow and cancel runs, to read their records, and to answer tickets.
- **Bridges.** Through an interface the program fills: carrying a message to a place or to a person directly, by an out run under the name and labels Requests gives, and saying which bridges reach people directly.

Used by:

- **Bridges.** Hands over each thing heard: a new request, or an answer to a wait, found by its `ref`, by the message it replies to, or as the latest question carried to the sender; and asks whether a team takes requests.
- **Screens.** The newest requests a person may see, up to a limit; each request in full, with its decisions, each with its held jobs as `{ name, does, args }` whether or not it came to people, and the calls each colleague made; the waits on a person, each with its `question`, `notes` and `jobs`; asking, answering, stopping, and the files said with a request.
