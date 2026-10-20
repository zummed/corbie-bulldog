# Work

Bulldog drives Corbie, and this area is how. It starts runs, watches them, answers the tickets they wait on, and cancels them. It is the one part of Bulldog that speaks to Corbie: the other areas ask it for a run and are told how the run ended.

## What starts a run

A run is one instance of a worker that Bulldog started, on one request.

- **A request reaching a team** starts the team's lead on it, once the team's heed, where it weighs what was asked, lets it through. A request asked at a place on a bridge reaches the team that place names, the office where it names none, and a sub-request reaches the team asked. A team whose lead is a person starts no run: the request waits on that person.
- **Something said to the team in a request's thread**, a reply, an answer to a question it asked, or a decider's refusal of something it would have said outside the team, starts the lead on the request again, once no run of the request is going and the team's heed, where it weighs what was said, lets it through.
- **A routine coming round** opens a request of the team's own and starts the worker the routine names.

Nothing else starts a worker on a request. A worker used as a tool by a colleague is not started by Bulldog: Corbie runs it inside the run that called it, and what it does is in Corbie's record of that run. The other areas ask work for runs of their own, and work starts and labels them like any other: Decisions for a rule or a worker asked to decide, and Bridges for a bridge's machines.

A request has one run going at a time, its lead's or its heed's, and a worker may have runs going on several requests. What is said in a request while a run goes cannot reach that run, since an agent acts on nothing sent to it, so it is given to the next run, which starts as soon as that one ends.

The office has at most `runs:` runs working at once, from the top team's file. They are the runs of its workers on requests: a heed's run does not count, and a run another area asks for, to decide or for a bridge, is that area's, and does not count either. A run that waits on a person, another team or a decision is not working, and does not count. A request ready to start waits its turn, oldest first. A worker the office reports as having a problem is given no run. Asking a team whose lead cannot run is refused where it is asked, saying why, as the requests page says; a request already open when its lead stops is answered saying the team cannot answer now, and the team's people are told the problem.

A request is never left working with nothing going, nothing waiting to start and nothing it waits on.

A run starts with no memory of the last. It is given the thread, never an earlier run's conversation, because a conversation is what was read.

## A team's heed

A team may name a heed, one rule that weighs what is said at a channel place before its lead starts on it, as the office page says. It is asked only when everything new to the lead is a message said at the request's own place, through its bridge, by a person or a guest: what was asked there, or said there more. Anything else new starts the lead on all of it with no heed: a direct message, something said on the screens or in the chat, a routine's request, a sub-request, a reply to what the lead put to someone, a late answer, a decider's refusal, and the opening of a source thing, such as an issue, which is addressed to whoever watches it. A comment in the issue is weighed. What is said while the lead's run goes is weighed once that run ends, and what is said while the heed weighs is weighed next, unless the heed lets through what it weighed, when the lead starts on it all.

The heed is a run of the team's, noted before its start is sent and taken as a lead's run is, never waited on: work starts it and goes on. It is the rule as it is written, a bare script or a definition, in the team's folder or among those Bulldog ships, started with three inputs and no `context`: `said`, each message it weighs on its own line with when it was said and who said it; `request`, the request as its lead would be given it, with those messages as `new`; and `settings`, every setting the team file writes beside the rule, whole, or nothing for a heed written as a path alone. Bulldog reads none of the settings, and keeps no time limit of its own on a heed: its time is the rule's.

The heed answers `{ says: act | note, note }`:

- **note**: each message it weighed has its word on it, as the requests page says, and its lead does not start on them. Nothing runs and nothing is said. The lead is given them in the thread when it next starts, each marked as noted, and never the heed's word.
- **act**: the lead starts on everything new to it.
- A heed that fails, runs out of time, is cancelled or lost, or answers anything else, counts as act: the lead starts, and the team's people are told what Corbie said and that the lead was started. The asker is told nothing.

A heed's run counts toward neither `runs:` nor the runs in a row, and a heed with a problem is not asked: the lead starts as if there were none. What a heed read is its team's, whichever folder holds its rule.

Bulldog ships two heeds, each in its folder in `.bulldog/rules/` with its page. Each acts if any message it weighs is addressed to the lead: marked by its bridge as naming the bot, or said straight after the lead's answer by whoever that answer went to.

- **mentioned**, `mentioned/mentioned.js`, a bare script with no settings that asks no model, notes everything else, "Not addressed to bot.".
- **for-us**, `for-us/for-us.yaml`, a statechart, asks a System One model of everything else whether the newest message is for the team, given the team, the thread before and what is new, and acts when the model is at least as sure as its threshold, or notes "Not for Platform support (0.12)". It needs the secret `TYPESAFE_KEY`, for `api.typesafe.ai`. Its settings are `timeout`, 10s; `threshold`, 0.5; `model`, `jev-latest`; and `question`, what the model is asked, each default written in the definition where the setting is read. `timeout` is a timer of the chart, around everything it does: past it, the heed ends as act, with the note "timed out".

## What a run is given

Bulldog starts a worker as its definition is written, with one thing filled in: a model named by one word is replaced by the model of that name the office gives, from the worker's own team or else the nearest team around it, as the office page says. A model written out in full is sent as it is, and nothing else is filled in: not the tools file, not the ask. It gives three inputs, and a definition reads the ones it wants.

| Input | What it is |
|---|---|
| `said` | what is new for this run, as text: what was asked, or what was said to the team since its last run |
| `context` | as text a model reads well: the date and time; the team, its people and the workers who sit with it; the team's notes, each whole under its name; the teams it may ask, as the tools below say, each with its `about`, the one line its team file gives for what it looks after; who asked, as a person of the office with the teams they are in, or as a guest with the identity the bridge vouched for; where it was asked, and who will read the answer; the thread so far, less what is new for this run, each message the team's heed noted marked so, and without the heed's words on them, left out when nothing else is in it; then, under "New for you", each entry new for this run on its own line with when it was said and who said it, whole; and last, that its last reply is its answer, and, for a routine or at a channel place, but from a run started again on a refusal, that an empty reply says nothing |
| `request` | the same as data, for a flow or a statechart: the request's number, its asker, its origin, for a sub-request with the requests above it, top first, who reads its answer, its entries with who each is by and to, whether the team's heed noted it and whether its bridge marked it as naming its bot, and apart from them those new for this run, its sub-requests and where each stands, and the team's notes |

Who sits with a worker, which teams it may ask and its team's notes are read from the files, and the requests above from the store, each time a run starts, and kept nowhere else. Every run Bulldog starts of a team's worker, its lead, a routine's worker, or a worker asked to decide for it, is given that team's notes, and no other team's, whether or not its definition names the notes tool; a colleague called as a tool is given what its caller gives it.

A routine's run is given, in `context` and in `request`, its team's open requests, those working or waiting on someone, each with its number, its asker, its state and the first line of what was asked. It acts on none of them: every tool a run calls acts on the run's own request alone.

Every time a run is given is in the office's zone, and says so.

A sub-request's run is given its own thread, and nothing of the request above it, which was said inside another team. It is told who asked it; the requests above it, top first, each by its number and team; and who will read the answer at the end: the origin of the request at the top.

What a run is told about where its answer goes is true for that run.

## How a run ends

A run's answer is its output: an agent's last reply, or a flow's or a statechart's output. Text is the answer as it is. Data with a text `answer` gives that text, and other data is written out as text.

The answer is an entry in the thread, by the worker, to the asker, and the request is answered. It goes to the origin for a request, to the run that asked for a sub-request, and to the team's people for a routine. An empty answer to a routine, or at a channel place, any place on a bridge but its direct place, says nothing: the request is answered, nothing is posted, and no disclosure is asked of it. To anyone else, in the chat, in a direct message or to the run that asked, it is said as the team having given no answer.

An answer leaving a team that set a disclosure check goes to Decisions first, and is said only once it is disclosed. A refusal comes back as an entry by the decider to the team's lead, with the decider's note, and starts the lead again on the same request, so it can say it another way. The asker hears nothing of a refusal while the lead tries again: they are told what the lead next says that is disclosed. When the lead gives up, ending with no answer, or the refused answers use up the runs in a row, the asker is told one line, as the decisions page says: that the team did not disclose its answer, and who refused it, with nothing of the answer or the note.

A run that fails, runs out of time or of turns, or is refused a permission, with nothing else open in its request, leaves the request answered, with Bulldog's entry to the asker, sent to the origin, saying that the team's work failed and it could not answer. The team's people are told what Corbie said, with the line that would fix a refusal. The asker is never told the error, since an error can hold what was read. Nothing is tried again by itself: saying more starts the request again. An error in starting or ending a run is said to the team's people, and never dropped.

When a person stops a request, Bulldog cancels its run in Corbie once, which stops everything below it, closes the questions it asked, and stops the sub-requests it made. A tool of the run being cancelled is given nothing, not even a stopped sub-request's line, so the run cannot go on before its cancel reaches it. The request is answered as stopped, saying by whom. A request from an issue is stopped the same way when the issue is closed at its source, and the entry says who closed it; nothing else that changes at the source starts or stops anything.

An ending is applied once, and only to the run it is for. Each run is kept with its request: the worker, when it started and ended, how it ended, and the tokens Corbie counted, also for a run that failed.

## A team with no people

A team that is all workers has nobody of its own to read what goes wrong. Wherever this page says a team's people, for such a team it means the people its decisions reach, who are its people for every purpose of sight and control, as the requests page says.

## Labels

Every run Bulldog starts carries labels, and is found by them:

| Label | What it holds |
|---|---|
| `bulldog` | the office's identity, which is its own and never its name |
| `kind` | `work` for a worker on a request, `heed` for a team's heed weighing what was said in one, `clock` for a routine's clock, or the kind the area that asked for the run names: `bridge-in`, `bridge-out` and `bridge-sign-in` for a bridge's, `decide` for one asked to decide |
| `team` | the team the run is for |
| `request` | the request's number, on a run that has one |
| `routine` | on a routine's clock, a fingerprint of the routine as its team writes it, its time, worker and words, so a routine changed is a new one, with a clock of its own |
| `bridge` | on a bridge's run, in or out, the bridge's name |
| `thing` | on a bridge's out run, a sign-in link's among them, the number of the thing it carries |
| `decision`, `step` | on a run asked to decide, the decision's number and the step of its walk |

Corbie gives a run's labels to every run below it, so a call from a colleague three levels down is found with the lead's run. Labels are public in Corbie, so they hold kinds, numbers and identities, never words of a request or anyone's name.

A run started to decide belongs to the team it decides for: a rule's run, and a worker's run, a worker of another team's included. That team is its `team`, and what it read is seen only by that team's people. A rule is run as it is written: a bare script, given the decision as input, or a definition that holds its own permissions, in the team's folder, with a model named by one word filled in as a worker's is, or among those Bulldog ships, sent as it ships. Bulldog grants a rule nothing and writes no definition around one; a bare script is wrapped by Corbie's client, in a flow of one stage with no permission.

Bulldog chooses each run's name and notes it, with the team and request it is for, before it sends the start. What a run is for is read from that note; labels are for finding runs.

## The tools Bulldog ships

A worker names in its own definition what of Bulldog's it uses, each by its path in `.bulldog/`, and has nothing of Bulldog's it does not name. A worker that asks or tells anyone outside its own colleagues names the talk tools, `tools/talk/talk.yaml`, in its `tools_file:`. A worker that writes its team's notes names the notes tool, `tools/notes/notes.yaml`, there too; a team that does not want its workers writing its notes names it in none of its workers' definitions. Every tool in a tools file Bulldog ships, the talk tools, the notes tool and the builder's, `tools/build/build.yaml`, is one Bulldog does, and Bulldog does no other; each runs the call Bulldog ships, `.bulldog/call.yaml`, and a tool is Bulldog's only when that is its job.

Corbie gives a held job to the nearest ask at or above the machine that holds it. So a worker Bulldog starts at the top, such as a team's lead or a routine's worker, names the ask Bulldog ships, `ask.yaml`, as its `ask:` when anything it runs can be held, its colleagues called as tools included. A colleague called as a tool uses the nearest ask above it unless it names the ask Bulldog ships itself, which it may, and must when it can also be started at the top; every ask a worker names is Bulldog's, and naming any other is a problem of that worker, as the office page says.

The talk tools:

| Tool | What it does |
|---|---|
| `ask_team(team, question)` | asks another team of the office, and waits for its answer. The question is a sub-request, which starts that team's lead in a run of its own |
| `ask_person(question, who)` | asks a person of the worker's team, or, with nobody named, the asker of the request, and waits for the answer |
| `tell(text, who)` | says something that needs no answer, to the asker or to a person of the worker's team, and goes on |
| `read_thread(from, place)` | reads the thread of the request it works on, oldest first, from an entry on; or, with `place`, what was said before it where it was asked, newest first, from an entry back |

The notes tool:

| Tool | What it does |
|---|---|
| `write_notes(name, text, why)` | writes one of the team's own notes, whole, at once, which Changes commits, saying which worker wrote it and why. No decision is asked: it is the one write to the office's files that is not a decided change, and it is still a commit with who and why |

There is no tool for reading notes: a run Bulldog starts of a worker is given its team's notes in its `context`.

`read_thread` gives a run more of what it is given, and nothing it could not be. Without `place`, it reads the request's thread as a run of it is given it, the noted messages marked and the heed's words left out, each entry on its own line with its number, when, by whom and to whom, and what it is, and says how many more there are and where to read on. With `place`, it reads what was said at the listed place the request was asked at, in the team's other requests asked there before it: their messages, noted or not, and the team's answers, and only of the requests every reader of this one reads, so a bot asked at the top of a channel can read the talk noted before it. A request from another team, a routine's request and one asked in the chat were asked at no place, and `place` is refused for them.

`ask_person` and `tell` reach the people of the worker's own team and the asker, and nobody else; anything for anyone else goes as a request to a team, with `ask_team`. The worker that called is the nearest run above the call that is a worker, so a colleague used as a tool asks and tells in its own name. Each question, sub-request and tell is an entry in the request's thread, by that worker.

A worker never asks its own team with `ask_team`: inside a team, workers use each other as Corbie tools. Nor does it ask a team of a request above the one it works on: what it needs from that team goes back in its answer, and the question is refused before any disclosure is asked; a team asked again through a bridge is not caught. So the teams a worker may ask are every other team but those above, worked out from the files and the request each time, and that one list is what its run is given, what a refusal names and what the screens show. A worker that may ask only some teams writes its own `ask_team` over the call Bulldog ships, as the talk tools' page shows, with the teams by name in an `enum` on `team`; it stands before the talk tools' own, Corbie refuses any other team, and the list holds only those. The asker is asked or told only when the asker is a person or a guest; a worker that asked reads only the answer. A question that cannot go to the one named, or to a person nothing reaches, is refused, and nobody else is asked in their place.

Everything a team says outside itself goes through its disclosure check, where it set one, as the decisions page says: its answers, what a worker asks or tells an asker outside the team, and the sub-requests it makes with `ask_team`, before they are made. A refusal of something said by a tool is the tool's result, with the decider's note, and the worker goes on; the refusal is also an entry by the decider to the team's lead.

`write_notes` writes only a note of the calling worker's own team, one file directly in its `notes/`, named by a plain name with no path in it; any other name is refused, and nothing is written. Notes are files of the office. They are not sealed: the builder reads them, and so does whoever decides a change. `write_notes` says so to the worker that holds it: a secret belongs in Corbie's secrets, and what the team read belongs in its runs, never in a note.

A refused call gives the worker the reason in a sentence.

## A run that waits

Every shipped tool is the same small statechart, which issues a ticket and waits. Bulldog finds its run by the labels it carries from the run Bulldog started, and reads the ticket's value and the call from the run's record. It does what is asked, keeps the answer, and answers the ticket. The answer is the chart's output, and so the tool's result. Nothing in Corbie calls Bulldog.

The builder's tools wait the same way, and Bulldog hands each call to Changes. The ask Bulldog ships, `ask.yaml`, waits the same way too. When Corbie holds an action, it starts the nearest ask at or above the machine that holds it, which issues a ticket with the held jobs and waits; Bulldog hands the jobs to Decisions and answers the ticket with what was decided. A shipped tool or the ask left unanswered for as long as a shipped tool waits ends by its own timer: a tool with words saying the question is still open, and the ask approving nothing, which Decisions takes as the decision withdrawn.

A call is done once. What it does and the answer kept for it are written together, before the ticket is answered, so a call found waiting again after a restart is given the same answer. The kept answer goes when the call's run ends. A call is done only for a run Bulldog noted for this office that has not ended, and one whose input cannot be read yet is looked at again.

An answer goes to the tool while the tool waits. One that comes once the tool has stopped waiting is handed to Requests, which keeps it in the request that asked; it goes to one or the other, never both, and as an entry it starts the lead again. A tool waits at most a week; then the worker is told the question is still open, and goes on.

## Routines

A routine is written in its team file as a time, `at`, which is a time of day with a weekday before it or not; the worker it is said to; and what is said. Each routine runs the clock Bulldog ships, `.bulldog/clock.yaml`, given the routine's time as `due`:

```yaml
statechart: clock
input: [due]
initial: waiting
states:
  waiting: { timers: { due: "${input.due}" }, on: { due: come } }
  come:    { ticket: came, on: { came: waiting } }
```

The time is the routine's `at`, and a time written without a zone is in the office's zone: `mon 09:00 Europe/London`. When its time comes, the clock waits on its ticket. Bulldog opens the routine's request, notes which coming round it is for, then answers the ticket, and the clock waits for its next time, never coming round twice for one time. A time that came while Bulldog was away is acted on once when it is back. One that comes while the routine's last request is still open, working or waiting on someone, or for a worker that cannot run, is let go by.

A routine's request is the team's own: its answer goes to the team's people, and whatever comes of it waits on them and the team's deciders.

Bulldog looks for the clocks by their labels when it reads the office, and when a clock ends. A routine with no clock gets one, and a clock for a routine that is no longer written is cancelled. A changed routine is a new one.

## When Bulldog was away

Corbie needs nothing from Bulldog to keep going. A run goes on, a call waits, and a clock keeps time. Everything waiting to start, and every open question and sub-request, is kept in Bulldog's store.

Bulldog watches every run with its office's label, and begins again a second later if watching cannot begin. When the first listing ends, it settles: each run it has noted as going that is not running is read for how it ended, each waiting call is answered from what was kept or done now, each waiting clock is acted on, and what waits to start is started.

A run is never taken to have ended from a record that could not be read. A run with the office's label that Bulldog has no note of is left over from an office that no longer has it, and is cancelled. A noted run Corbie never took up, or has no record of, is ended as lost.

Bulldog looks for runs to start when a request is opened, something is said, a run ends, a call is answered, or the office is read again. The one timer of this area's own looks again at what it could not read or reach. What people do on the screens never waits on Corbie.

A run goes on as it was started when the files change, and the worker's next run reads them again. A run of a worker or a team that is no longer in the office is stopped.

## Limits

| Limit | Value |
|---|---|
| runs working at once in the office | `runs:`, 3 if not given |
| the thread given to a run | its newest entries within 20,000 characters, the opening entry always kept, and a line saying how many are not shown; what is new for the run is given whole, apart from it |
| the notes a run is given | 50,000 characters in all, and a line naming what is not shown |
| the open requests a routine's run is given | the newest 50, and a line saying how many more there are |
| what `read_thread` gives | its whole entries within 20,000 characters, a first one longer than that cut, and a line saying how many more there are and where to read on |
| levels of sub-requests below a request | 8 |
| how long a heed weighs | its rule's own: Bulldog keeps no limit; for-us's is its `timeout`, 10s unless the team file sets it |
| runs of one request in a row with nothing said in it by a person | 20; then the request is answered as stopped at the limit, or, where the last of them was started by a refused answer, the asker is told that the team did not disclose its answer; a person saying more in it starts it again |
| how long a shipped tool waits | 7 days |
| how far up Bulldog looks for the worker that called | 16 runs |
| when Bulldog looks again at what it could not read or reach | 30 seconds |

Text is never cut inside a character.

## Watching a run

A run can be watched as it happens: the model's words as they arrive, each tool call and its result, each log line, and each refusal with what was tried, which layer refused it and the line that would fix it. That is Corbie's own record of the run, read as it is written, and it is what was read.

## Tests

Every test of this area moves a clock the test holds and steps the work until nothing more is left to do, and never waits a fixed time. `mise run check` runs every test with the race detector on.

## Interfaces

- **Office** gives the teams, their leads, workers, people, notes, routines, zone and `runs:`, each team's `about`, and its heed with its rule and settings, each read from the files when it is needed; looks up the model a one-word name stands for, in the worker's own team and then each team around it, which work fills into the definition it sends; says which workers have a problem, a worker whose model's name is found nowhere, one started at the top that can hold an action and names no ask, and one that names an ask other than Bulldog's among them; and tells work when the office has been read again.
- **Requests** says who a team with no people has as its people. It keeps the requests, their threads, states and sub-requests, and the source thing a request's asking opened; work opens and answers them, adds entries, writes what a team's heed noted, hands it answers that come once nothing waits for them, and reads who asked, with the person or guest that is, the origin, who reads the answer, a team's open requests for a routine's run, and what was said before a request where it was asked, of the requests every reader of it reads, each reader the office does not name counted as itself. It tells work when an issue a request came from is closed at its source.
- **Decisions** is handed every held action, through the ask Bulldog ships, and everything said outside a team that set a disclosure check, and work answers Corbie, gives a tool its refusal or starts the lead again with what it decides; it asks work to start the runs its rules and deciding workers need, each labelled with the team it decides for, and owns the one line an asker is told when a team does not disclose its answer; work calls it each time its clock moves, so a rule or a deciding worker past its time limit passes.
- **Bridges** carries what work says to an asker or a person, and hands work what people answer; its settings say, for each place it listens in, which team is asked there; it asks work to start its in and out machines, which carry the same labels.
- **Changes** commits what `write_notes` writes, the one write to the office's files that is not a decided change, as one commit with who and why, and is handed each call to the builder's tools, whose answer work keeps and gives.
- **Server** keeps the office's identity, and asks work, by it, whether another server is serving the office (`Serving`), which reads nothing but Corbie, before it opens or reads anything of the office; has work start and renew its own run (`Live`, `Renew`), which work otherwise leaves alone; is told each reading every area has taken (`Took`), and reads it (`Read`), before it gives any person a link or sends one; has the bridges asked at once for a sign-in link to be carried (`Carry`); has work stop a request when a person who may stop it asks, and shows a run as it is watched to those who may see what was read; for a team with no people, those are the people the requests page gives it.
- **Corbie** runs everything.
